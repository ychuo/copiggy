//go:build darwin

package guard

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"copiggy/internal/macos/pasteboard"
	"github.com/ebitengine/purego"
)

const (
	keyDown            = 10
	keyUp              = 11
	tapDisabledTimeout = 0xfffffffe
	tapDisabledUser    = 0xffffffff
	keyboardKeycode    = 9
	eventTargetPID     = 40
)

var (
	ErrAccessibility    = errors.New("Accessibility permission is required")
	ErrGuardUnavailable = errors.New("paste guard is unavailable")
)

// Bridge owns a native event-tap thread. Its guard never posts keyboard events
// or reads clipboard content; it only queries changeCount.
type Bridge struct {
	observer      *pasteboard.Reader
	id            uintptr
	pid           int
	events        chan Event
	copies        copyState
	panelVisible  atomic.Bool
	enabled       atomic.Bool
	stopRequested atomic.Bool
	tap           atomic.Uintptr
	runLoop       atomic.Uintptr
	stopSource    atomic.Uintptr
	control       sync.Mutex
	nativeState   sync.Mutex
	done          chan struct{}
	sequence      keySequence     // event-loop thread only
	copySequence  copyKeySequence // event-loop thread only
}

var (
	loadOnce         sync.Once
	loadError        error
	nextBridgeID     atomic.Uint64
	bridges          sync.Map
	tapCallback      uintptr
	stopCallback     uintptr
	cfRelease        func(uintptr)
	cfLoopCurrent    func() uintptr
	cfLoopRun        func()
	cfLoopStop       func(uintptr)
	cfLoopAdd        func(uintptr, uintptr, uintptr)
	cfLoopRemove     func(uintptr, uintptr, uintptr)
	cfLoopWake       func(uintptr)
	cfSourceCreate   func(uintptr, int64, unsafe.Pointer) uintptr
	cfSourceSignal   func(uintptr)
	cfPortSource     func(uintptr, uintptr, int64) uintptr
	cfInvalidate     func(uintptr)
	cfDictCreate     func(uintptr, unsafe.Pointer, unsafe.Pointer, int64, uintptr, uintptr) uintptr
	cgTapCreate      func(uint32, uint32, uint32, uint64, uintptr, uintptr) uintptr
	cgTapEnable      func(uintptr, bool)
	cgTapEnabled     func(uintptr) bool
	cgEventFlags     func(uintptr) uint64
	cgSetEventFlags  func(uintptr, uint64)
	cgEventField     func(uintptr, uint32) int64
	axTrusted        func() bool
	axTrustedOpts    func(uintptr) bool
	cgListen         func() bool
	cgPost           func() bool
	defaultMode      uintptr
	cfTrue           uintptr
	axPromptKey      uintptr
	cfKeyCallbacks   uintptr
	cfValueCallbacks uintptr
)

// New loads public system frameworks without opening a window or requesting
// permission. Native symbols and the single process-lifetime callback are
// initialized once; purego callbacks cannot be freed.
func New() (*Bridge, error) {
	observer, err := pasteboard.New()
	if err != nil {
		return nil, err
	}
	loadOnce.Do(func() {
		loadError = loadNative()
	})
	if loadError != nil {
		return nil, loadError
	}
	b := &Bridge{
		observer: observer,
		id:       uintptr(nextBridgeID.Add(1)),
		pid:      os.Getpid(),
		events:   make(chan Event, 16),
	}
	b.copies.load()
	return b, nil
}

func loadNative() (err error) {
	// RegisterLibFunc panics on a missing symbol; expose initialization errors
	// through New rather than allowing an application startup panic.
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("macOS native bridge: %v", failure)
		}
	}()
	open := func(path string) uintptr {
		handle, e := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if e != nil {
			panic(e)
		}
		// Framework handles intentionally live for the process lifetime.
		return handle
	}
	symbol := func(handle uintptr, name string) uintptr {
		address, e := purego.Dlsym(handle, name)
		if e != nil {
			panic(e)
		}
		return address
	}
	constant := func(handle uintptr, name string) uintptr {
		address := symbol(handle, name)
		// The symbol is a pointer-valued C global, outside the Go heap.
		return **(**uintptr)(unsafe.Pointer(&address))
	}
	cf := open("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	cg := open("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	ax := open("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
	purego.RegisterLibFunc(&cfLoopCurrent, cf, "CFRunLoopGetCurrent")
	purego.RegisterLibFunc(&cfLoopRun, cf, "CFRunLoopRun")
	purego.RegisterLibFunc(&cfLoopStop, cf, "CFRunLoopStop")
	purego.RegisterLibFunc(&cfLoopAdd, cf, "CFRunLoopAddSource")
	purego.RegisterLibFunc(&cfLoopRemove, cf, "CFRunLoopRemoveSource")
	purego.RegisterLibFunc(&cfLoopWake, cf, "CFRunLoopWakeUp")
	purego.RegisterLibFunc(&cfSourceCreate, cf, "CFRunLoopSourceCreate")
	purego.RegisterLibFunc(&cfSourceSignal, cf, "CFRunLoopSourceSignal")
	purego.RegisterLibFunc(&cfPortSource, cf, "CFMachPortCreateRunLoopSource")
	purego.RegisterLibFunc(&cfInvalidate, cf, "CFMachPortInvalidate")
	purego.RegisterLibFunc(&cfDictCreate, cf, "CFDictionaryCreate")
	purego.RegisterLibFunc(&cgTapCreate, cg, "CGEventTapCreate")
	purego.RegisterLibFunc(&cgTapEnable, cg, "CGEventTapEnable")
	purego.RegisterLibFunc(&cgTapEnabled, cg, "CGEventTapIsEnabled")
	purego.RegisterLibFunc(&cgEventFlags, cg, "CGEventGetFlags")
	purego.RegisterLibFunc(&cgSetEventFlags, cg, "CGEventSetFlags")
	purego.RegisterLibFunc(&cgEventField, cg, "CGEventGetIntegerValueField")
	purego.RegisterLibFunc(&cgListen, cg, "CGPreflightListenEventAccess")
	purego.RegisterLibFunc(&cgPost, cg, "CGPreflightPostEventAccess")
	purego.RegisterLibFunc(&axTrusted, ax, "AXIsProcessTrusted")
	purego.RegisterLibFunc(&axTrustedOpts, ax, "AXIsProcessTrustedWithOptions")
	defaultMode = constant(cf, "kCFRunLoopDefaultMode")
	cfTrue = constant(cf, "kCFBooleanTrue")
	axPromptKey = constant(ax, "kAXTrustedCheckOptionPrompt")
	cfKeyCallbacks = symbol(cf, "kCFTypeDictionaryKeyCallBacks")
	cfValueCallbacks = symbol(cf, "kCFTypeDictionaryValueCallBacks")
	tapCallback = purego.NewCallback(func(proxy uintptr, kind uint32, event, user uintptr) uintptr {
		if bridge, ok := bridges.Load(user); ok {
			return bridge.(*Bridge).handleEvent(kind, event)
		}
		return event
	})
	stopCallback = purego.NewCallback(func(user uintptr) {
		if bridge, ok := bridges.Load(user); ok {
			b := bridge.(*Bridge)
			if b.stopRequested.Load() {
				cfLoopStop(cfLoopCurrent())
			}
		}
	})
	return nil
}

// Observations use the same trusted native reader as the event listener.
func (b *Bridge) ChangeCount() int64               { return b.observer.ChangeCount() }
func (b *Bridge) Frontmost() pasteboard.App        { return b.observer.Frontmost() }
func (b *Bridge) targetApp(pid int) pasteboard.App { return b.observer.TargetApp(pid) }

func (b *Bridge) Permissions() PermissionState {
	return PermissionState{
		Accessibility: axTrusted(),
		ListenEvents:  cgListen(),
		PostEvents:    cgPost(),
	}
}

// RequestAccessibility is the only API which can open a system authorization
// prompt. Call it only after the user explicitly selects an enable action.
func (b *Bridge) RequestAccessibility() bool {
	keys, values := [1]uintptr{axPromptKey}, [1]uintptr{cfTrue}
	dict := cfDictCreate(0, unsafe.Pointer(&keys[0]), unsafe.Pointer(&values[0]), 1, cfKeyCallbacks, cfValueCallbacks)
	if dict == 0 {
		return false
	}
	defer cfRelease(dict)
	return axTrustedOpts(dict)
}

// SetPolicy copies the caller's slice. A nil or empty allowlist disables
// restrictions for that clipboard generation.
func (b *Bridge) SetPolicy(version int64, allowedApps []string) {
	apps := make([]string, 0, len(allowedApps))
	for _, app := range allowedApps {
		if app = strings.TrimSpace(app); app != "" && !slices.Contains(apps, app) {
			apps = append(apps, app)
		}
	}
	b.copyCancelled(b.copies.setPolicy(Policy{Version: version, AllowedApps: apps}), "Another clipboard item was selected")
}

func (b *Bridge) CurrentPolicy() Policy { return b.copies.currentPolicy() }

// visible means the panel currently takes the keyboard, not that a regular
// testing window is merely still on screen behind another application.
func (b *Bridge) SetPanelVisible(visible bool) {
	b.panelVisible.Store(visible)
	if visible {
		if request := b.copies.load().pending; request != nil {
			b.CancelCopy(request.ID, "Clipboard panel opened")
		}
	}
}
func (b *Bridge) Enabled() bool {
	if !b.enabled.Load() {
		return false
	}
	b.nativeState.Lock()
	defer b.nativeState.Unlock()
	tap := b.tap.Load()
	enabled := tap != 0 && cgTapEnabled(tap) && axTrusted()
	if !enabled {
		b.copyCancelled(b.copies.setActive(false), "Keyboard listener stopped")
		b.enabled.Store(false)
	}
	return enabled
}
func (b *Bridge) Events() <-chan Event { return b.events }

func (b *Bridge) notify(event Event) {
	select {
	case b.events <- event:
	default:
		// Never hold up the system's keyboard for rendering or logging.
	}
}

func (b *Bridge) StartGuard() error {
	b.control.Lock()
	defer b.control.Unlock()
	if b.enabled.Load() {
		return nil
	}
	if b.done != nil {
		b.requestStop()
		<-b.done
		b.done = nil
	}
	if !axTrusted() {
		b.notify(Event{Kind: "permission-required", Message: ErrAccessibility.Error()})
		return ErrAccessibility
	}
	b.done = make(chan struct{})
	b.stopRequested.Store(false)
	started := make(chan error, 1)
	go b.runGuard(started, b.done)
	return <-started
}

func (b *Bridge) runGuard(started chan<- error, done chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)
	bridges.Store(b.id, b)
	defer bridges.Delete(b.id)
	tap := cgTapCreate(1, 0, 0, (1<<keyDown)|(1<<keyUp), tapCallback, b.id)
	if tap == 0 {
		started <- fmt.Errorf("%w: macOS denied the keyboard listener; check Accessibility and Input Monitoring permissions", ErrGuardUnavailable)
		return
	}
	defer cfRelease(tap)
	defer cfInvalidate(tap)
	source := cfPortSource(0, tap, 0)
	if source == 0 {
		started <- fmt.Errorf("%w: cannot create the event-loop source", ErrGuardUnavailable)
		return
	}
	defer cfRelease(source)
	loop := cfLoopCurrent()
	// A signaled source survives until CFRunLoopRun begins. This prevents
	// StopGuard immediately after StartGuard from being lost before the
	// worker thread has entered its run loop.
	stopContext := struct {
		version                                                                    int64
		info, retain, release, description, equal, hash, schedule, cancel, perform uintptr
	}{info: b.id, perform: stopCallback}
	stopSource := cfSourceCreate(0, 0, unsafe.Pointer(&stopContext))
	if stopSource == 0 {
		started <- fmt.Errorf("%w: cannot create the listener stop signal", ErrGuardUnavailable)
		return
	}
	defer cfRelease(stopSource)
	cfLoopAdd(loop, source, defaultMode)
	defer cfLoopRemove(loop, source, defaultMode)
	cfLoopAdd(loop, stopSource, defaultMode)
	defer cfLoopRemove(loop, stopSource, defaultMode)
	b.sequence = keySequence{}
	b.copySequence = copyKeySequence{}
	b.nativeState.Lock()
	b.tap.Store(tap)
	b.runLoop.Store(loop)
	b.stopSource.Store(stopSource)
	b.nativeState.Unlock()
	defer func() {
		b.nativeState.Lock()
		defer b.nativeState.Unlock()
		b.enabled.Store(false)
		b.copyCancelled(b.copies.setActive(false), "Keyboard listener stopped")
		b.tap.Store(0)
		b.runLoop.Store(0)
		b.stopSource.Store(0)
	}()
	cgTapEnable(tap, true)
	if !cgTapEnabled(tap) {
		started <- fmt.Errorf("%w: keyboard listener did not start", ErrGuardUnavailable)
		return
	}
	b.copies.setActive(true)
	b.enabled.Store(true)
	started <- nil
	b.notify(Event{Kind: "enabled", Message: "Paste guard on"})
	cfLoopRun()
	cgTapEnable(tap, false)
}

func (b *Bridge) StopGuard() {
	b.control.Lock()
	defer b.control.Unlock()
	b.copyCancelled(b.copies.setActive(false), "Paste guard off")
	b.enabled.Store(false)
	if b.done == nil {
		return
	}
	b.requestStop()
	<-b.done
	b.done = nil
	b.notify(Event{Kind: "stopped", Message: "Paste guard off"})
}

func (b *Bridge) requestStop() {
	b.stopRequested.Store(true)
	b.nativeState.Lock()
	defer b.nativeState.Unlock()
	if source := b.stopSource.Load(); source != 0 {
		cfSourceSignal(source)
	}
	if loop := b.runLoop.Load(); loop != 0 {
		cfLoopWake(loop)
	}
}

// inputTargetsPanel uses the event's known recipient before the UI's focus
// snapshot. A nonactivating panel may receive events without a target PID, but
// delayed focus notifications must never exempt a known external recipient.
func (b *Bridge) inputTargetsPanel(targetPID int) bool {
	return targetPID == b.pid || (targetPID <= 0 && b.panelVisible.Load())
}

func (b *Bridge) handleEvent(kind uint32, event uintptr) uintptr {
	if kind == tapDisabledTimeout || kind == tapDisabledUser {
		b.copyCancelled(b.copies.setActive(false), "Keyboard listener stopped")
		b.enabled.Store(false)
		message, reason := "Paste guard paused: macOS disabled the keyboard listener", "disabled-user"
		if kind == tapDisabledTimeout {
			message, reason = "Paste guard paused: keyboard listener timed out", "disabled-timeout"
		}
		b.notify(Event{Kind: reason, Message: message})
		if loop := b.runLoop.Load(); loop != 0 {
			cfLoopStop(loop)
		}
		return event
	}
	if event == 0 || (kind != keyDown && kind != keyUp) || !b.enabled.Load() {
		return event
	}
	key := uint16(cgEventField(event, keyboardKeycode))
	flags := cgEventFlags(event)
	if kind == keyUp {
		if b.copySequence.finish(key) {
			cgSetEventFlags(event, flags&^ModifierCtrl)
			return event
		}
		if b.sequence.decide(false, key, false) {
			return 0
		}
		return event
	}
	if key == KeyC && b.copySequence.active {
		// Do not send additional Cmd+C operations or create transactions
		// while a single physical protected-copy chord is held down.
		return 0
	}
	if key == KeyV && b.sequence.blockedV {
		return 0
	}
	targetPID := int(int32(cgEventField(event, eventTargetPID)))
	if b.inputTargetsPanel(targetPID) {
		if request, ok := b.PendingCopy(); ok {
			b.CancelCopy(request.ID, "Clipboard panel opened")
		}
		return event
	}
	if IsProtectedCopyShortcut(key, flags) {
		return b.beginProtectedCopy(targetPID, event, flags)
	}
	paste := IsPasteShortcut(key, flags)
	if request, ok := b.PendingCopy(); ok && !paste {
		reason := "Other input interrupted protected copy"
		if isPlainCopyOrCut(key, flags) {
			reason = "A regular copy or cut replaced protected copy"
		}
		b.CancelCopy(request.ID, reason)
	}
	if !paste {
		return event
	}
	state := b.copies.load()
	if state.pending == nil && len(state.policy.AllowedApps) == 0 {
		return event
	}
	version := b.ChangeCount()
	if state.pending == nil && state.policy.Version != version {
		return event
	}
	target := b.targetApp(targetPID)
	// Reload the joint state after native queries: confirmation may have
	// atomically turned a pending request into its new version-bound policy.
	state = b.copies.load()
	version = b.ChangeCount()
	pendingBlock := blocksPendingCopy(state.pending, target, time.Now())
	block := !b.inputTargetsPanel(targetPID) && (pendingBlock ||
		(Evaluate(state.policy, version, target.BundleID, false, key, flags) &&
			b.ChangeCount() == version))
	if b.sequence.decide(true, key, block) {
		name := target.Name
		if name == "" {
			name = "the current app"
		}
		message := "Paste blocked in " + name + ": app restriction"
		var requestID uint64
		if pendingBlock {
			requestID = state.pending.ID
			message = "Paste blocked in " + name + ": protected copy pending"
		}
		b.notify(Event{Kind: "blocked", RequestID: requestID, Version: version, Message: message, Target: target})
		return 0
	}
	return event
}
