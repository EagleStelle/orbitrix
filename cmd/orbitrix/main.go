// Command orbitrix is the desktop and web (WASM) entry point.
package main

import (
	"flag"
	"image"
	"log"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/client"
	"orbitrix/internal/appicon"
)

func main() {
	fps := flag.Bool("fps", false, "show the frame rate (F3 toggles it)")
	flag.Parse()

	ebiten.SetWindowTitle("Orbitrix")
	if runtime.GOOS != "js" {
		ebiten.SetWindowIcon([]image.Image{appicon.Image(16), appicon.Image(32), appicon.Image(48), appicon.Image(256)})
	}
	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowSizeLimits(1040, 640, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.MaximizeWindow()
	g := client.New()
	g.ShowFPS = *fps
	if err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{X11ClassName: "Orbitrix", X11InstanceName: "orbitrix"}); err != nil {
		log.Fatal(err)
	}
}
