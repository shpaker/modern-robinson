package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters/mouse"
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/minigame"
	"github.com/shpaker/modern-robinson/internal/types"
)

// The hero played from outside the game (interfaces.IControl, served over MCP
// by adapters/mcp). A driver call never touches the game from its own
// goroutine: it hands a job to the game loop, which steps it at the top of
// every Update until the job is done. Every action is made of the clicks a
// player would make, through Game.click and the bar, so the quest sees nothing
// it would not see from the mouse — and the driver learns only what is on
// screen and in the ear: captions, item names, lines and a puzzle's sounds,
// never variables, scripts or files.

// Limits of a driver's waits, in ticks of the 60 TPS loop.
const (
	waitMax    = 60 * 60 // an action or a wait gives up after a minute
	settleTick = 15      // free this long before the hero counts as free
	cameraMax  = 3 * 60  // the view gets this long to reach a target
	puzzleTick = 45      // a puzzle's own answer (the checkers AI waits 0.7 s)
	saidMax    = 64      // lines kept per call
	soundMax   = 64      // puzzle sounds kept per call
)

// job is one driver call running on the game loop: its steps run in order,
// each once per tick until it reports done.
type job struct {
	steps []func() (bool, error)
	i     int
	err   error
	done  chan struct{}
	dead  atomic.Bool // the caller stopped waiting
}

// step runs the job for one tick and reports whether it is over. A step that
// finishes hands over to the next within the same tick.
func (j *job) step() bool {
	for j.i < len(j.steps) {
		ok, err := j.steps[j.i]()
		if err != nil {
			j.err = err
			return true
		}
		if !ok {
			return false
		}
		j.i++
	}
	return true
}

// control is the game's side of IControl.
type control struct {
	g    *Game
	mu   sync.Mutex // one driver call at a time
	jobs chan *job
	cur  *job

	said   []string // lines shown since the running call began
	sounds []string // what a puzzle sounded since then, as heard

	// hold is the puzzle pointer the driver holds until a real press.
	hold *heldPointer

	canvas *ebiten.Image // where Sight renders

	misses int // actions in a row that came to nothing
}

// glance is the world as it stood before an action, to tell what changed.
type glance struct {
	p     types.Percept
	scene string
}

// look takes a glance.
func (g *Game) glance() glance { return glance{g.percept(), g.sceneName} }

// heldPointer is the pointer a puzzle click drives.
type heldPointer struct {
	x, y        int
	left, right bool
}

var _ interfaces.IControl = (*control)(nil)

// Control hands the hero to a driver. The first call also starts a run when
// none is underway: the driver plays the hero, not the boot screens and the
// menu. Call it before the game loop starts.
func (g *Game) Control() interfaces.IControl {
	if g.ctl == nil {
		g.ctl = &control{g: g, jobs: make(chan *job, 1)}
		if !g.started {
			g.restart()
		}
	}
	return g.ctl
}

// Stop ends the game loop on its next tick; safe from any goroutine.
func (g *Game) Stop() { g.stopped.Store(true) }

// tick steps the running job and drives the held pointer; the game loop calls
// it first thing every Update.
func (c *control) tick() {
	if c == nil {
		return
	}
	if c.cur == nil {
		select {
		case c.cur = <-c.jobs:
			c.said, c.sounds = nil, nil
		default:
		}
	}
	if j := c.cur; j != nil && (j.dead.Load() || j.step()) {
		close(j.done)
		c.cur = nil
	}
	c.drivePointer()
}

// heard notes a line the game has put on screen.
func (c *control) heard(s string) {
	if c == nil || c.cur == nil || len(c.said) >= saidMax {
		return
	}
	c.said = append(c.said, unquote(s))
}

// sounded notes a sound a puzzle has played, by what the ear makes of it.
func (c *control) sounded(file string) {
	if c == nil || c.cur == nil || len(c.sounds) >= soundMax {
		return
	}
	c.sounds = append(c.sounds, soundLabel(file))
}

// unquote drops the quotes TEXT.DAT keeps for the text box: what was said is
// data, not its print.
func unquote(s string) string { return strings.TrimSpace(strings.Trim(s, `"`)) }

// callMax bounds a call in wall time: a loop that stops ticking (the window
// put away) must not hold the driver forever.
const callMax = 3 * time.Minute

// errStalled is a call the game loop never got round to.
var errStalled = errors.New("игра не отвечает: окно свёрнуто или закрыто?")

// run hands the steps to the game loop and waits until they are through.
func (c *control) run(
	ctx context.Context,
	steps ...func() (bool, error),
) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, callMax)
	defer cancel()
	j := &job{steps: steps, done: make(chan struct{})}
	select {
	case c.jobs <- j:
	case <-ctx.Done():
		return stalled(ctx)
	}
	select {
	case <-j.done:
		return j.err
	case <-ctx.Done():
		j.dead.Store(true)
		return stalled(ctx)
	}
}

// stalled is why a call ended before the game answered.
func stalled(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errStalled
	}
	return ctx.Err()
}

// once is a step that runs a single time.
func once(f func() error) func() (bool, error) {
	return func() (bool, error) { return true, f() }
}

// ticks is a step that lets n ticks go by, ending early once stop says so.
func ticks(n int, stop func() bool) func() (bool, error) {
	left := n
	return func() (bool, error) {
		if left <= 0 || (stop != nil && stop()) {
			return true, nil
		}
		left--
		return false, nil
	}
}

// untilFree is a step that waits for the hero to be free and to stay so for
// a moment — an action's end often hands straight over to the next scene's
// entry, and that gap is no time to act.
func (c *control) untilFree() func() (bool, error) {
	left, calm := waitMax, 0
	return func() (bool, error) {
		if c.g.busyWith() == "" {
			if calm++; calm >= settleTick {
				return true, nil
			}
		} else {
			calm = 0
		}
		if left--; left <= 0 {
			return true, nil
		}
		return false, nil
	}
}

// mark is a step that takes the glance an outcome is measured against.
func (c *control) mark(before *glance) func() (bool, error) {
	return once(func() error {
		*before = c.g.glance()
		return nil
	})
}

// outcome fills in what came of an action once it has played out: what was
// said and heard, whether the hero may act, what he sees now and what changed
// since before. acted counts the call among the hero's own tries (a wait is
// not), and a try that came to nothing adds to the misses in a row.
func (c *control) outcome(
	out *types.Outcome, before *glance, acted bool,
) func() (bool, error) {
	return once(func() error {
		out.Said = append([]string(nil), c.said...)
		out.Heard = append([]string(nil), c.sounds...)
		out.Ready = c.g.busyWith() == ""
		after := c.g.glance()
		out.Look = after.p
		out.Changes = changes(*before, after)
		if acted {
			if out.Reacted || len(out.Said) > 0 {
				c.misses = 0
			} else {
				c.misses++
			}
		}
		out.Changes.Misses = c.misses
		return nil
	})
}

// changes is what the player would notice between two glances. Things around
// and the ways out are compared within one place only, and the rest only
// where the bar and the stage are in view.
func changes(before, after glance) types.Changes {
	var ch types.Changes
	b, a := before.p, after.p
	seen := func(p types.Percept) bool {
		return p.Where == types.WhereIsland || p.Where == types.WhereMap
	}
	if !seen(b) || !seen(a) {
		return ch
	}
	ch.NewPlace = before.scene != after.scene
	ch.Gained, ch.Lost = diffNames(things(b), things(a))
	if !ch.NewPlace {
		ch.Appeared, ch.Vanished = diffNames(
			thingNames(b.Around), thingNames(a.Around))
		ch.Opened, ch.Closed = diffNames(
			thingNames(b.Exits), thingNames(a.Exits))
	}
	if b.Where == types.WhereIsland && a.Where == types.WhereIsland {
		ch.FridayCame = b.Friday == nil && a.Friday != nil
		ch.FridayLeft = b.Friday != nil && a.Friday == nil && !ch.NewPlace
		ch.MapGained = !b.Map && a.Map
	}
	return ch
}

// things is everything the hero has on him: the item in hand and the rest.
func things(p types.Percept) []string {
	out := append([]string(nil), p.Carry...)
	if p.Hands != "" && !p.EmptyHands {
		out = append(out, p.Hands)
	}
	return out
}

func thingNames(list []types.Thing) []string {
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, t.Name)
	}
	return out
}

// diffNames is what the second list has that the first has not, and the
// other way round, counting repeats.
func diffNames(before, after []string) (added, gone []string) {
	count := map[string]int{}
	for _, s := range before {
		count[s]++
	}
	for _, s := range after {
		if count[s] > 0 {
			count[s]--
		} else {
			added = append(added, s)
		}
	}
	for _, s := range before {
		if count[s] > 0 {
			count[s]--
			gone = append(gone, s)
		}
	}
	return added, gone
}

// Look is the world at a glance.
func (c *control) Look(ctx context.Context) (types.Percept, error) {
	var p types.Percept
	err := c.run(ctx, once(func() error {
		p = c.g.percept()
		return nil
	}))
	return p, err
}

// Sight renders what is in view into a PNG.
func (c *control) Sight(ctx context.Context) ([]byte, error) {
	var png []byte
	err := c.run(ctx, once(func() error {
		var err error
		png, err = c.sight()
		return err
	}))
	return png, err
}

// sight renders the view: a puzzle's whole screen, or the scene without the
// bar and the cursor, as tall as the scene reaches.
func (c *control) sight() ([]byte, error) {
	g := c.g
	if c.canvas == nil {
		c.canvas = ebiten.NewImage(ViewW, ViewH)
	}
	c.canvas.Clear()
	switch g.where() {
	case types.WherePause:
		return nil, errors.New("пауза: открыто меню")
	case types.WherePuzzle:
		g.mg.Draw(c.canvas)
		return encodePNG(c.canvas)
	}
	if g.mode != modePlay {
		g.drawScreens(c.canvas)
		return encodePNG(c.canvas)
	}
	g.drawPlay(c.canvas, false)
	g.drawFade(c.canvas)
	h := thumbSourceH(g.h)
	return encodePNG(
		c.canvas.SubImage(image.Rect(0, 0, ViewW, h)).(*ebiten.Image),
	)
}

// Use aims an item at a thing around, or at the hero himself.
func (c *control) Use(
	ctx context.Context, target, item string,
) (types.Outcome, error) {
	return c.act(ctx, "Roby", target, item, false)
}

// Go takes an exit, or a place on the island map.
func (c *control) Go(ctx context.Context, to string) (types.Outcome, error) {
	return c.act(ctx, "Roby", to, "", true)
}

// AskFriday has Friday act, and hands control back to the hero after.
func (c *control) AskFriday(
	ctx context.Context, target, item string,
) (types.Outcome, error) {
	return c.act(ctx, "Frid", target, item, false)
}

// act is an action of either character: take control of him through the
// portrait, put the item in his hand through the bar, bring the target into
// view, click it, and wait for the game to play it out. Friday's turn ends
// with the portrait handing control back to the hero.
func (c *control) act(
	ctx context.Context, who, target, item string, leaving bool,
) (types.Outcome, error) {
	g := c.g
	var out types.Outcome
	var aim sighted
	var before glance
	steps := []func() (bool, error){
		c.mark(&before),
		once(func() error {
			if err := g.canAct(); err != nil {
				return err
			}
			if who == "Frid" && !g.fridWithHero() {
				return errors.New("рядом нет Пятницы")
			}
			return g.takeControl(who)
		}),
		once(func() error {
			var err error
			aim, err = g.aimAt(target, leaving)
			return err
		}),
		c.pickItem(item),
		once(func() error {
			g.aimCamera(aim.at.X)
			return nil
		}),
		c.untilCamera(),
		once(func() error {
			// The view took a while: a scene that has begun since would
			// take the click as a skip.
			if err := g.canAct(); err != nil {
				return err
			}
			out.Reacted = g.clickWorld(aim)
			return nil
		}),
		c.untilFree(),
		once(func() error {
			if who == "Frid" {
				_ = g.takeControl("Roby")
			}
			return nil
		}),
		c.outcome(&out, &before, true),
	}
	err := c.run(ctx, steps...)
	if err != nil && who == "Frid" {
		// A refusal half way through still hands control back.
		_ = c.run(ctx, once(func() error { return g.takeControl("Roby") }))
	}
	return out, err
}

// untilCamera waits for the view to reach its target.
func (c *control) untilCamera() func() (bool, error) {
	return ticks(cameraMax, func() bool {
		d := c.g.camTarget - c.g.camXf
		return d > -1 && d < 1
	})
}

// OpenMap unfolds the island map with its button on the bar.
func (c *control) OpenMap(ctx context.Context) (types.Outcome, error) {
	g := c.g
	var out types.Outcome
	var before glance
	err := c.run(ctx,
		c.mark(&before),
		once(func() error {
			if err := g.canAct(); err != nil {
				return err
			}
			switch {
			case g.where() == types.WhereMap:
				return errors.New("карта острова уже открыта")
			case !g.gs.UI["map"] || g.bar == nil:
				return errors.New("карты острова ещё нет")
			}
			if err := g.takeControl("Roby"); err != nil {
				return err
			}
			g.click(boxCentre(g.bar.ScisorsBox))
			out.Reacted = g.pending != nil
			return nil
		}),
		c.untilFree(),
		c.outcome(&out, &before, true),
	)
	return out, err
}

// Wait lets the game run: until the hero is free, or for the seconds given.
func (c *control) Wait(
	ctx context.Context, seconds float64,
) (types.Outcome, error) {
	var out types.Outcome
	var before glance
	wait := c.untilFree()
	if seconds > 0 {
		n := int(seconds * 60)
		wait = ticks(min(n, waitMax), nil)
	}
	err := c.run(ctx, c.mark(&before), wait, c.outcome(&out, &before, false))
	return out, err
}

// PuzzleClick clicks a puzzle screen: the pointer goes there, presses and
// lets go, and stays put while the puzzle answers.
func (c *control) PuzzleClick(
	ctx context.Context, x, y int, right bool,
) (types.Outcome, error) {
	return c.puzzle(ctx, []image.Point{{x, y}}, c.click(x, y, right)...)
}

// A move lets the puzzle take in each of its clicks for a beat, and turns a
// piece at most three quarters round: a fourth brings it back.
const (
	moveBeat = 2
	turnsMax = 3
)

// PuzzleMove carries a piece the way the player does, click by click: one
// at from takes it up, the pointer brings it to to, where the right button
// turns it as many times as asked and a last click puts it down. When the
// first click takes nothing up of its own — whatever was in hand before —
// the move ends there, and the answer is what that click came to. So does
// a press on the real mouse while the piece is carried: the pointer is the
// player's again.
func (c *control) PuzzleMove(
	ctx context.Context, fromX, fromY, toX, toY, turns int,
) (types.Outcome, error) {
	if turns < 0 || turns > turnsMax {
		return types.Outcome{}, fmt.Errorf("turns — от 0 до %d", turnsMax)
	}
	var before int
	took := false
	moves := []func() (bool, error){once(func() error {
		before = c.g.takes()
		return nil
	})}
	moves = append(moves, c.click(fromX, fromY, false)...)
	moves = append(moves,
		ticks(moveBeat, nil),
		once(func() error {
			took = c.g.takes() > before
			return nil
		}))
	var carry []func() (bool, error)
	for range turns {
		carry = append(carry, c.click(toX, toY, true)...)
		carry = append(carry, ticks(moveBeat, nil))
	}
	carry = append(carry, c.click(toX, toY, false)...)
	carrying := func() bool { return took && c.hold != nil }
	moves = append(moves, when(carrying, carry...)...)
	return c.puzzle(ctx,
		[]image.Point{{fromX, fromY}, {toX, toY}}, moves...)
}

// puzzle plays a player's moves on the puzzle screen, at the points given —
// all of them on it — and answers once the puzzle has had its say, or, when
// it is over, once the scene it hands back to is through.
func (c *control) puzzle(
	ctx context.Context, at []image.Point, moves ...func() (bool, error),
) (types.Outcome, error) {
	g := c.g
	var out types.Outcome
	var before glance
	screen := image.Rect(0, 0, ViewW, ViewH)
	steps := []func() (bool, error){
		c.mark(&before),
		once(func() error {
			if g.mg == nil {
				return errors.New("головоломки нет")
			}
			for _, p := range at {
				if !p.In(screen) {
					return fmt.Errorf("точка %d,%d вне экрана 640×480",
						p.X, p.Y)
				}
			}
			out.Reacted = true
			return nil
		}),
	}
	steps = append(steps, moves...)
	steps = append(steps,
		ticks(puzzleTick, func() bool { return g.mg == nil }),
		c.afterPuzzle(),
		c.outcome(&out, &before, false),
	)
	err := c.run(ctx, steps...)
	return out, err
}

// point is a step that holds the puzzle pointer at x,y with the buttons
// given. A puzzle that is over has let the pointer go, and is left be.
func (c *control) point(x, y int, left, right bool) func() (bool, error) {
	return once(func() error {
		if c.g.mg != nil {
			c.holdPointer(heldPointer{x, y, left, right})
		}
		return nil
	})
}

// click is the player's click at x,y: the pointer comes to rest there for a
// tick, then the button goes down for a tick and comes back up.
func (c *control) click(x, y int, right bool) []func() (bool, error) {
	return []func() (bool, error){
		c.point(x, y, false, false), ticks(1, nil),
		c.point(x, y, !right, right), ticks(1, nil),
		c.point(x, y, false, false),
	}
}

// when guards steps with a condition, read as each of them comes up: while
// it does not hold, they pass straight through.
func when(
	ok func() bool, steps ...func() (bool, error),
) []func() (bool, error) {
	out := make([]func() (bool, error), len(steps))
	for i, s := range steps {
		out[i] = func() (bool, error) {
			if !ok() {
				return true, nil
			}
			return s()
		}
	}
	return out
}

// takes counts the pieces the player has taken up in the puzzle, as he saw
// each one go; a puzzle with nothing to carry takes none.
func (g *Game) takes() int {
	if c, ok := g.mg.(minigame.Carrier); ok {
		return c.Takes()
	}
	return 0
}

// PuzzleGiveUp leaves the puzzle unsolved, the way Esc does.
func (c *control) PuzzleGiveUp(ctx context.Context) (types.Outcome, error) {
	g := c.g
	var out types.Outcome
	var before glance
	err := c.run(ctx,
		c.mark(&before),
		once(func() error {
			if g.mg == nil {
				return errors.New("головоломки нет")
			}
			g.finishMinigame(0)
			out.Reacted = true
			return nil
		}),
		c.untilFree(),
		c.outcome(&out, &before, false),
	)
	return out, err
}

// afterPuzzle waits out the scene a finished puzzle hands back to.
func (c *control) afterPuzzle() func() (bool, error) {
	wait := c.untilFree()
	return func() (bool, error) {
		if c.g.mg != nil {
			return true, nil // still solving: the puzzle waits for a click
		}
		return wait()
	}
}

// Save writes the run into a slot.
func (c *control) Save(ctx context.Context, slot int) error {
	g := c.g
	return c.run(ctx, once(func() error {
		if err := checkSlot(slot); err != nil {
			return err
		}
		if err := g.canAct(); err != nil {
			return err
		}
		g.saveSlot(slot)
		if _, err := g.store().Read(slotName(slot)); err != nil {
			return fmt.Errorf("сохранить не вышло: %v", err)
		}
		return nil
	}))
}

// Load restores the run from a slot and waits for it to settle.
func (c *control) Load(
	ctx context.Context, slot int,
) (types.Outcome, error) {
	g := c.g
	var out types.Outcome
	var before glance
	err := c.run(ctx,
		c.mark(&before),
		once(func() error {
			if err := checkSlot(slot); err != nil {
				return err
			}
			if g.mode != modePlay {
				return errors.New("загрузка сейчас невозможна: " +
					g.busyWith())
			}
			if !g.loadSlot(slot) {
				return fmt.Errorf("слот %d пуст", slot)
			}
			out.Reacted = true
			return nil
		}),
		c.untilFree(),
		c.outcome(&out, &before, false),
	)
	return out, err
}

func checkSlot(slot int) error {
	if slot < 0 || slot >= slotCount {
		return fmt.Errorf("слоты — от 0 до %d", slotCount-1)
	}
	return nil
}

// holdPointer drives the puzzle pointer from this tick on.
func (c *control) holdPointer(p heldPointer) { c.hold = &p }

// drivePointer feeds the held pointer to the minigames every tick. The real
// mouse takes over with a press, not a move: a hand resting on it while the
// driver plays used to snatch the pointer between two calls, and the piece
// being carried went after the real cursor, off the window. A finished puzzle
// lets go on its own.
func (c *control) drivePointer() {
	if c.hold == nil {
		return
	}
	if c.g.mg == nil ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		c.hold = nil
		mouse.Release()
		return
	}
	mouse.Hold(c.hold.x, c.hold.y, c.hold.left, c.hold.right)
}

// where says where the hero is.
func (g *Game) where() string {
	switch {
	case g.mode == modeOptions || g.mode == modeSave || g.mode == modeLoad:
		return types.WherePause
	case g.mode != modePlay || isIntroScene(g.sceneName):
		return types.WhereScene
	case g.mg != nil:
		return types.WherePuzzle
	case strings.EqualFold(g.sceneName, "MAPSCR"):
		return types.WhereMap
	}
	return types.WhereIsland
}

// busyWith is what keeps the hero from acting, as the player sees it: the
// waiting clock, someone walking, a transition; "" when he is free. A puzzle
// is waiting for its player, so it is never busy.
func (g *Game) busyWith() string {
	switch {
	case g.mode == modeOptions || g.mode == modeSave || g.mode == modeLoad:
		return "пауза"
	case g.mode != modePlay:
		return "заставка"
	case g.mg != nil:
		return ""
	case g.fadeCurve != nil || g.pending != nil:
		return "переход"
	case g.awaitsClick():
		return "" // the movie holds for his click, an item in hand
	case g.act != nil || !g.gs.UI["mouse"]:
		return "сцена"
	case g.roby.walking():
		return "Роби идёт"
	case len(g.fridPath) > 0:
		return "Пятница идёт"
	}
	return ""
}

// canAct refuses an action the hero cannot take right now.
func (g *Game) canAct() error {
	switch g.where() {
	case types.WherePuzzle:
		return errors.New("сейчас головоломка: puzzle_click, " +
			"puzzle_move или puzzle_give_up")
	case types.WherePause:
		return errors.New("пауза: открыто меню")
	}
	if b := g.busyWith(); b != "" {
		return errors.New("занято: " + b + " — wait")
	}
	return nil
}

// fridWithHero reports whether Friday is at the hero's side and seen.
func (g *Game) fridWithHero() bool {
	return g.gs.Var("FridIs") == 1 && !g.fridHidden
}

// takeControl gives control to a character with the portrait, as the player
// does.
func (g *Game) takeControl(who string) error {
	if strings.EqualFold(g.gs.ActiveChar, who) {
		return nil
	}
	if g.bar != nil {
		g.click(boxCentre(g.bar.CharBox))
	}
	if !strings.EqualFold(g.gs.ActiveChar, who) {
		if who == "Frid" {
			return errors.New("управление Пятнице не передаётся")
		}
		return errors.New("управление Роби не вернулось")
	}
	return nil
}

// barBeat is how long the bar holds after each click the driver makes on it,
// so the scroll and the lit slot are there to be seen before the hero moves.
const barBeat = 15

// pickItem is a step that takes an item the player's way, one click on the
// bar per beat: the arrows until its slot is in view, then the slot. An item
// already in hand takes no click, and no name leaves the hand as it is.
func (c *control) pickItem(name string) func() (bool, error) {
	beat := 0
	return func() (bool, error) {
		if name == "" {
			return true, nil
		}
		if beat > 0 {
			beat--
			return false, nil
		}
		done, err := c.g.barClick(name)
		if done || err != nil {
			return true, err
		}
		beat = barBeat
		return false, nil
	}
}

// barClick makes the next click on the bar that brings an item into the
// acting character's hand, and reports done once it is there.
func (g *Game) barClick(name string) (bool, error) {
	inv := g.gs.Inventory()
	idx := -1
	for i, it := range inv {
		if sameName(g.itemLabel(it), name) || strings.EqualFold(it, name) {
			idx = i
			break
		}
	}
	if idx < 0 {
		who := "у Роби"
		if strings.EqualFold(g.gs.ActiveChar, "Frid") {
			who = "у Пятницы"
		}
		return false, fmt.Errorf("%s нет «%s»; с собой: %s", who, name,
			strings.Join(g.carryOf(g.gs.ActiveChar, true), ", "))
	}
	if strings.EqualFold(g.gs.Active, inv[idx]) {
		return true, nil
	}
	if g.bar == nil {
		return false, errors.New("панели нет")
	}
	before := g.invScroll
	switch {
	case idx < g.invScroll:
		g.click(boxCentre(g.bar.LeftArrow))
	case idx >= g.invScroll+g.bar.ItemsShown:
		g.click(boxCentre(g.bar.RightArrow))
	default:
		iw, ih := g.itemCell()
		slot := idx - g.invScroll
		g.click(g.bar.Inventory[0]+slot*iw+iw/2, g.bar.Inventory[1]+ih/2)
		if !strings.EqualFold(g.gs.Active, inv[idx]) {
			return false, errors.New("панель сейчас не даёт взять вещь")
		}
		return false, nil
	}
	if g.invScroll == before {
		return false, errors.New("панель сейчас не листается")
	}
	return false, nil
}

// boxCentre is the middle of an x0,y0,x1,y1 box.
func boxCentre(
	b [4]int,
) (int, int) {
	return (b[0] + b[2]) / 2, (b[1] + b[3]) / 2
}

// clickWorld clicks a world point from the viewport and reports whether the
// world answered: an action, a walk, a transition.
func (g *Game) clickWorld(s sighted) bool {
	g.click(s.at.X-g.camX, s.at.Y)
	return g.act != nil || g.pending != nil || g.roby.walking() ||
		len(g.fridPath) > 0
}

// selfNames are how the driver points at the acting character himself.
var selfNames = []string{"себя", "себе", "сам", "сама", "самого себя"}

// aimAt finds what an action is aimed at: a thing around, a way out, or the
// acting character himself. leaving restricts it to the ways out (and, on the
// map, to its places).
func (g *Game) aimAt(name string, leaving bool) (sighted, error) {
	if !leaving {
		for _, s := range selfNames {
			if sameName(name, s) {
				return g.selfPoint()
			}
		}
	}
	all := g.sights()
	var pool []sighted
	for _, s := range all {
		if !leaving || s.exit || g.where() == types.WhereMap {
			pool = append(pool, s)
		}
	}
	for _, s := range pool {
		if sameName(s.name, name) {
			return s, nil
		}
	}
	names := make([]string, 0, len(pool))
	for _, s := range pool {
		names = append(names, s.name)
	}
	what := "вокруг"
	if leaving {
		what = "выходы"
	}
	if len(names) == 0 {
		if leaving {
			return sighted{}, errors.New("выходов нет")
		}
		return sighted{}, fmt.Errorf("нет «%s»; вокруг пусто", name)
	}
	return sighted{}, fmt.Errorf("нет «%s»; %s: %s", name, what,
		strings.Join(names, ", "))
}

// selfPoint is a point on the acting character's own cell that no zone
// covers, so the click reaches him and not an object in front of him.
func (g *Game) selfPoint() (sighted, error) {
	cell, standing := g.actingCell()
	if !standing {
		return sighted{}, errors.New("персонаж ещё идёт — wait")
	}
	if strings.EqualFold(g.gs.ActiveChar, "Frid") && g.fridHidden ||
		!strings.EqualFold(g.gs.ActiveChar, "Frid") && g.charHidden {
		return sighted{}, errors.New("персонажа сейчас не видно")
	}
	x0, y0 := g.grid.Corner(cell[0], cell[1])
	gw, gh := 1, 1
	if g.sc != nil {
		gw, gh = max(g.sc.GridSize[0], 1), max(g.sc.GridSize[1], 1)
	}
	for _, p := range scanRect(image.Rect(x0, y0, x0+gw, y0+gh)) {
		cx, cy := g.grid.ToCell(p.X, p.Y)
		if [2]int{cx, cy} == cell && g.clickable(p) &&
			g.hotspotAt(p.X, p.Y) == nil {
			return sighted{name: "себя", at: p}, nil
		}
	}
	return sighted{}, errors.New("к персонажу не подступиться")
}

// sighted is something on stage the hero can be sent to: the name he knows
// it by, the world point a click on it lands on, whether it is a way out and
// which side of him it is on. The object behind it stays inside the game.
type sighted struct {
	name string
	at   image.Point
	exit bool
	side string
}

// exitWords name a way out by its arrow cursor (docs/08-scene-objects.md):
// the side ones, and the doors into the depth of a room and back out of it.
var exitWords = map[int]string{
	1: "налево", 2: "направо", 3: "вперёд", 4: "назад",
}

// stockExitCaption is the caption the game gives every plain way out. An
// exit that says something of its own ("Вход в пещеру") goes by that; the
// rest go by their arrows.
const stockExitCaption = "Идти дальше?"

// caption is an object's hover caption as a name: TEXT.DAT keeps the quotes
// the text box shows.
func (g *Game) caption(id int) string { return unquote(g.textLine(id)) }

// sights lists what the hero can see and be sent to, left to right: every
// object on stage the player could find with the mouse — one with a caption,
// or a way out under an arrow cursor — with a point that a click really
// reaches. A zone buried under others, or a single pixel, is out of sight for
// the player and so for the hero.
func (g *Game) sights() []sighted {
	var out []sighted
	done := map[string]bool{}
	for _, s := range g.sceneObjs {
		if s.removed || s.ob == nil || done[strings.ToLower(s.ref.Name)] {
			continue
		}
		done[strings.ToLower(s.ref.Name)] = true
		name := g.caption(s.ob.Text)
		exit := s.ob.Cursor >= 1 && s.ob.Cursor <= 4
		if exit && (name == "" || name == stockExitCaption) {
			name = exitWords[s.ob.Cursor]
		}
		if name == "" {
			continue
		}
		at, ok := g.zonePoint(s.ref.Name)
		if !ok {
			continue
		}
		out = append(out, sighted{name: name, at: at, exit: exit})
	}
	sortSights(out)
	count := map[string]int{}
	for _, s := range out {
		count[strings.ToLower(s.name)]++
	}
	seen := map[string]int{}
	hx := g.heroVisualX()
	near := 120
	if g.sc != nil && g.sc.GridSize[0] > 0 {
		near = g.sc.GridSize[0]
	}
	onMap := g.where() == types.WhereMap
	for i := range out {
		k := strings.ToLower(out[i].name)
		if count[k] > 1 {
			seen[k]++
			out[i].name = fmt.Sprintf("%s %d", out[i].name, seen[k])
		}
		if !onMap {
			out[i].side = sideOf(float64(out[i].at.X)-hx, near)
		}
	}
	return out
}

// sortSights orders sights left to right, keeping ObjectList order within a
// column.
func sortSights(s []sighted) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].at.X < s[j-1].at.X; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// sideOf says which side of the hero a point dx pixels from him is on.
func sideOf(dx float64, near int) string {
	switch {
	case dx < -float64(near):
		return types.SideLeft
	case dx > float64(near):
		return types.SideRight
	}
	return types.SideNear
}

// zonePoint is a world point where a click lands on the object: inside one
// of its zones, on the scene above the bar, and not under another object's
// zone — the centre when it can be, else the first free point of a scan.
func (g *Game) zonePoint(key string) (image.Point, bool) {
	for i := range g.hotspots {
		hs := &g.hotspots[i]
		if !strings.EqualFold(hs.key, key) ||
			hs.rect.Dx()*hs.rect.Dy() <= 1 {
			continue
		}
		for _, p := range scanRect(hs.rect) {
			if !g.clickable(p) {
				continue
			}
			if h := g.hotspotAt(p.X, p.Y); h != nil &&
				strings.EqualFold(h.key, key) {
				return p, true
			}
		}
	}
	return image.Point{}, false
}

// clickable reports whether a world point is on the scene, above the bar.
func (g *Game) clickable(p image.Point) bool {
	h := min(g.h, PlayH)
	if h <= 0 {
		h = PlayH
	}
	return p.X >= 0 && p.X < g.w && p.Y >= 0 && p.Y < h
}

// scanRect lists the points to try in a rectangle: its centre first, then a
// coarse grid over it.
func scanRect(r image.Rectangle) []image.Point {
	pts := []image.Point{{(r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2}}
	step := max(min(r.Dx(), r.Dy())/8, 2)
	for y := r.Min.Y + step/2; y < r.Max.Y; y += step {
		for x := r.Min.X + step/2; x < r.Max.X; x += step {
			pts = append(pts, image.Pt(x, y))
		}
	}
	return pts
}

// sameName compares names the way a person would type them: case, spacing
// and ё/е aside.
func sameName(a, b string) bool {
	norm := func(s string) string {
		s = strings.Trim(strings.TrimSpace(s), `"«»`)
		s = strings.ToLower(strings.Join(strings.Fields(s), " "))
		return strings.ReplaceAll(s, "ё", "е")
	}
	return norm(a) == norm(b)
}

// itemLabel is the name an item goes by in the game (BAR.BAR); its own name
// when the bar gives none.
func (g *Game) itemLabel(item string) string {
	if i := g.itemIndex(item); i >= 0 && g.bar != nil &&
		i < len(g.bar.ItemLabels) && g.bar.ItemLabels[i] != "" {
		return g.bar.ItemLabels[i]
	}
	return item
}

// isHand reports the bare hand, which is a slot of the bar rather than a
// thing carried (hand, handfr).
func isHand(item string) bool { return scriptTok(item) == "HAN" }

// carryOf is what a character has on him, by name, in bar order: with all,
// the item in hand too; the bare hand never.
func (g *Game) carryOf(char string, all bool) []string {
	acting := strings.EqualFold(g.gs.ActiveChar, char)
	var out []string
	for _, it := range g.gs.InventoryOf(char) {
		if isHand(it) || !all && acting && strings.EqualFold(it, g.gs.Active) {
			continue
		}
		out = append(out, g.itemLabel(it))
	}
	return out
}

// percept is the world at a glance, in the hero's terms.
func (g *Game) percept() types.Percept {
	p := types.Percept{
		Where:   g.where(),
		Busy:    g.busyWith(),
		Hearing: unquote(g.msg),
	}
	if p.Where == types.WherePause || p.Where == types.WherePuzzle {
		return p
	}
	for _, s := range g.sights() {
		t := types.Thing{Name: s.name, Side: s.side}
		if s.exit {
			p.Exits = append(p.Exits, t)
		} else {
			p.Around = append(p.Around, t)
		}
	}
	fridActs := strings.EqualFold(g.gs.ActiveChar, "Frid")
	if !fridActs {
		p.Hands = g.itemLabel(g.gs.Active)
		p.EmptyHands = isHand(g.gs.Active)
	}
	p.Carry = g.carryOf("Roby", false)
	if g.fridWithHero() && p.Where != types.WhereMap {
		f := &types.Companion{Carry: g.carryOf("Frid", false)}
		if fridActs {
			f.Hands = g.itemLabel(g.gs.Active)
		}
		near := 120
		if g.sc != nil && g.sc.GridSize[0] > 0 {
			near = g.sc.GridSize[0]
		}
		f.Side = sideOf(g.fridPos[0]-g.heroVisualX(), near)
		p.Friday = f
	}
	p.Map = g.gs.UI["map"] && p.Where != types.WhereMap
	return p
}
