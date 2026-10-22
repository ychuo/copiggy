package nativeui

import (
	"encoding/base64"
	"encoding/pem"
	"strings"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// Sensitive content has its own paper, but shares the application's controls.
// Codes are readable at a glance; credentials expose only their identifying
// ends until the user explicitly reveals the exact saved representation.
func nativeSecretCardColors(kind clipmodel.ContentType) nativeColors {
	p := panelColors(false)
	switch kind {
	case clipmodel.TypeOTP:
		p.paper, p.paperText, p.paperMeta = ui.Hex("#eeebfd"), ui.Hex("#262e40"), ui.Hex("#626b83")
		p.icon = ui.Hex("#7a7f96")
	case clipmodel.TypeToken:
		p = panelColors(true)
		p.paper, p.paperText, p.paperMeta = ui.Hex("#303944"), ui.Hex("#dbe2f1"), ui.Hex("#b7c0d1")
	case clipmodel.TypePrivateKey:
		p.paper, p.paperText, p.paperMeta = ui.Hex("#faf3e5"), ui.Hex("#333a46"), ui.Hex("#717780")
		p.icon = ui.Hex("#7a8089")
	}
	p.text, p.muted = p.paperText, p.paperMeta
	return p
}

func nativeSecretPreviewHeight(kind clipmodel.ContentType) int {
	switch kind {
	case clipmodel.TypeOTP:
		return 300
	case clipmodel.TypePrivateKey:
		return 320
	default:
		return 264
	}
}

func nativeSecretArtwork(p *ui.Painter, r ui.Rect, kind clipmodel.ContentType) {
	switch kind {
	case clipmodel.TypeOTP:
		p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#f5f2ff"), To: ui.Hex("#e7e3fb"), Angle: 135}, 0)
	case clipmodel.TypeToken:
		p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#3b444f"), To: ui.Hex("#272d34"), Angle: 135}, 0)
	case clipmodel.TypePrivateKey:
		p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#fffaf0"), To: ui.Hex("#f3e7d1"), Angle: 135}, 0)
		left, right, bottom := r.X, r.X+r.W, r.Y+r.H
		middle, shoulder, point := r.X+r.W/2, r.Y+r.H*.72, r.Y+r.H*.835
		folds := new(ui.Path).MoveTo(left, shoulder).LineTo(middle, point).LineTo(right, shoulder)
		p.StrokePath(folds, 2.8, ui.Hex("#b29b7c").Alpha(.06))
		p.StrokePath(folds, .65, ui.Hex("#c6b699").Alpha(.42))
		p.StrokePath(new(ui.Path).MoveTo(left, shoulder+1).LineTo(middle, point+1).LineTo(right, shoulder+1), .65, ui.RGBA(255, 255, 255, .6))
		front := new(ui.Path).MoveTo(left, bottom).LineTo(middle, point).LineTo(right, bottom).Close()
		p.FillPathGradient(front, ui.LinearGradient{From: ui.Hex("#f6ecd9"), To: ui.Hex("#f0e3cb"), Angle: 180})
		// The corner is folded paper, independent of the actual app lock badge.
		corner := new(ui.Path).MoveTo(right-26, r.Y).LineTo(right-26, r.Y+36).QuadTo(right-26, r.Y+40, right-22, r.Y+44).LineTo(right, r.Y+66).LineTo(right, r.Y).Close()
		p.StrokePath(corner, 3, ui.Hex("#b9a383").Alpha(.05))
		p.FillPathGradient(corner, ui.LinearGradient{From: ui.Hex("#faf1df"), To: ui.Hex("#e8d9bf"), Angle: 135})
		p.StrokePath(new(ui.Path).MoveTo(right-26, r.Y).LineTo(right-26, r.Y+36).QuadTo(right-26, r.Y+40, right-22, r.Y+44).LineTo(right, r.Y+66), .6, ui.Hex("#cab79a").Alpha(.5))
		// A small wax seal occupies the center, leaving the shared pin anchor free.
		seal := ui.Rect{X: middle - 13, Y: point - 7, W: 26, H: 26}
		p.Fill(ui.Rect{X: seal.X, Y: seal.Y + 1, W: seal.W, H: seal.H}, ui.Hex("#b49b81").Alpha(.18), 13)
		p.FillGradient(seal, ui.LinearGradient{From: ui.Hex("#e6d3c1"), To: ui.Hex("#cdb59f"), Angle: 135}, 13)
		p.Stroke(seal, ui.Hex("#bba28b").Alpha(.55), 13, .65)
		p.Stroke(ui.Rect{X: seal.X + 3, Y: seal.Y + 3, W: 20, H: 20}, ui.Hex("#f5e8db").Alpha(.78), 10, .7)
		p.Stroke(ui.Rect{X: seal.X + 4.5, Y: seal.Y + 4.5, W: 17, H: 17}, ui.Hex("#bda48c").Alpha(.45), 8.5, .6)
		mark := new(ui.Path).MoveTo(middle-4, point+6).QuadTo(middle-2, point+2, middle, point+6).QuadTo(middle+2, point+10, middle+4, point+6)
		p.StrokePath(mark, .85, ui.Hex("#ad9278").Alpha(.5))
	}
}

func nativeSecretValid(item clipmodel.Item, value clipmodel.Analysis) bool {
	return value.Type == item.Type && value.Secret != nil && clipmodel.IsSensitiveType(item.Type)
}

func (s *application) nativeSecretCard(c *ui.Context, item clipmodel.Item, value clipmodel.Analysis) {
	ink := nativeSecretCardColors(item.Type)
	nativeCardHero(c).Children(func() {
		if !nativeSecretValid(item, value) {
			ui.Text(c, s.tr("Hidden content")).FontSize(nativeTextMeta).TextColor(ink.muted).SingleLine()
			return
		}
		if item.Type == clipmodel.TypeOTP {
			s.nativeSecretCode(c, *value.Secret, nativeCardSize-2*nativeCardPadding, false, ink)
			return
		}
		partial := nativeSecretPartial(item, value)
		if partial == "" {
			ui.Text(c, s.tr("Hidden content")).FontSize(nativeTextMeta).TextColor(ink.muted).SingleLine()
			return
		}
		ui.Text(c, partial).Font(nativeCodeFont).FontSize(13).FontWeight(nativeWeightValue).TextColor(ink.text).LineHeight(1.3).TextAlign(ui.Center)
	})
}

// Every accepted code length uses equal spacing. Underlines and readable
// digits share these exact centers in both the card and preview.
func nativeSecretCodeCenters(width float32, count int) ([]float32, float32) {
	count = max(4, min(count, 8))
	step := width / float32(count)
	centers := make([]float32, count)
	for i := range centers {
		centers[i] = (float32(i) + .5) * step
	}
	return centers, step
}

func (s *application) nativeSecretActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	if item.Type != clipmodel.TypeOTP {
		s.nativeSecretRevealAction(c, item, p)
	}
}

func (s *application) nativeSecretRevealAction(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	icon, label := "Eye", s.tr("Reveal")
	if state.Revealed {
		icon, label = "EyeSlash", s.tr("Hide")
	}
	if s.nativeActionButton(c, icon, label, p).Disabled(!nativeSecretValid(item, clipmodel.Analyze(item.Text))).Clicked() {
		state.Revealed = !state.Revealed
		s.publish()
	}
}

// A displayed PEM boundary is reconstructed from a successfully parsed,
// allowlisted type. Arbitrary header values and malformed delimiters stay out.
func nativeSecretKeyBoundary(item clipmodel.Item, value clipmodel.Analysis) string {
	if item.Type != clipmodel.TypePrivateKey || !nativeSecretValid(item, value) {
		return ""
	}
	block, rest := pem.Decode([]byte(value.Secret.Value))
	if block == nil || strings.TrimSpace(string(rest)) != "" {
		return ""
	}
	switch block.Type {
	case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY", "DSA PRIVATE KEY", "OPENSSH PRIVATE KEY":
		return block.Type
	default:
		return ""
	}
}

// The same six-character ends identify a credential in its card and preview.
// A fixed-size, multiline mask keeps long content recognizable as a block
// without exposing its interior or encoding the secret's exact length.
// For PEM, use its encoded payload: repeating BEGIN/END would hide the useful
// distinction between keys. Never include PEM headers or the masked middle in
// the view tree or accessibility labels.
func nativeSecretPartial(item clipmodel.Item, value clipmodel.Analysis) string {
	if !nativeSecretValid(item, value) || item.Type == clipmodel.TypeOTP {
		return ""
	}
	raw := value.Secret.Value
	if item.Type == clipmodel.TypePrivateKey {
		if nativeSecretKeyBoundary(item, value) == "" {
			return ""
		}
		block, _ := pem.Decode([]byte(raw))
		raw = base64.StdEncoding.EncodeToString(block.Bytes)
	}
	ends := min(6, (len(raw)-1)/2)
	if ends < 1 {
		return ""
	}
	rows, columns := 3, 16
	if item.Type == clipmodel.TypePrivateKey {
		rows = 4
	}
	lines := make([]string, rows)
	for i := range lines {
		lines[i] = strings.Repeat("•", columns)
	}
	lines[0] = raw[:ends] + strings.Repeat("•", columns-ends)
	lines[rows-1] = strings.Repeat("•", columns-ends) + raw[len(raw)-ends:]
	return strings.Join(lines, "\n")
}

func (s *application) nativeSecretPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	width, height := c.Size()
	value, state := clipmodel.Analyze(item.Text), s.semanticState(item)
	valid := nativeSecretValid(item, value)
	ink := nativeSecretCardColors(clipmodel.TypeToken)
	from, to := ui.Hex("#343d48"), ui.Hex("#29323d")
	if item.Type == clipmodel.TypeOTP {
		ink = nativeSecretCardColors(item.Type)
		from, to = ui.Hex("#faf7ff"), ui.Hex("#eeebfc")
	}
	readingHeight := height - 128
	if item.Type == clipmodel.TypeOTP && len(item.AllowedApps) > 0 {
		readingHeight -= 27
	}
	ui.Column(c).Fill().Padding(12, 8, 8).Gap(8).Children(func() {
		ui.Box(c).Height(nativeControlSize).FillWidth().Padding(0, 14).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(from, to, 135).Padding(18).Gap(12).TextColor(ink.text).Children(func() {
			if !valid {
				ui.Column(c).Grow(1).MinHeight(0).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, s.tr("Hidden content")).FontSize(nativeTextPreview).TextColor(ink.muted)
				})
			} else if item.Type == clipmodel.TypeOTP {
				ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
					ui.Column(c).MinHeight(max(120, readingHeight)).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Gap(24).Children(func() {
						ui.Text(c, s.tr("Verification code")).FontSize(25).FontWeight(nativeWeightTitle).SingleLine().TextAlign(ui.Center)
						s.nativeSecretCode(c, *value.Secret, min(390, width-80), true, ink)
					})
				})
			} else if state.Revealed {
				ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
					ui.Text(c, item.Text).Font(nativeCodeFont).FontSize(14).LineHeight(1.6).FillWidth().Wrap()
				})
			} else {
				ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
					minimum := float32(58)
					if item.Type == clipmodel.TypePrivateKey {
						minimum = 150
					}
					ui.Column(c).MinHeight(max(minimum, readingHeight)).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Gap(14).Children(func() {
						boundary := nativeSecretKeyBoundary(item, value)
						if boundary != "" {
							ui.Text(c, "-----BEGIN "+boundary+"-----").Font(nativeCodeFont).FontSize(13).TextColor(ink.muted).SingleLine().TextAlign(ui.Center)
						}
						ui.Text(c, nativeSecretPartial(item, value)).Font(nativeCodeFont).FontSize(20).FontWeight(nativeWeightValue).TextColor(ink.text).LineHeight(1.35).TextAlign(ui.Center)
						if boundary != "" {
							ui.Text(c, "-----END "+boundary+"-----").Font(nativeCodeFont).FontSize(13).TextColor(ink.muted).SingleLine().TextAlign(ui.Center)
						}
					})
				})
			}
			ui.Row(c).Height(nativeCompactControl).Shrink(0).Gap(nativeToolbarGap).AlignItems(ui.Center).TextColor(ink.muted).Children(func() {
				if item.Type == clipmodel.TypeOTP {
					if s.nativeCopyButton(c, ink).Clicked() {
						s.chooseNative(false)
					}
					label := s.tr("Verification code")
					if valid {
						label = s.tr("%d digits", value.Secret.Length)
					}
					ui.Text(c, label).FontSize(nativeTextMeta).FontWeight(nativeWeightValue).SingleLine()
					ui.Spacer(c)
				} else {
					ui.Box(c).Grow(1).MinWidth(0).Clip().Children(func() { s.nativePreviewScope(c, item, ink) })
					if valid {
						ui.Text(c, s.tr(value.Secret.Kind)).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Shrink(0).SingleLine()
					}
				}
			})
			if item.Type == clipmodel.TypeOTP {
				s.nativePreviewScope(c, item, ink)
			}
		})
	})
}

func (s *application) nativeSecretCode(c *ui.Context, secret clipmodel.SecretValue, width float32, preview bool, ink nativeColors) {
	height, size, lineY, lineWidth := float32(34), float32(22), float32(32), float32(2)
	if preview {
		height, size, lineY, lineWidth = 56, 32, 45, 2.5
	}
	ui.Box(c).Width(width).Height(height).Shrink(0).Label(secret.Value).Draw(func(p *ui.Painter, r ui.Rect) {
		centers, step := nativeSecretCodeCenters(r.W, secret.Length)
		for _, x := range centers {
			p.Fill(ui.Rect{X: r.X + x - step*.34, Y: r.Y + lineY, W: step * .68, H: lineWidth}, ui.Hex("#cac6e7"), lineWidth/2)
		}
	}).Children(func() {
		centers, step := nativeSecretCodeCenters(width, secret.Length)
		for index, digit := range secret.Value {
			ui.Text(c, string(digit)).Absolute().Left(centers[index] - step/2).Top(0).Width(step).Height(lineY - 2).Font(nativeContentFont).FontSize(size).FontWeight(nativeWeightTitle).TextColor(ink.text).FontFeatures("tnum").SingleLine().TextAlign(ui.Center)
		}
	})
}
