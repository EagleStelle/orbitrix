// Command orbitrix is the desktop and web (WASM) entry point.
package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/client"
)

func main() {
	fps := flag.Bool("fps", false, "show the frame rate (F3 toggles it)")
	flag.Parse()

	ebiten.SetWindowTitle("Orbitrix")
	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowSizeLimits(1040, 640, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.MaximizeWindow()
	g := client.New()
	g.ShowFPS = *fps
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
