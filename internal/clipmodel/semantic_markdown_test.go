package clipmodel

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMarkdownRecognizesStructuredDocuments(t *testing.T) {
	for _, input := range []string{
		"# Heading\n\nBody with **strong** and *emphasis*.",
		"Heading\n=======\n\nParagraph.",
		"- First\n- Second", "1. First\n2. Second",
		"> A quoted paragraph", "- [x] Completed", "**Strong text**",
		"Use `code` and *emphasis*.", "~~old~~ and **new**",
		"Read [the documentation](https://example.com/docs).",
		"```go\nfmt.Println(42)\n```", "~~~\ncode\n~~~",
		"| Name | State |\n| --- | --- |\n| A | Ready |",
		"![An image](https://example.com/image.png)",
	} {
		t.Run(input, func(t *testing.T) {
			value := AnalyzeMarkdown(input)
			if value == nil || len(value.Blocks) == 0 || value.Summary == "" {
				t.Fatalf("not recognized: %+v", value)
			}
			if analysis := Analyze(input); analysis.Type != TypeMarkdown || analysis.Markdown == nil {
				t.Fatalf("wrong classification: %+v", analysis)
			}
		})
	}
}

func TestMarkdownRejectsWeakOrUnboundedSignals(t *testing.T) {
	for _, input := range []string{
		"plain text", "#tag", "*", "#", "---", "-123", "- one item", "> 42",
		"a * b", "one *word*", "`only code`", "    indented text", "#!/bin/sh\necho hi",
		"```go\nunclosed", "~~~\nunclosed", "```", "~~~", "<div>HTML only</div>", "[broken](",
		"# title\x00", "# " + strings.Repeat("a", maxMarkdownBytes),
		"-----BEGIN PRIVATE KEY-----\n-----\n**malformed**\n-----END PRIVATE KEY-----",
		"# Title\n" + strings.Repeat("text\n", 2049), strings.Repeat("> ", 40) + "nested",
	} {
		t.Run(input[:min(40, len(input))], func(t *testing.T) {
			if got := AnalyzeMarkdown(input); got != nil {
				t.Fatalf("weak or unbounded syntax recognized: %+v", got)
			}
		})
	}
}

func TestMarkdownReadingDataKeepsStylesWithoutExecutableContent(t *testing.T) {
	raw := "# Header\n\n**bold** *italic* `a < b` ~~gone~~ &amp; \\*literal\\* [site](https://example.com)\n\n![Alt text](https://example.com/tracker.png)\n\n<script>alert(1)</script>\n\n- [x] Done\n- [ ] Waiting"
	value := AnalyzeMarkdown(raw)
	if value == nil {
		t.Fatal("missing document")
	}
	var all string
	var bold, italic, code, strike, link, image, literalHTML bool
	for _, block := range value.Blocks {
		literalHTML = literalHTML || block.Kind == "literal"
		for _, span := range block.Spans {
			all += span.Text
			bold = bold || span.Strong
			italic = italic || span.Emphasis
			code = code || span.Code && span.Text == "a < b"
			strike = strike || span.Strike
			link = link || span.Link && span.Text == "site"
			image = image || span.Image && span.Text == "Alt text"
		}
	}
	if !bold || !italic || !code || !strike || !link || !image || !literalHTML {
		t.Fatalf("missing native formatting: %+v", value)
	}
	if !strings.Contains(all, "& *literal*") || !strings.Contains(all, "☑ Done") || !strings.Contains(all, "☐ Waiting") {
		t.Fatalf("wrong readable content: %q", all)
	}
	if strings.Contains(all, "tracker.png") {
		t.Fatal("image destination replaced its alt text")
	}
}

func TestMarkdownRecognitionPreservesSourceAndScope(t *testing.T) {
	raw := "  # Original\n\n- First\n- **Second**\n"
	store := NewStore(1)
	allowed := []string{"com.example.editor"}
	item, ok := store.CaptureProtectedContent(Content{Text: raw}, "Fixture", allowed)
	if !ok || item.Type != TypeMarkdown || item.Text != raw || !reflect.DeepEqual(item.AllowedApps, allowed) {
		t.Fatal("Markdown changed source or protection")
	}
}

func TestMarkdownDoesNotOverrideExistingWholeValueTypes(t *testing.T) {
	for _, tc := range []struct {
		text string
		kind ContentType
	}{
		{"sk-abcdefghijkl__mnopqrstuv__wxyz", TypeToken},
		{"/tmp/**important**/notes.md", TypeFilePath},
		{"https://example.com/**important**", TypeLink},
		{"[Read **carefully**](https://example.com)", TypeLink},
		{"aweme://video/123", TypeAppLink},
		{"[Read **carefully**](aweme://video/123)", TypeAppLink},
		{"Verification code: 123456", TypeOTP},
		{"123456", TypeText},
		{"1234", TypeText},
		{`{"title":"**important**"}`, TypeJSON},
	} {
		if got := Analyze(tc.text); got.Type != tc.kind {
			t.Errorf("%q classified as %s, want %s", tc.text, got.Type, tc.kind)
		}
	}
}

func TestMarkdownKeepsPathAndURLListsLiteralAcrossCaptureAndRestore(t *testing.T) {
	for _, tc := range []struct {
		name, format, encoded, text string
	}{
		{"Finder paths", FileListFormat, "file:///tmp/**one**.txt\nfile:///tmp/two.txt", "/tmp/**one**.txt\n/tmp/two.txt"},
		{"URI-only list", URIListFormat, "# URLs\r\nhttps://example.com/**one**\r\naweme://video/__two__", "https://example.com/**one**\naweme://video/__two__"},
		{"plain paths", TextFormat, "/tmp/**one**.txt\n/tmp/two.txt", "/tmp/**one**.txt\n/tmp/two.txt"},
		{"plain URLs", TextFormat, "https://example.com/**one**\n\nhttps://example.org/__two__", "https://example.com/**one**\n\nhttps://example.org/__two__"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if Analyze(tc.text).Type != TypeText || AnalyzeMarkdown(tc.text) != nil {
				t.Fatal("location list acquired Markdown formatting")
			}
			data := Representations{{{Format: tc.format, Bytes: []byte(tc.encoded)}}}
			before := data.Clone()
			content, err := ContentFromRepresentations(data)
			if err != nil || content.Text != tc.text {
				t.Fatalf("presentation = %+v, %v", content, err)
			}
			store := NewStore(1)
			item, ok := store.CaptureProtectedContent(content, "Editor", []string{"editor.app"})
			if !ok || item.Type != TypeText || item.Text != tc.text {
				t.Fatalf("captured list = %+v, %v", item, ok)
			}
			// Reclassify records saved by a version that mistakenly interpreted
			// markers inside the locations, without changing source or scope.
			item.Type = TypeMarkdown
			restored, err := RestoreRecords([]Record{{Item: item, Data: data}})
			if err != nil {
				t.Fatal(err)
			}
			got := restored.Items()[0]
			if got.Type != TypeText || got.Text != tc.text || got.ID != item.ID ||
				!reflect.DeepEqual(got.AllowedApps, item.AllowedApps) || !reflect.DeepEqual(data, before) {
				t.Fatal("restoring a location list changed source, identity or protection")
			}
			if _, selected := restored.Current(); selected {
				t.Fatal("reclassification acquired clipboard authority")
			}
		})
	}
	for _, raw := range []string{
		"# Locations\n\n/tmp/**one**.txt\n/tmp/two.txt",
		"- /tmp/**one**.txt\n- /tmp/two.txt",
		"- https://example.com/**one**\n- https://example.org/two",
		"[One](https://example.com)\n[Two](https://example.org)",
	} {
		if Analyze(raw).Type != TypeMarkdown {
			t.Errorf("explicit Markdown structure lost: %q", raw)
		}
	}
}

func TestMarkdownReadingRespectsEscapedAndCompleteEntityReferences(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{`\&copy; &copy; &copy &notit;`, "&copy; © &copy &notit;"},
		{`\&#65; &#65; &#0000065; &#12345678;`, "&#65; A A &#12345678;"},
		{`&#x41; &#x000041; &#x0000041; &#x80;`, "A A &#x0000041; \u0080"},
		{`&amp;copy; \*literal\* &lt;span&gt;`, "&copy; *literal* <span>"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			document := AnalyzeMarkdown("# Entities\n\n" + tc.source)
			if document == nil || len(document.Blocks) != 2 {
				t.Fatalf("unexpected document: %+v", document)
			}
			var got strings.Builder
			for _, span := range document.Blocks[1].Spans {
				got.WriteString(span.Text)
			}
			if got.String() != tc.want {
				t.Fatalf("reading = %q, want %q", got.String(), tc.want)
			}
		})
	}
}

func TestMarkdownRestoreReclassifiesWithoutClipboardAuthority(t *testing.T) {
	raw := "  # Saved document\n\n- One\n- Two\n"
	data := Representations{{{Format: TextFormat, Bytes: []byte(raw)}, {Format: HTMLFormat, Bytes: []byte("<h1>Saved document</h1>")}}}
	before := data.Clone()
	saved := Item{ID: "item-7", Type: TypeText, Text: raw, SourceApp: "Editor", SourceAppID: "com.example.editor", AllowedApps: []string{"com.example.editor"}, CreatedAt: time.Now(), Pinned: true}
	store, err := RestoreRecords([]Record{{Item: saved, Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	item := store.Items()[0]
	if item.Type != TypeMarkdown || item.Text != raw || item.ID != saved.ID || !reflect.DeepEqual(item.AllowedApps, saved.AllowedApps) || !reflect.DeepEqual(data, before) {
		t.Fatal("Markdown restore changed captured data, identity or scope")
	}
	if _, current := store.Current(); current {
		t.Fatal("Markdown restore acquired clipboard authority")
	}
}
