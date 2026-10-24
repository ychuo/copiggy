package nativeui

import (
	"strings"
	"unicode/utf8"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeMarkdownPreviewHeight = 460

// Cards show a few real document blocks at the shared reading sizes. They never
// scale a whole document down or replace its contents with decorative lines.
type nativeMarkdownExcerpt struct {
	Block clipmodel.MarkdownBlock
	Lines int
}

func nativeMarkdownCard(c *ui.Context, item clipmodel.Item, document *clipmodel.MarkdownValue) {
	container := ui.Column(c).Fill().MinHeight(0).Gap(nativeInlineGap).Clip()
	if document == nil {
		document = *ui.Local(container, item.Text, func() *clipmodel.MarkdownValue { return clipmodel.AnalyzeMarkdown(item.Text) })
	}
	container.Children(func() {
		if document == nil {
			nativeNoteCard(c, item)
			return
		}
		for _, excerpt := range nativeMarkdownCardExcerpt(document) {
			nativeMarkdownCardBlock(c, excerpt, nativeMarkdownColors())
		}
	})
}

func nativeMarkdownCardExcerpt(document *clipmodel.MarkdownValue) []nativeMarkdownExcerpt {
	blocks := document.Blocks
	// The approved document card uses its real title and first list entries
	// when present. Introductory prose stays available in the full preview.
	if len(blocks) > 0 && blocks[0].Kind == "heading" {
		outline := []clipmodel.MarkdownBlock{blocks[0]}
		listDepth := -1
		for _, block := range blocks[1:] {
			if block.Kind == "heading" {
				break
			}
			if block.Prefix != "" {
				if listDepth < 0 {
					listDepth = block.Depth
				}
				if block.Depth != listDepth {
					continue
				}
				outline = append(outline, block)
				if len(outline) == 3 {
					break
				}
			}
		}
		if len(outline) > 1 {
			blocks = outline
		}
	}
	remaining := nativeCardSize - 2*nativeCardPadding - nativeCardHeaderHeight - nativeCardFooterHeight - 2*nativeCardContentGap
	var excerpts []nativeMarkdownExcerpt
	for _, block := range blocks {
		if block.Kind == "rule" || len(block.Spans) == 0 && len(block.Rows) == 0 {
			continue
		}
		if len(excerpts) == 3 {
			break
		}
		if len(excerpts) > 0 {
			remaining -= nativeInlineGap
		}
		lineHeight, padding, limit := nativeTextBody+nativeInlineGap, 0, 2
		switch {
		case block.Kind == "heading":
			lineHeight, limit = nativeTextTitle+nativeInlineGap, 1
			if len(document.Blocks) == 1 {
				limit = 2
			}
		case block.Kind == "table":
			limit = min(3, len(block.Rows))
		case block.Kind == "code" || block.Kind == "literal":
			padding, limit = 2*nativeInlineGap, 3
		case block.Prefix != "":
			limit = 1
		case len(document.Blocks) == 1:
			limit = 4
		}
		if block.Quote {
			padding = 2 * nativeInlineGap
		}
		lines := min(limit, (remaining-padding)/lineHeight)
		if lines < 1 {
			break
		}
		excerpts = append(excerpts, nativeMarkdownExcerpt{Block: block, Lines: lines})
		remaining -= lines*lineHeight + padding
	}
	return excerpts
}

func nativeMarkdownCardBlock(c *ui.Context, excerpt nativeMarkdownExcerpt, p nativeColors) {
	block := excerpt.Block
	row := ui.Row(c).FillWidth().Shrink(0).Gap(nativeInlineGap)
	if block.Quote || block.Kind == "code" || block.Kind == "literal" {
		row.Padding(nativeInlineGap).Radius(nativeControlRadius).Background(p.muted.Alpha(.07))
	}
	row.Children(func() {
		if block.Prefix != "" {
			ui.Text(c, block.Prefix).Width(nativeIconSize).Shrink(0).FontSize(nativeTextBody).FontWeight(nativeWeightBody).FixedLineHeight(nativeTextBody + nativeInlineGap).TextColor(p.muted)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			if block.Kind == "table" {
				for index, cells := range block.Rows[:excerpt.Lines] {
					ui.Row(c).FillWidth().Gap(nativeInlineGap).Children(func() {
						for _, cell := range cells[:min(2, len(cells))] {
							ui.Box(c).Width(0).Grow(1).MinWidth(0).Children(func() {
								weight := nativeWeightBody
								if index == 0 {
									weight = nativeWeightTitle
								}
								nativeMarkdownText(c, cell, nativeTextBody, weight, p).FixedLineHeight(nativeTextBody + nativeInlineGap).MaxLines(1)
							})
						}
					})
				}
				return
			}
			size, weight := float32(nativeTextBody), nativeWeightBody
			if block.Kind == "heading" {
				size, weight = nativeTextTitle, nativeWeightTitle
			}
			text := nativeMarkdownText(c, block.Spans, size, weight, p).FixedLineHeight(size + nativeInlineGap).MaxLines(excerpt.Lines)
			if block.Kind == "code" || block.Kind == "literal" {
				text.Font(nativeCodeFont)
			}
		})
	})
}

func (s *application) nativeMarkdownActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	nativeSegmentGroup(c, p).Children(func() {
		for index, label := range []string{"Reading", "Source"} {
			if s.nativeFormatSegment(c, s.tr(label), state.MarkdownSource == (index == 1), p).Clicked() {
				state.MarkdownSource = index == 1
				s.publish()
			}
		}
	})
}

// Both modes are presentation-only: neither text selection nor a Markdown
// library may install a clipboard writer that bypasses the saved protection.
func (s *application) nativeMarkdownPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	ink := nativeMarkdownColors()
	layout := nativePreviewLayout(c)
	document := *ui.Local(layout, item.Text, func() *clipmodel.MarkdownValue { return clipmodel.AnalyzeMarkdown(item.Text) })
	layout.Children(func() {
		nativePreviewToolbarRow(c).Children(func() { s.nativePreviewToolbar(c, item, p) })
		ui.Box(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Background(ink.paper).TextColor(ink.text).Children(func() {
			nativeMarkdownPaperEdgeView(c)
			ui.Column(c).Fill().Padding(nativePreviewReadingInset).Children(func() {
				if state.MarkdownSource || document == nil {
					ui.ScrollBoth(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
						ui.Box(c).Padding(nativePreviewDataInset - nativePreviewReadingInset).Children(func() {
							// Source retains indentation and long lines through the same
							// code inset and horizontal scrolling used for JSON.
							ui.Text(c, item.Text).Font(nativeCodeFont).FontSize(nativeTextBody).FontWeight(nativeWeightBody).LineHeight(1.5)
						})
					})
				} else {
					ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
						ui.Column(c).FillWidth().Padding(nativeToolbarGap, nativePreviewReadingInset, nativeInlineGap).Gap(nativePreviewContentGap).Children(func() {
							for _, block := range document.Blocks {
								nativeMarkdownBlock(c, block, ink)
							}
						})
					})
				}
			})
		})
		ui.Box(c).FillWidth().Shrink(0).Padding(0, nativePreviewReadingInset).Children(func() {
			s.nativePreviewFooter(c, item, p, func() {
				ui.Text(c, s.tr("%d characters", utf8.RuneCountInString(item.Text))).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).FontFeatures("tnum").Grow(1).MinWidth(0).SingleLine()
				ui.Text(c, s.tr("Markdown")).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).SingleLine().Shrink(0)
			})
		})
	})
}

func nativeMarkdownBlock(c *ui.Context, block clipmodel.MarkdownBlock, p nativeColors) {
	left := float32(max(0, block.Depth-1)) * nativePreviewHeroInset
	row := ui.Row(c).FillWidth().Gap(nativeToolbarGap).Padding(0, 0, 0, left)
	if block.Quote {
		row.Padding(nativeToolbarGap, nativePreviewContentGap, nativeToolbarGap, left+nativePreviewContentGap).Radius(nativeControlRadius).Background(p.muted.Alpha(.07))
	}
	row.Children(func() {
		if block.Prefix != "" {
			ui.Text(c, block.Prefix).Width(nativeCompactControl).Shrink(0).FontSize(nativeTextPreview).FontWeight(nativeWeightBody).TextColor(p.muted)
		}
		body := ui.Column(c).Grow(1).MinWidth(0)
		if block.Kind == "code" || block.Kind == "literal" {
			body.Padding(nativeToolbarGap).Radius(nativeControlRadius).Background(p.muted.Alpha(.08))
		}
		body.Children(func() {
			switch block.Kind {
			case "rule":
				ui.Box(c).Height(nativeInlineGap).FillWidth()
			case "table":
				nativeMarkdownTable(c, block.Rows, p)
			default:
				size, weight := float32(nativeTextPreview), nativeWeightBody
				if block.Kind == "heading" {
					size = nativeTextTitle
					weight = nativeWeightTitle
				}
				nativeMarkdownText(c, block.Spans, size, weight, p)
			}
		})
	})
}

func nativeMarkdownText(c *ui.Context, content []clipmodel.MarkdownSpan, size float32, weight int, p nativeColors) ui.Element {
	spans := nativeMarkdownSpans(content, p)
	var text strings.Builder
	for _, span := range content {
		text.WriteString(span.Text)
	}
	language := nativeContentLanguage(text.String())
	return ui.RichText(c, spans...).Font(nativeFontForLanguage(language)).Lang(language).FontSize(size).FontWeight(weight).LineHeight(1.5).FillWidth().Wrap()
}

func nativeMarkdownSpans(content []clipmodel.MarkdownSpan, p nativeColors) []ui.Span {
	spans := make([]ui.Span, 0, len(content))
	for _, part := range content {
		span := ui.Span{Text: part.Text, Italic: part.Emphasis, Strikethrough: part.Strike}
		if part.Strong {
			span.Weight = nativeWeightTitle
		}
		if part.Code {
			span.Font, span.Background, span.Radius = nativeCodeFont, p.muted.Alpha(.08), 2
		}
		if part.Link {
			span.Color = ui.Hex("#376b97")
		}
		if part.Image {
			span.Color, span.Italic = p.muted, true
		}
		spans = append(spans, span)
	}
	return spans
}

func nativeMarkdownTable(c *ui.Context, rows [][][]clipmodel.MarkdownSpan, p nativeColors) {
	columns := 0
	for _, cells := range rows {
		columns = max(columns, len(cells))
	}
	// Wide tables scroll within the reading area instead of shrinking every
	// column until its words become a vertical stack of individual characters.
	ui.ScrollHorizontal(c).FillWidth().Children(func() {
		ui.Column(c).FillWidth().MinWidth(float32(columns * nativeTextPreview * 8)).Gap(2).Children(func() {
			for index, cells := range rows {
				row := ui.Row(c).FillWidth().Gap(nativeToolbarGap).Padding(nativeInlineGap).Radius(nativeControlRadius)
				if index == 0 {
					row.Background(p.muted.Alpha(.08))
				}
				row.Children(func() {
					for _, cell := range cells {
						ui.Box(c).Width(0).Grow(1).MinWidth(nativeTextPreview*8 - nativeToolbarGap).Children(func() {
							weight := nativeWeightBody
							if index == 0 {
								weight = nativeWeightTitle
							}
							nativeMarkdownText(c, cell, nativeTextPreview, weight, p)
						})
					}
				})
			}
		})
	})
}
