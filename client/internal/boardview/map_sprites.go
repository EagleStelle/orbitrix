package boardview

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

type mapSpriteKey struct {
	cell                            float64
	ground, tile, ink, dim, partner color.RGBA
	terrain                         core.Terrain
	feature                         core.Feature
	circuit                         core.CircuitKind
	group, durability, corner       uint8
	pressed, lock, cracks, crate    bool  // crate: the faint crate in Floor a Box filled
	squash                          uint8 // a Spring's pad pressed toward its base, in steps of springSquash
	portalAxes                      core.PortalAxes
	arrow                           uint8 // a Reservoir's outlet side, 1 up to 4 left
	shield                          uint8 // a Vault's raised shield, drawn over the orbs in its holder's ink
	holder                          int8
}

// Bound both texture memory and resize history. Large, sparse boards can draw
// directly; dense boards reuse these small sprites instead of rasterizing paths.
var mapSprites = make(map[mapSpriteKey]*ebiten.Image)
var mapSpritePixels int

func drawMapSprite(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, key mapSpriteKey) {
	drawMapSpriteFaded(dst, sc, r, key, 1, 1)
}

// drawMapSpriteFaded draws a sprite scaled about its centre and faded, as a Box falling into the pit it fills.
// shortcut: cells too large for a sprite draw unscaled and opaque; cache them too if that shows.
func drawMapSpriteFaded(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, key mapSpriteKey, scale, alpha float64) {
	key.cell = r.W
	key.ground, key.ink, key.dim, key.partner = sc.Ground, sc.Ink, sc.Dim, PartnerColor(sc, key.group)
	key.tile = sc.Tile
	const pad = 2
	side := int(math.Ceil(r.W)) + 2*pad
	if side > 256 || side <= 2*pad {
		switch {
		case key.corner != 0:
			renderMapWallCorner(dst, r, mapArtWallColor(sc, key.terrain), int(key.corner-1))
		case key.cracks:
			renderMapCracks(dst, r, key.durability, crackInk(sc, key.terrain))
		case key.terrain == core.Sinkhole:
			renderSinkhole(dst, r, sc.Pit)
		default:
			renderMapSprite(dst, sc, r, key)
		}
		return
	}
	img := mapSprites[key]
	if img == nil {
		if len(mapSprites) >= 512 || mapSpritePixels+side*side > 4<<20 {
			for _, cached := range mapSprites {
				cached.Deallocate()
			}
			clear(mapSprites)
			mapSpritePixels = 0
		}
		img = ebiten.NewImage(side, side)
		renderMapSprite(img, sc, ui.Rect{X: pad, Y: pad, W: r.W, H: r.W}, key)
		mapSprites[key] = img
		mapSpritePixels += side * side
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(-pad-r.W/2, -pad-r.W/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(r.Cx(), r.Cy())
	op.Filter = ebiten.FilterLinear
	if key.corner != 0 {
		ui.Tint(&op.ColorScale, mapArtWallColor(sc, key.terrain))
	} else if key.cracks {
		ui.Tint(&op.ColorScale, crackInk(sc, key.terrain))
	} else if key.terrain == core.Sinkhole {
		ui.Tint(&op.ColorScale, sc.Pit)
	}
	op.ColorScale.ScaleAlpha(float32(alpha))
	dst.DrawImage(img, &op)
}

func renderMapSprite(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, key mapSpriteKey) {
	switch {
	case key.corner != 0:
		renderMapWallCorner(dst, r, ui.PureWhite, int(key.corner-1))
	case key.cracks:
		renderMapCracks(dst, r, key.durability, ui.PureWhite)
	case key.terrain == core.Sinkhole:
		renderSinkhole(dst, r, ui.PureWhite)
	case key.shield != 0:
		renderVaultShield(dst, sc, r, sc.Players[key.holder], key.shield)
	case key.feature == core.Spring:
		renderSpring(dst, sc, r, key.arrow, key.squash)
	case key.crate:
		s := r.W * crateFloorScale
		renderCrate(dst, sc, ui.Rect{X: r.Cx() - s/2, Y: r.Cy() - s/2, W: s, H: s}, sc.Tile, ui.Mix(sc.Dim, sc.Pit, .35))
	case key.lock:
		_, inner, _, _, width := mapDeviceGeometry(r)
		path := mapDoorLockPath(inner, width, false)
		ink := otherTheme(sc).Tile
		ui.StrokePath(dst, path, width*1.5, PartnerColor(sc, key.group))
		ui.StrokePath(dst, path, width, ink)
	case key.feature != core.NoFeature:
		renderMapFeature(dst, sc, key.feature, key.group, key.portalAxes, key.arrow, r, key.pressed)
	default:
		renderMapCircuit(dst, sc, key.circuit, key.group, r, key.pressed)
	}
}

// otherTheme is the palette of the theme not in use, light in dark mode and dark in light mode.
func otherTheme(sc *ui.Palette) *ui.Palette {
	if ui.Luminance(sc.Tile) < .25 {
		return &ui.Palettes[ui.Light]
	}
	return &ui.Palettes[ui.Dark]
}

// crackInk cuts Breakable Wall's fracture through to the mat, and Breakable Floor's in the other theme's mat.
func crackInk(sc *ui.Palette, t core.Terrain) color.RGBA {
	if t == core.BreakableFloor {
		return otherTheme(sc).Ground
	}
	return sc.Ground
}

func drawMapFeature(dst *ebiten.Image, sc *ui.Palette, feature core.Feature, group uint8, axes core.PortalAxes, arrow uint8, r ui.Rect, pressed bool) {
	if feature != core.NoFeature {
		drawMapSprite(dst, sc, r, mapSpriteKey{feature: feature, group: group, portalAxes: axes, arrow: arrow, pressed: pressed})
	}
}

func drawMapCircuit(dst *ebiten.Image, sc *ui.Palette, component core.CircuitKind, group uint8, r ui.Rect, pressed bool) {
	if component != core.NoCircuit {
		drawMapSprite(dst, sc, r, mapSpriteKey{circuit: component, group: group, pressed: pressed})
	}
}
