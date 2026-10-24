package nativeui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestMarkdownReadingAndSourcePreserveProtectedCopy(t *testing.T) {
	raw := "  # Original\n\n- First\n- **Second**\n"
	for _, actions := range [][]string{nil, {"Source"}, {"Source", "Reading"}} {
		s, item := semanticPreviewFixture(t, raw)
		if item.Type != clipmodel.TypeMarkdown {
			t.Fatalf("wrong type: %s", item.Type)
		}
		if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
			t.Fatal(err)
		}
		s.filter()
		before := s.store.Records()
		writer := &nativeVariantWriter{}
		nativeVariantCopyService(s, writer)
		width, height := nativeContentPreviewSize(item.Type)
		tt := ui.NewTester(s.nativePopoverView, width, height)
		if !tt.HasText("Original") || tt.HasText(raw) {
			t.Fatal("reading mode did not render heading")
		}
		for _, action := range actions {
			if err := tt.Click(action); err != nil {
				t.Fatal(err)
			}
		}
		if len(actions) == 1 && !tt.HasText(raw) {
			t.Fatal("source mode lost exact text")
		}
		if writer.calls != 0 || !reflect.DeepEqual(before, s.store.Records()) || s.semanticPreview.CopyFormat != "" {
			t.Fatal("presentation changed source or wrote clipboard")
		}
		tt.Key(0, ui.KeyEnter)
		bytes, err := writer.data.Read(clipmodel.TextFormat)
		if err != nil || string(bytes) != raw || writer.calls != 1 || !reflect.DeepEqual(writer.scope, []string{"com.example.Editor"}) {
			t.Fatalf("copy lost original or scope: %q %v", bytes, err)
		}
		if !reflect.DeepEqual(before, s.store.Records()) {
			t.Fatal("copy changed history")
		}
	}
}

func TestMarkdownSourceModeResetsAfterCloseAndNavigation(t *testing.T) {
	s, _ := semanticPreviewFixture(t, "neighboring text")
	item := nativeCaptureText(t, s, "# Heading\n\nA paragraph.", "Fixture")
	s.filter()
	s.selectNative(0, true)
	width, height := nativeContentPreviewSize(item.Type)
	tt := ui.NewTester(s.nativePopoverView, width, height)
	if err := tt.Click("Source"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEscape)
	s.preview = true
	tt.Frame()
	if s.semanticPreview.MarkdownSource {
		t.Fatal("source mode survived close")
	}
	if err := tt.Click("Source"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyRight)
	tt.Key(0, ui.KeyLeft)
	if s.semanticPreview.MarkdownSource {
		t.Fatal("source mode survived navigation")
	}
}

func TestMarkdownNativeFormattingAndFixedFooter(t *testing.T) {
	parts := []clipmodel.MarkdownSpan{{Text: "strong", Strong: true}, {Text: "italic", Emphasis: true}, {Text: "code", Code: true}, {Text: "gone", Strike: true}, {Text: "link", Link: true}, {Text: "image", Image: true}}
	spans := nativeMarkdownSpans(parts, nativeNoteColors())
	if spans[0].Weight != 500 || !spans[1].Italic || spans[2].Font != nativeCodeFont || !spans[3].Strikethrough || !spans[5].Italic {
		t.Fatalf("format styles lost: %+v", spans)
	}
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		s, item := semanticPreviewFixture(t, "# Heading\n\n"+strings.Repeat("- A line\n", 100))
		s.language = language
		item.AllowedApps = []string{"com.example.Editor"}
		before := s.store.Records()
		tt := ui.NewTester(func(c *ui.Context) {
			p := panelColors(false)
			setNativeTheme(c, p, false, language)
			s.nativeMarkdownPreview(c, item, p)
		}, 600, nativeMarkdownPreviewHeight)
		footer, ok := tt.Find(s.tr("Markdown"))
		if !ok || footer.Y < nativeMarkdownPreviewHeight-80 {
			t.Fatalf("long document displaced footer: %+v", footer)
		}
		for _, label := range []string{"Reading", "Source", "Pin", "Delete item · ⌘⌫", "Close preview"} {
			r, ok := tt.Find(s.tr(label))
			if !ok || r.Y+r.H > 42 || r.X < 8 || r.X+r.W > 592 {
				t.Fatalf("toolbar escaped: %s %+v", label, r)
			}
		}
		if tt.HasText(s.tr("Copy")) {
			t.Fatal("Markdown installed redundant clipboard controls")
		}
		if !reflect.DeepEqual(before, s.store.Records()) {
			t.Fatal("render changed history")
		}
	}
}

func TestMarkdownCardsKeepReadableStructureInsideSharedFrame(t *testing.T) {
	cases := []struct {
		name, raw string
		visible   []string
	}{
		{"heading-list", "# Project notes\n\n- First task\n- Second task\n- Omitted task", []string{"Project notes", "First task", "Second task"}},
		{"list", "1. First entry\n2. Second entry\n3. Third entry\n4. Omitted entry", []string{"1.", "First entry", "Second entry", "Third entry"}},
		{"code", "```go\nanswer := 42\nreturn answer\n```", []string{"answer := 42\nreturn answer\n"}},
		{"quote", "> A quiet quotation.", []string{"A quiet quotation."}},
		{"table", "| Name | State | Extra |\n| --- | --- | --- |\n| Item | Ready | Omitted column |", []string{"Name", "State", "Item", "Ready"}},
	}
	var sourceReference, lockReference, footerReference, pinReference ui.Rect
	for _, tc := range cases {
		for _, dark := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/light", true: "/dark"}[dark], func(t *testing.T) {
				s, item := semanticPreviewFixture(t, tc.raw)
				item.Pinned, item.AllowedApps = true, []string{"com.example.Editor"}
				p, analysis := panelColors(dark), clipmodel.AnalyzeItem(item)
				background, foreground, metadata := nativeCardPalette(item.Type, analysis, p)
				tt := ui.NewTester(func(c *ui.Context) {
					setNativeTheme(c, p, dark)
					card := ui.Box(c).Fill().Radius(nativeCardRadius).Clip().Background(background)
					card.Children(func() {
						s.nativeCardContentWithAssets(c, card, item, analysis, foreground, metadata, p, nativeCardAssets{})
					})
				}, nativeCardSize, nativeCardSize)
				for _, content := range tc.visible {
					r, ok := tt.Find(content)
					if !ok || r.X < nativeCardPadding || r.X+r.W > nativeCardSize-nativeCardPadding+.5 || r.Y < nativeCardPadding+nativeCardHeaderHeight || r.Y+r.H > nativeCardSize-nativeCardPadding-nativeCardFooterHeight-nativeCardContentGap+.5 {
						t.Fatalf("structured content escaped reading body: %q %+v", content, r)
					}
					if r.H < nativeTextBody {
						t.Fatalf("content was reduced to unreadable size: %q %+v", content, r)
					}
				}
				if tt.HasText("Omitted task") || tt.HasText("Omitted entry") || tt.HasText("Omitted column") {
					t.Fatal("card squeezed in the whole document")
				}
				source, sourceOK := tt.Find(s.tr("Source: %s", s.sourceAppName(item)))
				lock, lockOK := tt.Find(s.scopeLabel(item.AllowedApps))
				footer, footerOK := tt.Find(s.tr("Markdown"))
				pin, pinOK := tt.Find(s.tr("Unpin"))
				if !sourceOK || !lockOK || !footerOK || !pinOK {
					t.Fatal("Markdown lost shared source, protection or footer")
				}
				if sourceReference.W == 0 {
					sourceReference, lockReference, footerReference, pinReference = source, lock, footer, pin
				}
				if source != sourceReference || lock != lockReference || footer != footerReference || pin != pinReference {
					t.Fatal("document structure displaced shared card chrome")
				}
			})
		}
	}
}

func TestMarkdownWideTablesAndSourceKeepReadableWidth(t *testing.T) {
	raw := "# Reading table\n\n| First column | Second column | Third column | Fourth column |\n| --- | --- | --- | --- |\n| A descriptive value | Another value | Third value | Fourth value |"
	for _, height := range []int{nativeMarkdownPreviewHeight, 220} {
		s, item := semanticPreviewFixture(t, raw)
		tt := ui.NewTester(s.nativePopoverView, 520, height)
		first, ok := tt.Find("First column")
		last, lastOK := tt.Find("Fourth column")
		footer, footerOK := tt.Find(s.tr("Markdown"))
		if !ok || !lastOK || first.W < 100 || last.X-first.X < 3*100 || first.H < nativeTextPreview {
			t.Fatalf("wide table reduced its reading size: first=%+v last=%+v", first, last)
		}
		if !footerOK || footer.Y < float32(height-60) {
			t.Fatalf("table displaced footer: %+v", footer)
		}
		if err := tt.Click("Source"); err != nil {
			t.Fatal(err)
		}
		source, ok := tt.Find(item.Text)
		if !ok || source.X < nativePreviewEdgeInset+nativePreviewDataInset || source.W < 520-nativePreviewEdgeInset*2-nativePreviewDataInset*2 {
			t.Fatalf("source did not retain long line width: %+v", source)
		}
	}
}

func TestMarkdownDesignUsesRealTitleAndListOutline(t *testing.T) {
	raw := "# Release notes\n\nSmall improvements, clearly written.\n\n- Faster capture\n- Better previews\n\nUse `copy()` to copy items."
	document := clipmodel.AnalyzeMarkdown(raw)
	if document == nil {
		t.Fatal("fixture did not parse")
	}
	excerpts := nativeMarkdownCardExcerpt(document)
	if len(excerpts) != 3 || excerpts[0].Block.Kind != "heading" || excerpts[1].Block.Prefix != "•" || excerpts[2].Block.Prefix != "•" {
		t.Fatalf("card did not select the actual outline: %+v", excerpts)
	}
	var content strings.Builder
	for _, excerpt := range excerpts {
		for _, span := range excerpt.Block.Spans {
			content.WriteString(span.Text)
		}
	}
	if content.String() != "Release notesFaster captureBetter previews" {
		t.Fatalf("card invented or reordered its outline: %q", content.String())
	}
	if !strings.Contains(document.Summary, "Small improvements") {
		t.Fatal("card selection changed document contents")
	}
}

func TestMarkdownCardOutlineKeepsListItemsAtTheSameDepth(t *testing.T) {
	raw := "# Outline\n\n- Parent\n  - Child\n    - Grandchild\n- Peer\n- Later"
	document := clipmodel.AnalyzeMarkdown(raw)
	if document == nil {
		t.Fatal("nested-list fixture did not parse")
	}
	excerpts := nativeMarkdownCardExcerpt(document)
	if len(excerpts) != 3 || excerpts[1].Block.Depth != excerpts[2].Block.Depth {
		t.Fatalf("outline mixed list levels: %+v", excerpts)
	}
	var content strings.Builder
	for _, excerpt := range excerpts {
		for _, span := range excerpt.Block.Spans {
			content.WriteString(span.Text)
		}
	}
	if content.String() != "OutlineParentPeer" {
		t.Fatalf("nested child displaced its parent's peer: %q", content.String())
	}
	if !strings.Contains(document.Summary, "Child") || !strings.Contains(document.Summary, "Grandchild") {
		t.Fatal("card outline changed the complete document")
	}
}

func TestMarkdownDesignKeepsPaperAndProtectedFooterIndependent(t *testing.T) {
	paper := nativeMarkdownColors()
	if paper.paper == nativeNoteColors().paper || paper.paperText != paper.text || paper.paperMeta != paper.muted || nativeMarkdownPaperEdge >= nativeCardPadding {
		t.Fatal("Markdown lost its distinct paper or safe edge inset")
	}
	raw := "# A protected document\n\n" + strings.Repeat("- A retained list entry\n", 70)
	for _, dark := range []bool{false, true} {
		for _, height := range []int{nativeMarkdownPreviewHeight, 220} {
			s, item := semanticPreviewFixture(t, raw)
			item.AllowedApps = []string{"com.example.Editor"}
			s.appNames[item.AllowedApps[0]] = strings.Repeat("Long application name ", 20)
			tt := ui.NewTester(func(c *ui.Context) {
				p := panelColors(dark)
				setNativeTheme(c, p, dark)
				s.nativeMarkdownPreview(c, item, p)
			}, 600, height)
			footer, footerOK := tt.Find(s.tr("Markdown"))
			scope, scopeOK := tt.Find(s.scopeLabel(item.AllowedApps) + " · " + s.tr("Inactive"))
			inactive, inactiveOK := tt.Find(s.tr("Inactive"))
			if !footerOK || !scopeOK || !inactiveOK || footer.Y < float32(height-70) || scope.Y <= footer.Y+footer.H || scope.Y+scope.H > float32(height-nativePreviewEdgeInset) || inactive.X+inactive.W > 600-nativePreviewEdgeInset-nativePreviewReadingInset {
				t.Fatalf("paper contents displaced protected metadata: footer=%+v scope=%+v inactive=%+v", footer, scope, inactive)
			}
			if err := tt.Click(s.tr("Source")); err != nil {
				t.Fatal(err)
			}
			after, ok := tt.Find(s.tr("Markdown"))
			if !ok || after != footer {
				t.Fatal("switching paper presentation moved its fixed footer")
			}
		}
	}
}
