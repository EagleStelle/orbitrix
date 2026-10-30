// Package client is the Ebitengine game: scenes, rendering, input, audio and settings.
package client

import "github.com/hajimehoshi/ebiten/v2"

// Game is the root ebiten.Game shared by every client target.
type Game struct{}

// New returns the root game.
func New() *Game { return &Game{} }

// Update advances the game by one tick.
func (g *Game) Update() error { return nil }

// Draw renders the current frame.
func (g *Game) Draw(screen *ebiten.Image) {}

// Layout uses the window size as the logical screen size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
