package clipmodel

import (
	"bufio"
	"html"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	htmlrenderer "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// MarkdownValue is a bounded, display-only document. It contains no executable
// HTML, image loaders, clipboard callbacks or external-action authority.
type MarkdownValue struct {
	Blocks  []MarkdownBlock
	Summary string
}

type MarkdownBlock struct {
	Kind         string
	Level, Depth int
	Quote        bool
	Prefix       string
	Spans        []MarkdownSpan
	Rows         [][][]MarkdownSpan
}

type MarkdownSpan struct {
	Text                                        string
	Strong, Emphasis, Code, Strike, Link, Image bool
}

const maxMarkdownBytes = 64 << 10

// AnalyzeMarkdown requires explicit structure instead of treating all prose as
// Markdown. Parsing and rendering never fetch images or open links. The exact
// saved text remains the sole source for ordinary copying.
func AnalyzeMarkdown(value string) *MarkdownValue {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > maxMarkdownBytes || !utf8.ValidString(value) ||
		strings.ContainsRune(value, 0) || strings.Count(value, "\n") > 2048 {
		return nil
	}
	// An invalid or concatenated PEM candidate must keep the existing plain-text
	// fallback, even if its envelope happens to resemble a Setext heading.
	if strings.HasPrefix(value, "-----BEGIN ") {
		return nil
	}
	if ParseLink(value) != nil || markdownLocationList(value) {
		return nil
	}
	if !strings.ContainsAny(value, "#*_-`~[>|") &&
		!(strings.Contains(value, "\n") && (strings.Contains(value, ". ") || strings.Contains(value, "==="))) {
		return nil
	}
	source := []byte(value)
	parser := goldmark.New(goldmark.WithExtensions(extension.Table, extension.TaskList, extension.Strikethrough)).Parser()
	root := parser.Parse(text.NewReader(source))
	count, depth, inlineSignals, strongSignals := 0, 0, 0, 0
	valid := true
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			depth--
			return ast.WalkContinue, nil
		}
		count++
		depth++
		if count > 4096 || depth > 32 {
			valid = false
			return ast.WalkStop, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			if len(strings.TrimSpace(string(node.Text(source)))) > 0 {
				strongSignals++
			}
		case *ast.List:
			if node.ChildCount() >= 2 {
				strongSignals++
			}
		case *ast.Blockquote:
			if strings.IndexFunc(string(node.Text(source)), unicode.IsLetter) >= 0 {
				strongSignals++
			}
		case *ast.FencedCodeBlock:
			if markdownFenceClosed(node, source) {
				strongSignals++
			}
		case *extast.Table:
			if len(node.Alignments) > 16 || node.ChildCount() > 256 {
				valid = false
				return ast.WalkStop, nil
			}
			strongSignals++
		case *extast.TaskCheckBox, *ast.Image, *ast.Link:
			strongSignals++
		case *ast.Emphasis:
			inlineSignals++
			if node.Level == 2 {
				strongSignals++
			}
		case *ast.CodeSpan, *extast.Strikethrough:
			inlineSignals++
		}
		return ast.WalkContinue, nil
	})
	if !valid || strongSignals == 0 && inlineSignals < 2 {
		return nil
	}
	document := &MarkdownValue{}
	markdownBlocks(document, root, source, 0, false, "")
	var summary strings.Builder
	for _, block := range document.Blocks {
		if summary.Len() > 0 {
			summary.WriteByte('\n')
		}
		for _, span := range block.Spans {
			summary.WriteString(span.Text)
		}
		if block.Kind == "table" && len(block.Rows) > 0 {
			for index, cell := range block.Rows[0] {
				if index > 0 {
					summary.WriteString(" · ")
				}
				for _, span := range cell {
					summary.WriteString(span.Text)
				}
			}
		}
		if summary.Len() >= 512 {
			break
		}
	}
	runes := []rune(strings.TrimSpace(summary.String()))
	if len(runes) > 512 {
		runes = append(runes[:511], '…')
	}
	document.Summary = string(runes)
	return document
}

// Finder paths and complete URL lines remain literal multiline text. Asterisks
// and underscores in their names are data, not Markdown emphasis. Keep this
// decision in the shared parser so capture, restore and rendering agree without
// trusting a caller-supplied content type or adding persisted presentation flags.
func markdownLocationList(value string) bool {
	if !strings.ContainsRune(value, '\n') {
		return false
	}
	count := 0
	for line := range strings.SplitSeq(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if analyzeFilePath(line) == nil {
			link := ParseLink(line)
			if link == nil || link.Title != "" {
				return false
			}
		}
		count++
	}
	return count > 1
}

func markdownFenceClosed(node *ast.FencedCodeBlock, source []byte) bool {
	// Goldmark accepts unfinished fences; they are too weak a recognition signal.
	if node.Lines().Len() == 0 || strings.TrimSpace(string(node.Lines().Value(source))) == "" {
		return false
	}
	end := node.Lines().At(node.Lines().Len() - 1).Stop
	for _, line := range strings.Split(string(source[end:]), "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= 3 && (strings.Trim(line, "`") == "" || strings.Trim(line, "~") == "") {
			return true
		}
	}
	return false
}

func markdownBlocks(document *MarkdownValue, parent ast.Node, source []byte, depth int, quote bool, prefix string) {
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		block := MarkdownBlock{Depth: depth, Quote: quote, Prefix: prefix}
		switch n := node.(type) {
		case *ast.Heading:
			block.Kind, block.Level = "heading", n.Level
			block.Spans = markdownInlines(n, source, MarkdownSpan{})
		case *ast.Paragraph, *ast.TextBlock:
			block.Kind = "paragraph"
			block.Spans = markdownInlines(n, source, MarkdownSpan{})
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			block.Kind = "code"
			block.Spans = []MarkdownSpan{{Text: string(n.Lines().Value(source)), Code: true}}
		case *ast.HTMLBlock:
			block.Kind = "literal"
			literal := string(n.Lines().Value(source))
			if n.HasClosure() {
				literal += string(n.ClosureLine.Value(source))
			}
			block.Spans = []MarkdownSpan{{Text: literal, Code: true}}
		case *ast.ThematicBreak:
			block.Kind = "rule"
		case *ast.Blockquote:
			markdownBlocks(document, n, source, depth, true, prefix)
			continue
		case *ast.List:
			index := n.Start
			for item := n.FirstChild(); item != nil; item = item.NextSibling() {
				marker := "•"
				if n.IsOrdered() {
					marker = strconv.Itoa(index) + "."
				}
				markdownBlocks(document, item, source, depth+1, quote, marker)
				index++
			}
			continue
		case *extast.Table:
			block.Kind = "table"
			for row := n.FirstChild(); row != nil; row = row.NextSibling() {
				var cells [][]MarkdownSpan
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					cells = append(cells, markdownInlines(cell, source, MarkdownSpan{}))
				}
				block.Rows = append(block.Rows, cells)
			}
		default:
			markdownBlocks(document, n, source, depth, quote, prefix)
			continue
		}
		document.Blocks = append(document.Blocks, block)
		prefix = ""
	}
}

func markdownInlines(parent ast.Node, source []byte, style MarkdownSpan) []MarkdownSpan {
	var spans []MarkdownSpan
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		span := style
		switch n := node.(type) {
		case *ast.Text:
			span.Text = string(n.Segment.Value(source))
			if !style.Code && !n.IsRaw() {
				span.Text = markdownReadingText(span.Text)
			}
			if n.HardLineBreak() {
				span.Text += "\n"
			} else if n.SoftLineBreak() {
				span.Text += " "
			}
		case *ast.String:
			span.Text = string(n.Value)
		case *ast.CodeSpan:
			span.Code = true
			spans = append(spans, markdownInlines(n, source, span)...)
			continue
		case *ast.Emphasis:
			if n.Level == 2 {
				span.Strong = true
			} else {
				span.Emphasis = true
			}
			spans = append(spans, markdownInlines(n, source, span)...)
			continue
		case *extast.Strikethrough:
			span.Strike = true
			spans = append(spans, markdownInlines(n, source, span)...)
			continue
		case *ast.Link:
			span.Link = true
			spans = append(spans, markdownInlines(n, source, span)...)
			continue
		case *ast.Image:
			span.Image = true
			children := markdownInlines(n, source, span)
			if len(children) == 0 {
				span.Text = string(n.Destination)
				children = []MarkdownSpan{span}
			}
			spans = append(spans, children...)
			continue
		case *ast.AutoLink:
			span.Text, span.Link = string(n.Label(source)), true
		case *ast.RawHTML:
			span.Text, span.Code = string(n.Segments.Value(source)), true
		case *extast.TaskCheckBox:
			span.Text = "☐ "
			if n.IsChecked {
				span.Text = "☑ "
			}
		default:
			spans = append(spans, markdownInlines(n, source, span)...)
			continue
		}
		spans = append(spans, span)
	}
	return spans
}

func markdownReadingText(value string) string {
	if !strings.ContainsAny(value, "\\&") {
		return value
	}
	// Reuse Goldmark's CommonMark escape/entity rules. Decoding with the HTML
	// package first would consume escaped ampersands and invalid or unterminated
	// references. This writer handles a text span only, emits no markup, and the
	// final unescape merely removes its output escaping for native text rendering.
	var escaped strings.Builder
	writer := bufio.NewWriterSize(&escaped, 64)
	htmlrenderer.DefaultWriter.Write(writer, []byte(value))
	_ = writer.Flush() // strings.Builder writes cannot fail.
	return html.UnescapeString(escaped.String())
}
