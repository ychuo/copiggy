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
	pageWidth                    int
	loading, committed           bool
	canBack, dark                bool
	failure                      string
	started                      time.Time
	action                       string
	off                          []func()
}

// A page belongs to one saved record and one visible popover host. QR mode
// reuses that session but never creates one: omitting ui.WebView from the frame
// makes MyGo hide its native view without destroying the document or history.
// Keeping this reconciliation independent of native creation also lets tests
// exercise host changes and QR transitions without opening WebKit.
func (s *application) reconcileNativeLinkPreview(item clipmodel.Item, host uintptr) (*nativeLinkPreviewState, bool) {
	link := clipmodel.ParseLink(item.Text)
	if !s.preview || s.quitting || s.hiding || item.Type != clipmodel.TypeLink || link == nil || link.App || host == 0 {
		s.destroyNativeLinkPreview()
		return nil, false
	}
	raw, valid := nativeLinkURL(link.URL)
	if !valid {
		s.destroyNativeLinkPreview()
		return nil, false
	}
	state := s.linkPreview
	if state != nil && (state.itemID != item.ID || state.url != raw || state.host != host || (state.view != nil && state.view.IsDestroyed())) {
		s.destroyNativeLinkPreview()
		state = nil
	}
	if s.semanticState(item).LinkQR {
		return state, false
	}
	if state == nil {
		state = &nativeLinkPreviewState{itemID: item.ID, url: raw, current: raw, target: raw, host: host, loading: true, started: time.Now(), dark: s.nativeState().Dark}
		s.linkPreview = state
		return state, true
	}
	return state, false
}

// No remote requests happen when capturing or selecting a link. This is called
// only after the user opens its popover and AppKit has made its real NSWindow.
func (s *application) syncNativeLinkPreview(item clipmodel.Item) {
	var host uintptr
	if s.popover != nil && s.popover.IsVisible() {
		host = s.popover.NativeHandle()
	}
	state, create := s.reconcileNativeLinkPreview(item, host)
	if state == nil {
		return
	}
	if s.semanticState(item).LinkQR {
		// A hidden page keeps its existing load and policies, including the
		// bounded loading watchdog. QR itself never creates or navigates it.
		if state.view != nil && state.stopTimedOutLoad(time.Now(), state.view.Page().Stop) {
			s.publish()
		}
		return
	}
	// Popover synchronization resizes the host before this point. Retain the
	// last page width while QR is shown so its smaller surface cannot briefly
	// resize the hidden WKWebView when the user switches back to Page.
	if state.pageWidth != s.popoverWidth {
		state.pageWidth = s.popoverWidth
		s.popover.Invalidate()
	}
	if create {
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
		if err := page.LoadURL(state.url); err != nil {
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
	if state.stopTimedOutLoad(time.Now(), page.Stop) {
		s.publish()
	}
}

func (state *nativeLinkPreviewState) stopTimedOutLoad(now time.Time, stop func()) bool {
	if !state.loading || now.Sub(state.started) <= 25*time.Second {
		return false
	}
	stop()
	state.loading = false
	// A committed document may have a slow resource. Preserve its content and
	// navigation whether it is visible or temporarily hidden behind QR.
	if !state.committed {
		state.failure = "Page timed out"
	}
	return true
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
	if item.Type == clipmodel.TypeAppLink {
		s.nativeAppLinkActions(c, item, p)
		return
	}
	state := s.nativeLinkState(item)
	semantic := s.semanticState(item)
	link := clipmodel.ParseLink(item.Text)
	nativeSegmentGroup(c, p).Children(func() {
		for _, option := range []struct {
			title, label string
			qr           bool
		}{{"Page", "Show link", false}, {"QR", "Show QR code", true}} {
			if s.nativeFormatSegment(c, s.tr(option.title), semantic.LinkQR == option.qr, p).
				Label(s.tr(option.label)).Clicked() && semantic.LinkQR != option.qr {
				s.toggleNativeLinkQR(item)
			}
		}
	})
	if semantic.LinkQR {
		return // QR identifies the original destination once, in the copy footer.
	}
	web := link != nil && !link.App && !semantic.LinkQR
	if web && state != nil && state.canBack {
		if s.nativePreviewButton(c, "ArrowLeft", s.tr("Back"), p).Clicked() {
			state.action = "back"
		}
	}
	icon, label, action := "ArrowClockwise", "Reload page", "reload"
	if state != nil && state.loading {
		icon, label, action = "X", "Stop loading", "stop"
	}
	if web && s.nativePreviewButton(c, icon, s.tr(label), p).Clicked() && state != nil {
		state.action = action
	}
	ui.Row(c).Grow(1).MinWidth(0).Gap(nativeToolbarGap).AlignItems(ui.Center).Children(func() {
		var bitmap *ui.Bitmap
		// A previously cached favicon belongs to the copied origin. A redirect
		// to another site gets a neutral globe, never the previous site's logo.
		if s.nativeLinkShowsOriginalOrigin(item) {
			bitmap = s.nativeState().WebsiteIcons[item.ID]
		}
		if link != nil && link.App {
			nativeGlyph(c, "LinkSimple", nativeToolbarIconSize).TextColor(p.icon)
		} else {
			s.nativeLinkSiteIcon(c, bitmap, nativeToolbarIconSize, p)
		}
		nativeContentText(c, s.nativeLinkLocation(item)).FontSize(nativeTextControl).FontWeight(nativeWeightBody).TextColor(p.muted).Grow(1).MinWidth(0).SingleLine()
	})
}

func (s *application) nativeLinkOpenAction(c *ui.Context, item clipmodel.Item, p nativeColors) {
	label := "Open in browser"
	if link := clipmodel.ParseLink(item.Text); link != nil && link.App {
		label = "Open in app"
	}
	if s.nativePreviewButton(c, "ArrowSquareOut", s.tr(label), p).Clicked() {
		s.openNativeLink(item)
	}
}

func (s *application) openNativeLink(item clipmodel.Item) {
	target, app := s.nativeLinkOpenDestination(item.ID)
	if target == "" {
		return
	}
	failure := "Could not open browser"
	if app {
		failure = "No app could open this link"
	}
	s.perform(func() error { return mygo.Shell.OpenExternal(target) }, func(err error) {
		if err != nil {
			s.reportNativeStatus(s.tr(failure))
		}
	})
}

func (s *application) nativeLinkOpenDestination(id string) (target string, app bool) {
	// Derive the destination again from history, never from a UI-supplied URL.
	record, exists := s.store.Record(id)
	if !exists || !nativeLinkItemType(record.Item.Type) {
		return "", false
	}
	link := clipmodel.ParseLink(record.Item.Text)
	if link == nil {
		return "", false
	}
	target = link.URL
	if !link.App {
		original, valid := nativeLinkURL(link.URL)
		if !valid {
			return "", false
		}
		if state := s.nativeLinkState(record.Item); state != nil && state.url == original && state.current != "" && !s.semanticState(record.Item).LinkQR {
			target = state.displayedURL()
		}
		var ok bool
		if target, ok = nativeLinkURL(target); !ok {
			return "", false
		}
	}
	return target, link.App
}

func (s *application) nativeLinkContent(c *ui.Context, item clipmodel.Item, p nativeColors) {
	link := clipmodel.ParseLink(item.Text)
	semantic := s.semanticState(item)
	if semantic.LinkQR {
		s.nativeLinkQRContent(c, item, semantic.QRCode, p)
		return
	}
	if link != nil && link.App {
		s.nativeAppLinkContent(c, item, link, p)
		return
	}
	state := s.nativeLinkState(item)
	valid := false
	if link != nil {
		_, valid = nativeLinkURL(link.URL)
	}
	failure := ""
	if !valid {
		failure = "Web preview supports HTTP and HTTPS links"
	} else if state != nil {
		failure = state.failure
	}
	width, _ := c.Size()
	// Page is wider than QR. Wait for the popover's next geometry sync before
	// placing the retained WebView, avoiding a brief narrow-page reflow.
	waitingForSize := state != nil && state.pageWidth > 0 && int(width) != state.pageWidth
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Children(func() {
		if waitingForSize {
			ui.Box(c).Fill().Background(nativeLinkColors().paper)
			return
		}
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
	link := clipmodel.ParseLink(item.Text)
	if link == nil {
		return ""
	}
	if link.App {
		return link.Scheme + "://"
	}
	raw := link.URL
	if state := s.nativeLinkState(item); state != nil && state.current != "" {
		raw = state.displayedURL()
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return ""
}

func (s *application) nativeLinkShowsOriginalOrigin(item clipmodel.Item) bool {
	link := clipmodel.ParseLink(item.Text)
	if link == nil || link.App || link.Inferred {
		return false
	}
	original, err := url.Parse(link.URL)
	if err != nil || original.Hostname() == "" {
		return false
	}
	raw := link.URL
	if state := s.nativeLinkState(item); state != nil {
		raw = state.displayedURL()
	}
	current, err := url.Parse(raw)
	return err == nil && strings.EqualFold(original.Scheme, current.Scheme) && strings.EqualFold(original.Host, current.Host)
}
