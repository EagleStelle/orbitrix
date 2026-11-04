package client

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/platform"
	"orbitrix/proto"
)

// inviteView keeps room access, QR (the link), code, Copy code and Copy link in one surface on every platform.
type inviteView struct {
	qr                     qrCode
	settings, laidSettings bool
	lv                     rect
	lu                     float64
	moduleCount            int
	panel, box, code, done rect
	copies                 [2]rect // Copy code, Copy link
	opts                   [2]rect
	titleY                 float64
	face                   *text.GoTextFace
	slot                   float64
	cached                 string
	runes                  []rune
	offset, lastX          float64
	dragging               bool
	slots                  [8]rect
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
	if safe == v.lv && a.u == v.lu && v.settings == v.laidSettings && v.moduleCount == v.qr.n {
		return
	}
	v.lv, v.laidSettings = safe, v.settings
	v.lu = a.u
	v.moduleCount = v.qr.n
	pad, gap, button, column := 28*u, 16*u, 52*u, 400*u
	if safe.w < 600*u || safe.h < 400*u {
		pad = 16 * u
	}
	if safe.h < 360*u {
		gap = 12 * u
	}
	column = min(column, max(280*u, (safe.w-32*u-2*pad-gap)*0.55))
	access := 0.0
	if v.settings {
		access = button + gap
	}
	w := min(520*u, safe.w-32*u)
	// Short screens put the QR beside the controls, leaving its full height available for scanning.
	rest := 40*u + gap + access + 56*u + gap + button
	stacked := max(0, min(w-2*pad, safe.h-32*u-2*pad-rest-gap, 340*u))
	side := max(0, min(safe.h-32*u-2*pad, safe.w-32*u-2*pad-gap-column, 340*u))
	// Dense codes need three device pixels per module when the screen can provide them.
	readable := float64(v.qr.n+2*quietZone) * 3
	if v.qr.n > 0 && side > stacked && readable > side && readable <= safe.h-16*u {
		available := safe.w - 16*u - 2*pad - gap
		if readable+240*u <= available {
			column = min(column, available-readable)
			side = readable
		}
	}
	var col rect
	if side > stacked {
		h := max(side, rest)
		panelH := min(2*pad+h, max(2*pad+rest, safe.h-16*u))
		v.panel = rect{0, 0, 2*pad + side + gap + column, panelH}
		v.panel.x, v.panel.y = safe.cx()-v.panel.w/2, safe.cy()-v.panel.h/2
		v.box = rect{v.panel.x + pad, v.panel.cy() - side/2, side, side}
		col = rect{v.box.right() + gap, v.panel.cy() - rest/2, column, rest}
	} else {
		v.panel = rect{safe.cx() - w/2, 0, w, 2*pad + stacked + gap + rest}
		v.panel.y = safe.cy() - v.panel.h/2
		col = rect{v.panel.x + pad, v.panel.y + pad, w - 2*pad, rest + gap + stacked}
		v.box = rect{v.panel.cx() - stacked/2, col.y + 40*u + gap + access, stacked, stacked}
	}
	v.titleY = col.y + 20*u
	v.opts = [2]rect{}
	if v.settings {
		ow := (col.w - 8*u) / 2
		v.opts = [2]rect{{col.x, col.y + 40*u + gap, ow, button}, {col.x + ow + 8*u, col.y + 40*u + gap, ow, button}}
	}
	v.done = rect{col.x, col.bottom() - button, col.w - 2*(button+8*u), button}
	v.copies[0] = rect{v.done.right() + 8*u, v.done.y, button, button}
	v.copies[1] = rect{v.copies[0].right() + 8*u, v.done.y, button, button}
	v.slot = codeSlotSize(u, col.w)
	rowW := proto.CodeLen*v.slot + (proto.CodeLen-1)*8*u
	v.code = rect{col.cx() - rowW/2, v.done.y - gap - v.slot, rowW, v.slot}
	v.face = newFace(srcDisplay, v.slot*0.5, 700)
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
	if a.in.esc || a.in.clicked && !v.panel.contains(a.in.x, a.in.y) {
		return true
	}
	for i, s := range [2]string{code, link} {
		if v.copiedT[i] > 0 {
			v.copiedT[i]--
		}
		if s != "" && a.clicked(compactHit(v.copies[i], a.u)) {
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
	v.qr.set(link)
	v.layout(a)
	v.setCode(code, a.u)
	u, sc := a.u, a.sc
	fillRound(dst, v.panel, 28*u, sc.tile)
	drawText(dst, title, a.f.title, (v.done.x+v.copies[1].right())/2, v.titleY, text.AlignCenter, sc.ink)
	v.qr.draw(dst, v.box)
	if v.qr.err != nil {
		v.drawState(dst, a, "Invite too long to scan")
	}
	r := image.Rect(int(v.code.x), int(v.code.y), int(math.Ceil(v.code.right())), int(math.Ceil(v.code.bottom())))
	if v.contentOf != dst || v.contentRect != r || v.content == nil {
		v.contentOf, v.contentRect = dst, r
		v.content = dst.SubImage(r).(*ebiten.Image)
	}
	pitch := v.slot + 8*u
	first := int(v.offset / pitch)
	count := min(len(v.slots), max(proto.CodeLen, len(v.runes))-first)
	for i := range count {
		v.slots[i] = rect{v.code.x + float64(first+i)*pitch - v.offset, v.code.y, v.slot, v.slot}
	}
	var visible []rune
	if first < len(v.runes) {
		visible = v.runes[first:min(len(v.runes), first+count)]
	}
	drawCodeSlots(v.content, a, v.slots[:count], v.face, visible, false)
	if len(v.runes) > proto.CodeLen {
		track := v.code.w
		thumb := track * proto.CodeLen / float64(len(v.runes))
		thumb = max(20*u, thumb)
		maxOffset := float64(len(v.runes)-proto.CodeLen) * pitch
		fillRound(dst, rect{v.code.x + (track-thumb)*v.offset/maxOffset, v.code.bottom() + 3*u, thumb, 3 * u}, 1.5*u, sc.dim)
	}
	drawCopyButton(dst, a, v.copies[0], false, code != "", v.copiedT[0] > 0)
	drawCopyButton(dst, a, v.copies[1], true, link != "", v.copiedT[1] > 0)
	drawButton(dst, a, v.done, action, "", btnInk, enabled)
}

func (v *inviteView) drawState(dst *ebiten.Image, a *app, state string) {
	if state != v.state || v.stateWidth != v.box.w {
		v.state, v.stateWidth, v.stateLines = state, v.box.w, wrap(state, a.f.label, v.box.w)
	}
	for i, line := range v.stateLines {
		y := v.box.cy() + (float64(i)-float64(len(v.stateLines)-1)/2)*24*a.u
		drawText(dst, line, a.f.label, v.box.cx(), y, text.AlignCenter, a.sc.dim)
	}
}
