// Command orbitrix is the desktop and web (WASM) entry point.
package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/client"
)

func main() {
	ebiten.SetWindowTitle("Orbitrix")
	ebiten.SetWindowSize(1280, 800)
	if err := ebiten.RunGame(client.New()); err != nil {
		log.Fatal(err)
	}
}
