package nativeui

import (
	"net/url"
	"strings"
	"time"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Only the selected link's page uses WebKit. The window, toolbar, loading/error
// states and every other clipboard preview remain MyGo Native UI. MyGo does not
// expose WKWebsiteDataStore configuration, so this uses its normal website store.
type nativeLinkPreviewState struct {
	view                         *mygo.WebView
	itemID, url, current, target string
	host                         uintptr
	loading, committed           bool
	canBack, dark                bool
	failure                      string
	started                      time.Time
	action                       string
	off                          []func()
}

// No remote requests happen when capturing or selecting a link. This is called
// only after the user opens its popover and AppKit has made its real NSWindow.
func (s *application) syncNativeLinkPreview(item clipmodel.Item) {
	if !s.preview || item.Type != clipmodel.TypeLink || s.popover == nil || !s.popover.IsVisible() {
		s.destroyNativeLinkPreview()
		return
	}
	host := s.popover.NativeHandle()
	if host == 0 {
		s.destroyNativeLinkPreview()
		return
	}
	raw, valid := nativeLinkURL(item.Text)
	if !valid {
		s.destroyNativeLinkPreview()
		return
	}
	state := s.linkPreview
	if state == nil || state.itemID != item.ID || state.url != raw || state.host != host || (state.view != nil && state.view.IsDestroyed()) {
		s.destroyNativeLinkPreview()
		state = &nativeLinkPreviewState{itemID: item.ID, url: raw, current: raw, target: raw, host: host, loading: true, started: time.Now(), dark: s.nativeState().Dark}
		s.linkPreview = state
		// Leave URL empty until all policies and callbacks are installed.
		view, err := s.popover.NewWebView(mygo.WebViewOptions{
			Page:            mygo.PageOptions{DevTools: mygo.DevToolsDisabled},
			BackgroundColor: nativeLinkBackground(state.dark),
		})
		if err != nil {
			state.loading, state.failure = false, "Page unavailable"
			s.publish()
			return
		}
		state.view = view
		page := view.Page()
		page.SetPermissionHandler(func(mygo.PermissionRequest) bool { return false })
		page.SetWindowOpenHandler(func(mygo.WindowOpenRequest) *mygo.WindowOptions {
			// A preview never launches new windows or external applications on a
			// site's behalf. The native toolbar provides an explicit browser action.
			return nil
		})
		state.off = append(state.off,
			page.OnWillDownload(func(e *mygo.DownloadEvent) {
				e.PreventDefault()
				if s.linkPreview == state {
					if checked, ok := nativeLinkURL(e.URL); ok {
						state.target = checked
					}
					state.loading, state.failure = false, "Downloads open in browser"
					s.publish()
				}
			}),
			page.OnWillNavigate(func(e *mygo.NavigateEvent) {
				checked, allowed := nativeLinkURL(e.URL)
				if !allowed {
					e.PreventDefault()
					return
				}
				if s.linkPreview == state {
					state.target = checked
					state.loading, state.failure, state.started = true, "", time.Now()
					s.publish()
				}
			}),
			page.OnDidNavigate(func(raw string) {
				if s.linkPreview != state {
					return
				}
				if checked, ok := nativeLinkURL(raw); ok {
					state.current, state.target = checked, checked
				}
				state.committed, state.loading, state.failure = true, true, ""
				state.started = time.Now()
				s.publish()
			}),
			page.OnDidFinishLoad(func() {
				if s.linkPreview == state {
					state.committed, state.loading, state.failure = true, false, ""
					s.publish()
				}
			}),
			page.OnDidFailLoad(func(e *mygo.LoadError) {
				// WebKit reports user stop, replacement navigation and dismissal as
				// NSURLErrorCancelled. None is a network failure.
				if s.linkPreview == state && e.Code != -999 {
					if checked, ok := nativeLinkURL(e.URL); ok {
						state.target = checked
					}
					state.loading, state.failure = false, "Page unavailable"
					s.publish()
				}
			}),
			page.OnRenderProcessGone(func(string) {
				if s.linkPreview == state {
					state.loading, state.failure = false, "Page unavailable"
					s.publish()
				}
			}),
		)
		if err := page.LoadURL(raw); err != nil {
			state.loading, state.failure = false, "Page unavailable"
		}
		s.publish()
		return
	}
	if (state.action == "retry" || state.action == "reload") && state.view == nil {
		// Retry creation as well as navigation when the native engine was not
		// available for the first attempt.
		s.destroyNativeLinkPreview()
		s.syncNativeLinkPreview(item)
		return
	}
	if state.view == nil {
		return
	}
	page := state.view.Page()
	if state.dark != s.nativeState().Dark {
		state.dark = s.nativeState().Dark
		_ = state.view.SetBackgroundColor(nativeLinkBackground(state.dark))
	}
	if action := state.action; action != "" {
		state.action = ""
		switch action {
		case "stop":
			page.Stop()
			state.loading = false
			if !state.committed {
				state.failure = "Page unavailable"
			}
		case "retry", "reload":
			state.loading, state.failure, state.started = true, "", time.Now()
			if action == "retry" || !state.committed {
				if err := page.LoadURL(state.retryURL()); err != nil {
					state.loading, state.failure = false, "Page unavailable"
				}
			} else {
				page.Reload()
			}
		case "back":
			if page.CanGoBack() {
				state.loading, state.failure, state.started = true, "", time.Now()
				page.GoBack()
			}
		}
		s.publish()
	}
	if back := page.CanGoBack(); back != state.canBack {
		state.canBack = back
		s.popover.Invalidate()
	}
	if state.loading && time.Since(state.started) > 25*time.Second {
		page.Stop()
		state.loading = false
		// A visible document may have a slow analytics resource. Keep its
		// readable content instead of replacing it with an error screen.
		if !state.committed {
			state.failure = "Page timed out"
		}
		s.publish()
	}
}

func (s *application) destroyNativeLinkPreview() {
	state := s.linkPreview
	s.linkPreview = nil // late delegate callbacks must not update the next item
	if state == nil {
		return
	}
	for _, off := range state.off {
		off()
	}
	if state.view != nil && !state.view.IsDestroyed() {
		state.view.Destroy()
	}
}

func nativeLinkURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.Opaque != "" || u.User != nil {
		return "", false
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	return u.String(), true
}

func nativeLinkBackground(dark bool) string {
	if dark {
		return "#23272b"
	}
	return "#f7f8fa"
}

func (s *application) nativeLinkState(item clipmodel.Item) *nativeLinkPreviewState {
	if state := s.linkPreview; state != nil && state.itemID == item.ID {
		return state
	}
	return nil
}

func (state *nativeLinkPreviewState) retryURL() string {
	if checked, ok := nativeLinkURL(state.target); ok {
		return checked
	}
	return state.url
}

func (state *nativeLinkPreviewState) displayedURL() string {
	if state.failure != "" || state.loading {
		return state.retryURL()
	}
	if state.current != "" {
		return state.current
	}
	return state.url
}

func (s *application) nativeLinkActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.nativeLinkState(item)
	if state != nil && state.canBack {
		if s.nativePreviewButton(c, "ArrowLeft", s.tr("Back"), p).Clicked() {
			state.action = "back"
		}
	}
	icon, label, action := "ArrowClockwise", "Reload page", "reload"
	if state != nil && state.loading {
		icon, label, action = "X", "Stop loading", "stop"
	}
	if s.nativePreviewButton(c, icon, s.tr(label), p).Clicked() && state != nil {
		state.action = action
	}
	ui.Row(c).Grow(1).MinWidth(0).Gap(nativeToolbarGap).AlignItems(ui.Center).Children(func() {
		var bitmap *ui.Bitmap
		// A previously cached favicon belongs to the copied origin. A redirect
		// to another site gets a neutral globe, never the previous site's logo.
		if s.nativeLinkShowsOriginalOrigin(item) {
			bitmap = s.nativeState().WebsiteIcons[item.ID]
		}
		s.nativeLinkSiteIcon(c, bitmap, nativeToolbarIconSize, p)
		nativeContentText(c, s.nativeLinkLocation(item)).FontSize(nativeTextControl).FontWeight(nativeWeightBody).TextColor(p.muted).Grow(1).MinWidth(0).SingleLine()
	})
}

func (s *application) nativeLinkOpenAction(c *ui.Context, item clipmodel.Item, p nativeColors) {
	if s.nativePreviewButton(c, "ArrowSquareOut", s.tr("Open in browser"), p).Clicked() {
		s.openNativeLink(item)
	}
}

func (s *application) openNativeLink(item clipmodel.Item) {
	raw := item.Text
	if state := s.nativeLinkState(item); state != nil && state.current != "" {
		raw = state.displayedURL()
	}
	if checked, ok := nativeLinkURL(raw); ok {
		go func() {
			if err := mygo.Shell.OpenExternal(checked); err != nil {
				mygo.RunOnMain(func() { s.reportNativeStatus(s.tr("Could not open browser")) })
			}
		}()
	}
}

func (s *application) nativeLinkContent(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.nativeLinkState(item)
	_, valid := nativeLinkURL(item.Text)
	failure := ""
	if !valid {
		failure = "Web preview supports HTTP and HTTPS links"
	} else if state != nil {
		failure = state.failure
	}
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Children(func() {
		if state != nil && state.view != nil && failure == "" {
			ui.WebView(c, state.view).Grow(1).MinHeight(0).Radius(nativePreviewPaperRadius).Label(s.tr("Web page"))
		}
		if failure != "" || state == nil || !state.committed {
			ink := nativeLinkColors()
			overlay := ui.Column(c).Fill().MinHeight(0).Padding(24).Gap(14).AlignItems(ui.Center).Justify(ui.Center).Background(ink.paper)
			if state != nil && state.view != nil && failure == "" {
				overlay.Absolute().Top(0).Left(0).Right(0).Bottom(0)
			}
			overlay.Children(func() {
				nativeGlyph(c, "Globe", 25).TextColor(ink.icon)
				label := "Loading page…"
				if failure != "" {
					label = failure
				}
				ui.Text(c, s.tr(label)).FontSize(nativeTextBase).FontWeight(nativeWeightBody).TextColor(ink.muted).Wrap().TextAlign(ui.Center).Role(ui.RoleStatus)
				if failure != "" && valid {
					if s.nativeActionButton(c, "ArrowClockwise", s.tr("Retry"), ink).Clicked() && state != nil {
						state.action = "retry"
					}
				}
			})
		}
	})
}

func (s *application) nativeLinkLocation(item clipmodel.Item) string {
	raw := item.Text
	if state := s.nativeLinkState(item); state != nil && state.current != "" {
		raw = state.displayedURL()
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return ""
}

func (s *application) nativeLinkShowsOriginalOrigin(item clipmodel.Item) bool {
	original, err := url.Parse(strings.TrimSpace(item.Text))
	if err != nil || original.Hostname() == "" {
		return false
	}
	raw := item.Text
	if state := s.nativeLinkState(item); state != nil {
		raw = state.displayedURL()
	}
	current, err := url.Parse(raw)
	return err == nil && strings.EqualFold(original.Scheme, current.Scheme) && strings.EqualFold(original.Host, current.Host)
}
