package client

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/proto"
)

// inviteView keeps room access, QR (the link), code, Copy code and Copy link in one surface on every platform.
type inviteView struct {
	qr                     ui.QR
	settings, laidSettings bool
	lv                     ui.Rect
	lu                     float64
	moduleCount            int
	panel, box, code, done ui.Rect
	copies                 [2]ui.Rect // Copy code, Copy link
	opts                   [2]ui.Rect
	titleY                 float64
	face                   *text.GoTextFace
	slot                   float64
	cached                 string
	runes                  []rune
	offset, lastX          float64
	dragging               bool
	slots                  [8]ui.Rect
	content, contentOf     *ebiten.Image
	contentRect            image.Rectangle
	copiedT                [2]int // ticks left showing each copy button's check
	state                  string
	stateWidth             float64
	stateLines             []string
}

func codeSlotSize(u, width float64) float64 {
	return min(56*u, (width-(proto.CodeLen-1)*8*u)/proto.CodeLen)
}

func (v *inviteView) layout(a *app) {
	safe, u := a.safe(), a.u
	if safe == v.lv && a.u == v.lu && v.settings == v.laidSettings && v.moduleCount == v.qr.N {
		return
	}
	v.lv, v.laidSettings = safe, v.settings
	v.lu = a.u
	v.moduleCount = v.qr.N
	pad, gap, button, column := 28*u, 16*u, 52*u, 400*u
	if safe.W < 600*u || safe.H < 400*u {
		pad = 16 * u
	}
	if safe.H < 360*u {
		gap = 12 * u
	}
	column = min(column, max(280*u, (safe.W-32*u-2*pad-gap)*0.55))
	access := 0.0
	if v.settings {
		access = button + gap
	}
	w := min(520*u, safe.W-32*u)
	// Short screens put the QR beside the controls, leaving its full height available for scanning.
	rest := 40*u + gap + access + 56*u + gap + button
	stacked := max(0, min(w-2*pad, safe.H-32*u-2*pad-rest-gap, 340*u))
	side := max(0, min(safe.H-32*u-2*pad, safe.W-32*u-2*pad-gap-column, 340*u))
	// Dense codes need three device pixels per module when the screen can provide them.
	readable := float64(v.qr.N+2*ui.QuietZone) * 3
	if v.qr.N > 0 && side > stacked && readable > side && readable <= safe.H-16*u {
		available := safe.W - 16*u - 2*pad - gap
		if readable+240*u <= available {
			column = min(column, available-readable)
			side = readable
		}
	}
	var col ui.Rect
	if side > stacked {
		h := max(side, rest)
		panelH := min(2*pad+h, max(2*pad+rest, safe.H-16*u))
		v.panel = ui.Rect{X: 0, Y: 0, W: 2*pad + side + gap + column, H: panelH}
		v.panel.X, v.panel.Y = safe.Cx()-v.panel.W/2, safe.Cy()-v.panel.H/2
		v.box = ui.Rect{X: v.panel.X + pad, Y: v.panel.Cy() - side/2, W: side, H: side}
		col = ui.Rect{X: v.box.Right() + gap, Y: v.panel.Cy() - rest/2, W: column, H: rest}
	} else {
		v.panel = ui.Rect{X: safe.Cx() - w/2, Y: 0, W: w, H: 2*pad + stacked + gap + rest}
		v.panel.Y = safe.Cy() - v.panel.H/2
		col = ui.Rect{X: v.panel.X + pad, Y: v.panel.Y + pad, W: w - 2*pad, H: rest + gap + stacked}
		v.box = ui.Rect{X: v.panel.Cx() - stacked/2, Y: col.Y + 40*u + gap + access, W: stacked, H: stacked}
	}
	v.titleY = col.Y + 20*u
	v.opts = [2]ui.Rect{}
	if v.settings {
		ow := (col.W - 8*u) / 2
		v.opts = [2]ui.Rect{{X: col.X, Y: col.Y + 40*u + gap, W: ow, H: button}, {X: col.X + ow + 8*u, Y: col.Y + 40*u + gap, W: ow, H: button}}
	}
	v.done = ui.Rect{X: col.X, Y: col.Bottom() - button, W: col.W - 2*(button+8*u), H: button}
	v.copies[0] = ui.Rect{X: v.done.Right() + 8*u, Y: v.done.Y, W: button, H: button}
	v.copies[1] = ui.Rect{X: v.copies[0].Right() + 8*u, Y: v.done.Y, W: button, H: button}
	v.slot = codeSlotSize(u, col.W)
	rowW := proto.CodeLen*v.slot + (proto.CodeLen-1)*8*u
	v.code = ui.Rect{X: col.Cx() - rowW/2, Y: v.done.Y - gap - v.slot, W: rowW, H: v.slot}
	v.face = ui.NewFace(ui.SrcDisplay, v.slot*0.5, 700)
	v.content = nil
}

func (v *inviteView) setCode(code string, u float64) {
	if code != v.cached {
		v.cached, v.runes, v.offset = code, []rune(code), 0
	}
	v.offset = max(0, min(v.offset, float64(max(0, len(v.runes)-proto.CodeLen))*(v.slot+8*u)))
}

// update reports dismissal; the caller owns the primary action and its settings transaction.
func (v *inviteView) update(a *app, link, code string) (closed bool) {
	v.layout(a)
	v.setCode(code, a.u)
	if a.in.esc || a.in.clicked && !v.panel.Contains(a.in.x, a.in.y) {
		return true
	}
	for i, s := range [2]string{code, link} {
		if v.copiedT[i] > 0 {
			v.copiedT[i]--
		}
		if s != "" && a.clicked(ui.CompactHit(v.copies[i], a.u)) {
			platform.Copy(s, func(copied bool) {
				if copied {
					v.copiedT[i] = 120
				}
			})
		}
	}
	if len(v.runes) > proto.CodeLen {
		if a.in.pressed && a.over(v.code) {
			v.dragging, v.lastX = true, a.in.x
		}
		if v.dragging && a.in.down {
			v.offset -= a.in.x - v.lastX
			v.lastX = a.in.x
		}
		if !a.in.down {
			v.dragging = false
		}
		if a.over(v.code) {
			v.offset -= a.in.wheel * 40 * a.u
		}
		v.setCode(code, a.u)
	}
	return false
}

func (v *inviteView) draw(dst *ebiten.Image, a *app, title, link, code, action string, enabled bool) {
	v.qr.Set(link)
	v.layout(a)
	v.setCode(code, a.u)
	u, sc := a.u, a.sc
	ui.FillRound(dst, v.panel, 28*u, sc.Tile)
	ui.DrawText(dst, title, a.f.Title, (v.done.X+v.copies[1].Right())/2, v.titleY, text.AlignCenter, sc.Ink)
	v.qr.Draw(dst, v.box)
	if v.qr.Err != nil {
		v.drawState(dst, a, "Invite too long to scan")
	}
	r := image.Rect(int(v.code.X), int(v.code.Y), int(math.Ceil(v.code.Right())), int(math.Ceil(v.code.Bottom())))
	if v.contentOf != dst || v.contentRect != r || v.content == nil {
		v.contentOf, v.contentRect = dst, r
		v.content = dst.SubImage(r).(*ebiten.Image)
	}
	pitch := v.slot + 8*u
	first := int(v.offset / pitch)
	count := min(len(v.slots), max(proto.CodeLen, len(v.runes))-first)
	for i := range count {
		v.slots[i] = ui.Rect{X: v.code.X + float64(first+i)*pitch - v.offset, Y: v.code.Y, W: v.slot, H: v.slot}
	}
	var visible []rune
	if first < len(v.runes) {
		visible = v.runes[first:min(len(v.runes), first+count)]
	}
	drawCodeSlots(v.content, a, v.slots[:count], v.face, visible, false)
	if len(v.runes) > proto.CodeLen {
		track := v.code.W
		thumb := track * proto.CodeLen / float64(len(v.runes))
		thumb = max(20*u, thumb)
		maxOffset := float64(len(v.runes)-proto.CodeLen) * pitch
		ui.FillRound(dst, ui.Rect{X: v.code.X + (track-thumb)*v.offset/maxOffset, Y: v.code.Bottom() + 3*u, W: thumb, H: 3 * u}, 1.5*u, sc.Dim)
	}
	drawCopyButton(dst, a, v.copies[0], false, code != "", v.copiedT[0] > 0)
	drawCopyButton(dst, a, v.copies[1], true, link != "", v.copiedT[1] > 0)
	ui.DrawButton(dst, a.ui(), v.done, action, "", ui.ButtonInk, enabled)
}

func (v *inviteView) drawState(dst *ebiten.Image, a *app, state string) {
	if state != v.state || v.stateWidth != v.box.W {
		v.state, v.stateWidth, v.stateLines = state, v.box.W, wrap(state, a.f.Label, v.box.W)
	}
	for i, line := range v.stateLines {
		y := v.box.Cy() + (float64(i)-float64(len(v.stateLines)-1)/2)*24*a.u
		ui.DrawText(dst, line, a.f.Label, v.box.Cx(), y, text.AlignCenter, a.sc.Dim)
	}
}
