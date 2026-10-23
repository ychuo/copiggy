// Package nativeui renders the native panel, popover, menus and their UI session.
// Business records and durable mutations are provided by application services.
package nativeui

import (
	"context"
	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	macos "copiggy/internal/macos/appkit"
	"copiggy/internal/macos/guard"
	"copiggy/internal/settings"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"log"
	"strings"
	"time"
)

const summonShortcut = "Ctrl+Alt+Shift+Space"

type GuardState interface {
	Enabled() bool
	CurrentPolicy() guard.Policy
	ChangeCount() int64
	SetPanelVisible(bool)
}

// Config connects presentation to trusted services. Dispatch serializes commands
// away from AppKit and delivers completion on the main thread.
type Config struct {
	History         *clipboard.History
	Guard           GuardState
	Preferences     settings.Preferences
	SystemRegion    string
	Dispatch        func(func() error, func(error))
	SavePreferences func(settings.Preferences) error
	Copy            func(context.Context, string, bool) (clipmodel.Item, error)
	CopyVariant     func(context.Context, string, clipmodel.CopyFormat) (clipmodel.Item, error)
	SetGuard        func(bool) error
	SetMonitoring   func(bool)
	Refresh         func()
}

// Controller is owned by app lifecycle; all its methods run on the main thread.
type Controller = application

func New(config Config) *Controller {
	_ = loadNativeFonts()
	s := &application{config: config, native: config.Guard, store: config.History,
		appNames: map[string]string{}, itemSources: map[string]string{}, appIcons: map[string][]byte{}}
	s.applyPreferences(config.Preferences)
	s.Changed()
	s.ready()
	return s
}
func (s *application) perform(work func() error, done func(error)) {
	if s.quitting {
		return
	}
	if s.config.Dispatch != nil {
		s.config.Dispatch(work, done)
	} else {
		done(work())
	}
}
func (s *application) Show() { s.show() }
func (s *application) Changed() {
	if s.quitting {
		return
	}
	s.pruneUnavailableNativeContentAssets()
	selected, hadSelection := s.selected()
	if hadSelection {
		if _, exists := s.store.Record(selected.ID); !exists {
			s.closeNativePreview()
		}
	}
	for _, item := range s.store.Items() {
		s.rememberSource(item)
	}
	s.filter()
	s.pruneHistory()
	s.publish()
}
func (s *application) Report(operation string, err error, fallback string) {
	s.reportNativeError(operation, err, fallback)
}
func (s *application) Tick() {
	if s.quitting {
		return
	}
	s.reconcileNativeFocus()
	s.expireCopyFeedback()
	s.syncNativeTray(time.Now())
	s.syncNativePopover()
}
func (s *application) Event(event guard.Event) {
	if event.Kind == "copy-pending" {
		return
	}
	if (event.Kind == "enabled" && !s.native.Enabled()) || ((event.Kind == "stopped" || strings.HasPrefix(event.Kind, "disabled-")) && s.native.Enabled()) {
		return
	}
	if event.Kind == "copy-protected" && (!s.native.Enabled() || s.native.ChangeCount() != event.Version) {
		return
	}
	s.copyFeedback = nil
	if event.Kind == "copy-protected" {
		policy := s.native.CurrentPolicy()
		if policy.Version != event.Version || len(policy.AllowedApps) == 0 {
			return
		}
		s.copyFeedback = &policy
	}
	s.reportNativeEvent(event)
	s.publish()
}
func (s *application) BeginQuit() {
	s.quitting = true
	s.cancelNativeDraft()
	s.cancelNativeContentAssets()
}
func (s *application) Close() {
	s.BeginQuit()
	if s.viewState != nil && s.viewState.LinkIcons != nil {
		s.viewState.LinkIcons.Clear()
	}
	s.destroyNativeLinkPreview()
	if s.popover != nil {
		s.popover.Destroy()
	}
	if s.popoverAppearance != nil {
		s.popoverAppearance.Close()
		s.popoverAppearance = nil
	}
	if s.panel != nil {
		_ = macos.ClearTrayGlass(s.panel.NativeHandle())
	}
	mygo.GlobalShortcut.Unregister(summonShortcut)
	if s.tray != nil {
		s.tray.Destroy()
	}
}

type application struct {
	preferenceGeneration        uint64
	config                      Config
	commandBusy                 bool
	native                      GuardState
	store                       *clipboard.History
	panel                       *mygo.Window
	tray                        *mygo.Tray
	search                      ui.Handle
	query                       string
	language                    string
	themePreference             string
	phoneRegion                 string
	retentionDays               int
	guardPreference             bool
	results                     []clipmodel.Item
	chosen                      int
	preview                     bool
	draft                       *clipmodel.Draft
	busy                        bool
	cancel                      context.CancelFunc
	message                     string
	monitoring                  bool
	quitting                    bool
	appNames                    map[string]string
	trayStatus                  trayStatusState
	copyFeedback                *guard.Policy
	viewState                   *nativePanelState
	popover                     *mygo.Window
	linkPreview                 *nativeLinkPreviewState
	contentAssets               *nativeContentAssetsState
	semanticPreview             nativeSemanticPreviewState
	popoverAnchor               mygo.Rectangle
	popoverWidth, popoverHeight int
	popoverAppearance           *macos.PopoverAppearance
	popoverDark                 bool
	glassMaterial               string
	glassError                  string
	focusGeneration             uint64
	hiding                      bool
	uiGeneration                uint64
	itemSources                 map[string]string
	appIcons                    map[string][]byte
}

func (s *application) ready() {
	s.panel = mygo.NewWindow(mygo.WindowOptions{
		Title: "Copiggy", Width: 1180, Height: nativeTrayHeight,
		Panel: true, Hidden: true, Frameless: true, DisableResize: true,
		UseContentSize: true, Transparent: true, Content: ui.View(s.nativeView),
	})
	if err := macos.MakeTrayBorderless(s.panel.NativeHandle()); err != nil {
		log.Printf("native tray frame: %v", err)
	}
	s.panel.SetHasShadow(true)
	s.syncNativeGlass()
	s.panel.OnClose(func(e *mygo.CloseEvent) {
		if !s.quitting {
			e.PreventDefault()
			s.hide()
		} else {
			// Restore the native surface before MyGo destroys its NSWindow.
			_ = macos.ClearTrayGlass(s.panel.NativeHandle())
		}
	})
	s.panel.OnFocus(s.nativeWindowFocused)
	s.panel.OnBlur(s.nativeWindowBlurred)
	mygo.App.OnActivate(func(_ bool) {
		if !s.quitting && !s.panel.IsVisible() {
			s.show()
		}
	})
	mygo.App.OnDidResignActive(func() { s.native.SetPanelVisible(false) })
	var err error
	s.tray, err = mygo.NewTray(mygo.TrayOptions{
		ID: "dev.copiggy", Icon: clipboardTrayIcon(false), IconIsTemplate: true,
	})
	if err != nil {
		log.Printf("tray unavailable: %v", err)
	} else {
		s.tray.OnClick(s.toggle)
		s.tray.OnRightClick(s.showNativeTrayMenu)
		s.syncNativeTray(time.Now())
	}
	if err := mygo.GlobalShortcut.Register(summonShortcut, s.toggle); err != nil {
		s.reportNativeError("register panel shortcut", err, "Shortcut unavailable")
	}
}

func (s *application) show() {
	if s.quitting {
		return
	}
	s.native.SetPanelVisible(true)
	if s.config.Refresh != nil {
		s.config.Refresh()
	}
	display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	s.panel.SetSize(nativeTrayWidth(display.WorkArea.Width), min(nativeTrayHeight, display.WorkArea.Height-48))
	w, h := s.panel.Size()
	y := display.WorkArea.Y + display.WorkArea.Height - h - 28
	s.panel.SetPosition(display.WorkArea.X+(display.WorkArea.Width-w)/2, y)
	if s.viewState != nil {
		s.viewState.Focus.Focus()
	}
	s.panel.Show()
	s.publish()
}

func (s *application) hide() {
	if s.hiding {
		return
	}
	s.hiding = true
	defer func() { s.hiding = false }()
	s.focusGeneration++
	s.cancelNativeDraft()
	s.cancelNativeContentAssets()
	if s.viewState != nil {
		s.viewState.EmptyAnimationStart = time.Time{}
		s.viewState.PinAnimationStart = time.Time{}
	}
	s.preview = false
	s.destroyNativeLinkPreview()
	if s.popover != nil {
		s.popover.Hide()
	}
	if s.panel != nil {
		s.panel.Hide()
	}
	if s.native != nil {
		s.native.SetPanelVisible(false)
	}
}

func (s *application) toggle() {
	if s.panel.IsVisible() {
		s.hide()
	} else {
		s.show()
	}
}

func (s *application) filter() {
	id := ""
	if len(s.results) > 0 && s.chosen >= 0 && s.chosen < len(s.results) {
		id = s.results[s.chosen].ID
	}
	s.results = s.store.Search(s.query)
	if s.viewState != nil && s.viewState.PinnedOnly {
		filtered := s.results[:0]
		for _, item := range s.results {
			if item.Pinned {
				filtered = append(filtered, item)
			}
		}
		s.results = filtered
	}
	s.chosen = 0
	for i, item := range s.results {
		if item.ID == id {
			s.chosen = i
			break
		}
	}
}

func (s *application) selected() (clipmodel.Item, bool) {
	if len(s.results) == 0 || s.chosen < 0 || s.chosen >= len(s.results) {
		return clipmodel.Item{}, false
	}
	return s.results[s.chosen], true
}

func (s *application) choose() {
	if s.quitting {
		return
	}
	if s.commandBusy {
		return
	}
	id, isDraft := "", false
	var format clipmodel.CopyFormat
	if s.draft != nil {
		id, isDraft = s.draft.ID, true
	} else if item, ok := s.selected(); ok {
		id = item.ID
		if s.preview && s.semanticPreview.ItemID == id {
			format = s.semanticPreview.CopyFormat
		}
	} else {
		return
	}
	if format != "" && s.config.CopyVariant == nil || format == "" && s.config.Copy == nil {
		return
	}
	generation := s.uiGeneration
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.commandBusy = true
	s.perform(func() error {
		if format != "" {
			_, err := s.config.CopyVariant(ctx, id, format)
			return err
		}
		_, err := s.config.Copy(ctx, id, isDraft)
		return err
	}, func(err error) {
		s.commandBusy = false
		cancel()
		if generation != s.uiGeneration {
			s.Changed()
			return
		}
		s.cancel = nil
		if err != nil {
			s.reportNativeError("copy history item", err, "Could not copy")
			s.publish()
			return
		}
		s.draft = nil
		s.Changed()
		s.hide()
	})
}
func (s *application) enhance(kind clipmodel.EnhancementKind) {
	if s.quitting {
		return
	}
	item, ok := s.selected()
	if !ok {
		return
	}
	s.cancelNativeDraft()
	s.preview = true
	if s.viewState != nil {
		s.viewState.PreviewFocus.Focus()
	}
	s.busy = true
	generation := s.uiGeneration
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.message = ""
	var draft clipmodel.Draft
	s.perform(func() error {
		var err error
		draft, err = s.store.Enhance(ctx, item.ID, kind)
		return err
	}, func(err error) {
		defer s.publish()
		if generation != s.uiGeneration {
			if draft.ID != "" {
				s.perform(func() error { s.store.Cancel(draft.ID); return nil }, func(error) {})
			}
			cancel()
			return
		}
		s.busy = false
		s.cancel = nil
		canceled := ctx.Err()
		cancel()
		if canceled != nil {
			if draft.ID != "" {
				s.perform(func() error { s.store.Cancel(draft.ID); return nil }, func(error) {})
			}
			return
		}
		if err != nil {
			s.reportNativeError("enhance content", err, "Could not enhance content")
			return
		}
		s.draft = &draft
		s.message = ""
	})
}

func (s *application) scopeLabel(apps []string) string {
	names := make([]string, 0, len(apps))
	for _, id := range apps {
		name := s.appNames[id]
		if name == "" {
			name = id
		}
		names = append(names, name)
	}
	return s.tr("Only in %s", strings.Join(names, ", "))
}

func nativeTrayWidth(available int) int {
	return max(1, min(available-32, max(360, int(float64(available)*.8))))
}
