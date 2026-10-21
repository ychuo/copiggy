// Package storage implements local, versioned repositories. It does not own
// history mutation rules, current selection, or native clipboard authority.
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"copiggy/internal/clipmodel"
)

const MaxHistoryFileBytes = 160 << 20
const MaxPinnedFileBytes = 96 << 20

// Kept separate from the model so the existing on-disk version remains stable.
type savedHistory struct {
	Version int                `json:"version"`
	Records []clipmodel.Record `json:"records"`
}

type HistoryRepository struct {
	Path, LegacyPath string
	Writer           *Writer
	// Write is an optional fault-injection boundary for repository tests.
	Write func(string, []byte) error
}

func NewHistoryRepository(path, legacy string) *HistoryRepository {
	return &HistoryRepository{Path: path, LegacyPath: legacy, Writer: NewWriter()}
}

func DefaultDirectory() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Copiggy"), nil
}
func (r *HistoryRepository) path() (string, error) {
	if r.Path != "" {
		return r.Path, nil
	}
	dir, err := DefaultDirectory()
	return filepath.Join(dir, "history.json"), err
}

func (r *HistoryRepository) Load(ctx context.Context) ([]clipmodel.Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	path, err := r.path()
	if err != nil {
		return nil, false, err
	}
	data, exists, err := readBounded(path, MaxHistoryFileBytes)
	if err != nil {
		return nil, exists, err
	}
	if exists {
		records, err := DecodeHistory(data, false)
		return records, true, err
	}
	legacy := r.LegacyPath
	if legacy == "" {
		legacy = filepath.Join(filepath.Dir(path), "pinned.json")
	}
	data, exists, err = readBounded(legacy, MaxPinnedFileBytes)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	records, err := DecodeHistory(data, true)
	return records, false, err
}

func (r *HistoryRepository) Save(ctx context.Context, records []clipmodel.Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshot := clipmodel.CloneRecords(records)
	job := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		encoded, err := EncodeHistory(snapshot)
		if err != nil {
			return err
		}
		path, err := r.path()
		if err != nil {
			return err
		}
		write := r.Write
		if write == nil {
			write = AtomicWrite
		}
		return write(path, encoded)
	}
	if r.Writer == nil {
		return job()
	}
	// Once accepted, wait for the real outcome, even if ctx is canceled. Returning
	// early could report a rollback while the accepted write commits afterward.
	return <-r.Writer.Submit(job)
}

func (r *HistoryRepository) Close() {
	if r.Writer != nil {
		r.Writer.CloseAndWait()
	}
}

func EncodeHistory(records []clipmodel.Record) ([]byte, error) {
	if _, err := clipmodel.RestoreRecords(records); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(savedHistory{Version: 1, Records: records})
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxHistoryFileBytes {
		return nil, errors.New("saved history exceeds file limit")
	}
	return encoded, nil
}

func DecodeHistory(data []byte, pinsOnly bool) ([]clipmodel.Record, error) {
	limit := MaxHistoryFileBytes
	if pinsOnly {
		limit = MaxPinnedFileBytes
	}
	if len(data) > limit {
		return nil, errors.New("saved history exceeds file limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var saved savedHistory
	if err := decoder.Decode(&saved); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("invalid trailing history data")
	}
	if saved.Version != 1 {
		return nil, errors.New("unsupported saved history")
	}
	if pinsOnly {
		for _, r := range saved.Records {
			if !r.Item.Pinned {
				return nil, clipmodel.ErrInvalidPin
			}
		}
	}
	model, err := clipmodel.RestoreRecords(saved.Records)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]clipmodel.Record, len(saved.Records))
	for _, r := range saved.Records {
		byID[r.Item.ID] = r
	}
	out := make([]clipmodel.Record, 0, len(saved.Records))
	for _, item := range model.Items() {
		r := byID[item.ID]
		r.Item = item
		out = append(out, r)
	}
	return out, nil
}

func readBounded(path string, limit int64) ([]byte, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, true, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, true, errors.New("saved history exceeds file limit")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(data)) > limit {
		return nil, true, errors.New("saved history exceeds file limit")
	}
	return data, true, err
}
