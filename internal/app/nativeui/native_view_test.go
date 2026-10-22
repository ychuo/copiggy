package nativeui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"slices"
	"testing"
	"time"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"copiggy/internal/macos/guard"
	"github.com/egoist/mygo/ui"
)

func nativeViewFixture(t *testing.T) *application {
	t.Helper()
	s := &application{language: "zh-Hans", native: &guard.Bridge{}, store: clipboard.NewHistory(nil, 30), monitoring: true, appNames: map[string]string{"com.apple.TextEdit": "文本编辑"}, appIcons: map[string][]byte{"com.apple.TextEdit": nil}, itemSources: map[string]string{}, viewState: &nativePanelState{Images: map[string]*ui.Bitmap{}, Icons: map[string]*ui.Bitmap{}, CardBounds: map[string]ui.Rect{}}}
	nativeCaptureText(t, s, "周五前确认第一版交互方案", "备忘录")
	nativeCaptureText(t, s, "https://github.com/egoist/mygo", "Safari")
	nativeCaptureText(t, s, "让每一次复制，都更从容。", "备忘录")
	item := nativeCapture(t, s, clipmodel.TextData(`{"project":"Copiggy","theme":"glass","version":1}`), clipmodel.Source{Name: "文本编辑", BundleID: "com.apple.TextEdit"}, "com.apple.TextEdit")
	s.itemSources[item.ID] = "com.apple.TextEdit"
	s.filter()
	return s
}
func TestNativePanelKeyboardAndCardGeometry(t *testing.T) {
	s := nativeViewFixture(t)
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	_, _, _, alpha := tt.Image().At(700, 10).RGBA()
	if alpha != 0 {
		t.Fatal("native UI painted over the transparent desktop area")
	}
	current, _ := s.store.Current()
	first := s.results[0]
	if err := tt.Click(s.nativeCardLabel(first)); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeySpace)
	if !s.preview {
		t.Fatal("Space on a selected card did not open native preview")
	}
	tt.Key(0, ui.KeyRight)
	if s.chosen != 1 || !s.preview {
		t.Fatal("right arrow did not keep preview open on the next item")
	}
	tt.Key(0, ui.KeyEscape)
	if s.preview {
		t.Fatal("Escape did not close preview")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	if !s.viewState.SearchOpen {
		t.Fatal("Cmd-F did not expand search")
	}
	tt.Type("hello world")
	if s.query != "hello world" {
		t.Fatalf("search lost whitespace: %q", s.query)
	}
	if s.preview {
		t.Fatal("typing a space in search opened preview")
	}
	tt.Key(0, ui.KeyEscape)
	if s.query != "" {
		t.Fatal("Escape did not clear search")
	}
	tt.Key(0, ui.KeyEscape)
	if s.viewState.SearchOpen {
		t.Fatal("second Escape did not collapse search")
	}
	if after, ok := s.store.Current(); !ok || after.ID != current.ID {
		t.Fatal("browsing or searching changed the current clipboard item")
	}
	for _, bounds := range s.viewState.CardBounds {
		if bounds.W > 0 && (bounds.W != nativeCardSize || bounds.H != nativeCardSize) {
			t.Fatalf("not square: %+v", bounds)
		}
	}
}

func TestNativeBlankPanelClickKeepsSelectedCardPreview(t *testing.T) {
	for _, point := range []ui.Point{{X: 600, Y: 25}, {X: 1100, Y: 150}, {X: 600, Y: 275}} {
		t.Run(fmt.Sprintf("%.0f-%.0f", point.X, point.Y), func(t *testing.T) {
			s := nativeViewFixture(t)
			tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
			selected := s.results[s.chosen]
			if err := tt.Click(s.nativeCardLabel(selected)); err != nil {
				t.Fatal(err)
			}
			tt.ClickAt(point.X, point.Y)
			if current, ok := s.selected(); !ok || current.ID != selected.ID {
				t.Fatal("blank area click changed the visible selection")
			}
			tt.Key(0, ui.KeySpace)
			if !s.preview {
				t.Fatal("Space did not preview the selected card after clicking panel whitespace")
			}
		})
	}
}

func TestNativePreviewShortcutWithoutFocusPreservesInputAndControls(t *testing.T) {
	s := nativeViewFixture(t)
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	current, _ := s.store.Current()
	tt.Key(0, ui.KeySpace)
	if !s.preview {
		t.Fatal("initial selected card could not be previewed")
	}
	tt.Key(0, ui.KeyEscape)
	tt.SetFocused(false)
	tt.SetFocused(true)
	tt.ClickAt(1100, 150)
	tt.Key(0, ui.KeySpace)
	if !s.preview {
		t.Fatal("selected card could not be previewed after the window regained focus")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("hello")
	tt.TypeKey(0, ui.KeySpace, " ")
	tt.Type("world")
	if s.preview || s.query != "hello world" {
		t.Fatalf("preview shortcut took a search space: preview=%v query=%q", s.preview, s.query)
	}
	if after, ok := s.store.Current(); !ok || after.ID != current.ID {
		t.Fatal("previewing changed current clipboard history")
	}

	controls := nativeViewFixture(t)
	keyboard := ui.NewTester(controls.nativeView, 1140, nativeTrayHeight)
	keyboard.Key(0, ui.KeyTab)
	if !keyboard.Focused(controls.tr("Search · ⌘F")) {
		t.Fatal("first tab stop was not the search control")
	}
	keyboard.Key(0, ui.KeySpace)
	if !controls.viewState.SearchOpen || controls.preview {
		t.Fatal("preview shortcut stole keyboard activation from the search control")
	}

	empty := nativeViewFixture(t)
	empty.store = clipboard.NewHistory(nil, 30)
	empty.filter()
	noItems := ui.NewTester(empty.nativeView, 1140, nativeTrayHeight)
	noItems.ClickAt(1100, 150)
	noItems.Key(0, ui.KeySpace)
	if empty.preview {
		t.Fatal("empty history opened a preview")
	}
}

func TestNativeLinkFooterAlignsWithOtherCards(t *testing.T) {
	s := nativeViewFixture(t)
	s.language = "en"
	link := nativeCaptureText(t, s, "https://zustand.docs.pmnd.rs/learn/guides/advanced-typescript", "Browser")
	if err := s.store.SetPinned(context.Background(), link.ID, true); err != nil {
		t.Fatal(err)
	}
	s.filter()
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	footer, ok := tt.Find("zustand.docs.pmnd.rs")
	if !ok {
		t.Fatal("link footer missing")
	}
	original, ok := tt.Find("3 properties")
	if !ok {
		t.Fatal("JSON footer missing")
	}
	if footer.Y != original.Y || footer.H != original.H {
		t.Fatalf("link footer does not align with JSON footer: link=%+v json=%+v", footer, original)
	}
	title, ok := tt.Find(nativeLinkCardTitle(link, ""))
	if !ok || title.Y+title.H > footer.Y-4 {
		t.Fatalf("link title crowds its footer: title=%+v footer=%+v", title, footer)
	}
	pin, ok := tt.Find("Unpin")
	card, cardFound := tt.Find(s.nativeCardLabel(link))
	if !ok || !cardFound || pin.Y+pin.H > card.Y+card.H-12 {
		t.Fatalf("link pin crowds the lower edge: pin=%+v card=%+v", pin, card)
	}
}

func TestNativePinFilterAndDeleteShortcuts(t *testing.T) {
	s := nativeViewFixture(t)
	s.language = "en"
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	item := s.results[0]
	if err := tt.Click(s.nativeCardLabel(item)); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyP)
	if len(s.store.Pinned()) != 1 {
		t.Fatal("Cmd-P did not pin the selected record")
	}
	if err := tt.Click("Pinned"); err != nil {
		t.Fatal(err)
	}
	if !s.viewState.PinnedOnly || len(s.results) != 1 || s.results[0].ID != item.ID {
		t.Fatal("pinned entry did not filter history")
	}
	current, _ := s.store.Current()
	if current.ID != item.ID {
		t.Fatal("pinning changed current clipboard")
	}
	tt.Key(ui.Cmd, ui.KeyBackspace)
	if len(s.store.Pinned()) != 0 || len(s.results) != 0 || !tt.HasText("No pinned items") {
		t.Fatal("Cmd-Backspace did not delete pinned record")
	}
	if err := tt.Click("All history"); err != nil {
		t.Fatal(err)
	}
	if len(s.results) != 3 {
		t.Fatal("deletion affected unrelated history")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("Copiggy")
	tt.Key(ui.Cmd, ui.KeyBackspace)
	if len(s.store.Items()) != 3 {
		t.Fatal("search text deletion deleted a history record")
	}
}

func TestNativeUnpinActivePreviewDiscardsItsDraft(t *testing.T) {
	s := nativeViewFixture(t)
	item := s.results[0]
	s.setNativePinned(item.ID, true)
	s.viewState.PinnedOnly = true
	s.filter()
	draft, err := s.store.Enhance(context.Background(), item.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	s.draft, s.preview = &draft, true
	s.setNativePinned(item.ID, false)
	if s.draft != nil || s.preview || len(s.results) != 0 {
		t.Fatal("unpinning left a draft bound to an absent selection")
	}
}

func TestNativeCompactWidthAndScrollMargins(t *testing.T) {
	if nativeTrayWidth(1800) != 1440 || nativeTrayWidth(1200) != 960 {
		t.Fatal("tray width is not 80 percent of the work area")
	}
	s := nativeViewFixture(t)
	for i := 0; i < 12; i++ {
		nativeCaptureText(t, s, fmt.Sprintf("edge-%d", i), "Notes")
	}
	s.filter()
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	// Scroll through the middle. Neither side may keep an empty gutter:
	// partially visible cards must paint right up to the viewport edges.
	tt.Scroll(400, 150, 260, 0)
	for _, x := range []int{5, 1134} {
		if alpha := tt.Image().RGBAAt(x, 150).A; alpha != 255 {
			t.Fatalf("fixed viewport gutter at x=%d (alpha=%d)", x, alpha)
		}
	}
	for i := 0; i < len(s.results)-1; i++ {
		tt.Key(0, ui.KeyRight)
	}
	tt.Advance(250 * time.Millisecond)
	last := s.viewState.CardBounds[s.results[len(s.results)-1].ID]
	if last.X+last.W > 1140-nativeTrayInset || last.X+last.W < 1140-nativeTrayInset-4 {
		t.Fatalf("right edge has no inset: %+v", last)
	}
	for i := 0; i < len(s.results)-1; i++ {
		tt.Key(0, ui.KeyLeft)
	}
	tt.Advance(250 * time.Millisecond)
	first := s.viewState.CardBounds[s.results[0].ID]
	if first.X < nativeTrayInset || first.X > nativeTrayInset+4 {
		t.Fatalf("left edge has no inset: %+v", first)
	}
}

func TestNativeSearchWidthAnimatesAndRespectsReducedMotion(t *testing.T) {
	s := nativeViewFixture(t)
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Advance(70 * time.Millisecond)
	mid, ok := tt.Find(s.tr("Search clipboard history"))
	if !ok || mid.W <= 0 || mid.W >= 280 {
		t.Fatalf("search jumped instead of expanding: %+v", mid)
	}
	tt.Advance(180 * time.Millisecond)
	full, _ := tt.Find(s.tr("Search clipboard history"))
	if full.W <= mid.W {
		t.Fatal("search did not finish expanding")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Cmd, ui.KeyF)
	instant, _ := tt.Find(s.tr("Search clipboard history"))
	if instant.W != full.W {
		t.Fatal("search ignored reduce motion")
	}
}

func TestNativeTrayStaysTransparentAndKeepsHeaderDuringPreview(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s := nativeViewFixture(t)
		s.viewState.Dark = dark
		tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
		if err := tt.Click(s.nativeCardLabel(s.results[0])); err != nil {
			t.Fatal(err)
		}
		tt.Key(0, ui.KeySpace)
		s.message = "剪贴板已变化，请重新确认内容"
		tt.Frame()
		for _, label := range []string{"搜索 · ⌘F", "防误贴"} {
			if _, ok := tt.Find(label); !ok {
				t.Fatalf("parent lost %q while preview is open", label)
			}
		}
		for _, label := range []string{"预览 · 空格 / ⌘Y", "内容预览", "格式化 JSON", "放大图片"} {
			if _, ok := tt.Find(label); ok {
				t.Fatalf("parent rendered popover-only content %q", label)
			}
		}
		// AppKit supplies the backdrop; no painted glass or edge belongs to the
		// MyGo tray root, including when the independent preview is open.
		for _, point := range []image.Point{{0, 0}, {12, 150}, {1100, 70}, {1100, nativeTrayHeight - 1}} {
			if _, _, _, alpha := tt.Image().At(point.X, point.Y).RGBA(); alpha != 0 {
				t.Fatalf("dark=%v: tray paints over system material at %v", dark, point)
			}
		}
		if _, ok := tt.Find(s.message); ok {
			t.Fatal("transient status must stay in the menu bar")
		}
		search, _ := tt.Find("搜索 · ⌘F")
		count, _ := tt.Find(fmt.Sprintf("%d 条", len(s.results)))
		guard, _ := tt.Find("防误贴")
		if search.H != 30 || search.Y+search.H > 53 || count.Y+count.H > 53 || guard.H != 24 {
			t.Fatalf("controls no longer fit the compact tray: search=%+v count=%+v guard=%+v", search, count, guard)
		}
		for _, label := range []string{"暂停记录", "切换深色外观", "切换浅色外观"} {
			if _, ok := tt.Find(label); ok {
				t.Fatalf("menu setting %q still appears in panel", label)
			}
		}
	}
}

func TestNativeToolbarPreservesHistoryNavigationWithoutClosingPreview(t *testing.T) {
	s := nativeViewFixture(t)
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	if err := tt.Click(s.nativeCardLabel(s.results[0])); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"深色", "浅色"} {
		s.applyNativeTheme(label == "深色")
		tt.Frame()
		tt.Key(0, ui.KeyRight)
		if s.chosen != 1 {
			t.Fatalf("%s stranded history navigation", label)
		}
		tt.Key(0, ui.KeyLeft)
		if s.chosen != 0 {
			t.Fatalf("%s lost the selected card", label)
		}
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("Copiggy")
	tt.Key(0, ui.KeyLeft)
	tt.Type("X")
	if s.query != "CopiggXy" {
		t.Fatalf("history navigation stole the search caret: %q", s.query)
	}
}

func TestNativeHoverStaysQuietWithoutTooltip(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s := nativeViewFixture(t)
		s.viewState.Dark = dark
		tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
		search, _ := tt.Find("搜索 · ⌘F")
		before := tt.Texts()
		tt.Move(search.X+search.W/2, search.Y+search.H/2)
		tt.Advance(time.Second)
		if _, found := tt.Find("搜索 · ⌘F"); !found || s.trayStatus.title(time.Now()) != "" {
			t.Fatal("hover should retain its accessibility label without a menu-bar hint")
		}
		if !slices.Equal(before, tt.Texts()) {
			t.Fatal("hover created tooltip content in the panel")
		}
		tt.Move(1100, 55)
		tt.Frame()
		if s.trayStatus.title(time.Now()) != "" {
			t.Fatal("leaving a control produced a menu-bar hint")
		}
		s.reportNativeStatus(s.tr("Paste blocked"))
		until := s.trayStatus.eventUntil
		tt.Move(search.X+search.W/2, search.Y+search.H/2)
		tt.Frame()
		if s.trayStatus.title(time.Now()) != s.tr("Paste blocked") || s.trayStatus.eventUntil != until {
			t.Fatal("hover replaced or renewed important menu-bar feedback")
		}
	}
}

func TestNativeSelectionCoversTheCardEdge(t *testing.T) {
	s := nativeViewFixture(t)
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tt.SetScale(2)
	card := s.viewState.CardBounds[s.results[0].ID]
	// MyGo strokes inward. The outermost opaque mid-edge must be the
	// selection color, not an exposed strip of the card's background.
	for _, point := range []image.Point{
		{int(card.X*2) + 1, int((card.Y + card.H/2) * 2)},
		{int((card.X+card.W)*2) - 2, int((card.Y + card.H/2) * 2)},
		{int((card.X + card.W/2) * 2), int(card.Y*2) + 1},
		{int((card.X + card.W/2) * 2), int((card.Y+card.H)*2) - 2},
	} {
		p := tt.Image().RGBAAt(point.X, point.Y)
		if p.B < 150 || p.B <= p.R || p.G < 100 {
			t.Fatalf("card background exposed outside selection at %v: %v", point, p)
		}
	}
}

func TestNativeEmptyHistoryHasNoHorizontalScroll(t *testing.T) {
	s := nativeViewFixture(t)
	s.query = "没有匹配的复制记录"
	s.filter()
	// The previous populated strip may have been scrolled before filtering.
	s.viewState.Scroll.X = 100
	s.viewState.Scroll.MaxX = 100
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tray := s.viewState.Tray
	tt.Scroll(tray.X+100, tray.Y+150, 200, 0)
	if s.viewState.Scroll.X != 0 || s.viewState.Scroll.MaxX != 0 {
		t.Fatalf("empty history retained a horizontal scroll range: %+v", s.viewState.Scroll)
	}
}

func TestNativeHistoryScrollsWithoutVisibleScrollbar(t *testing.T) {
	s := nativeViewFixture(t)
	for i := 0; i < 10; i++ {
		nativeCaptureText(t, s, fmt.Sprintf("滚动记录 %d", i), "备忘录")
	}
	s.filter()
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tt.Scroll(400, 150, 180, 0)
	if s.viewState.Scroll.X <= 0 {
		t.Fatal("hiding the scrollbar disabled horizontal scrolling")
	}
	// The strip has 14px bottom spacing. Even during active scrolling, the
	// native surface must leave this area transparent for AppKit's material.
	for y := 246; y < 254; y++ {
		for x := 30; x < 1110; x++ {
			if tt.Image().RGBAAt(x, y).A != 0 {
				t.Fatalf("visible scrollbar below cards at %d,%d", x, y)
			}
		}
	}
}

func TestNativeThemeMenuKeepsCardContentsAndKeyboardFocus(t *testing.T) {
	s := nativeViewFixture(t)
	jsonItem := s.results[0]
	pixels := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			pixels.SetRGBA(x, y, color.RGBA{R: 20, G: 155, B: 80, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, pixels); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	s.appIcons["com.apple.TextEdit"] = data
	imageItem := nativeCaptureImage(t, s, data, "预览")
	s.filter()
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tt.SetScale(2)
	if err := tt.Click(s.nativeCardLabel(jsonItem)); err != nil {
		t.Fatal(err)
	}
	card := s.viewState.CardBounds[jsonItem.ID]
	imageCard := s.viewState.CardBounds[imageItem.ID]
	region := func(r ui.Rect) []byte {
		img := tt.Image()
		var pixels []byte
		for y := int(r.Y * 2); y < int((r.Y+r.H)*2); y++ {
			for x := int(r.X * 2); x < int((r.X+r.W)*2); x++ {
				p := img.RGBAAt(x, y)
				pixels = append(pixels, p.R, p.G, p.B, p.A)
			}
		}
		return pixels
	}
	body := ui.Rect{X: card.X + 18, Y: card.Y + 56, W: 145, H: 90}
	source := ui.Rect{X: card.X + 150, Y: card.Y + 20, W: 12, H: 12}
	photo := ui.Rect{X: imageCard.X + 60, Y: imageCard.Y + 60, W: 80, H: 80}
	beforeBody, beforeSource, beforePhoto := region(body), region(source), region(photo)
	tt.Key(0, ui.KeySpace)
	for cycle := 0; cycle < 4; cycle++ {
		// Exercise the same callback used by the menu without opening a window.
		s.applyNativeTheme(!s.viewState.Dark)
		tt.Frame()
		if !bytes.Equal(region(body), beforeBody) || !bytes.Equal(region(source), beforeSource) || !bytes.Equal(region(photo), beforePhoto) {
			t.Fatalf("card text, source bitmap or image changed after theme transition %d", cycle)
		}
		tt.Key(0, ui.KeyEscape)
		if !tt.Focused(s.nativeCardLabel(jsonItem)) {
			t.Fatal("closing preview did not restore the same card's focus")
		}
		tt.Key(0, ui.KeyRight)
		if s.chosen != 2 {
			t.Fatal("card lost keyboard navigation after theme transition")
		}
		tt.Key(0, ui.KeyLeft)
		tt.Key(0, ui.KeySpace)
		if !s.preview {
			t.Fatal("card could not reopen preview after theme transition")
		}
	}
}

func TestNativeImageCardMetadataPaintsAboveImage(t *testing.T) {
	s := nativeViewFixture(t)
	solidPNG := func(c color.RGBA) []byte {
		t.Helper()
		pixels := image.NewRGBA(image.Rect(0, 0, 40, 20))
		for y := 0; y < 20; y++ {
			for x := 0; x < 40; x++ {
				pixels.SetRGBA(x, y, c)
			}
		}
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, pixels); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	background := color.RGBA{R: 20, G: 30, B: 50, A: 255}
	sourceColor := color.RGBA{R: 240, G: 120, B: 30, A: 255}
	data := solidPNG(background)
	item := nativeCaptureImage(t, s, data, "预览")
	s.itemSources[item.ID] = "com.apple.Preview"
	s.appIcons["com.apple.Preview"] = solidPNG(sourceColor)
	s.filter()
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tt.SetScale(2)
	assertMetadata := func() {
		t.Helper()
		card := s.viewState.CardBounds[item.ID]
		rendered := tt.Image()
		_, ink := nativeImageGlass(s.viewState.Dark)
		countInk := func(r ui.Rect) int {
			count := 0
			for y := int(r.Y * 2); y < int((r.Y+r.H)*2); y++ {
				for x := int(r.X * 2); x < int((r.X+r.W)*2); x++ {
					p := rendered.RGBAAt(x, y)
					if nativePixelMatchesInk(p, ink) {
						count++
					}
				}
			}
			return count
		}
		header := ui.Rect{X: card.X + 18, Y: card.Y + 18, W: 90, H: 24}
		dimensions, ok := tt.Find("40 × 20")
		if !ok || countInk(header) < 30 || countInk(dimensions) < 30 {
			t.Fatal("image hides its type/time or dimensions")
		}
		source, ok := tt.Find("来源：预览")
		if !ok {
			t.Fatal("image card has no source icon")
		}
		if got := rendered.RGBAAt(int((source.X+source.W/2)*2), int((source.Y+source.H/2)*2)); got != sourceColor {
			t.Fatalf("image hides source bitmap: %v", got)
		}
		if got := rendered.RGBAAt(int((card.X+102)*2), int((card.Y+102)*2)); got != background {
			t.Fatalf("metadata overlay changed the image itself: %v", got)
		}
	}
	assertMetadata()
	s.applyNativeTheme(true)
	tt.Frame()
	assertMetadata()
	if err := tt.Click(s.nativeCardLabel(item)); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeySpace)
	assertMetadata()
}

func TestNativeWhiteImageKeepsLegibleMetadataWithoutColoredFringe(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s := nativeViewFixture(t)
		s.viewState.Dark = dark
		pixels := image.NewRGBA(image.Rect(0, 0, 40, 40))
		for y := 0; y < 40; y++ {
			for x := 0; x < 40; x++ {
				pixels.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, pixels); err != nil {
			t.Fatal(err)
		}
		data := buffer.Bytes()
		item := nativeCaptureImage(t, s, data, "预览")
		s.filter()
		s.chosen = 1 // Inspect an unselected image, without the blue selection.
		tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
		tt.SetScale(2)
		card := s.viewState.CardBounds[item.ID]
		at := func(x, y float32) color.RGBA {
			return tt.Image().RGBAAt(int((card.X+x)*2), int((card.Y+y)*2))
		}
		// Only the small label bounds are frosted; no full-width top/bottom band.
		for _, point := range []ui.Point{{X: 12, Y: 30}, {X: 12, Y: 168}} {
			pixel := at(point.X, point.Y)
			if pixel != (color.RGBA{255, 255, 255, 255}) {
				t.Fatalf("dark=%v: unwanted image shading at %+v: %v", dark, point, pixel)
			}
		}
		if center := at(102, 102); center != (color.RGBA{255, 255, 255, 255}) {
			t.Fatalf("metadata changes the image center: %v", center)
		}
		_, glassInk := nativeImageGlass(dark)
		for _, region := range []ui.Rect{{X: 18, Y: 18, W: 90, H: 24}, {X: 16, Y: 156, W: 100, H: 20}} {
			inkPixels := 0
			for y := region.Y; y < region.Y+region.H; y += .5 {
				for x := region.X; x < region.X+region.W; x += .5 {
					p := at(x, y)
					if nativePixelMatchesInk(p, glassInk) {
						inkPixels++
					}
				}
			}
			if inkPixels < 30 {
				t.Fatalf("dark=%v: no sharp metadata over glass in %+v", dark, region)
			}
		}
		for _, point := range []ui.Point{{X: .5, Y: 102}, {X: nativeCardSize - .5, Y: 102}, {X: 6, Y: 10}} {
			pixel := at(point.X, point.Y)
			if pixel.R != pixel.G || pixel.G != pixel.B {
				t.Fatalf("colored image-card fringe at %+v: %v", point, pixel)
			}
		}
	}
}

func nativePixelMatchesInk(pixel color.RGBA, ink ui.Color) bool {
	near := func(a, b uint8) bool { return max(int(a), int(b))-min(int(a), int(b)) <= 8 }
	return pixel.A > 240 && near(pixel.R, ink.R) && near(pixel.G, ink.G) && near(pixel.B, ink.B)
}
