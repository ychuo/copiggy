package app

import (
	"context"
	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"copiggy/internal/macos/guard"
	"copiggy/internal/macos/pasteboard"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// Called only from the command worker. AppKit operations remain main-thread
// operations while parsing, image validation and persistence run off that thread.
func onMain[T any](work func() T, fallback ...T) T {
	var value T
	if len(fallback) > 0 {
		value = fallback[0]
	}
	// MyGo already waits for this callback, and drops it after event-loop
	// shutdown. Retain an explicit failure fallback for operations with effects.
	mygo.RunOnMain(func() { value = work() })
	return value
}

type readResult struct {
	data clipmodel.Representations
	err  error
}

func materialize(data transfer.Data) (clipmodel.Representations, error) {
	out := make(clipmodel.Representations, 0, len(data.Items()))
	for _, item := range data.Items() {
		var reps []clipmodel.Representation
		for _, format := range item.Formats() {
			value, err := item.Read(format)
			if err != nil {
				return nil, err
			}
			reps = append(reps, clipmodel.Representation{Format: string(format), Bytes: value})
		}
		out = append(out, reps)
	}
	return out, nil
}
func nativeData(data clipmodel.Representations) transfer.Data {
	var items []transfer.Item
	for _, item := range data {
		var reps []transfer.Representation
		for _, rep := range item {
			reps = append(reps, transfer.Bytes(transfer.Format(rep.Format), rep.Bytes))
		}
		items = append(items, transfer.NewItem(reps...))
	}
	return transfer.New(items...)
}
func sourceApp(source pasteboard.App) clipboard.SourceApp {
	return clipboard.SourceApp{Name: source.Name, BundleID: source.BundleID, PID: source.PID}
}

type nativeClipboard struct {
	reader *pasteboard.Reader
	guard  *guard.Bridge
}

func (n nativeClipboard) read() (clipmodel.Representations, error) {
	r := onMain(func() readResult {
		data, err := n.reader.Read()
		if err != nil {
			return readResult{err: err}
		}
		reps, err := materialize(data)
		return readResult{reps, err}
	}, readResult{err: context.Canceled})
	return r.data, r.err
}
func (n nativeClipboard) ChangeCount() int64 { return onMain(n.reader.ChangeCount) }
func (n nativeClipboard) Frontmost() clipboard.SourceApp {
	return onMain(func() clipboard.SourceApp { return sourceApp(n.reader.Frontmost()) })
}
func (n nativeClipboard) PendingCopy() (clipboard.CopyRequest, bool) {
	type result struct {
		request clipboard.CopyRequest
		pending bool
	}
	r := onMain(func() result {
		req, ok := n.guard.PendingCopy()
		return result{clipboard.CopyRequest{ID: req.ID, Source: sourceApp(req.Source), BaselineVersion: req.BaselineVersion, Deadline: req.Deadline}, ok}
	})
	return r.request, r.pending
}
func (n nativeClipboard) CancelCopy(id uint64, reason string) bool {
	return onMain(func() bool { return n.guard.CancelCopy(id, reason) })
}
func (n nativeClipboard) ConfirmCopy(id uint64, revision int64) (bool, error) {
	type result struct {
		ok  bool
		err error
	}
	r := onMain(func() result { ok, err := n.guard.ConfirmCopy(id, revision); return result{ok, err} })
	return r.ok, r.err
}
func (n nativeClipboard) Write(ctx context.Context, data clipmodel.Representations, allowed []string) (int64, error) {
	type result struct {
		revision int64
		err      error
	}
	r := onMain(func() result {
		if err := ctx.Err(); err != nil {
			return result{err: err}
		}
		if request, ok := n.guard.PendingCopy(); ok {
			n.guard.CancelCopy(request.ID, "Protected copy cancelled: selecting clipboard history")
		}
		revision, err := n.reader.Write(nativeData(data))
		if err != nil {
			return result{revision, err}
		}
		n.guard.SetPolicy(revision, allowed)
		return result{revision, nil}
	}, result{err: context.Canceled})
	return r.revision, r.err
}
