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
	pressed, lock, cracks           bool
	portalAxes                      core.PortalAxes
}

// Bound both texture memory and resize history. Large, sparse boards can draw
// directly; dense boards reuse these small sprites instead of rasterizing paths.
var mapSprites = make(map[mapSpriteKey]*ebiten.Image)
var mapSpritePixels int

func drawMapSprite(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, key mapSpriteKey) {
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
			renderMapCracks(dst, r, key.durability, sc.Ground)
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
	op.GeoM.Translate(r.X-pad, r.Y-pad)
	op.Filter = ebiten.FilterLinear
	if key.corner != 0 {
		ui.Tint(&op.ColorScale, mapArtWallColor(sc, key.terrain))
	} else if key.cracks {
		ui.Tint(&op.ColorScale, sc.Ground)
	}
	dst.DrawImage(img, &op)
}

func renderMapSprite(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, key mapSpriteKey) {
	switch {
	case key.corner != 0:
		renderMapWallCorner(dst, r, ui.PureWhite, int(key.corner-1))
	case key.cracks:
		renderMapCracks(dst, r, key.durability, ui.PureWhite)
	case key.lock:
		_, inner, _, _, width := mapDeviceGeometry(r)
		path := mapDoorLockPath(inner, width, false)
		ink := ui.Palettes[ui.Dark].Tile
		if ui.Luminance(sc.Tile) < .25 {
			ink = ui.Palettes[ui.Light].Tile
		}
		ui.StrokePath(dst, path, width*1.5, PartnerColor(sc, key.group))
		ui.StrokePath(dst, path, width, ink)
	case key.feature != core.NoFeature:
		renderMapFeature(dst, sc, key.feature, key.group, key.portalAxes, r, key.pressed)
	default:
		renderMapCircuit(dst, sc, key.circuit, key.group, r, key.pressed)
	}
}

func drawMapFeature(dst *ebiten.Image, sc *ui.Palette, feature core.Feature, group uint8, axes core.PortalAxes, r ui.Rect, pressed bool) {
	if feature != core.NoFeature {
		drawMapSprite(dst, sc, r, mapSpriteKey{feature: feature, group: group, portalAxes: axes, pressed: pressed})
	}
}

func drawMapCircuit(dst *ebiten.Image, sc *ui.Palette, component core.CircuitKind, group uint8, r ui.Rect, pressed bool) {
	if component != core.NoCircuit {
		drawMapSprite(dst, sc, r, mapSpriteKey{circuit: component, group: group, pressed: pressed})
	}
}
