package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters/mouse"
	"github.com/shpaker/modern-robinson/internal/minigame"
	"github.com/shpaker/modern-robinson/internal/minigame/catalog"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

// heroGame is a whole game dropped straight into a scene, as ROBINSON_SCENE
// does, with the hero handed to a driver.
func heroGame(t *testing.T, scene string, env map[string]string) *Game {
	t.Helper()
	t.Setenv("ROBINSON_SCENE", scene)
	for k, v := range env {
		t.Setenv(k, v)
	}
	res := repositories.NewResources(testutil.GameRoot(t))
	g := newGame(res, DefaultConfig(), &fakeAudio{})
	g.Control()
	return g
}

// drive runs a driver call against the live loop: the call waits on the
// game, and the game ticks here until it answers.
func drive[T any](
	t *testing.T, g *Game, call func(context.Context) (T, error),
) (T, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	type answer struct {
		v   T
		err error
	}
	ch := make(chan answer, 1)
	go func() {
		v, err := call(ctx)
		ch <- answer{v, err}
	}()
	for {
		select {
		case a := <-ch:
			return a.v, a.err
		case <-ctx.Done():
			t.Fatal("the game never answered")
		default:
		}
		_ = g.Update()
		runtime.Gosched()
	}
}

func sameList(a, b []string) bool {
	return strings.Join(a, "|") == strings.Join(b, "|")
}

// On the beach the hero sees what the player finds with the mouse, by the
// captions the game gives it, left to right — the exit by its arrow rather
// than by its stock "Идти дальше?" — and his things by their bar names. No
// object key or cell leaks into what he knows.
func TestHeroSeesTheBeachByItsCaptions(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	p, err := drive(t, g, g.ctl.Look)
	if err != nil {
		t.Fatal(err)
	}
	if p.Where != types.WhereIsland || p.Busy != "" {
		t.Errorf("where = %q busy = %q", p.Where, p.Busy)
	}
	wantAround := []string{"Кокосы на пальме", "Пальма", "Краб", "Лужа"}
	if got := thingNames(p.Around); !sameList(got, wantAround) {
		t.Errorf("around = %q, want %q", got, wantAround)
	}
	if got := thingNames(p.Exits); !sameList(got, []string{"налево"}) {
		t.Errorf("exits = %q, want the left arrow", got)
	}
	if p.Exits[0].Side != types.SideLeft {
		t.Errorf("the left exit is %q of the hero", p.Exits[0].Side)
	}
	if p.Hands != "Рука" || !sameList(p.Carry, []string{"Панама"}) {
		t.Errorf("hands = %q carry = %q", p.Hands, p.Carry)
	}
	raw, _ := json.Marshal(p)
	for _, s := range g.sceneObjs {
		if strings.Contains(string(raw), `"`+s.ref.Name+`"`) {
			t.Errorf("object key %q leaked into %s", s.ref.Name, raw)
		}
	}
}

// An action is the player's click: the hero walks up, the script plays, and
// the driver hears the line it put on screen.
func TestUseClicksAndHearsTheLine(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "краб", "")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Reacted || !out.Ready {
		t.Errorf("reacted = %v ready = %v", out.Reacted, out.Ready)
	}
	if len(out.Said) == 0 || out.Said[0] != "Он меня чуть не укусил!!!" {
		t.Errorf("said = %q, want the crab's bite, quotes off", out.Said)
	}
	if len(out.Heard) > 0 {
		t.Errorf("heard = %q, want no puzzle sounds on the beach", out.Heard)
	}
	if g.gs.UI["mouse"] != true || g.act != nil {
		t.Error("the action must be over when the call returns")
	}
}

// The hat on the hero himself is ROHAT, through a click on his own cell.
func TestUseOnSelfTakesTheItemFirst(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "себя", "Панама")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Reacted || out.Look.Hands != "Панама" {
		t.Errorf("reacted = %v hands = %q", out.Reacted, out.Look.Hands)
	}
}

// Leaving is a click on the exit: the departure script walks him off and the
// next scene's entry brings him in; the call returns once he is free there.
func TestGoLeavesThroughTheExit(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Go(ctx, "Налево")
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.sceneName != "SCENA1" || !out.Ready {
		t.Fatalf("scene = %s ready = %v", g.sceneName, out.Ready)
	}
	if ch := out.Changes; !ch.NewPlace || len(ch.Appeared) > 0 ||
		len(ch.Gained) > 0 {
		t.Errorf("changes = %+v, want just the new place", ch)
	}
	if len(out.Look.Around) == 0 {
		t.Error("the new place shows nothing around")
	}
	// Only a way out takes a Go.
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Go(ctx, "Нора")
	}); err == nil || !strings.Contains(err.Error(), "выходы") {
		t.Errorf("going into the burrow: %v", err)
	}
}

// A name that is not there is refused with what is.
func TestUnknownTargetListsWhatIsAround(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	_, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Слон", "")
	})
	if err == nil || !strings.Contains(err.Error(), "Краб") {
		t.Fatalf("err = %v, want the list of what is around", err)
	}
	_, err = drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Краб", "Топор")
	})
	if err == nil || !strings.Contains(err.Error(), "Панама") {
		t.Fatalf("err = %v, want what he carries", err)
	}
}

// While a scene plays the hero cannot act — a click then would skip it — and
// Wait sits it out.
func TestBusyHeroIsToldToWait(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	if !g.startObjectAction("crb") {
		t.Fatal("no crab script")
	}
	_, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Пальма", "")
	})
	if err == nil || !strings.Contains(err.Error(), "сцена") {
		t.Fatalf("err = %v, want a refusal while the scene plays", err)
	}
	if g.act == nil {
		t.Fatal("the refusal must not have skipped the scene")
	}
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Wait(ctx, 0)
	})
	if err != nil || !out.Ready || g.act != nil {
		t.Fatalf("wait: %v ready = %v", err, out.Ready)
	}
}

// Taking an item goes through the bar: the arrows scroll until its slot is
// in view, then the slot is clicked.
func TestPickItemScrollsTheBar(t *testing.T) {
	g := heroGame(t, "SCENA0", map[string]string{
		"ROBINSON_ITEMS": "coco,axe,rope,stick,fish",
	})
	scrolls, took := pickOnBar(t, g, "Рыба")
	if g.gs.Active != "fish" || g.invScroll == 0 {
		t.Errorf("active = %q scroll = %d", g.gs.Active, g.invScroll)
	}
	// The bar is seen at every step: one arrow click per beat, the scroll
	// never jumping, and the fish lit only after the last of them.
	for i, s := range scrolls {
		if s != i+1 {
			t.Fatalf("scroll went %v, want one slot per click", scrolls)
		}
	}
	if clicks := len(scrolls) + 1; took < clicks*barBeat {
		t.Errorf("%d clicks in %d ticks, want a beat of %d after each",
			clicks, took, barBeat)
	}
	if _, took := pickOnBar(t, g, "рука"); g.gs.Active != "hand" ||
		g.invScroll != 0 || took == 0 {
		t.Errorf("back to the hand: active = %q scroll = %d in %d ticks",
			g.gs.Active, g.invScroll, took)
	}
	if _, took := pickOnBar(t, g, "рука"); took != 0 {
		t.Errorf("the hand already held took %d ticks", took)
	}
}

// pickOnBar runs the driver's item step tick by tick, as the game loop does,
// and returns the bar's scroll after each change of it and the ticks spent.
func pickOnBar(t *testing.T, g *Game, name string) ([]int, int) {
	t.Helper()
	step := g.ctl.pickItem(name)
	var scrolls []int
	last := g.invScroll
	for n := 0; n < waitMax; n++ {
		done, err := step()
		if err != nil {
			t.Fatal(err)
		}
		if g.invScroll != last {
			last = g.invScroll
			scrolls = append(scrolls, last)
		}
		if done {
			return scrolls, n
		}
	}
	t.Fatal("the item never came to hand")
	return nil, 0
}

// Friday acts through the portrait, and the portrait hands control back.
func TestAskFridayHandsControlBack(t *testing.T) {
	g := heroGame(t, "SCENA0", map[string]string{
		"ROBINSON_VARS": "FridIs=1", "ROBINSON_FRID": "2,3",
	})
	p, err := drive(t, g, g.ctl.Look)
	if err != nil || p.Friday == nil {
		t.Fatalf("Friday unseen: %v %+v", err, p)
	}
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.AskFriday(ctx, "Пальма", "")
	}); err != nil {
		t.Fatal(err)
	}
	if g.gs.ActiveChar != "Roby" {
		t.Errorf("control stayed with %s", g.gs.ActiveChar)
	}
	g.fridHidden = true
	_, err = drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.AskFriday(ctx, "Пальма", "")
	})
	if err == nil {
		t.Error("an unseen Friday cannot be asked")
	}
}

// A puzzle is played by picture: the driver holds the pointer while it clicks,
// and giving up is Esc's result 0; the pointer goes back to the mouse.
func TestPuzzleClickAndGiveUp(t *testing.T) {
	g := heroGame(t, "SCENA0", map[string]string{"ROBINSON_MINIGAME": "5"})
	defer mouse.Release()
	if g.mg == nil {
		t.Skip("no translator assets")
	}
	p, err := drive(t, g, g.ctl.Look)
	if err != nil || p.Where != types.WherePuzzle {
		t.Fatalf("where = %q: %v", p.Where, err)
	}
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Краб", "")
	}); err == nil {
		t.Error("the scene must be out of reach under a puzzle")
	}
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.PuzzleClick(ctx, 320, 240, false)
	}); err != nil {
		t.Fatal(err)
	}
	for range 120 { // a carried piece waits for the next call at the point
		_ = g.Update()
	}
	if x, y := mouse.Position(); !mouse.Held() || x != 320 || y != 240 {
		t.Errorf("pointer held = %v at %d,%d, want it kept at 320,240",
			mouse.Held(), x, y)
	}
	g.gs.SetVar("DebugResult", 7)
	if _, err := drive(t, g, g.ctl.PuzzleGiveUp); err != nil {
		t.Fatal(err)
	}
	if g.mg != nil || g.gs.Var("DebugResult") != 0 {
		t.Errorf("puzzle left: mg = %v result = %d", g.mg != nil,
			g.gs.Var("DebugResult"))
	}
	_ = g.Update()
	if mouse.Held() {
		t.Error("a finished puzzle hands the pointer back")
	}
}

// A puzzle is heard as well as seen: each click answers with what it sounded,
// in the ear's words and only its own, while the sounds still play.
func TestPuzzleClickHearsItsSounds(t *testing.T) {
	g := heroGame(t, "SCENA0", map[string]string{"ROBINSON_MINIGAME": "5"})
	defer mouse.Release()
	if g.mg == nil {
		t.Skip("no translator assets")
	}
	click := func(x, y int) []string {
		t.Helper()
		out, err := drive(t, g,
			func(ctx context.Context) (types.Outcome, error) {
				return g.ctl.PuzzleClick(ctx, x, y, false)
			})
		if err != nil {
			t.Fatal(err)
		}
		return out.Heard
	}
	// The strip's first letter comes to hand; dropped off the text, it goes
	// back to the strip with the error sound.
	if heard := click(15, 36); !sameList(heard, []string{"взял"}) {
		t.Errorf("taking a letter: heard = %q", heard)
	}
	if heard := click(320, 60); !sameList(heard, []string{"ошибка"}) {
		t.Errorf("dropping it off the text: heard = %q", heard)
	}
	var played []string
	for _, key := range g.audio.(*fakeAudio).played {
		if strings.HasPrefix(key, "r_") {
			played = append(played, key)
		}
	}
	if !sameList(played, []string{"r_take.wav", "r_error.wav"}) {
		t.Errorf("played = %q, want both sounds still played", played)
	}
}

// padGame stands for any puzzle played by carrying: a click takes a piece
// up (when there is one to take) and the next puts it down. It notes every
// press it sees and where the pointer was; with quits, a click ends it, as
// the floppy button does, and with grab, a hand on the real mouse takes the
// pointer back as the right button goes.
type padGame struct {
	offers, quits bool
	held          bool
	taken         int
	grab          func()
	presses       []string
}

func (p *padGame) Update(float64) (bool, int) {
	x, y := minigame.Cursor()
	if minigame.RightClicked() {
		p.presses = append(p.presses, fmt.Sprintf("right %d,%d", x, y))
		if p.grab != nil {
			p.grab()
		}
	}
	if !minigame.Clicked() {
		return false, 0
	}
	p.presses = append(p.presses, fmt.Sprintf("left %d,%d", x, y))
	switch {
	case p.held:
		p.held = false
	case p.offers:
		p.held = true
		p.taken++
	}
	return p.quits, 0
}

func (*padGame) Draw(*ebiten.Image) {}

func (p *padGame) Takes() int { return p.taken }

// move runs one puzzle move against the live loop.
func move(
	t *testing.T, g *Game, from, to image.Point, turns int,
) (types.Outcome, error) {
	t.Helper()
	return drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.PuzzleMove(ctx, from.X, from.Y, to.X, to.Y, turns)
	})
}

// A move is the player's clicks made in one call: the piece is taken at
// from, the pointer carries it to to, the right button turns it there as
// many times as asked and a click puts it down, each press seen by the
// puzzle on its own. A first click that takes nothing up — on nothing, or
// putting down what was in hand — ends the move where it was made, and a
// press on the real mouse ends it where the player took over; a move that
// is out of reach is refused before any click.
func TestPuzzleMoveClicksLikeThePlayer(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	defer mouse.Release()
	pad := &padGame{offers: true}
	g.mg = pad
	from, to := image.Pt(10, 20), image.Pt(300, 400)
	out, err := move(t, g, from, to, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"left 10,20", "right 300,400", "right 300,400", "right 300,400",
		"left 300,400",
	}
	if !sameList(pad.presses, want) || pad.held {
		t.Errorf("presses = %q held = %v, want %q", pad.presses, pad.held,
			want)
	}
	if !out.Reacted || out.Look.Where != types.WherePuzzle {
		t.Errorf("reacted = %v where = %q", out.Reacted, out.Look.Where)
	}
	for _, tc := range []struct {
		name         string
		offers, held bool
	}{
		{"nothing to take", false, false},
		{"a piece in hand", true, true},
	} {
		pad.presses, pad.offers, pad.held = nil, tc.offers, tc.held
		if _, err := move(t, g, from, to, 3); err != nil {
			t.Fatal(err)
		}
		if !sameList(pad.presses, []string{"left 10,20"}) || pad.held {
			t.Errorf("%s: pressed %q held = %v, want the first click only",
				tc.name, pad.presses, pad.held)
		}
		if x, y := mouse.Position(); x != from.X || y != from.Y {
			t.Errorf("%s: pointer at %d,%d, want it left at the first "+
				"click", tc.name, x, y)
		}
	}
	pad.presses, pad.offers = nil, true
	pad.grab = func() { g.ctl.drivePointer(true, false) } // a real press
	if _, err := move(t, g, from, to, 3); err != nil {
		t.Fatal(err)
	}
	if !sameList(pad.presses, []string{"left 10,20", "right 300,400"}) ||
		!pad.held || mouse.Held() {
		t.Errorf("taken over: pressed %q held = %v pointer held = %v",
			pad.presses, pad.held, mouse.Held())
	}
	pad.presses, pad.held, pad.grab = nil, false, nil
	for _, bad := range []struct {
		from, to image.Point
		turns    int
	}{
		{from, to, 4},
		{from, to, -1},
		{from, image.Pt(640, 10), 0},
		{image.Pt(-1, 5), to, 0},
	} {
		if _, err := move(t, g, bad.from, bad.to, bad.turns); err == nil {
			t.Errorf("%v -> %v turns %d: not refused", bad.from, bad.to,
				bad.turns)
		}
	}
	if len(pad.presses) > 0 {
		t.Errorf("a refused move pressed %q", pad.presses)
	}
	// The floppy under the first click ends the puzzle: the answer is where
	// the hero is back, as after giving up.
	pad.quits = true
	out, err = move(t, g, from, to, 1)
	if err != nil || g.mg != nil || out.Look.Where != types.WhereIsland {
		t.Errorf("quit by the move: %v mg = %v where = %q", err, g.mg != nil,
			out.Look.Where)
	}
	if _, err := move(t, g, from, to, 0); err == nil {
		t.Error("no puzzle, yet the move went")
	}
}

// openPuzzle is a game with puzzle id open, as the quest opens it.
func openPuzzle(t *testing.T, id int) *Game {
	t.Helper()
	g := heroGame(t, "SCENA0", nil)
	g.gs.SetVar("Param", catalog.Games[id].Param)
	g.startMinigame([]string{strconv.Itoa(id), "Res", "Param"})
	if g.mg == nil {
		t.Skip("no puzzle assets")
	}
	return g
}

// Every puzzle played by carrying takes a piece across in one call — the
// organ's tube and the checker are picked up without a sound, and a turn
// asked of what does not turn changes nothing — and the call hears how it
// ended, the sailor's answer too; a click on nothing moves nothing. The hut
// and the chart pick their pieces by pixel, which a test cannot read before
// the game loop runs: just puzzle-move checks them headless.
func TestPuzzleMoveCarriesAPiece(t *testing.T) {
	for _, tc := range []struct {
		name     string
		game     int
		from, to image.Point
		heard    []string
	}{
		{
			"letter onto a sign", 5, image.Pt(15, 36), image.Pt(58, 99),
			[]string{"взял", "поставил"},
		},
		{"nothing to take", 5, image.Pt(320, 60), image.Pt(58, 99), nil},
		{
			"tube into a mouth", 4, image.Pt(39, 44), image.Pt(35, 435),
			[]string{"до3"},
		},
		{
			"a man a step up", 2, image.Pt(428, 264), image.Pt(471, 221),
			[]string{"ход", "ход"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := openPuzzle(t, tc.game)
			defer mouse.Release()
			out, err := move(t, g, tc.from, tc.to, 1)
			if err != nil {
				t.Fatal(err)
			}
			if !sameList(out.Heard, tc.heard) {
				t.Errorf("heard = %q, want %q", out.Heard, tc.heard)
			}
			end := tc.to
			if tc.heard == nil {
				end = tc.from
			}
			if x, y := mouse.Position(); x != end.X || y != end.Y {
				t.Errorf("pointer at %d,%d, want %v", x, y, end)
			}
		})
	}
}

// A checker stays framed after the call that picked it out. A move whose
// first click frames nothing of its own — here an empty square — leaves the
// board alone rather than moving that man for it; the man is still there to
// move, framed again.
func TestPuzzleMoveTakesAtItsFrom(t *testing.T) {
	g := openPuzzle(t, 2)
	defer mouse.Release()
	man, empty, up := image.Pt(428, 264), image.Pt(471, 264), image.Pt(471, 221)
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.PuzzleClick(ctx, man.X, man.Y, false)
	}); err != nil {
		t.Fatal(err)
	}
	out, err := move(t, g, empty, up, 0)
	if err != nil || len(out.Heard) > 0 {
		t.Errorf("from an empty square: heard = %q (%v), want no move",
			out.Heard, err)
	}
	out, err = move(t, g, man, up, 0)
	if err != nil || !sameList(out.Heard, []string{"ход", "ход"}) {
		t.Errorf("from the man: heard = %q (%v), want his move and the "+
			"answer", out.Heard, err)
	}
}

// clockGame stands for a puzzle that runs in time, as the balloon flies: it
// counts the ticks it has run. A click sets it playing something out by
// itself for a while, as the organ plays its phrase; with wins, what it plays
// out is the win, and the game closes after it.
type clockGame struct {
	ran  int // ticks run
	left int // ticks left of what a click set going
	wins bool
}

// clockPlay is how long a click sets the clock playing out: well past the
// puzzle's answer to the click itself.
const clockPlay = 2 * puzzleTick

func (c *clockGame) Update(float64) (bool, int) {
	c.ran++
	if c.left > 0 {
		c.left--
		return c.left == 0 && c.wins, 1
	}
	if minigame.Clicked() {
		c.left = clockPlay
	}
	return false, 0
}

func (*clockGame) Draw(*ebiten.Image) {}

func (c *clockGame) Busy() bool { return c.left > 0 }

// waitFor runs a wait against the live loop.
func waitFor(t *testing.T, g *Game, seconds float64) types.Outcome {
	t.Helper()
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Wait(ctx, seconds)
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// clickAt runs a puzzle click against the live loop.
func clickAt(t *testing.T, g *Game, x, y int) types.Outcome {
	t.Helper()
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.PuzzleClick(ctx, x, y, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// run ticks the game loop n times with no driver's call running.
func run(g *Game, n int) {
	for range n {
		_ = g.Update()
	}
}

// A driven puzzle waits for the driver's move as for a player's click: its
// time stands between his calls, a look among them, and runs through a wait
// for as long as asked. A press on the real mouse lets it run as it does for
// the player, until the driver's next move or wait takes it back; without a
// driver the puzzle runs as ever.
func TestPuzzleStandsBetweenMoves(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	defer mouse.Release()
	clock := &clockGame{}
	g.mg = clock
	run(g, 60)
	if _, err := drive(t, g, g.ctl.Look); err != nil {
		t.Fatal(err)
	}
	if clock.ran != 0 {
		t.Fatalf("ran %d ticks between calls, want none", clock.ran)
	}
	waitFor(t, g, 1)
	run(g, 60)
	if clock.ran != 60 {
		t.Errorf("a second's wait ran %d ticks, want 60", clock.ran)
	}
	g.ctl.drivePointer(true, false) // a press on the real mouse
	run(g, 30)
	if clock.ran != 90 {
		t.Errorf("taken over by the mouse: ran %d ticks, want 90", clock.ran)
	}
	waitFor(t, g, 0.5) // the mouse's puzzle runs on until the wait comes up
	ran := clock.ran
	run(g, 60)
	if ran < 120 || clock.ran != ran {
		t.Errorf("taken back by a wait: ran %d ticks, then %d more, want "+
			"at least 120 and none after", ran, clock.ran-ran)
	}
	g.ctl.drivePointer(true, false)
	run(g, 30)
	clickAt(t, g, 100, 100)
	ran = clock.ran
	run(g, 60)
	if clock.ran != ran {
		t.Errorf("taken back by a click: ran %d ticks after it, want none",
			clock.ran-ran)
	}
	alone := newGame(repositories.NewResources(testutil.GameRoot(t)),
		DefaultConfig(), &fakeAudio{})
	free := &clockGame{}
	alone.mg = free
	run(alone, 30)
	if free.ran != 30 {
		t.Errorf("without a driver: ran %d ticks, want 30", free.ran)
	}
}

// A key pressed at the window takes the puzzle over as a press on the mouse
// does, within the tick — the puzzle reads it, Esc or the balloon's arrows —
// but leaves the pointer where the driver holds it: a key moves no pointer.
func TestKeyTakesThePuzzleOver(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	defer mouse.Release()
	clock := &clockGame{}
	g.mg = clock
	clickAt(t, g, 100, 100)
	run(g, 30)
	ran := clock.ran
	g.ctl.drivePointer(false, true) // a key on the real keyboard
	g.updateMinigame(1.0 / 60)
	if clock.ran != ran+1 {
		t.Errorf("the key's tick ran %d ticks, want 1", clock.ran-ran)
	}
	run(g, 30)
	if clock.ran != ran+31 {
		t.Errorf("taken over by a key: ran %d ticks, want 31", clock.ran-ran)
	}
	if x, y := mouse.Position(); !mouse.Held() || x != 100 || y != 100 {
		t.Errorf("pointer held = %v at %d,%d, want it kept at 100,100",
			mouse.Held(), x, y)
	}
}

// The player's hold on a puzzle ends with it: the next one a driver opens
// stands between his calls from the start.
func TestPuzzleEndTakesTheHoldAway(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	defer mouse.Release()
	g.mg = &clockGame{}
	g.ctl.drivePointer(true, false) // the player takes the puzzle over
	g.finishMinigame(0)             // and solves it, say
	run(g, 1)
	next := &clockGame{}
	g.mg = next
	run(g, 60)
	if next.ran != 0 {
		t.Errorf("the next puzzle ran %d ticks before a call, want none",
			next.ran)
	}
}

// Giving up waits out what the puzzle plays out by itself, as the player's
// Esc waits between moves: a wait may have stopped in the middle of it. What
// it plays out is given up after, and a win it shows closes it solved.
func TestGiveUpWaitsOutWhatPlaysOut(t *testing.T) {
	for _, tc := range []struct {
		name   string
		wins   bool
		result int
	}{{"a phrase", false, 0}, {"the win", true, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			g := heroGame(t, "SCENA0", nil)
			defer mouse.Release()
			clock := &clockGame{left: clockPlay, wins: tc.wins}
			g.mg, g.mgVar = clock, "Res"
			g.gs.SetVar("Res", 7)
			if _, err := drive(t, g, g.ctl.PuzzleGiveUp); err != nil {
				t.Fatal(err)
			}
			if clock.left > 0 || g.mg != nil ||
				g.gs.Var("Res") != tc.result {
				t.Errorf("given up %d ticks short: puzzle on = %v "+
					"result = %d, want %d", clock.left, g.mg != nil,
					g.gs.Var("Res"), tc.result)
			}
		})
	}
}

// A click answers once the puzzle has played out what it set going, however
// long past its own answer, and one that ends in the win answers once the
// puzzle is closed. What is still playing out keeps the hero busy with it,
// and a plain wait sits it out.
func TestPuzzleAnswersOncePlayedOut(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	defer mouse.Release()
	clock := &clockGame{}
	g.mg = clock
	out := clickAt(t, g, 100, 100)
	if clock.left > 0 || clock.ran < clockPlay || !out.Ready ||
		out.Look.Busy != "" {
		t.Errorf("answered %d ticks short after %d: ready = %v busy = %q",
			clock.left, clock.ran, out.Ready, out.Look.Busy)
	}
	clock.left = clockPlay // set going by the real mouse, say
	if p, err := drive(t, g, g.ctl.Look); err != nil || p.Busy == "" {
		t.Errorf("a look while it plays out: busy = %q (%v)", p.Busy, err)
	}
	if out := waitFor(t, g, 0); clock.left > 0 || !out.Ready {
		t.Errorf("the wait ended %d ticks short: ready = %v", clock.left,
			out.Ready)
	}
	clock.wins = true
	out = clickAt(t, g, 100, 100)
	if g.mg != nil || out.Look.Where != types.WhereIsland || !out.Ready {
		t.Errorf("the win: puzzle on = %v where = %q ready = %v",
			g.mg != nil, out.Look.Where, out.Ready)
	}
}

// Every puzzle tells when it plays something out by itself, and a fresh one
// plays nothing out: it waits for its player — the balloon's flight too, or
// the answer to a move would never come.
func TestEveryPuzzleWaitsForItsPlayer(t *testing.T) {
	for id, e := range catalog.Games {
		t.Run(e.Name, func(t *testing.T) {
			g := openPuzzle(t, id)
			p, ok := g.mg.(minigame.Performer)
			if !ok {
				t.Fatal("it cannot tell when it plays out")
			}
			if p.Busy() {
				t.Error("busy before a move")
			}
		})
	}
}

// The drummer plays the organ's whole phrase over the mouths, a note a beat
// and the dud for an empty mouth: one click on him answers with all fifteen
// heard, by their names, the organ quiet again and waiting. The tube that
// stands in a mouth sounds on every beat of that mouth.
func TestOrganPhraseIsHeardInOneAnswer(t *testing.T) {
	g := openPuzzle(t, 4)
	defer mouse.Release()
	listen := func() types.Outcome {
		t.Helper()
		out := clickAt(t, g, 232, 250)
		if !out.Ready || g.puzzleBusy() {
			t.Errorf("ready = %v busy = %v, want the phrase played out",
				out.Ready, g.puzzleBusy())
		}
		return out
	}
	if out := listen(); !sameList(out.Heard,
		slices.Repeat([]string{"глухо"}, 15)) {
		t.Errorf("an empty organ: heard = %q, want the dud each beat",
			out.Heard)
	}
	duds := 0
	for _, key := range g.audio.(*fakeAudio).played {
		if key == "pipe00.wav" {
			duds++
		}
	}
	if duds != 15 {
		t.Errorf("the dud sounded %d times, want every beat", duds)
	}
	// Tube 2 into the first mouth, tube 0 into the second.
	for _, m := range [][2]image.Point{
		{image.Pt(199, 44), image.Pt(35, 435)},
		{image.Pt(39, 44), image.Pt(110, 435)},
	} {
		if _, err := move(t, g, m[0], m[1], 0); err != nil {
			t.Fatal(err)
		}
	}
	want := strings.Fields("ми3 до3 ми3 до3 глухо ми3 глухо глухо глухо " +
		"глухо глухо глухо до3 до3 глухо")
	if out := listen(); !sameList(out.Heard, want) {
		t.Errorf("two tubes in: heard = %q, want %q", out.Heard, want)
	}
}

// Friday, clicked, whistles his aria: the answer comes once he is done, and
// the aria is heard as one sound, by the notes he whistles.
func TestFridaysAriaIsHeardByItsNotes(t *testing.T) {
	g := openPuzzle(t, 4)
	defer mouse.Release()
	out := clickAt(t, g, 114, 285)
	if want := []string{soundLabel("melody.wav")}; !sameList(out.Heard,
		want) || !out.Ready || g.puzzleBusy() {
		t.Errorf("heard = %q ready = %v busy = %v, want the aria whole",
			out.Heard, out.Ready, g.puzzleBusy())
	}
	if played := g.audio.(*fakeAudio).played; !slices.Contains(played,
		"melody.wav") {
		t.Errorf("played = %q, want the aria still played", played)
	}
}

// A call given up while Friday whistles leaves the organ standing busy with
// the aria, which sounds on meanwhile. The next move waits the rest of it
// out before its clicks, as the player would, and the tube goes in rather
// than the click being lost to an aria no longer heard.
func TestAMoveWaitsOutTheAriaOfACallGivenUp(t *testing.T) {
	g := openPuzzle(t, 4)
	defer mouse.Release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gone := make(chan error, 1)
	go func() {
		_, err := g.ctl.PuzzleClick(ctx, 114, 285, false)
		gone <- err
	}()
	for n := 0; !g.puzzleBusy(); n++ {
		if n > waitMax {
			t.Fatal("Friday never whistled")
		}
		_ = g.Update()
		runtime.Gosched()
	}
	run(g, 2*60) // two seconds into the aria
	cancel()
	for err := error(nil); err == nil; {
		select {
		case err = <-gone:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("the call given up: %v", err)
			}
		default:
			_ = g.Update()
			runtime.Gosched()
		}
	}
	run(g, 1)
	if g.ctl.cur != nil || !g.puzzleBusy() {
		t.Fatalf("call on = %v busy = %v, want the aria left standing",
			g.ctl.cur != nil, g.puzzleBusy())
	}
	out, err := move(t, g, image.Pt(39, 44), image.Pt(35, 435), 0)
	if err != nil || !sameList(out.Heard, []string{"до3"}) || !out.Ready {
		t.Errorf("the next move: heard = %q ready = %v (%v), want the tube in",
			out.Heard, out.Ready, err)
	}
}

// memStore is a save store in memory.
type memStore map[string][]byte

func (m memStore) Read(name string) ([]byte, error) {
	if b, ok := m[name]; ok {
		return b, nil
	}
	return nil, errors.New("no " + name)
}

func (m memStore) Write(name string, b []byte) error {
	m[name] = b
	return nil
}

// Load is out of the story: it restores a slot and returns once the hero
// is free; an empty slot is refused.
func TestLoadRestoresASlot(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	store := memStore{}
	g.saves = store
	sd := g.snapshot()
	sd.Scene = "SCENA3"
	raw, _ := json.Marshal(sd)
	store[slotName(4)] = raw
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Load(ctx, 4)
	})
	if err != nil || g.sceneName != "SCENA3" || !out.Ready {
		t.Fatalf("load: %v scene = %s", err, g.sceneName)
	}
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Load(ctx, 5)
	}); err == nil {
		t.Error("an empty slot must be refused")
	}
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Load(ctx, 12)
	}); err == nil {
		t.Error("slot 12 does not exist")
	}
}

// A driver gets the game from its first run on, not from the boot screens.
func TestControlStartsARun(t *testing.T) {
	res := repositories.NewResources(testutil.GameRoot(t))
	g := newGame(res, DefaultConfig(), &fakeAudio{})
	if g.started {
		t.Fatal("a plain start sits on the boot screens")
	}
	g.Control()
	if g.mode != modeLoading || g.sceneName != "INT0" {
		t.Fatalf("mode = %d scene = %s, want the new run's loading break",
			g.mode, g.sceneName)
	}
	p, err := drive(t, g, g.ctl.Look)
	if err != nil || p.Where != types.WhereScene {
		t.Fatalf("where = %q: %v", p.Where, err)
	}
}

// Stop ends the loop on the next tick.
func TestStopEndsTheLoop(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	g.Stop()
	if err := g.Update(); err == nil {
		t.Fatal("the loop went on after Stop")
	}
}

// The crab is caught the player's way, with the hat taken from the bar and
// aimed at it: the crab leaves the beach and rides in the hat, and the puddle
// takes him back and returns the hat.
func TestCrabCaughtInTheHatAndLetGo(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Краб", "Панама")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sameList(out.Look.Carry, []string{"Краб в шляпе"}) {
		t.Errorf("carry = %q, want the crab in the hat", out.Look.Carry)
	}
	ch := out.Changes
	if !sameList(ch.Gained, []string{"Краб в шляпе"}) ||
		!sameList(ch.Lost, []string{"Панама"}) ||
		!sameList(ch.Vanished, []string{"Краб"}) || ch.NewPlace {
		t.Errorf("changes = %+v", ch)
	}
	for _, s := range thingNames(out.Look.Around) {
		if s == "Краб" {
			t.Error("the caught crab is still on the beach")
		}
	}
	out, err = drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Лужа", "Краб в шляпе")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sameList(out.Look.Carry, []string{"Панама"}) || len(out.Said) == 0 {
		t.Errorf("carry = %q said = %q, want the hat back", out.Look.Carry,
			out.Said)
	}
}

// Changes are compared where the player can see them: the map hides the bar
// and the stage, so opening it loses nothing and nobody; a puzzle shows
// neither.
func TestChangesSeeOnlyWhatIsInView(t *testing.T) {
	beach := glance{scene: "SCENA0", p: types.Percept{
		Where: types.WhereIsland, Hands: "Рука", EmptyHands: true,
		Carry:  []string{"Панама", "Камни", "Камни"},
		Around: []types.Thing{{Name: "Краб"}}, Friday: &types.Companion{},
	}}
	onMap := glance{scene: "MAPSCR", p: types.Percept{
		Where: types.WhereMap, Hands: "Рука", EmptyHands: true,
		Carry: []string{"Панама", "Камни", "Камни"},
	}}
	ch := changes(beach, onMap)
	if !ch.NewPlace || ch.FridayLeft || len(ch.Lost) > 0 ||
		len(ch.Vanished) > 0 {
		t.Errorf("to the map: %+v", ch)
	}
	puzzle := glance{scene: "SCENA0", p: types.Percept{
		Where: types.WherePuzzle,
	}}
	if ch := changes(beach, puzzle); len(ch.Lost) > 0 || ch.NewPlace {
		t.Errorf("into a puzzle: %+v", ch)
	}
	fewer := beach
	fewer.p.Carry = []string{"Панама", "Камни"}
	fewer.p.Hands, fewer.p.EmptyHands = "Камни", false
	if ch := changes(beach, fewer); len(ch.Lost) > 0 || len(ch.Gained) > 0 {
		t.Errorf("taking a stone in hand is no change: %+v", ch)
	}
}

// Tries that come to nothing count up until one lands; waits do not count.
func TestMissesCountTriesInARow(t *testing.T) {
	g := heroGame(t, "SCENA0", nil)
	c := g.ctl
	before := g.glance()
	miss := func(acted, reacted bool) int {
		out := types.Outcome{Reacted: reacted}
		_, _ = c.outcome(&out, &before, acted)()
		return out.Changes.Misses
	}
	if miss(true, false) != 1 || miss(true, false) != 2 {
		t.Fatal("two misses in a row")
	}
	if miss(false, false) != 2 {
		t.Error("a wait is no try")
	}
	if miss(true, true) != 0 {
		t.Error("a try that lands ends the streak")
	}
}

// The rope thrown over the bananas leaves its end in the hero's hand and the
// movie waits for the player's click (ROROPBAN's negative-Delay frame). The
// driver is free to act then, the pause holds for him however long he takes,
// and the end tied to the dead tree brings the rope back.
func TestRopeEndWaitsForTheClick(t *testing.T) {
	g := heroGame(t, "SCENA3", nil)
	g.gs.AddItem("rope")
	out, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Бананы", "Веревка")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Ready || !g.awaitsClick() {
		t.Fatalf("ready = %v, awaits = %v: the rope end is not held for a click",
			out.Ready, g.awaitsClick())
	}
	if g.gs.Active != "rp1" {
		t.Fatalf("in hand %q, want the rope end", g.gs.Active)
	}
	for range 60 * 30 { // half a minute: far past the authored five seconds
		_ = g.Update()
	}
	if !g.awaitsClick() {
		t.Fatal("the pause ran out under the driver")
	}
	out, err = drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Use(ctx, "Сухое дерево", "")
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, tied := g.objCell("t2banan"); !tied || !g.gs.HasItem("rope") ||
		g.gs.HasItem("rp1") || g.awaitsClick() {
		t.Errorf("tied = %v rope = %v rp1 = %v awaits = %v, want the end "+
			"tied (RORP1CTR)", tied, g.gs.HasItem("rope"), g.gs.HasItem("rp1"),
			g.awaitsClick())
	}
	if len(out.Said) == 0 || !out.Ready {
		t.Errorf("said = %q ready = %v", out.Said, out.Ready)
	}
}

// ropeInHand plays the rope onto the bananas for a player at the mouse (no
// driver) until the movie stops for his click.
func ropeInHand(t *testing.T) *Game {
	t.Helper()
	t.Setenv("ROBINSON_SCENE", "SCENA3")
	res := repositories.NewResources(testutil.GameRoot(t))
	g := newGame(res, DefaultConfig(), &fakeAudio{})
	g.gs.AddItem("rope")
	g.gs.Active = "rope"
	if !g.startObjectAction("banana") {
		t.Fatal("no script for the rope on the bananas")
	}
	for i := 0; !g.awaitsClick(); i++ {
		if i > 60*30 {
			t.Fatal("the movie never stopped for the click")
		}
		_ = g.Update()
	}
	return g
}

// At the mouse the click made while the movie waits goes to the scene with
// the rope end, not to skipping the movie.
func TestPlayerTiesTheRopeEndDuringThePause(t *testing.T) {
	g := ropeInHand(t)
	aim, err := g.aimAt("Сухое дерево", false)
	if err != nil {
		t.Fatal(err)
	}
	if !g.clickWorld(aim) || g.act == nil || g.awaitsClick() {
		t.Fatal("the click did not start the tying")
	}
	for i := 0; g.act != nil && i < 60*60; i++ {
		_ = g.Update()
	}
	if _, _, tied := g.objCell("t2banan"); !tied || !g.gs.HasItem("rope") {
		t.Errorf("tied = %v rope = %v, want the end tied (RORP1CTR)", tied,
			g.gs.HasItem("rope"))
	}
}

// Left alone, the pause runs its authored length and the movie takes the
// rope end back, as without a click in the original.
func TestUnansweredPauseTakesTheRopeEndBack(t *testing.T) {
	g := ropeInHand(t)
	for i := 0; g.act != nil && i < 60*30; i++ {
		_ = g.Update()
	}
	if g.act != nil || g.gs.HasItem("rp1") || !g.gs.UI["mouse"] {
		t.Errorf("act = %v rp1 = %v mouse = %v", g.act != nil,
			g.gs.HasItem("rp1"), g.gs.UI["mouse"])
	}
}
