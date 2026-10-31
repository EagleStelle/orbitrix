package client

import (
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// TestQueueRound checks the nine-slice shapes: the outer vertices are the rectangle's corners, corners are the radius
// square (clamped to a pill), a border leaves its middle open, a pill skips its empty middle row, a circle draws only
// its corners, empty rectangles draw nothing, and shapes with one radius share a sprite so they batch.
func TestQueueRound(t *testing.T) {
	defer func() { shapeVerts, shapeIdx = shapeVerts[:0], shapeIdx[:0] }()
	img := queueRound(rect{10, 20, 100, 40}, 12, 0, nil, pureWhite)
	v := shapeVerts
	if len(v) != 16 || len(shapeIdx) != 54 {
		t.Fatalf("fill: %d vertices, %d indices", len(v), len(shapeIdx))
	}
	if v[0].DstX != 10 || v[0].DstY != 20 || v[15].DstX != 110 || v[15].DstY != 60 || v[1].DstX != 22 || v[4].DstY != 32 {
		t.Errorf("fill corners: %+v", v)
	}
	if queueRound(rect{0, 0, 30, 30}, 12, 0, nil, pureWhite) != img {
		t.Error("shapes with one radius must share a sprite")
	}
	shapeVerts, shapeIdx = shapeVerts[:0], shapeIdx[:0]
	queueRound(rect{0, 0, 100, 20}, 24, 2, nil, pureWhite)
	if len(shapeIdx) != 36 || shapeVerts[1].DstX != 10 {
		t.Errorf("pill border: %d indices, corner %g", len(shapeIdx), shapeVerts[1].DstX)
	}
	shapeVerts, shapeIdx = shapeVerts[:0], shapeIdx[:0]
	queueRound(rect{0, 0, 30, 30}, 15, 0, nil, pureWhite)
	if len(shapeIdx) != 24 {
		t.Errorf("circle: %d indices, want its four corners", len(shapeIdx))
	}
	if queueRound(rect{0, 0, 0, 10}, 4, 0, nil, pureWhite) != nil {
		t.Error("an empty rectangle must draw nothing")
	}
}

// TestDrawTextCachesLabels checks that a label is laid out once, exactly as text.AppendGlyphs lays it out wherever it
// is drawn, and that the cache starts over at its cap.
func TestDrawTextCachesLabels(t *testing.T) {
	clear(labels)
	defer clear(labels)
	dst, f := ebiten.NewImage(200, 50), newFonts(1)
	drawText(dst, "Start", f.heading, 100, 25, text.AlignCenter, pureWhite)
	drawText(dst, "Start", f.heading, 10.5, 10, text.AlignCenter, pureWhite)
	want := text.AppendGlyphs(nil, "Start", f.heading,
		&text.LayoutOptions{PrimaryAlign: text.AlignCenter, SecondaryAlign: text.AlignCenter})
	got := labels[labelKey{"Start", f.heading, text.AlignCenter}]
	if len(labels) != 1 || len(got) != len(want) || got[0].X != want[0].X || got[0].Y != want[0].Y {
		t.Fatalf("%d labels; glyphs %+v, want %+v", len(labels), got, want)
	}
	for i := range labelCap {
		drawText(dst, strconv.Itoa(i), f.small, 0, 0, text.AlignStart, pureWhite)
	}
	if len(labels) > labelCap {
		t.Errorf("the cache grew past its cap: %d labels", len(labels))
	}
}

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
