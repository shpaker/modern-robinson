// Command minigames runs the six minigames on their own, without the adventure
// around them. It is a debugging aid and ships in no release.
//
//	go run ./cmd/minigames           menu: a key 0-5 or a click starts a game
//	go run ./cmd/minigames -game 4   straight into the organ
//
// The game folder is found the way the game finds it: a path argument, the
// working directory, next to the binary or the repository's extracted copy.
// ROBINSON_MINIGAME stands in for -game when there are no arguments to pass
// (the headless drivers run the binary bare).
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/app"
	"github.com/shpaker/modern-robinson/internal/minigame"
	"github.com/shpaker/modern-robinson/internal/minigame/catalog"
	"github.com/shpaker/modern-robinson/internal/repositories"
)

// Menu layout: one row per game.
const (
	rowX    = 72
	rowY    = 120
	rowStep = 36
	rowW    = 496
)

var (
	bg     = color.RGBA{24, 20, 12, 255}
	ink    = color.RGBA{230, 214, 170, 255}
	dim    = color.RGBA{150, 136, 100, 255}
	accent = color.RGBA{255, 200, 90, 255}
)

// runner shows the menu and hosts whichever game is running.
type runner struct {
	host minigame.Host
	cur  minigame.Game
	id   int
	last string // how the previous game ended
}

func (r *runner) Update() error {
	if r.cur != nil {
		done, result := r.cur.Update(1 / float64(ebiten.TPS()))
		if done {
			r.cur = nil
			r.last = fmt.Sprintf("%s: результат %d", catalog.Games[r.id].Name,
				result)
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	for id := range catalog.Games {
		if inpututil.IsKeyJustPressed(ebiten.Key0+ebiten.Key(id)) ||
			inpututil.IsKeyJustPressed(ebiten.KeyNumpad0+ebiten.Key(id)) {
			r.start(id)
			return nil
		}
	}
	if minigame.Clicked() {
		x, y := ebiten.CursorPosition()
		for id := range catalog.Games {
			if minigame.In(rowRect(id), x, y) {
				r.start(id)
				return nil
			}
		}
	}
	return nil
}

// start opens a game with the quest value it can be won with.
func (r *runner) start(id int) {
	e := catalog.Games[id]
	r.id = id
	r.cur = e.New(r.host, e.Param)
	if r.cur == nil {
		r.last = e.Name + ": нет ресурсов " + e.Pack + ".DAT"
	}
}

func rowRect(id int) image.Rectangle {
	y := rowY + id*rowStep
	return image.Rect(rowX-8, y-6, rowX+rowW, y+rowStep-10)
}

func (r *runner) Draw(screen *ebiten.Image) {
	if r.cur != nil {
		r.cur.Draw(screen)
		return
	}
	screen.Fill(bg)
	adapters.DrawText(screen, "Мини-игры «Нового Робинзона»", rowX, 56, ink)
	x, y := ebiten.CursorPosition()
	for id, e := range catalog.Games {
		clr := ink
		if minigame.In(rowRect(id), x, y) {
			clr = accent
		}
		ry := float64(rowY + id*rowStep)
		adapters.DrawText(screen, strconv.Itoa(id), rowX, ry, clr)
		adapters.DrawText(screen, e.Name, rowX+32, ry, clr)
		info := e.Pack + ".DAT"
		if e.Param != 0 {
			info += "  параметр " + strconv.Itoa(e.Param)
		}
		adapters.DrawText(screen, info, rowX+260, ry, dim)
	}
	if r.last != "" {
		adapters.DrawText(screen, r.last, rowX, 360, accent)
	}
	adapters.DrawText(screen,
		"0–5 или щелчок — начать; Esc в игре — сдаться, в меню — выход",
		rowX, 420, dim)
}

func (*runner) Layout(int, int) (int, int) {
	return minigame.ScreenW, minigame.ScreenH
}

func main() {
	game := flag.Int("game", -1, "start straight in this game (0-5)")
	flag.Parse()
	if *game < 0 {
		if v, err := strconv.Atoi(os.Getenv("ROBINSON_MINIGAME")); err == nil {
			*game = v
		}
	}
	root, ok := app.FindRoot(flag.Arg(0))
	if !ok {
		log.Fatal(
			"game resources not found: run inside the game folder (with DATA/) or pass its path",
		)
	}
	cfg := app.LoadConfig(root)
	audio := adapters.NewAudio(app.SampleRate)
	audio.SetVolume(cfg.Sound)
	audio.SetMusicVolume(cfg.Music)
	r := &runner{host: minigame.NewHost(repositories.NewResources(root), audio)}
	if _, ok := catalog.Get(*game); ok {
		r.start(*game)
	}

	ebiten.SetWindowSize(minigame.ScreenW*cfg.Scale, minigame.ScreenH*cfg.Scale)
	ebiten.SetWindowTitle("Новый Робинзон — мини-игры")
	if err := ebiten.RunGame(r); err != nil {
		log.Fatal(err)
	}
}
