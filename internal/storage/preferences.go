package storage

import (
	"context"
	"encoding/json"
	"path/filepath"

	"copiggy/internal/settings"
)

func ReadPreferences(path string) (settings.Preferences, error) {
	data, exists, err := readBounded(path, 64<<10)
	if err != nil {
		return settings.Preferences{}, err
	}
	if !exists {
		return settings.Default(), nil
	}
	var prefs settings.Preferences
	if err = json.Unmarshal(data, &prefs); err != nil {
		return prefs, err
	}
	return settings.Normalize(prefs), nil
}
func WritePreferences(path string, prefs settings.Preferences) error {
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, append(data, '\n'))
}

type PreferencesRepository struct {
	Path   string
	Writer *Writer
}

func (r *PreferencesRepository) path() (string, error) {
	if r.Path != "" {
		return r.Path, nil
	}
	dir, err := DefaultDirectory()
	return filepath.Join(dir, "preferences.json"), err
}
func (r *PreferencesRepository) Load(ctx context.Context) (settings.Preferences, error) {
	if err := ctx.Err(); err != nil {
		return settings.Preferences{}, err
	}
	path, err := r.path()
	if err != nil {
		return settings.Preferences{}, err
	}
	return ReadPreferences(path)
}
func (r *PreferencesRepository) Save(ctx context.Context, prefs settings.Preferences) error {
	snapshot := prefs.Clone()
	job := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		path, err := r.path()
		if err != nil {
			return err
		}
		return WritePreferences(path, snapshot)
	}
	if r.Writer == nil {
		return job()
	}
	return <-r.Writer.Submit(job)
}
