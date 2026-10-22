package nativeui

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// Exercise the native controls through the real copy service with an in-memory
// writer; no desktop, applications, or system clipboard are involved.
type nativeVariantWriter struct {
	data  clipmodel.Representations
	scope []string
	calls int
}

func (w *nativeVariantWriter) Write(ctx context.Context, data clipmodel.Representations, scope []string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	w.data, w.scope = data.Clone(), append([]string(nil), scope...)
	w.calls++
	return int64(w.calls), nil
}

func nativeVariantCopyService(s *application, writer *nativeVariantWriter) {
	service := clipboard.Copy{History: s.store, Port: writer}
	s.config.Copy = func(ctx context.Context, id string, draft bool) (clipmodel.Item, error) {
		if draft {
			item, _, err := service.Draft(ctx, id)
			return item, err
		}
		item, _, err := service.Item(ctx, id)
		return item, err
	}
	s.config.CopyVariant = func(ctx context.Context, id string, format clipmodel.CopyFormat) (clipmodel.Item, error) {
		item, _, err := service.Variant(ctx, id, format)
		return item, err
	}
}

func TestPreviewReturnCopiesChosenFormatWithoutAddingHistory(t *testing.T) {
	color := clipmodel.Analyze("#D78379").Color
	for _, tc := range []struct {
		name, raw, want string
		actions         []string
	}{
		{"json", `{"answer":42}`, "{\n  \"answer\": 42\n}", []string{"Format JSON"}},
		{"json-original", `{"answer":42}`, `{"answer":42}`, []string{"Format JSON", "Original / result"}},
		{"color-rgb", "#D78379", color.RGB, []string{"RGB"}},
		{"color-hsl", "#D78379", color.HSL, []string{"HSL"}},
		{"color-hex", "rgb(215, 131, 121)", "#D78379", []string{"HEX"}},
		{"phone", "+1 (202) 555-0148", "+12025550148", []string{"Dialing number"}},
		{"phone-original", "+1 (202) 555-0148", "+1 (202) 555-0148", []string{"Dialing number", "Original number"}},
		{"timestamp-ms", "1792645800", "1792645800000", []string{"Milliseconds"}},
		{"timestamp-s", "1792645800123", "1792645800", []string{"Seconds"}},
		{"timestamp-zone", "1792645800123", "1792645800123", []string{"Local time"}},
		{"untouched-color", "  rgb(215,131,121)  ", "  rgb(215,131,121)  ", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, item := semanticPreviewFixture(t, tc.raw)
			if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
				t.Fatal(err)
			}
			if err := s.store.SetPinned(context.Background(), item.ID, true); err != nil {
				t.Fatal(err)
			}
			s.filter()
			before := s.store.Records()
			writer := &nativeVariantWriter{}
			nativeVariantCopyService(s, writer)
			width, height := nativeContentPreviewSize(item.Type)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			for _, action := range tc.actions {
				if err := tt.Click(action); err != nil {
					t.Fatal(err)
				}
			}
			if writer.calls != 0 || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("format selection wrote clipboard or altered history before Return")
			}
			tt.Key(0, ui.KeyEnter)
			bytes, err := writer.data.Read(clipmodel.TextFormat)
			if err != nil || string(bytes) != tc.want || writer.calls != 1 {
				t.Fatalf("Return did not copy selected result: got=%q want=%q writes=%d err=%v", bytes, tc.want, writer.calls, err)
			}
			if !reflect.DeepEqual(writer.scope, []string{"com.example.Editor"}) || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("derived copy lost source protection or changed saved records")
			}
			if s.preview || s.draft != nil || s.semanticPreview.CopyFormat != "" {
				t.Fatal("completed copy retained preview conversion state")
			}
		})
	}
}

func TestPreviewFormatsResetOnCloseAndNavigation(t *testing.T) {
	s, _ := semanticPreviewFixture(t, "neighboring text")
	item := nativeCaptureText(t, s, "#D78379", "Fixture editor")
	s.filter()
	s.selectNative(0, true)
	width, height := nativeContentPreviewSize(item.Type)
	tt := ui.NewTester(s.nativePopoverView, width, height)
	if err := tt.Click("RGB"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEscape)
	s.preview = true
	tt.Frame()
	if s.semanticPreview.CopyFormat != "" || s.semanticPreview.ColorFormat != 0 {
		t.Fatal("reopened preview retained a prior conversion")
	}
	if err := tt.Click("HSL"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyRight)
	tt.Key(0, ui.KeyLeft)
	if s.semanticPreview.CopyFormat != "" || s.semanticPreview.ColorFormat != 0 {
		t.Fatal("returning to a record retained a prior conversion")
	}
}

func TestQueuedVariantCopyCancelsWhenPreviewChanges(t *testing.T) {
	s, _ := semanticPreviewFixture(t, "neighboring text")
	item := nativeCaptureText(t, s, "#D78379", "Fixture editor")
	s.filter()
	s.selectNative(0, true)
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	var requestedID string
	var requestedFormat clipmodel.CopyFormat
	var requestContext context.Context
	s.config.CopyVariant = func(ctx context.Context, id string, format clipmodel.CopyFormat) (clipmodel.Item, error) {
		requestedID, requestedFormat, requestContext = id, format, ctx
		return clipmodel.Item{}, ctx.Err()
	}
	state := s.semanticState(item)
	state.ColorFormat, state.CopyFormat = 1, clipmodel.ColorRGB
	s.chooseNative(false)
	s.selectNative(1, true)
	queue.runNext(t)()
	selected, _ := s.selected()
	if requestedID != item.ID || requestedFormat != clipmodel.ColorRGB || !errors.Is(requestContext.Err(), context.Canceled) {
		t.Fatal("queued copy did not retain its format and cancel on navigation")
	}
	if selected.ID == item.ID || !s.preview || s.commandBusy {
		t.Fatal("old copy completion disrupted the new preview")
	}
}

func TestCopyFormatStaysFixedWhileWriteIsPending(t *testing.T) {
	for _, tc := range []struct{ raw, choose, change string }{
		{"#D78379", "RGB", "HSL"},
		{"+1 (202) 555-0148", "Dialing number", "Original number"},
		{"1792645800", "Milliseconds", "Seconds"},
		{`{"answer":42}`, "Format JSON", "Original / result"},
	} {
		t.Run(tc.choose, func(t *testing.T) {
			s, item := semanticPreviewFixture(t, tc.raw)
			writer := &nativeVariantWriter{}
			nativeVariantCopyService(s, writer)
			width, height := nativeContentPreviewSize(item.Type)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			if err := tt.Click(tc.choose); err != nil {
				t.Fatal(err)
			}
			before, original := s.semanticPreview, s.nativeState().Original
			var queue nativeCommandQueue
			s.config.Dispatch = queue.dispatch
			tt.Key(0, ui.KeyEnter)
			if !s.commandBusy || writer.calls != 0 {
				t.Fatal("copy was not queued")
			}
			if err := tt.Click(tc.change); err != nil {
				t.Fatal(err)
			}
			if s.semanticPreview != before || s.nativeState().Original != original {
				t.Fatal("visible format changed during a pending copy")
			}
			queue.runNext(t)()
			if writer.calls != 1 || s.preview || s.commandBusy {
				t.Fatal("pending copy did not complete")
			}
		})
	}
}
