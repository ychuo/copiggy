package clipboard

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"copiggy/internal/clipmodel"
	"copiggy/internal/settings"
)

var ErrHistorySave = errors.New("could not save restored history")

// History is the sole owner of the model and its exact original bytes. Commands
// serialize across persistence acknowledgements, while queries read the last
// committed immutable snapshot without waiting for disk I/O.
type History struct {
	commands               sync.Mutex
	mu                     sync.RWMutex
	model                  *clipmodel.Store
	data                   map[string]clipmodel.Representations
	repo                   HistoryRepository
	retention              int
	loaded, blocked, dirty bool
}

func NewHistory(repo HistoryRepository, retentionDays int) *History {
	if !settings.ValidRetentionDays(retentionDays) {
		retentionDays = settings.DefaultRetentionDays
	}
	return &History{model: clipmodel.NewStore(clipmodel.MaxOrdinaryItems), data: make(map[string]clipmodel.Representations), repo: repo, retention: retentionDays, loaded: repo == nil}
}

func (h *History) Items() []clipmodel.Item {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.Items()
}
func (h *History) Search(query string) []clipmodel.Item {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.Search(query)
}
func (h *History) Pinned() []clipmodel.Item {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.Pinned()
}
func (h *History) Current() (clipmodel.Item, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.Current()
}
func (h *History) Record(id string) (clipmodel.Record, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, item := range h.model.Items() {
		if item.ID == id {
			data, ok := h.data[id]
			return clipmodel.Record{Item: item, Data: data.Clone()}, ok
		}
	}
	return clipmodel.Record{}, false
}
func (h *History) Records() []clipmodel.Record {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return clipmodel.CloneRecords(recordsOf(h.model, h.data))
}

func recordsOf(model *clipmodel.Store, data map[string]clipmodel.Representations) []clipmodel.Record {
	items := model.Items()
	records := make([]clipmodel.Record, 0, len(items))
	for _, item := range items {
		records = append(records, clipmodel.Record{Item: item, Data: data[item.ID]})
	}
	return records
}
func (h *History) candidate() (*clipmodel.Store, map[string]clipmodel.Representations) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data := make(map[string]clipmodel.Representations, len(h.data))
	for id, rep := range h.data {
		data[id] = rep
	}
	return h.model.Clone(), data
}
func (h *History) commit(model *clipmodel.Store, data map[string]clipmodel.Representations, dirty bool) {
	h.mu.Lock()
	h.model, h.data, h.dirty = model, data, dirty
	h.mu.Unlock()
}
func (h *History) save(ctx context.Context, model *clipmodel.Store, data map[string]clipmodel.Representations) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if h.repo == nil {
		return nil
	}
	if !h.loaded {
		return errors.New("history persistence is not initialized")
	}
	if h.blocked {
		return errors.New("existing history could not be loaded")
	}
	return h.repo.Save(ctx, clipmodel.CloneRecords(recordsOf(model, data)))
}
func trim(model *clipmodel.Store, data map[string]clipmodel.Representations, days int, now time.Time) error {
	retained, err := clipmodel.Retain(recordsOf(model, data), days, now)
	if err != nil {
		return err
	}
	live := make(map[string]bool, len(retained))
	for _, record := range retained {
		live[record.Item.ID] = true
	}
	for _, item := range model.Items() {
		if !live[item.ID] {
			_ = model.Remove(item.ID)
		}
	}
	for id := range data {
		if !live[id] {
			delete(data, id)
		}
	}
	return nil
}

func (h *History) Load(ctx context.Context) error {
	h.commands.Lock()
	defer h.commands.Unlock()
	if h.repo == nil {
		return nil
	}
	if len(h.Items()) != 0 {
		return errors.New("history must load before capture")
	}
	records, authoritative, err := h.repo.Load(ctx)
	h.loaded = true
	h.blocked = err != nil
	if err != nil {
		return err
	}
	model, err := clipmodel.RestoreRecords(records)
	if err != nil {
		h.blocked = true
		return err
	}
	data := make(map[string]clipmodel.Representations, len(records))
	for _, r := range records {
		data[r.Item.ID] = r.Data.Clone()
	}
	original := model.Clone()
	originalData := make(map[string]clipmodel.Representations, len(data))
	for id, r := range data {
		originalData[id] = r
	}
	if err = trim(model, data, h.retention, time.Now()); err != nil {
		return err
	}
	if !authoritative || len(model.Items()) != len(original.Items()) {
		if err = h.save(ctx, model, data); err != nil {
			h.commit(original, originalData, true)
			return fmt.Errorf("%w: %w", ErrHistorySave, err)
		}
	}
	h.commit(model, data, false)
	return nil
}

func (h *History) Persist(ctx context.Context) error {
	h.commands.Lock()
	defer h.commands.Unlock()
	if !h.dirty {
		return nil
	}
	model, data := h.candidate()
	if err := h.save(ctx, model, data); err != nil {
		return err
	}
	h.commit(model, data, false)
	return nil
}

// Capture commits an observed record even when disk saving fails, preserving a
// readable in-memory history and retrying on Persist. Destructive commands below
// instead publish only after a successful save.
func (h *History) Capture(ctx context.Context, data clipmodel.Representations, source clipmodel.Source, allowed []string) (clipmodel.Item, bool, error) {
	if err := data.Validate(); err != nil {
		return clipmodel.Item{}, false, err
	}
	data = data.Clone()
	content, err := clipmodel.ContentFromRepresentations(data)
	if err != nil {
		return clipmodel.Item{}, false, err
	}
	content.SourceAppID = source.BundleID
	h.commands.Lock()
	defer h.commands.Unlock()
	if err = ctx.Err(); err != nil {
		return clipmodel.Item{}, false, err
	}
	model, all := h.candidate()
	var item clipmodel.Item
	var recorded bool
	if len(allowed) > 0 {
		item, recorded = model.CaptureProtectedContent(content, source.Name, allowed)
	} else {
		item, recorded = model.CaptureContent(content, source.Name, false)
	}
	if !recorded {
		return item, false, nil
	}
	all[item.ID] = data
	if err = trim(model, all, h.retention, time.Now()); err != nil {
		return clipmodel.Item{}, false, err
	}
	h.commit(model, all, true)
	if err = h.save(ctx, model, all); err != nil {
		return item, true, fmt.Errorf("%w: %w", ErrHistorySave, err)
	}
	h.commit(model, all, false)
	return item, true, nil
}

func (h *History) SetPinned(ctx context.Context, id string, pinned bool) error {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	for _, item := range model.Items() {
		if item.ID == id && item.Pinned == pinned {
			return nil
		}
	}
	if err := model.SetPinned(id, pinned); err != nil {
		return err
	}
	if err := trim(model, data, h.retention, time.Now()); err != nil {
		return err
	}
	if err := h.save(ctx, model, data); err != nil {
		return err
	}
	h.commit(model, data, false)
	return nil
}
func (h *History) Delete(ctx context.Context, id string) error {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	if err := model.Remove(id); err != nil {
		return err
	}
	delete(data, id)
	if err := h.save(ctx, model, data); err != nil {
		return err
	}
	h.commit(model, data, false)
	return nil
}
func (h *History) Expire(ctx context.Context, now time.Time) (bool, error) {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	before := len(model.Items())
	if err := trim(model, data, h.retention, now); err != nil {
		return false, err
	}
	if len(model.Items()) == before {
		return false, nil
	}
	if err := h.save(ctx, model, data); err != nil {
		return false, err
	}
	h.commit(model, data, false)
	return true, nil
}
func (h *History) SetRetention(days int) {
	if !settings.ValidRetentionDays(days) {
		return
	}
	h.commands.Lock()
	h.retention = days
	h.commands.Unlock()
}
func (h *History) ClearCurrent() {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	model.Capture("", "", false)
	h.commit(model, data, h.dirty)
}
func (h *History) Select(id string) (clipmodel.Item, error) {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	item, err := model.Select(id)
	if err == nil {
		h.commit(model, data, h.dirty)
	}
	return item, err
}
func (h *History) Enhance(ctx context.Context, id string, kind clipmodel.EnhancementKind) (clipmodel.Draft, error) {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	draft, err := model.Enhance(ctx, id, kind)
	if err == nil {
		h.commit(model, data, h.dirty)
	}
	return draft, err
}
func (h *History) ValidateDraft(id string) (clipmodel.Draft, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.ValidateDraft(id)
}
func (h *History) Cancel(id string) {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	model.Cancel(id)
	h.commit(model, data, h.dirty)
}

// SetAllowedApps is a trusted service operation, never a UI-supplied content policy.
func (h *History) SetAllowedApps(ctx context.Context, id string, apps []string) error {
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	if err := model.SetAllowedApps(id, apps); err != nil {
		return err
	}
	if err := h.save(ctx, model, data); err != nil {
		return err
	}
	h.commit(model, data, false)
	return nil
}
