package app

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shpaker/modern-robinson/internal/adapters/mouse"
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

func thingNames(things []types.Thing) []string {
	out := make([]string, 0, len(things))
	for _, t := range things {
		out = append(out, t.Name)
	}
	return out
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
	if len(out.Said) == 0 || !strings.Contains(out.Said[0], "укусил") {
		t.Errorf("said = %q, want the crab's bite", out.Said)
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
	if len(out.Look.Around) == 0 {
		t.Error("the new place shows nothing around")
	}
	// Only a way out takes a Go.
	if _, err := drive(t, g, func(ctx context.Context) (types.Outcome, error) {
		return g.ctl.Go(ctx, "Нора")
	}); err == nil || !strings.Contains(err.Error(), "Выходы") {
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
	if err == nil || !strings.Contains(err.Error(), "идёт сцена") {
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
	if err := g.pickItem("Рыба"); err != nil {
		t.Fatal(err)
	}
	if g.gs.Active != "fish" || g.invScroll == 0 {
		t.Errorf("active = %q scroll = %d", g.gs.Active, g.invScroll)
	}
	if err := g.pickItem("рука"); err != nil || g.gs.Active != "hand" ||
		g.invScroll != 0 {
		t.Errorf("back to the hand: %v active = %q scroll = %d",
			err, g.gs.Active, g.invScroll)
	}
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
	if !mouse.Held() {
		t.Error("the pointer must stay where the driver left it")
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
