// Command puzzlemove checks the driver's puzzle move on the island chart
// (ROBINSON_MINIGAME=0) and the hut (1). They pick their pieces by pixel,
// which nothing but a running game loop can read — so not a go test: the
// check runs the game headless under .vmdriver and plays it the way the MCP
// driver does. It feels along the pieces' side of the screen for one to take
// and carries it across, turned once, then another as it lies. Each move must
// be heard taking its piece first, turning it as often as asked and ending in
// the puzzle's answer; a feel that hears nothing is passed over. Last, a
// piece is tried on the way the driver does before letting go: taken by a
// click, brought over by a point that sounds nothing, put down by a click.
// The last
// line it prints is "puzzlemove: ok", or what went wrong.
//
//	just puzzle-move 0
package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/app"
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories"
)

// move is one carry of a check: where to, turned how many times.
type move struct {
	to    image.Point
	turns int
}

// puzzle is how a check plays one of the two.
type puzzle struct {
	name  string
	area  image.Rectangle // where the pieces lie to begin with
	moves []move
	ends  []string // what a piece put down may sound
}

var puzzles = map[string]puzzle{
	"0": {
		"chart", image.Rect(10, 10, 640, 470),
		[]move{{image.Pt(150, 120), 1}, {image.Pt(300, 300), 0}},
		[]string{"положил", "склеилось"},
	},
	"1": {
		"hut", image.Rect(330, 10, 640, 470),
		[]move{{image.Pt(150, 100), 1}, {image.Pt(200, 150), 0}},
		[]string{"встало", "не туда", "вернул"},
	},
}

// exit is the floppy button of both, which a feel must not press.
var exit = image.Rect(565, 406, 633, 472)

// feel is how far apart the points felt for a piece lie.
const feel = 20

func main() {
	p, ok := puzzles[os.Getenv("ROBINSON_MINIGAME")]
	if !ok {
		fail("ROBINSON_MINIGAME: 0 (карта) или 1 (хижина)")
	}
	root, ok := app.FindRoot("")
	if !ok {
		fail("нет папки игры")
	}
	g := app.NewGameWith(repositories.NewResources(root), app.LoadConfig(root))
	hero := g.Control()
	go func() {
		if err := check(context.Background(), hero, p); err != nil {
			fail(p.name + ": " + err.Error())
		}
		fmt.Fprintln(os.Stderr, "puzzlemove: ok")
	}()
	ebiten.SetWindowSize(app.ViewW, app.ViewH)
	if err := ebiten.RunGame(g); err != nil {
		fail(err.Error())
	}
}

// check makes the moves, feeling for a piece before each.
func check(ctx context.Context, hero interfaces.IControl, p puzzle) error {
	pt := p.area.Min
	for _, m := range p.moves {
		for ; ; pt = next(pt, p.area) {
			if pt.Y >= p.area.Max.Y {
				return fmt.Errorf("не нашлось детали, чтобы нести в %v", m.to)
			}
			if pt.In(exit) {
				continue
			}
			o, err := hero.PuzzleMove(ctx, pt.X, pt.Y, m.to.X, m.to.Y,
				m.turns)
			if err != nil {
				return err
			}
			if len(o.Heard) == 0 {
				continue
			}
			fmt.Fprintf(os.Stderr, "puzzlemove: %v -> %v turns %d: %s\n",
				pt, m.to, m.turns, strings.Join(o.Heard, ", "))
			if err := heard(o.Heard, m.turns, p.ends); err != nil {
				return fmt.Errorf("%v -> %v: %w", pt, m.to, err)
			}
			break
		}
	}
	return tryOn(ctx, hero, p, pt)
}

// tryOn takes a piece by a click, feeling on from pt, points it over the
// first move's spot and puts it down there with a second click.
func tryOn(
	ctx context.Context, hero interfaces.IControl, p puzzle, pt image.Point,
) error {
	to := p.moves[0].to
	for ; ; pt = next(pt, p.area) {
		if pt.Y >= p.area.Max.Y {
			return errors.New("не нашлось детали для примерки")
		}
		if pt.In(exit) {
			continue
		}
		o, err := hero.PuzzleClick(ctx, pt.X, pt.Y, false)
		if err != nil {
			return err
		}
		if slices.Equal(o.Heard, []string{"взял"}) {
			break
		}
	}
	o, err := hero.PuzzlePoint(ctx, to.X, to.Y)
	if err != nil {
		return err
	}
	if len(o.Heard) > 0 {
		return fmt.Errorf("примерка в %v прозвучала: %q", to, o.Heard)
	}
	o, err = hero.PuzzleClick(ctx, to.X, to.Y, false)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "puzzlemove: %v примерка -> %v: %s\n",
		pt, to, strings.Join(o.Heard, ", "))
	if len(o.Heard) != 1 || !slices.Contains(p.ends, o.Heard[0]) {
		return fmt.Errorf("после примерки в %v: %q", to, o.Heard)
	}
	return nil
}

// next is the point felt after pt, row by row across the area.
func next(pt image.Point, a image.Rectangle) image.Point {
	if pt.X += feel; pt.X >= a.Max.X {
		pt.X, pt.Y = a.Min.X, pt.Y+feel
	}
	return pt
}

// heard checks what a move sounded: the take, the turns, the answer.
func heard(h []string, turns int, ends []string) error {
	turned := 0
	for _, s := range h {
		if s == "повернул" {
			turned++
		}
	}
	switch {
	case h[0] != "взял":
		return fmt.Errorf("начало не со взятия: %q", h)
	case turned != turns:
		return fmt.Errorf("поворотов %d, а просили %d: %q", turned, turns, h)
	case !slices.Contains(ends, h[len(h)-1]):
		return fmt.Errorf("конец не в %q: %q", ends, h)
	}
	return nil
}

func fail(why string) {
	fmt.Fprintln(os.Stderr, "puzzlemove: "+why)
	os.Exit(1)
}
