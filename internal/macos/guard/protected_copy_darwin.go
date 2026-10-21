//go:build darwin

package guard

import (
	"errors"
	"time"
)

// PendingCopy is the source of truth; event notifications may be dropped when
// the UI is busy. Call it while polling for a newly materialized clipboard.
// It expires an overdue request without changing the previous policy.
func (b *Bridge) PendingCopy() (CopyRequest, bool) {
	request := b.copies.load().pending
	if request == nil {
		return CopyRequest{}, false
	}
	if !time.Now().Before(request.Deadline) {
		b.CancelCopy(request.ID, ErrCopyExpired.Error())
		return CopyRequest{}, false
	}
	return *request, true
}

func (b *Bridge) copyCancelled(request *CopyRequest, reason string) {
	if request == nil {
		return
	}
	b.notify(Event{
		Kind: "copy-failed", RequestID: request.ID, Version: request.BaselineVersion,
		Message: reason + "; copy protection not confirmed", Target: request.Source,
	})
}

// CancelCopy can never cancel a newer request, and never clears an existing
// confirmed policy. A cancel does not undo the source application's copy.
func (b *Bridge) CancelCopy(id uint64, reason string) bool {
	request, cancelled := b.copies.cancel(id)
	if cancelled {
		if reason == "" {
			reason = "Protected copy cancelled"
		}
		b.copyCancelled(&request, reason)
	}
	return cancelled
}

// ConfirmCopy is called only after the main thread has read and validated a
// supported representation at version. It rechecks actual native state, then
// atomically replaces pending protection with a policy for that same version.
// An unchanged clipboard leaves the request pending for another poll.
func (b *Bridge) ConfirmCopy(id uint64, version int64) (bool, error) {
	request := b.copies.load().pending
	if request == nil || request.ID != id {
		return false, ErrCopyStale
	}
	fail := func(err error) (bool, error) {
		if !errors.Is(err, ErrCopyUnchanged) {
			b.CancelCopy(id, err.Error())
		}
		return false, err
	}
	if b.panelVisible.Load() {
		return fail(ErrCopySourceChanged)
	}
	if !b.Enabled() {
		return fail(ErrCopyGuardDisabled)
	}
	if b.ChangeCount() != version {
		return fail(ErrCopyVersionChanged)
	}
	source := b.Frontmost()
	if b.ChangeCount() != version {
		return fail(ErrCopyVersionChanged)
	}
	if b.panelVisible.Load() {
		return fail(ErrCopySourceChanged)
	}
	confirmed, err := b.copies.confirm(id, version, source, time.Now(), b.enabled.Load())
	if err != nil {
		return fail(err)
	}
	name := confirmed.Source.Name
	if name == "" {
		name = confirmed.Source.BundleID
	}
	b.notify(Event{
		Kind: "copy-protected", RequestID: confirmed.ID, Version: version,
		Message: "Protected copy: paste only in " + name, Target: confirmed.Source,
	})
	return true, nil
}

func (b *Bridge) beginProtectedCopy(targetPID int, event uintptr, flags uint64) uintptr {
	source := b.targetApp(targetPID)
	frontmost := b.Frontmost()
	if source.BundleID == "" || source.PID <= 0 ||
		source.PID != frontmost.PID || source.BundleID != frontmost.BundleID {
		b.notify(Event{Kind: "copy-failed", Message: "Protected copy skipped: source app unknown"})
		return event
	}
	baseline := b.ChangeCount()
	request, replaced := b.copies.begin(source, baseline, time.Now())
	if request.ID == 0 {
		return event
	}
	b.copyCancelled(replaced, "A new protected copy replaced the previous request")
	b.copySequence.active = true
	cgSetEventFlags(event, flags&^ModifierCtrl)
	b.notify(Event{
		Kind: "copy-pending", RequestID: request.ID, Version: baseline,
		Message: "Waiting for " + source.Name + " to copy…", Target: source,
	})
	return event
}
