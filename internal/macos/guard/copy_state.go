package guard

import (
	"copiggy/internal/macos/pasteboard"
	"errors"
	"slices"
	"sync/atomic"
	"time"
)

const ProtectedCopyTimeout = 2 * time.Second

var (
	ErrCopyStale          = errors.New("protected copy request is no longer active")
	ErrCopyExpired        = errors.New("protected copy timed out")
	ErrCopyUnchanged      = errors.New("clipboard has not changed yet")
	ErrCopySourceChanged  = errors.New("copy source changed")
	ErrCopyGuardDisabled  = errors.New("paste guard is off")
	ErrCopyVersionChanged = errors.New("clipboard changed before confirmation")
)

// CopyRequest records intent, not proof of pasteboard ownership. macOS does
// not identify a pasteboard writer reliably: this transaction correlates one
// shortcut, the still-frontmost source application, and a new stable version.
type CopyRequest struct {
	ID              uint64
	Source          pasteboard.App
	BaselineVersion int64
	Deadline        time.Time
}

// policy and pending live in one immutable atomically replaced value. In
// particular, confirmation cannot expose an interval with neither pending
// protection nor the newly confirmed policy.
type protectionState struct {
	policy  Policy
	pending *CopyRequest
	active  bool
}

type copyState struct {
	state atomic.Pointer[protectionState]
	next  atomic.Uint64
}

func (s *copyState) load() *protectionState {
	if state := s.state.Load(); state != nil {
		return state
	}
	initial := &protectionState{policy: Policy{Version: -1}}
	s.state.CompareAndSwap(nil, initial)
	return s.state.Load()
}

func (s *copyState) setPolicy(policy Policy) *CopyRequest {
	policy.AllowedApps = slices.Clone(policy.AllowedApps)
	for {
		before := s.load()
		after := &protectionState{policy: policy, active: before.active}
		if s.state.CompareAndSwap(before, after) {
			return before.pending
		}
	}
}

func (s *copyState) setActive(active bool) *CopyRequest {
	for {
		before := s.load()
		after := &protectionState{policy: before.policy, active: active}
		if active {
			after.pending = before.pending
		}
		if s.state.CompareAndSwap(before, after) {
			if !active {
				return before.pending
			}
			return nil
		}
	}
}

func (s *copyState) currentPolicy() Policy {
	policy := s.load().policy
	policy.AllowedApps = slices.Clone(policy.AllowedApps)
	return policy
}

func (s *copyState) begin(source pasteboard.App, baseline int64, now time.Time) (CopyRequest, *CopyRequest) {
	request := CopyRequest{
		ID: s.next.Add(1), Source: source, BaselineVersion: baseline,
		Deadline: now.Add(ProtectedCopyTimeout),
	}
	for {
		before := s.load()
		if !before.active {
			return CopyRequest{}, nil
		}
		after := &protectionState{policy: before.policy, pending: &request, active: before.active}
		if s.state.CompareAndSwap(before, after) {
			return request, before.pending
		}
	}
}

func (s *copyState) cancel(id uint64) (CopyRequest, bool) {
	for {
		before := s.load()
		if before.pending == nil || before.pending.ID != id {
			return CopyRequest{}, false
		}
		after := &protectionState{policy: before.policy, active: before.active}
		if s.state.CompareAndSwap(before, after) {
			return *before.pending, true
		}
	}
}

func validateCopy(request CopyRequest, version int64, source pasteboard.App, now time.Time, guardEnabled bool) error {
	switch {
	case !guardEnabled:
		return ErrCopyGuardDisabled
	case !now.Before(request.Deadline):
		return ErrCopyExpired
	case source.PID <= 0 || source.PID != request.Source.PID ||
		source.BundleID == "" || source.BundleID != request.Source.BundleID:
		return ErrCopySourceChanged
	case version == request.BaselineVersion:
		return ErrCopyUnchanged
	default:
		return nil
	}
}

func (s *copyState) confirm(id uint64, version int64, source pasteboard.App, now time.Time, enabled bool) (CopyRequest, error) {
	for {
		before := s.load()
		if before.pending == nil || before.pending.ID != id {
			return CopyRequest{}, ErrCopyStale
		}
		request := *before.pending
		if err := validateCopy(request, version, source, now, enabled && before.active); err != nil {
			return request, err
		}
		after := &protectionState{active: before.active, policy: Policy{
			Version: version, AllowedApps: []string{request.Source.BundleID},
		}}
		if s.state.CompareAndSwap(before, after) {
			return request, nil
		}
	}
}

func blocksPendingCopy(request *CopyRequest, target pasteboard.App, now time.Time) bool {
	return request != nil && now.Before(request.Deadline) &&
		(target.BundleID == "" || target.BundleID != request.Source.BundleID)
}
