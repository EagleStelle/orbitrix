package client

import (
	"image"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/proto"
)

// field takes text from the device's own keyboard: a physical one, or the on-screen keyboard Android and phone
// browsers show while the field has the focus (Ebitengine's exp/textinput). The page draws the text.
type field struct {
	text    []rune          // what the keyboard has committed
	shown   []rune          // text plus the word the keyboard is still composing: what the page draws
	edits   int             // counts changes to shown, so a page can cache what it draws
	max     int             // in bytes, as the wire counts a name
	keep    func(rune) rune // the rune a typed one becomes, or 0 to drop it
	secret  bool            // a password, which the keyboard must not suggest or learn
	caret   image.Rectangle // where the text goes, so an on-screen keyboard keeps it in view
	focused bool
	ime     textinput.Composer
}

func newField(max int, secret bool, keep func(rune) rune) *field {
	f := &field{max: max, secret: secret, keep: keep}
	// Each commit ends the keyboard's session and the next one starts from the text as the field filtered it.
	f.ime.OnNewSession = func() *textinput.SessionOptions {
		return &textinput.SessionOptions{CaretBounds: f.caret, TextBeforeCaret: string(f.text)}
	}
	f.ime.OnComposition = func(c *textinput.Composition) { f.show(c.Text()) }
	f.ime.OnCommit = func(c *textinput.Commit) {
		if before, after := c.IsSurroundingTextReplaced(); before || after { // e.g. an on-screen keyboard's delete
			b, a := c.SurroundingText()
			f.text = f.text[:0]
			f.add(b + c.Text() + a)
		} else {
			f.add(c.Text())
		}
		f.show("")
	}
	f.ime.OnEndByUser = func() { f.focused = false } // the player closed the on-screen keyboard
	return f
}

// codeField takes a room code: the code alphabet, in capitals.
func codeField() *field {
	return newField(proto.CodeLen, false, func(r rune) rune {
		if r = unicode.ToUpper(r); strings.ContainsRune(proto.CodeAlphabet, r) {
			return r
		}
		return 0
	})
}

// passwordField takes the host password: any printable character.
func passwordField() *field { return newField(maxPassword, true, printable) }

// nameField takes a player's name: any printable character.
func nameField() *field { return newField(maxName, false, printable) }

func printable(r rune) rune {
	if unicode.IsPrint(r) {
		return r
	}
	return 0
}

func (f *field) appendFiltered(dst []rune, s string) []rune {
	n := 0
	for _, r := range dst {
		n += utf8.RuneLen(r)
	}
	for _, r := range s {
		if r = f.keep(r); r != 0 && n+utf8.RuneLen(r) <= f.max {
			dst = append(dst, r)
			n += utf8.RuneLen(r)
		}
	}
	return dst
}

func (f *field) add(s string) { f.text = f.appendFiltered(f.text, s) }

// show sets what the page draws: the text, then what the keyboard is composing.
func (f *field) show(composing string) {
	f.edits++
	f.shown = f.appendFiltered(append(f.shown[:0], f.text...), composing)
}

// set replaces the text, between the keyboard's sessions.
func (f *field) set(s string) {
	f.ime.Confirm()
	f.text = f.text[:0]
	f.add(s)
	f.show("")
}

// value commits the word the keyboard is still composing and returns the text.
func (f *field) value() string {
	f.ime.Confirm()
	return string(f.text)
}

// tap focuses the field when the pointer goes down on r (a browser opens its on-screen keyboard only for a touch
// still under way), and on a touch screen gives the keyboard back when a tap lands elsewhere.
func (f *field) tap(a *app, r ui.Rect) {
	switch {
	case a.in.pressed && a.over(r):
		f.focused = true
	case a.in.clicked && a.in.finger && !a.over(r):
		f.focused = false
	}
}

// focus gives the field the device's keyboard, or takes it back, keeping what the keyboard had composed.
func (f *field) focus(a *app, on bool) {
	f.focused = on
	if !on {
		f.ime.Confirm()
		if a.typing == f {
			a.typing = nil
		}
		return
	}
	if a.typing == f {
		return
	}
	if a.typing != nil {
		a.typing.focus(a, false)
	}
	a.typing = f
	if platform.Keyboard != nil {
		platform.Keyboard(f.secret)
	}
}

// update takes this update's typing while the field has the focus, and reports Enter. A page sets focused (when it
// opens, or through tap) and update hands the keyboard over; the player closing the on-screen keyboard clears it.
func (f *field) update(a *app) (enter bool) {
	if f.focus(a, f.focused); !f.focused {
		return false
	}
	handled, err := f.ime.Update()
	if err != nil {
		f.focus(a, false) // the keyboard failed; a tap on the field tries again
		return false
	}
	if handled {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) || repeating(ebiten.KeyBackspace) {
		f.ime.Confirm() // the text changes only between the keyboard's sessions
		f.text = f.text[:max(0, len(f.text)-1)]
		f.show("")
	}
	return inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

// repeating reports a held key's auto-repeat, after half a second, ten a second.
func repeating(key ebiten.Key) bool {
	d := inpututil.KeyPressDuration(key)
	return d > 30 && d%6 == 0
}
