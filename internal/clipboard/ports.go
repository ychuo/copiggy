// Package clipboard coordinates trusted clipboard records, history transactions,
// and local enhancements. It has no UI or platform dependency.
package clipboard

import (
	"context"
	"time"

	"copiggy/internal/clipmodel"
)

// HistoryRepository loads one validated snapshot. authoritative is false only
// when history.json is missing and a one-time legacy import must be committed.
type HistoryRepository interface {
	Load(context.Context) ([]clipmodel.Record, bool, error)
	Save(context.Context, []clipmodel.Record) error
}

// ClipboardWriter must verify its per-write ownership marker against a stable
// system revision before binding allowedApps. It never simulates paste.
type ClipboardWriter interface {
	Write(context.Context, clipmodel.Representations, []string) (int64, error)
}

type SourceApp struct {
	Name, BundleID string
	PID            int
}
type CopyRequest struct {
	ID              uint64
	Source          SourceApp
	BaselineVersion int64
	Deadline        time.Time
}
type GuardSnapshot struct {
	Enabled     bool
	Revision    int64
	AllowedApps []string
}

// ProtectedCopyPort observes and confirms native-owned intent; it cannot write
// content or invent a protection policy. Native confirmation rechecks authority.
type ProtectedCopyPort interface {
	PendingCopy() (CopyRequest, bool)
	Frontmost() SourceApp
	ChangeCount() int64
	CancelCopy(uint64, string) bool
	ConfirmCopy(uint64, int64) (bool, error)
}
