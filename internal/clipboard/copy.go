package clipboard

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"copiggy/internal/clipmodel"
)

type Copy struct {
	History *History
	Port    ClipboardWriter
}

func (c Copy) Item(ctx context.Context, id string) (clipmodel.Item, int64, error) {
	if c.History == nil || c.Port == nil {
		return clipmodel.Item{}, 0, errors.New("clipboard copy is unavailable")
	}
	h := c.History
	h.commands.Lock()
	defer h.commands.Unlock()
	record, ok := h.Record(id)
	if !ok {
		return clipmodel.Item{}, 0, clipmodel.ErrItemNotFound
	}
	if err := ctx.Err(); err != nil {
		return clipmodel.Item{}, 0, err
	}
	revision, err := c.Port.Write(ctx, record.Data, slices.Clone(record.Item.AllowedApps))
	if err != nil {
		return clipmodel.Item{}, revision, err
	}
	model, data := h.candidate()
	item, err := model.Select(id)
	if err != nil {
		return clipmodel.Item{}, revision, err
	}
	h.commit(model, data, h.dirty)
	return item, revision, nil
}

func (c Copy) Draft(ctx context.Context, id string) (clipmodel.Item, int64, error) {
	if c.History == nil || c.Port == nil {
		return clipmodel.Item{}, 0, errors.New("clipboard copy is unavailable")
	}
	h := c.History
	h.commands.Lock()
	defer h.commands.Unlock()
	model, data := h.candidate()
	draft, err := model.ValidateDraft(id)
	if err != nil {
		return clipmodel.Item{}, 0, err
	}
	if err = ctx.Err(); err != nil {
		return clipmodel.Item{}, 0, err
	}
	representation := clipmodel.TextData(draft.Text)
	revision, err := c.Port.Write(ctx, representation.Clone(), slices.Clone(draft.AllowedApps))
	if err != nil {
		return clipmodel.Item{}, revision, err
	}
	item, err := model.Accept(id)
	if err != nil {
		return clipmodel.Item{}, revision, err
	}
	data[item.ID] = representation
	if err = trim(model, data, h.retention, time.Now()); err != nil {
		return clipmodel.Item{}, revision, err
	}
	h.commit(model, data, true)
	if err = h.save(context.WithoutCancel(ctx), model, data); err != nil {
		return item, revision, fmt.Errorf("%w: %w", ErrHistorySave, err)
	}
	h.commit(model, data, false)
	return item, revision, nil
}
