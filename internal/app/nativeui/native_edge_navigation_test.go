package nativeui

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestNativeEdgeNavigationUsesVisibleResultsWithoutCopying(t *testing.T) {
	for _, surface := range []string{"tray", "popover"} {
		for _, scope := range []string{"history", "pinned", "search"} {
			t.Run(surface+"/"+scope, func(t *testing.T) {
				s := nativeViewFixture(t)
				if scope == "pinned" {
					for _, index := range []int{1, 3} {
						if err := s.store.SetPinned(context.Background(), s.results[index].ID, true); err != nil {
							t.Fatal(err)
						}
					}
					s.viewState.PinnedOnly = true
				}
				if scope == "search" {
					for i := range 3 {
						nativeCaptureText(t, s, fmt.Sprintf("matching result %d", i), "Notes")
					}
					s.query = "matching result"
				}
				s.filter()
				s.chosen = 1
				before := s.store.Records()
				current, _ := s.store.Current()
				copied := false
				s.config.Copy = func(context.Context, string, bool) (clipmodel.Item, error) {
					copied = true
					return clipmodel.Item{}, nil
				}
				view, width, height := s.nativeView, 1140, nativeTrayHeight
				if surface == "popover" {
					s.preview = true
					view, width, height = s.nativePopoverView, 520, 360
				}
				tt := ui.NewTester(view, width, height)
				for _, edge := range []struct {
					key   ui.Key
					index int
				}{{ui.KeyRight, len(s.results) - 1}, {ui.KeyLeft, 0}} {
					tt.Key(ui.Cmd, edge.key)
					selected, ok := s.selected()
					if !ok || s.chosen != edge.index || selected.ID != s.results[edge.index].ID {
						t.Fatalf("edge shortcut selected %d, want visible result %d", s.chosen, edge.index)
					}
					if s.preview != (surface == "popover") {
						t.Fatal("edge navigation changed preview visibility")
					}
				}
				assertNativeCurrent(t, s, current.ID)
				if copied || !reflect.DeepEqual(before, s.store.Records()) {
					t.Fatal("edge navigation copied or modified saved history")
				}
			})
		}
	}
}

func TestNativeEdgeNavigationPreservesSearchCaret(t *testing.T) {
	for _, edge := range []struct {
		name, want string
		key        ui.Key
	}{{"left", "Xneedle", ui.KeyLeft}, {"right", "needleX", ui.KeyRight}} {
		t.Run(edge.name, func(t *testing.T) {
			s := nativeViewFixture(t)
			for i := range 3 {
				nativeCaptureText(t, s, fmt.Sprintf("needle %d", i), "Notes")
			}
			s.filter()
			tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
			tt.Key(ui.Cmd, ui.KeyF)
			tt.Type("needle")
			s.chosen = 1
			tt.Frame()
			tt.Key(ui.Cmd, edge.key)
			if s.chosen != 1 || !tt.Focused(s.tr("Search clipboard history")) {
				t.Fatal("edge navigation took the search editor's command arrow")
			}
			tt.Type("X")
			if s.query != edge.want {
				t.Fatalf("search caret did not move to its endpoint: got %q, want %q", s.query, edge.want)
			}
		})
	}
}

func TestNativeEdgeNavigationRevealsEndpointsWithPadding(t *testing.T) {
	s := nativeViewFixture(t)
	for i := range 12 {
		nativeCaptureText(t, s, fmt.Sprintf("scroll endpoint %d", i), "Notes")
	}
	s.filter()
	tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tt.Key(ui.Cmd, ui.KeyRight)
	tt.Advance(250 * time.Millisecond)
	last := s.viewState.CardBounds[s.results[len(s.results)-1].ID]
	if last.X+last.W > 1140-nativeTrayInset || last.X+last.W < 1140-nativeTrayInset-4 {
		t.Fatalf("jump to last card lost its trailing padding: %+v", last)
	}
	tt.Key(ui.Cmd, ui.KeyLeft)
	tt.Advance(250 * time.Millisecond)
	first := s.viewState.CardBounds[s.results[0].ID]
	if first.X < nativeTrayInset || first.X > nativeTrayInset+4 {
		t.Fatalf("jump to first card lost its leading padding: %+v", first)
	}
	// Trackpad scrolling does not change the selected card. Repeating the edge
	// shortcut must still bring that selected endpoint back into view.
	tt.Scroll(400, 150, 260, 0)
	tt.Frame()
	if first = s.viewState.CardBounds[s.results[0].ID]; first.X >= 0 || s.chosen != 0 {
		t.Fatalf("fixture did not scroll the selected first card away: %+v", first)
	}
	tt.Key(ui.Cmd, ui.KeyLeft)
	tt.Advance(250 * time.Millisecond)
	if first = s.viewState.CardBounds[s.results[0].ID]; first.X < nativeTrayInset || first.X > nativeTrayInset+4 {
		t.Fatalf("repeated endpoint shortcut did not reveal the selected card: %+v", first)
	}
}

func TestNativeEdgeNavigationCancelsPreviewDraft(t *testing.T) {
	s := nativeViewFixture(t)
	item, _ := s.selected()
	draft, err := s.store.Enhance(context.Background(), item.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	s.draft, s.preview = &draft, true
	tt := ui.NewTester(s.nativePopoverView, 520, 360)
	tt.Key(ui.Cmd, ui.KeyRight)
	if s.draft != nil || s.chosen != len(s.results)-1 || !s.preview {
		t.Fatal("edge navigation retained the previous record's preview draft")
	}
	if _, err := s.store.ValidateDraft(draft.ID); err == nil {
		t.Fatal("edge navigation left a stale formatting draft usable")
	}
}
