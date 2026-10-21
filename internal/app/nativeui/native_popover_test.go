package nativeui

import (
	"context"
	"errors"
	"image"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func nativePopoverImageFixture(t *testing.T, s *application) clipmodel.Item {
	t.Helper()
	item := nativeCaptureImage(t, s, encodedNativePNG(t, 40, 20), "预览")
	s.filter()
	s.selectNative(0, true)
	return item
}

func assertNativeCurrent(t *testing.T, s *application, id string) {
	t.Helper()
	if current, ok := s.store.Current(); !ok || current.ID != id {
		t.Fatalf("browsing changed current clipboard item: %q -> %q", id, current.ID)
	}
}

func TestNativePopoverContentByType(t *testing.T) {
	for _, kind := range []clipmodel.ContentType{clipmodel.TypeJSON, clipmodel.TypeText, clipmodel.TypeImage} {
		t.Run(string(kind), func(t *testing.T) {
			s := nativeViewFixture(t)
			height := 360
			switch kind {
			case clipmodel.TypeText:
				s.selectNative(1, true)
			case clipmodel.TypeImage:
				nativePopoverImageFixture(t, s)
				height = 420
			}
			s.preview = true
			current, _ := s.store.Current()
			tt := ui.NewTester(s.nativePopoverView, 520, height)
			if !tt.Focused("内容预览") {
				t.Fatal("independent preview did not acquire its own focus target")
			}
			for _, label := range []string{"搜索复制内容 · ⌘F", "防误贴"} {
				if _, ok := tt.Find(label); ok {
					t.Fatalf("popover rendered parent control %q", label)
				}
			}
			for _, point := range []image.Point{{0, 0}, {8, 180}, {2, height - 2}} {
				if _, _, _, alpha := tt.Image().At(point.X, point.Y).RGBA(); alpha != 0 {
					t.Fatalf("MyGo paints over AppKit popover material at %v", point)
				}
			}
			_, jsonAction := tt.Find("格式化 JSON")
			_, imageAction := tt.Find("放大图片")
			if jsonAction != (kind == clipmodel.TypeJSON) || imageAction != (kind == clipmodel.TypeImage) {
				t.Fatalf("wrong actions for %s: JSON=%v, image=%v", kind, jsonAction, imageAction)
			}
			for _, label := range []string{"收起预览", "格式化 JSON", "放大图片"} {
				if action, ok := tt.Find(label); ok && (action.Y < 0 || action.Y+action.H > 48) {
					t.Fatalf("%s action is not in the top toolbar: %+v", label, action)
				}
			}
			for _, label := range []string{"↵", "esc", "取用", "收起"} {
				if _, ok := tt.Find(label); ok {
					t.Fatalf("preview retained the redundant footer hint %q", label)
				}
			}
			switch kind {
			case clipmodel.TypeJSON:
				if !tt.HasText("Copiggy") || !tt.HasText("仅限 文本编辑") {
					t.Fatalf("JSON content or app restriction is absent: %v", tt.Texts())
				}
			case clipmodel.TypeText:
				item, _ := s.selected()
				if !tt.HasText(item.Text) {
					t.Fatal("plain text preview is absent")
				}
			case clipmodel.TypeImage:
				if _, ok := tt.Find("复制的图片"); !ok || !tt.HasText("40 × 20 · PNG") {
					t.Fatal("image preview or dimensions are absent")
				}
			}
			assertNativeCurrent(t, s, current.ID)
		})
	}
}

func TestNativePopoverNavigationAndCloseDoNotSelectClipboard(t *testing.T) {
	s := nativeViewFixture(t)
	s.preview = true
	current, _ := s.store.Current()
	tt := ui.NewTester(s.nativePopoverView, 520, 360)
	tt.Key(0, ui.KeyRight)
	if s.chosen != 1 || !s.preview || !tt.HasText(s.results[1].Text) {
		t.Fatal("popover did not navigate to and render the next record")
	}
	tt.Key(0, ui.KeyLeft)
	if s.chosen != 0 || !s.preview {
		t.Fatal("popover did not navigate back")
	}
	for _, key := range []struct {
		mods ui.Modifiers
		key  ui.Key
	}{{0, ui.KeySpace}, {0, ui.KeyEscape}, {ui.Cmd, ui.KeyY}} {
		s.preview = true
		tt.Frame()
		tt.Key(key.mods, key.key)
		if s.preview {
			t.Fatalf("preview did not close for %+v", key)
		}
		assertNativeCurrent(t, s, current.ID)
	}
}

func TestNativePopoverCommandFReturnsToParentSearch(t *testing.T) {
	s := nativeViewFixture(t)
	parent := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	s.preview = true
	popup := ui.NewTester(s.nativePopoverView, 520, 360)
	popup.Key(ui.Cmd, ui.KeyF)
	if s.preview || !s.viewState.SearchOpen {
		t.Fatal("Cmd-F in popover did not return to parent search")
	}
	parent.Frame()
	parent.Type("hello world")
	if s.query != "hello world" || s.preview {
		t.Fatalf("parent search did not receive typing after popover Cmd-F: %q", s.query)
	}
}

func TestNativePopoverDraftComparisonAndCancellationDoNotWrite(t *testing.T) {
	s := nativeViewFixture(t)
	item, _ := s.selected()
	current, _ := s.store.Current()
	draft, err := s.store.Enhance(context.Background(), item.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	s.draft, s.preview = &draft, true
	s.viewState.DraftCurrent = current.ID
	tt := ui.NewTester(s.nativePopoverView, 520, 360)
	if !tt.HasText("格式化草稿") {
		t.Fatal("preview did not identify the pending result")
	}
	for _, label := range []string{"原文 / 结果", "放弃结果", "取用结果"} {
		action, ok := tt.Find(label)
		if !ok || action.Y+action.H > 48 {
			t.Fatalf("draft action %q is absent from the top toolbar: %+v", label, action)
		}
	}
	if err := tt.Click("原文 / 结果"); err != nil {
		t.Fatal(err)
	}
	if !s.viewState.Original || !tt.HasText("原始内容") {
		t.Fatal("comparison did not show the original")
	}
	if _, ok := tt.Find("取用原文"); !ok || !tt.Focused("内容预览") {
		t.Fatal("comparison did not update confirmation or preserve preview keyboard focus")
	}
	assertNativeCurrent(t, s, current.ID)
	if err := tt.Click("放弃结果"); err != nil {
		t.Fatal(err)
	}
	if s.draft != nil || s.viewState.Original || !s.preview {
		t.Fatal("discard did not return to the original preview")
	}
	if _, err := s.store.ValidateDraft(draft.ID); !errors.Is(err, clipmodel.ErrDraftNotFound) {
		t.Fatalf("discarded result can still be accepted: %v", err)
	}
	assertNativeCurrent(t, s, current.ID)
}

func TestNativePopoverImageZoomKeepsNavigationWithinPreview(t *testing.T) {
	s := nativeViewFixture(t)
	parent := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	item := nativePopoverImageFixture(t, s)
	s.preview = true
	parent.Frame()
	popup := ui.NewTester(s.nativePopoverView, 520, 420)
	if err := popup.Click("放大图片"); err != nil {
		t.Fatal(err)
	}
	if !s.viewState.Zoom || !popup.HasText("完整图片") {
		t.Fatal("image did not open its large preview")
	}
	if close, ok := popup.Find("关闭图片预览"); !ok || close.Y+close.H > 48 {
		t.Fatal("zoom close action is absent from the top toolbar")
	}
	for _, label := range []string{"esc", "返回"} {
		if _, ok := popup.Find(label); ok {
			t.Fatalf("zoom retained the footer hint %q", label)
		}
	}
	before := s.chosen
	popup.Key(0, ui.KeyRight)
	popup.Key(0, ui.KeyEnter)
	if s.chosen != before || !s.viewState.Zoom || !s.preview {
		t.Fatal("zoom consumed navigation or Enter as a clipboard selection")
	}
	assertNativeCurrent(t, s, item.ID)
	popup.Key(0, ui.KeyEscape)
	if s.viewState.Zoom || !s.preview || !popup.Focused("内容预览") {
		t.Fatal("Escape did not return focus to normal preview")
	}
	popup.Key(0, ui.KeyRight)
	if s.chosen != before+1 {
		t.Fatal("navigation did not resume after leaving zoom")
	}
	popup.Key(0, ui.KeyLeft)
	if err := popup.Click("放大图片"); err != nil {
		t.Fatal(err)
	}
	if err := popup.Click("关闭图片预览"); err != nil {
		t.Fatal(err)
	}
	if s.viewState.Zoom || !popup.Focused("内容预览") {
		t.Fatal("close icon did not restore normal preview focus")
	}
	popup.Key(ui.Cmd, ui.KeyY)
	parent.Frame()
	parent.Key(0, ui.KeyRight)
	if s.preview || s.chosen != before+1 {
		t.Fatal("closing the popover did not restore parent card navigation")
	}
	assertNativeCurrent(t, s, item.ID)
}

func TestNativePreviewAnchorClipsToVisibleCard(t *testing.T) {
	tray := ui.Rect{W: 1140, H: 318}
	for _, tc := range []struct {
		name string
		card ui.Rect
		want mygo.Rectangle
		ok   bool
	}{
		{"inside", ui.Rect{X: 24, Y: 83, W: 204, H: 204}, mygo.Rectangle{X: 24, Y: 83, Width: 204, Height: 204}, true},
		{"left", ui.Rect{X: -80, Y: 83, W: 204, H: 204}, mygo.Rectangle{Y: 83, Width: 124, Height: 204}, true},
		{"right", ui.Rect{X: 1100, Y: 83, W: 204, H: 204}, mygo.Rectangle{X: 1100, Y: 83, Width: 40, Height: 204}, true},
		{"top", ui.Rect{X: 24, Y: -20, W: 204, H: 204}, mygo.Rectangle{X: 24, Width: 204, Height: 184}, true},
		{"bottom", ui.Rect{X: 24, Y: 300, W: 204, H: 204}, mygo.Rectangle{X: 24, Y: 300, Width: 204, Height: 18}, true},
		{"outside", ui.Rect{X: 1140, Y: 83, W: 204, H: 204}, mygo.Rectangle{}, false},
		{"empty", ui.Rect{}, mygo.Rectangle{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := nativePreviewAnchor(tc.card, tray)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("got %+v/%v, want %+v/%v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
