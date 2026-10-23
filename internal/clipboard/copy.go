package clipboard

import (
	"bytes"
	"context"
	"errors"
	"slices"

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
	files, err := clipmodel.FilesFromRepresentations(record.Data)
	if err != nil {
		return clipmodel.Item{}, 0, err
	}
	writeData := record.Data
	var paths string
	if len(files) > 0 {
		paths, err = clipmodel.FilePathsText(files)
		if err != nil {
			return clipmodel.Item{}, 0, err
		}
		writeData = clipmodel.TextData(paths)
	}
	if err := ctx.Err(); err != nil {
		return clipmodel.Item{}, 0, err
	}
	revision, err := c.Port.Write(ctx, writeData, slices.Clone(record.Item.AllowedApps))
	if err != nil {
		return clipmodel.Item{}, revision, err
	}
	model, data := h.candidate()
	if len(files) > 0 {
		// The clipboard now contains plain paths, not the captured file payload.
		// Only an existing exact text record may become current; no record is
		// created and the source's representations and protection stay intact.
		item := copiedTextMetadata(paths, record.Item.ID, record.Item.SourceApp, record.Item.SourceAppID, record.Item.RecognitionRegion, record.Item.AllowedApps)
		item = matchCopiedText(model, data, item)
		h.commit(model, data, h.dirty)
		return item, revision, nil
	}
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
	// A successful write consumes only the transient draft. Formatting does
	// not create a saved item or alter the original's bytes, time or position.
	model.Cancel(id)
	item := copiedTextMetadata(draft.Text, draft.ParentID, draft.SourceApp, draft.SourceAppID, draft.RecognitionRegion, draft.AllowedApps)
	item = matchCopiedText(model, data, item)
	h.commit(model, data, h.dirty)
	return item, revision, nil
}

func (c Copy) Variant(ctx context.Context, id string, format clipmodel.CopyFormat) (clipmodel.Item, int64, error) {
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
	text, err := clipmodel.CopyText(record.Item, format)
	if err != nil {
		return clipmodel.Item{}, 0, err
	}
	if err = ctx.Err(); err != nil {
		return clipmodel.Item{}, 0, err
	}
	revision, err := c.Port.Write(ctx, clipmodel.TextData(text), slices.Clone(record.Item.AllowedApps))
	if err != nil {
		return clipmodel.Item{}, revision, err
	}
	model, data := h.candidate()
	item := copiedTextMetadata(text, record.Item.ID, record.Item.SourceApp, record.Item.SourceAppID, record.Item.RecognitionRegion, record.Item.AllowedApps)
	item = matchCopiedText(model, data, item)
	h.commit(model, data, h.dirty)
	return item, revision, nil
}

// An acknowledgement has provenance and scope, but no invented history ID.
// The native writer independently binds that scope to its actual revision.
func copiedTextMetadata(text, parentID, sourceApp, sourceAppID, region string, allowed []string) clipmodel.Item {
	return clipmodel.Item{Type: clipmodel.AnalyzeInRegion(text, region).Type, Text: text, RecognitionRegion: region, ParentID: parentID, SourceApp: sourceApp, SourceAppID: sourceAppID, AllowedApps: slices.Clone(allowed)}
}

// Current must identify exact saved representations, not merely the source of
// a conversion. A rich original with the same text does not match plain text.
// Matching an existing item changes only current, never order or timestamps.
func matchCopiedText(model *clipmodel.Store, data map[string]clipmodel.Representations, copied clipmodel.Item) clipmodel.Item {
	scope := slices.Clone(copied.AllowedApps)
	slices.Sort(scope)
	for _, item := range model.Items() {
		if item.RecognitionRegion != copied.RecognitionRegion {
			continue
		}
		representations := data[item.ID]
		if len(representations) != 1 || len(representations[0]) != 1 || representations[0][0].Format != clipmodel.TextFormat || !bytes.Equal(representations[0][0].Bytes, []byte(copied.Text)) {
			continue
		}
		allowed := slices.Clone(item.AllowedApps)
		slices.Sort(allowed)
		if !slices.Equal(scope, allowed) {
			continue
		}
		selected, _ := model.Select(item.ID)
		return selected
	}
	model.Capture("", "", false)
	return copied
}
