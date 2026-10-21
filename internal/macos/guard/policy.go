// Package guard owns the native input listener and its revision-bound
// protected-copy transaction. AppKit UI and pasteboard observations are separate adapters.
package guard

import (
	"copiggy/internal/macos/pasteboard"
	"slices"
)

// PermissionState is observational. Reading it never requests authorization.
type PermissionState struct {
	Accessibility bool
	ListenEvents  bool
	PostEvents    bool
}

type Event struct {
	Kind      string
	Message   string
	Target    pasteboard.App
	RequestID uint64
	Version   int64
}

// Policy belongs to exactly one system pasteboard generation.
type Policy struct {
	Version     int64
	AllowedApps []string
}

const (
	KeyC          uint16 = 0x08
	KeyV          uint16 = 0x09
	KeyX          uint16 = 0x07
	ModifierShift uint64 = 1 << 17
	ModifierCtrl  uint64 = 1 << 18
	ModifierAlt   uint64 = 1 << 19
	ModifierCmd   uint64 = 1 << 20
)

func IsProtectedCopyShortcut(key uint16, flags uint64) bool {
	modifiers := flags & (ModifierShift | ModifierCtrl | ModifierAlt | ModifierCmd)
	return key == KeyC && modifiers == ModifierCtrl|ModifierCmd
}

func isPlainCopyOrCut(key uint16, flags uint64) bool {
	modifiers := flags & (ModifierShift | ModifierCtrl | ModifierAlt | ModifierCmd)
	return (key == KeyC || key == KeyX) && modifiers == ModifierCmd
}

// copyKeySequence remembers a C down whose Control modifier was removed.
// Repeats are suppressed: one physical chord delivers exactly one Cmd+C.
// Its matching key-up is still normalized after cancellation or confirmation.
type copyKeySequence struct{ active bool }

func (s *copyKeySequence) finish(key uint16) bool {
	if key != KeyC || !s.active {
		return false
	}
	s.active = false
	return true
}

// IsPasteShortcut recognizes only standard paste and paste-and-match-style.
// Caps Lock and device-independent status bits do not alter the decision.
func IsPasteShortcut(key uint16, flags uint64) bool {
	modifiers := flags & (ModifierShift | ModifierCtrl | ModifierAlt | ModifierCmd)
	return key == KeyV && (modifiers == ModifierCmd ||
		modifiers == ModifierCmd|ModifierAlt|ModifierShift)
}

// Evaluate returns true only when the current, version-matched restricted
// content should have its paste shortcut blocked. A visible keyboard-taking
// panel is excluded because its nonactivating NSPanel leaves the previous
// application reported as frontmost.
//
// Unknown application identity is not an allowed application. A changed
// clipboard generation is deliberately not assigned the previous policy.
func Evaluate(policy Policy, currentVersion int64, bundleID string, panelVisible bool, key uint16, flags uint64) bool {
	return !panelVisible &&
		IsPasteShortcut(key, flags) &&
		policy.Version == currentVersion &&
		len(policy.AllowedApps) > 0 &&
		!slices.Contains(policy.AllowedApps, bundleID)
}

// keySequence tracks a suppressed down event until its matching up event.
// It must be used by one event-loop thread.
type keySequence struct {
	blockedV bool
}

func (s *keySequence) decide(down bool, key uint16, shouldBlock bool) bool {
	if key != KeyV {
		return false
	}
	if !down {
		blocked := s.blockedV
		s.blockedV = false
		return blocked
	}
	// Auto-repeats of a blocked key stay blocked until key-up, even if the
	// clipboard or keyboard modifiers changed in the meantime.
	if s.blockedV {
		return true
	}
	s.blockedV = shouldBlock
	return shouldBlock
}
