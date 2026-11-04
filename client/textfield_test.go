package client

import "testing"

func TestFieldFilters(t *testing.T) {
	for _, c := range []struct {
		name      string
		f         *field
		set, comp string
		text      string // committed
		shown     string // with the composition
	}{
		{"code capitals", codeField(), "k7q", "", "K7Q", "K7Q"},
		{"code drops look-alikes", codeField(), "O0I1-k", "", "K", "K"},
		{"code stops at six", codeField(), "ABCDEFGH", "", "ABCDEF", "ABCDEF"},
		{"code composition", codeField(), "K7", "qx3mz", "K7", "K7QX3M"},
		{"password keeps symbols", passwordField(), "aB3$ x", "", "aB3$ x", "aB3$ x"},
		{"password drops controls", passwordField(), "a\tb\nc", "d", "abc", "abcd"},
		{"name stops at its bytes", nameField(), "Ana✨✨✨✨✨", "", "Ana✨✨✨✨", "Ana✨✨✨✨"},
		{"name composition", nameField(), "Bartholomew", "Smithe", "Bartholomew", "BartholomewSmith"},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.f.set(c.set)
			c.f.show(c.comp)
			if got := string(c.f.text); got != c.text {
				t.Errorf("text %q, want %q", got, c.text)
			}
			if got := string(c.f.shown); got != c.shown {
				t.Errorf("shown %q, want %q", got, c.shown)
			}
		})
	}
}

func TestFieldFocusHandsKeyboardOver(t *testing.T) {
	a := &app{}
	code, pw := codeField(), passwordField()
	code.focus(a, true)
	pw.focus(a, true)
	if a.typing != pw || code.focused {
		t.Fatalf("typing %p (code %p, pw %p), code focused %v", a.typing, code, pw, code.focused)
	}
	pw.focus(a, false)
	if a.typing != nil {
		t.Fatalf("typing %p after blur", a.typing)
	}
}
