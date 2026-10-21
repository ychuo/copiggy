package guard

import (
	"copiggy/internal/macos/pasteboard"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func copyFixture() (*copyState, pasteboard.App, time.Time) {
	state := &copyState{}
	state.setActive(true)
	state.setPolicy(Policy{Version: 7, AllowedApps: []string{"old.app"}})
	return state, pasteboard.App{BundleID: "source.app", PID: 123, Name: "Source"}, time.Unix(1000, 0)
}

func TestProtectedCopyChordIsExact(t *testing.T) {
	for _, flags := range []uint64{0, ModifierCmd, ModifierCtrl, ModifierCmd | ModifierCtrl | ModifierAlt, ModifierCmd | ModifierCtrl | ModifierShift} {
		if IsProtectedCopyShortcut(KeyC, flags) {
			t.Fatalf("unexpected protected-copy modifiers: %x", flags)
		}
	}
	if !IsProtectedCopyShortcut(KeyC, ModifierCmd|ModifierCtrl) ||
		IsProtectedCopyShortcut(KeyX, ModifierCmd|ModifierCtrl) {
		t.Fatal("protected copy must recognize exactly Control+Command+C")
	}
}

func TestProtectedCopyIsAtomicAndKeepsPreviousPolicyUntilConfirmed(t *testing.T) {
	state, source, now := copyFixture()
	request, old := state.begin(source, 10, now)
	if old != nil || request.ID == 0 || request.BaselineVersion != 10 ||
		!request.Deadline.Equal(now.Add(2*time.Second)) {
		t.Fatalf("unexpected request metadata: %+v", request)
	}
	if state.currentPolicy().Version != 7 {
		t.Fatal("starting a copy destroyed the old policy")
	}
	if !blocksPendingCopy(state.load().pending, pasteboard.App{BundleID: "other.app"}, now) ||
		blocksPendingCopy(state.load().pending, source, now) {
		t.Fatal("pending cross-application protection is incorrect")
	}
	confirmed, err := state.confirm(request.ID, 11, source, now.Add(time.Millisecond), true)
	if err != nil || confirmed.ID != request.ID {
		t.Fatalf("confirmation failed: %v", err)
	}
	after := state.load()
	if after.pending != nil || after.policy.Version != 11 ||
		!reflect.DeepEqual(after.policy.AllowedApps, []string{source.BundleID}) {
		t.Fatalf("confirmation failed to install a single complete state: %+v", after)
	}
	copy := state.currentPolicy()
	copy.AllowedApps[0] = "modified.app"
	if state.currentPolicy().AllowedApps[0] != source.BundleID {
		t.Fatal("CurrentPolicy exposes mutable internal policy storage")
	}
}

func TestConfirmationRejectsInvalidEvidenceWithoutChangingOldPolicy(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		version int64
		app     pasteboard.App
		elapsed time.Duration
		enabled bool
		want    error
	}{
		{"unchanged version", 10, pasteboard.App{BundleID: "source.app", Name: "Source", PID: 123}, 0, true, ErrCopyUnchanged},
		{"different process", 11, pasteboard.App{BundleID: "source.app", Name: "Source", PID: 124}, 0, true, ErrCopySourceChanged},
		{"different bundle", 11, pasteboard.App{BundleID: "other.app", Name: "Other", PID: 123}, 0, true, ErrCopySourceChanged},
		{"deadline reached", 11, pasteboard.App{BundleID: "source.app", Name: "Source", PID: 123}, ProtectedCopyTimeout, true, ErrCopyExpired},
		{"guard disabled", 11, pasteboard.App{BundleID: "source.app", Name: "Source", PID: 123}, 0, false, ErrCopyGuardDisabled},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state, source, now := copyFixture()
			request, _ := state.begin(source, 10, now)
			if _, err := state.confirm(request.ID, scenario.version, scenario.app, now.Add(scenario.elapsed), scenario.enabled); !errors.Is(err, scenario.want) {
				t.Fatalf("got %v, want %v", err, scenario.want)
			}
			if state.currentPolicy().Version != 7 || state.load().pending.ID != request.ID {
				t.Fatal("failed validation changed the policy or another transaction")
			}
		})
	}
}

func TestReplacingCancellingAndDisablingCannotConfirmAnOldRequest(t *testing.T) {
	state, source, now := copyFixture()
	first, _ := state.begin(source, 10, now)
	second, replaced := state.begin(source, 11, now)
	if second.ID == first.ID || replaced == nil || replaced.ID != first.ID {
		t.Fatal("new physical chord did not replace the old transaction")
	}
	if _, ok := state.cancel(first.ID); ok {
		t.Fatal("late cancellation removed a newer request")
	}
	if _, err := state.confirm(first.ID, 12, source, now, true); !errors.Is(err, ErrCopyStale) {
		t.Fatalf("late confirmation did not fail: %v", err)
	}
	if cancelled := state.setActive(false); cancelled == nil || cancelled.ID != second.ID {
		t.Fatal("stopping protection did not atomically clear pending")
	}
	if _, err := state.confirm(second.ID, 12, source, now, true); !errors.Is(err, ErrCopyStale) {
		t.Fatalf("confirmation survived guard shutdown: %v", err)
	}
	if request, _ := state.begin(source, 12, now); request.ID != 0 {
		t.Fatal("inactive guard accepted a new transaction")
	}
	if state.currentPolicy().Version != 7 {
		t.Fatal("cancellation destroyed previously confirmed protection")
	}
}

func TestExplicitSelectionCancelsPendingWithoutOverwritingItsPolicyLater(t *testing.T) {
	state, source, now := copyFixture()
	request, _ := state.begin(source, 10, now)
	cancelled := state.setPolicy(Policy{Version: 20, AllowedApps: []string{"selected.app"}})
	if cancelled == nil || cancelled.ID != request.ID || state.load().pending != nil {
		t.Fatal("explicit selection left a pending copy")
	}
	if _, err := state.confirm(request.ID, 21, source, now, true); !errors.Is(err, ErrCopyStale) {
		t.Fatal("a replaced copy overwrote explicit selection")
	}
	if state.currentPolicy().Version != 20 {
		t.Fatal("selected policy changed")
	}
}

func TestConfirmationAndNextRequestCannotInterleaveTheirTokens(t *testing.T) {
	state, source, now := copyFixture()
	for i := 0; i < 100; i++ {
		first, _ := state.begin(source, 10, now)
		var second CopyRequest
		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			_, _ = state.confirm(first.ID, 11, source, now, true)
		}()
		go func() {
			defer wait.Done()
			second, _ = state.begin(source, 20, now)
		}()
		wait.Wait()
		if pending := state.load().pending; pending == nil || pending.ID != second.ID {
			t.Fatal("older confirmation removed the newer transaction")
		}
	}
}
