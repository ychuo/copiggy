package clipboard

import (
	"context"

	"copiggy/internal/clipmodel"
)

type CapturedCopy struct {
	Item      clipmodel.Item
	Data      clipmodel.Representations
	Source    SourceApp
	Revision  int64
	SaveError error
}

// CompleteProtectedCopy materializes data before native confirmation. It can
// only observe a copy made by the source; it cannot write a replacement value.
func CompleteProtectedCopy(ctx context.Context, native ProtectedCopyPort, read func() (clipmodel.Representations, error), history *History) (CapturedCopy, bool) {
	request, pending := native.PendingCopy()
	if !pending {
		return CapturedCopy{}, false
	}
	source := native.Frontmost()
	if source.PID != request.Source.PID || source.BundleID != request.Source.BundleID {
		native.CancelCopy(request.ID, "Protected copy cancelled: source app changed")
		return CapturedCopy{}, false
	}
	revision := native.ChangeCount()
	if revision == request.BaselineVersion {
		return CapturedCopy{}, false
	}
	data, err := read()
	if err != nil || revision != native.ChangeCount() {
		return CapturedCopy{}, false
	}
	if _, err = clipmodel.ContentFromRepresentations(data); err != nil {
		if text, _ := data.Read(clipmodel.TextFormat); len(text) > 1<<20 {
			native.CancelCopy(request.ID, "Protected copy cancelled: text exceeds 1 MiB")
		}
		return CapturedCopy{}, false
	}
	if ctx.Err() != nil {
		return CapturedCopy{}, false
	}
	ok, err := native.ConfirmCopy(request.ID, revision)
	if err != nil || !ok {
		return CapturedCopy{}, false
	}
	item, recorded, saveErr := history.Capture(context.WithoutCancel(ctx), data, clipmodel.Source{Name: request.Source.Name, BundleID: request.Source.BundleID}, []string{request.Source.BundleID})
	if !recorded {
		return CapturedCopy{}, false
	}
	if native.ChangeCount() != revision {
		history.ClearCurrent()
	}
	return CapturedCopy{Item: item, Data: data.Clone(), Source: request.Source, Revision: revision, SaveError: saveErr}, true
}
