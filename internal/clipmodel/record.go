package clipmodel

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"time"
)

const (
	TextFormat              = "text/plain"
	HTMLFormat              = "text/html"
	PNGFormat               = "image/png"
	MaxClipboardBytes int64 = 16 << 20
	MaxHistoryBytes   int64 = 64 << 20
	MaxOrdinaryItems        = 100
)

// Source is historical provenance, never authority over the live clipboard.
type Source struct{ Name, BundleID string }

// Representation and its item boundary preserve the exact captured bytes.
// Values crossing a service boundary are deep copies; callers may own their copy.
type Representation struct {
	Format string `json:"format"`
	Bytes  []byte `json:"bytes"`
}
type Representations [][]Representation

type Record struct {
	Item Item            `json:"item"`
	Data Representations `json:"data"`
}

func TextData(text string) Representations {
	return Representations{{{Format: TextFormat, Bytes: []byte(text)}}}
}

func (data Representations) Clone() Representations {
	if data == nil {
		return nil
	}
	out := make(Representations, len(data))
	for i, item := range data {
		out[i] = make([]Representation, len(item))
		for j, rep := range item {
			out[i][j] = Representation{rep.Format, bytes.Clone(rep.Bytes)}
		}
	}
	return out
}

func (data Representations) Read(format string) ([]byte, error) {
	var parts [][]byte
	for _, item := range data {
		for _, rep := range item {
			if rep.Format == format {
				parts = append(parts, rep.Bytes)
				break
			}
		}
		if len(parts) > 0 && format != TextFormat && format != "text/uri-list" && format != "application/x-mygo-file-list" {
			break
		}
	}
	if len(parts) == 0 {
		return nil, errors.New("clipboard representation unavailable")
	}
	separator := []byte("\n")
	if format == "text/uri-list" || format == "application/x-mygo-file-list" {
		separator = []byte("\r\n")
	}
	return bytes.Join(parts, separator), nil
}

func (data Representations) ByteSize() int64 {
	var total int64
	for _, item := range data {
		for _, rep := range item {
			total += int64(len(rep.Bytes))
		}
	}
	return total
}

func (data Representations) Validate() error {
	if len(data) == 0 || len(data) > 64 {
		return errors.New("invalid clipboard item count")
	}
	for _, item := range data {
		if len(item) == 0 || len(item) > 64 {
			return errors.New("invalid clipboard format count")
		}
		seen := make(map[string]bool, len(item))
		for _, rep := range item {
			if rep.Format == "" || len(rep.Format) > 256 || strings.ContainsAny(rep.Format, "\x00\r\n") || seen[rep.Format] {
				return errors.New("invalid clipboard format")
			}
			seen[rep.Format] = true
		}
	}
	if data.ByteSize() > MaxClipboardBytes {
		return ContentError("Content exceeds 16 MiB · Not recorded")
	}
	return nil
}

// Fingerprint preserves the historical digest scheme. Item order matters;
// representation enumeration order does not.
func (data Representations) Fingerprint() ([sha256.Size]byte, error) {
	var zero [sha256.Size]byte
	if err := data.Validate(); err != nil {
		return zero, err
	}
	if len(data) == 1 && len(data[0]) == 1 && data[0][0].Format == TextFormat {
		return zero, nil
	}
	h := sha256.New()
	h.Write([]byte("copiggy/representations/v1\x00"))
	size := func(n int) { var b [8]byte; binary.BigEndian.PutUint64(b[:], uint64(n)); h.Write(b[:]) }
	size(len(data))
	for _, item := range data {
		reps := slices.Clone(item)
		slices.SortFunc(reps, func(a, b Representation) int { return strings.Compare(a.Format, b.Format) })
		size(len(reps))
		for _, rep := range reps {
			size(len(rep.Format))
			h.Write([]byte(rep.Format))
			size(len(rep.Bytes))
			h.Write(rep.Bytes)
		}
	}
	return [sha256.Size]byte(h.Sum(nil)), nil
}

func (record Record) Clone() Record { return Record{cloneItem(record.Item), record.Data.Clone()} }
func CloneRecords(records []Record) []Record {
	out := make([]Record, len(records))
	for i, r := range records {
		out[i] = r.Clone()
	}
	return out
}

// RestoreRecords validates the entire snapshot before exposing any record.
// It restores neither selection nor live clipboard/guard authority.
func RestoreRecords(records []Record) (*Store, error) {
	if len(records) > MaxOrdinaryItems+MaxPinnedItems {
		return nil, errors.New("saved history exceeds item limit")
	}
	model := NewStore(MaxOrdinaryItems)
	var total int64
	for _, record := range records {
		item := record.Item
		if len(item.ID) > 64 || len(item.SourceApp) > 4096 || len(item.SourceAppID) > 1024 || len(item.ParentID) > 64 || len(item.AllowedApps) > 100 {
			return nil, ErrInvalidPin
		}
		for _, app := range item.AllowedApps {
			if len(app) > 1024 {
				return nil, ErrInvalidPin
			}
		}
		content, err := ContentFromRepresentations(record.Data)
		if err != nil {
			return nil, err
		}
		total += record.Data.ByteSize()
		if total > MaxHistoryBytes {
			return nil, ErrPinLimit
		}
		content.SourceAppID = item.SourceAppID
		if err := model.RestoreItem(item, content); err != nil {
			return nil, err
		}
	}
	return model, nil
}

// Retain applies one common count, byte and age policy. Pins are age/count
// exempt from ordinary history, but still consume the shared byte budget.
func Retain(records []Record, days int, now time.Time) ([]Record, error) {
	var result []Record
	var total int64
	ordinary, pins := 0, 0
	cutoff := now.AddDate(0, 0, -days)
	for _, record := range records {
		if record.Item.Pinned {
			pins++
			if pins > MaxPinnedItems {
				return nil, ErrPinLimit
			}
		} else {
			if days > 0 && !record.Item.CreatedAt.After(cutoff) {
				continue
			}
			ordinary++
			if ordinary > MaxOrdinaryItems {
				continue
			}
		}
		result = append(result, record)
		total += record.Data.ByteSize()
	}
	for i := len(result) - 1; i >= 0 && total > MaxHistoryBytes; i-- {
		if result[i].Item.Pinned {
			continue
		}
		total -= result[i].Data.ByteSize()
		result = append(result[:i], result[i+1:]...)
	}
	if total > MaxHistoryBytes {
		return nil, ErrPinLimit
	}
	return result, nil
}

// Clone creates a private transaction candidate, including trusted drafts and
// current selection. No state is shared with the original Store.
func (s *Store) Clone() *Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := NewStore(s.maxItems)
	out.nextID = s.nextID
	out.currentID = s.currentID
	for _, item := range s.items {
		out.items = append(out.items, cloneItem(item))
	}
	for id, draft := range s.drafts {
		out.drafts[id] = cloneDraft(draft)
	}
	return out
}
