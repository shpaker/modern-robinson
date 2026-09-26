// Command nrobinson is the playable Go/Ebiten remake of "Новый Робинзон".
// Build a standalone binary and drop it into the game folder (with DATA/, *.MV,
// *.DAN); it reads resources from there. Cross-compiles to macOS/Linux/Windows.
//
// With -mcp the hero is played by an MCP client over stdin/stdout: the game
// starts a run at once and quits when the client hangs up.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters/mcp"
	"github.com/shpaker/modern-robinson/internal/app"
	"github.com/shpaker/modern-robinson/internal/repositories"
)

func main() {
	driven := flag.Bool("mcp", false,
		"герой под управлением MCP-клиента через stdin/stdout")
	flag.Parse()
	root, ok := app.FindRoot(flag.Arg(0))
	if !ok {
		log.Fatal(
			"game resources not found: run inside the game folder (with DATA/) or pass its path",
		)
	}
	cfg := app.LoadConfig(root)
	res := repositories.NewResources(root)
	g := app.NewGameWith(res, cfg)
	if *driven {
		hero := g.Control()
		go func() {
			if err := mcp.ServeStdio(
				context.Background(), hero, app.Version,
				app.Roles(root)...,
			); err != nil {
				log.Print(err)
			}
			g.Stop() // the client is gone, and so is its game
		}()
	}
	// The original is a fixed 640x480; the scale only magnifies it.
	ebiten.SetWindowSize(app.ViewW*cfg.Scale, app.ViewH*cfg.Scale)
	ebiten.SetFullscreen(cfg.Fullscreen)
	ebiten.SetWindowTitle("Новый Робинзон — " + app.Version)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
