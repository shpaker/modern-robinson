// Command nrobinson is the playable Go/Ebiten remake of "Новый Робинзон".
// Build a standalone binary and drop it into the game folder (with DATA/, *.MV,
// *.DAN); it reads resources from there. Cross-compiles to macOS/Linux/Windows.
package main

import (
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/app"
	"github.com/shpaker/modern-robinson/internal/repositories"
)

func main() {
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	root, ok := app.FindRoot(arg)
	if !ok {
		log.Fatal(
			"game resources not found: run inside the game folder (with DATA/) or pass its path",
		)
	}
	res := repositories.NewResources(root)
	g := app.NewGame(res)
	ebiten.SetWindowSize(
		app.ViewW*2,
		app.ViewH*2,
	) // 640x480 native, 2x for comfort
	ebiten.SetWindowTitle("Новый Робинзон — " + app.Version)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
