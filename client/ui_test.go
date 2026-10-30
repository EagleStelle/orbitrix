package client

import "testing"

func TestQuietButtonHoverMatchesMenu(t *testing.T) {
	for theme, sc := range schemes {
		menuFill, menuInk := buttonColors(&sc, btnTile, true, true)
		for _, style := range []btnStyle{btnGround, btnChosen} {
			fill, ink := buttonColors(&sc, style, true, true)
			if fill != menuFill || ink != menuInk {
				t.Errorf("theme %d style %d hover differs from Settings/Exit/Menu", theme, style)
			}
			coldFill, coldInk := buttonColors(&sc, style, false, false)
			fill, ink = buttonColors(&sc, style, false, true)
			if fill != coldFill || ink != coldInk {
				t.Errorf("theme %d style %d disabled button responds to hover", theme, style)
			}
		}
	}
}
