package app

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

// clickRow presses the mouse at the centre of main-menu row i.
func clickRow(g *Game, i int) {
	r := menuRows[i]
	g.updateOptionsMenu((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2, true)
}

// pressSlot presses and releases the mouse at (x, y) on a slot screen.
func pressSlot(g *Game, x, y int) {
	g.updateSlotScreen(mouseState{x: x, y: y, clicked: true, pressed: true})
	g.updateSlotScreen(mouseState{x: x, y: y, released: true})
}

// slotScreen is a save or load screen with nothing hovered or held.
func slotScreen(mode int) *Game {
	return &Game{mode: mode, slotHover: -1, slotSel: -1, btnDown: -1}
}

// Until a run starts, "continue" and "save" are unavailable, as in the
// original (ROBY.PDF p.24): the rows neither highlight nor click.
func TestStartMenuKeepsContinueAndSaveDead(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1}
	for _, row := range []int{1, 3} {
		clickRow(g, row)
		if g.optHover != -1 {
			t.Errorf("row %d highlights at the start menu", row)
		}
		if g.mode != modeOptions {
			t.Errorf("row %d clicked through: mode = %d", row, g.mode)
		}
	}
}

// "Restore" is how a save is reached from the start menu, so it must work
// before any run: it opens the load screen, and cancelling backs out to the
// menu rather than into a play that does not exist yet.
func TestStartMenuOpensTheLoadScreen(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1}
	clickRow(g, 2)
	if g.mode != modeLoad {
		t.Fatalf("mode = %d, want modeLoad (%d)", g.mode, modeLoad)
	}
	pressSlot(g, cancelButton.Min.X+1, cancelButton.Min.Y+1)
	if g.mode != modeOptions {
		t.Errorf("cancel: mode = %d, want back on the menu", g.mode)
	}
}

// "New game" from the start menu takes the same covered path as a restart from
// play: the loading screen sits out the intro bridge.
func TestStartMenuNewGameEntersLoading(t *testing.T) {
	g := &Game{
		res:        noScenes{},
		parser:     repositories.SceneParser{},
		gs:         types.NewGameState(),
		cycleCache: map[string]*walkCycle{},
		mode:       modeOptions,
		optHover:   -1,
		optDrag:    -1,
	}
	clickRow(g, 0)
	if g.mode != modeLoading {
		t.Errorf("mode = %d, want modeLoading (%d)", g.mode, modeLoading)
	}
	if g.started {
		t.Error("a run not yet handed to play must not count as started")
	}
}

// Once a run is underway the same two rows come alive again.
func TestMenuContinueAndSaveComeAliveInARun(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1, started: true}
	clickRow(g, 1)
	if g.mode != modePlay {
		t.Errorf("continue: mode = %d, want modePlay (%d)", g.mode, modePlay)
	}
	g.mode = modeOptions
	clickRow(g, 3)
	if g.mode != modeSave {
		t.Errorf("save: mode = %d, want modeSave (%d)", g.mode, modeSave)
	}
}

// Esc has no play to fall back to before the first run: it keeps the menu up
// and only backs the slot screens out into it.
func TestEscBeforeARunStaysOnTheMenu(t *testing.T) {
	g := &Game{mode: modeOptions}
	g.toggleOptions()
	if g.mode != modeOptions {
		t.Errorf("menu: mode = %d, want to stay on modeOptions", g.mode)
	}
	g.mode = modeLoad
	g.toggleOptions()
	if g.mode != modeOptions {
		t.Errorf("load screen: mode = %d, want modeOptions", g.mode)
	}
	g.started = true
	g.toggleOptions()
	if g.mode != modePlay {
		t.Errorf("in a run: mode = %d, want modePlay (%d)", g.mode, modePlay)
	}
}

// slotDir points the save files at a test directory while every container
// lookup still bows out early.
type slotDir struct {
	noScenes
	root string
}

func (d slotDir) Root() string { return d.root }

// Restoring a slot is the other way a run starts, and it must light up
// "continue" and "save" just like playing through the intro does.
func TestLoadSlotMarksTheRunStarted(t *testing.T) {
	dir := t.TempDir()
	sav := filepath.Join(dir, "robinson03.sav")
	if err := os.WriteFile(sav, []byte(`{"scene":"SCENA0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Game{
		res:        slotDir{root: dir},
		parser:     repositories.SceneParser{},
		gs:         types.NewGameState(),
		cycleCache: map[string]*walkCycle{},
	}
	if !g.loadSlot(3) {
		t.Fatal("the slot must load")
	}
	if !g.started {
		t.Error("a restored run must count as started")
	}
}

// The original ships the buttons pushed in, so pushing one has to last as long
// as the player holds it: the press only arms the button, the release runs it.
func TestSlotButtonRunsOnRelease(t *testing.T) {
	g := slotScreen(modeSave)
	x, y := cancelButton.Min.X+4, cancelButton.Min.Y+4
	g.updateSlotScreen(mouseState{x: x, y: y, clicked: true, pressed: true})
	if g.btnDown != 1 {
		t.Fatalf("btnDown = %d, want the cancel button held (1)", g.btnDown)
	}
	if g.mode != modeSave {
		t.Errorf("the press alone left the screen: mode = %d", g.mode)
	}
	g.updateSlotScreen(mouseState{x: x, y: y, released: true})
	if g.mode != modeOptions {
		t.Errorf("release: mode = %d, want modeOptions", g.mode)
	}
	if g.btnDown != -1 {
		t.Errorf("btnDown = %d, want no button held after release", g.btnDown)
	}
}

// Letting go anywhere but on the armed button takes it back, the way every
// button of that era did.
func TestSlotButtonCancelledBySlidingOff(t *testing.T) {
	g := slotScreen(modeSave)
	g.updateSlotScreen(mouseState{
		x: cancelButton.Min.X + 4, y: cancelButton.Min.Y + 4,
		clicked: true, pressed: true,
	})
	g.updateSlotScreen(mouseState{x: 4, y: 4, released: true})
	if g.mode != modeSave {
		t.Errorf("mode = %d, want to stay on the save screen", g.mode)
	}
	if g.btnDown != -1 {
		t.Errorf("btnDown = %d, want the button disarmed", g.btnDown)
	}
}

// optionsPack is OPTIONS.DAT as the game ships it: every bitmap, the menu's
// palette, and the one the save and load screens carry of their own.
func optionsPack(t *testing.T) (
	sprites map[string]*types.NGB, menuPal, slotPal types.Palette,
) {
	t.Helper()
	res := repositories.NewResources(testutil.GameRoot(t))
	sprites, menuPal = res.ScreenPack("OPTIONS")
	if len(sprites) == 0 {
		t.Fatal("OPTIONS.DAT holds no bitmaps")
	}
	_, slotPal = res.Screen("OPTIONS", "SAVE")
	return sprites, menuPal, slotPal
}

// meanRGBDiff is how far a sprite painted with pal lands from the backdrop it
// covers, averaged over its pixels and channels.
func meanRGBDiff(sp, bg *types.NGB, pal, bgPal types.Palette) float64 {
	total := 0
	for y := 0; y < sp.Height; y++ {
		for x := 0; x < sp.Width; x++ {
			a := pal[sp.Indices[y*sp.Width+x]]
			b := bgPal[bg.Indices[(int(sp.Y0)+y)*bg.Width+int(sp.X0)+x]]
			total += abs(int(a[0])-int(b[0])) + abs(int(a[1])-int(b[1])) +
				abs(int(a[2])-int(b[2]))
		}
	}
	return float64(total) / float64(sp.Width*sp.Height*3)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// labelMatch is the share of label pixels — the dark ones inside the frame —
// the sprite and the backdrop under it agree on, with the authored one-pixel
// push taken back out.
func labelMatch(sp, bg *types.NGB, pal types.Palette) float64 {
	dark := func(c [4]byte) bool {
		return int(c[0])*3+int(c[1])*6+int(c[2]) < 1100
	}
	both, either := 0, 0
	for y := 6; y < sp.Height-6; y++ {
		for x := 8; x < sp.Width-8; x++ {
			a := dark(pal[sp.Indices[y*sp.Width+x]])
			i := (int(sp.Y0)+y-1)*bg.Width + int(sp.X0) + x - 1
			b := dark(pal[bg.Indices[i]])
			if a && b {
				both++
			}
			if a || b {
				either++
			}
		}
	}
	if either == 0 {
		return 0
	}
	return float64(both) / float64(either)
}

// The buttons and the empty slot are drawn in the palette of the screens they
// sit on, which shares not one of its 256 entries with the menu's: painted with
// the wrong one they come out grey. Nothing else in the pack changes hands.
func TestSlotButtonsUseTheirScreenPalette(t *testing.T) {
	sprites, menuPal, slotPal := optionsPack(t)
	res := repositories.NewResources(testutil.GameRoot(t))
	if _, loadPal := res.Screen("OPTIONS", "LOAD"); loadPal != slotPal {
		t.Error("SAVE.COL and LOAD.COL must be the same palette")
	}
	if _, tempPal := res.Screen("OPTIONS", "TEMP"); tempPal != slotPal {
		t.Error("SAVE.COL and TEMP.COL must be the same palette")
	}
	shared := 0
	for i := range slotPal {
		if slotPal[i] == menuPal[i] {
			shared++
		}
	}
	if shared != 0 {
		t.Errorf("%d palette entries shared with the menu, want none", shared)
	}
	for name := range sprites {
		want := strings.HasPrefix(name, "BUT") || name == "TEMP"
		if got := slotScreenSprite(name); got != want {
			t.Errorf("slotScreenSprite(%q) = %v, want %v", name, got, want)
		}
	}
}

// Each button covers its own place on its own backdrop: painted with the
// screen's palette it all but disappears into it, and with the menu's it is
// twice as far off.
func TestSlotButtonsMatchTheirBackdrop(t *testing.T) {
	sprites, menuPal, slotPal := optionsPack(t)
	for _, c := range []struct{ mode, screen string }{
		{"save", "SAVE"},
		{"load", "LOAD"},
	} {
		mode := modeSave
		if c.mode == "load" {
			mode = modeLoad
		}
		bg := sprites[c.screen]
		if bg == nil {
			t.Fatalf("no %s backdrop", c.screen)
		}
		for _, name := range slotButtons(mode) {
			sp := sprites[name]
			if sp == nil {
				t.Fatalf("no %s bitmap", name)
			}
			own := meanRGBDiff(sp, bg, slotPal, slotPal)
			menu := meanRGBDiff(sp, bg, menuPal, slotPal)
			if own > 30 {
				t.Errorf("%s on %s: mean RGB gap %.1f, want under 30",
					name, c.screen, own)
			}
			if menu < own*1.5 {
				t.Errorf("%s on %s: menu palette gap %.1f, own %.1f — the "+
					"test no longer tells the palettes apart",
					name, c.screen, menu, own)
			}
		}
	}
}

// The label decides which screen a button belongs to: "Сохранение" is printed
// on the save backdrop and "Восстановление" on the load one, so a swapped pair
// shows up as a label that does not line up.
func TestSlotButtonLabelsBelongToTheirScreen(t *testing.T) {
	sprites, _, slotPal := optionsPack(t)
	for _, c := range []struct {
		mode       int
		own, other string
	}{
		{modeSave, "SAVE", "LOAD"},
		{modeLoad, "LOAD", "SAVE"},
	} {
		act := sprites[slotButtons(c.mode)[0]]
		if m := labelMatch(act, sprites[c.own], slotPal); m < 0.9 {
			t.Errorf("%s label on %s: %.2f, want over 0.90",
				slotButtons(c.mode)[0], c.own, m)
		}
		if m := labelMatch(act, sprites[c.other], slotPal); m > 0.5 {
			t.Errorf("%s label on %s: %.2f — the pairs are not distinct",
				slotButtons(c.mode)[0], c.other, m)
		}
		// The cancel button reads the same on both screens, so it only has to
		// sit right on its own.
		cancel := sprites[slotButtons(c.mode)[1]]
		if m := labelMatch(cancel, sprites[c.own], slotPal); m < 0.8 {
			t.Errorf("%s label on %s: %.2f, want over 0.80",
				slotButtons(c.mode)[1], c.own, m)
		}
	}
}

// The left button restores the slot picked on the same screen, and only once
// the player lets go of it.
func TestRestoreButtonLoadsTheSelectedSlot(t *testing.T) {
	dir := t.TempDir()
	sav := filepath.Join(dir, "robinson02.sav")
	if err := os.WriteFile(sav, []byte(`{"scene":"SCENA0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	g := slotScreen(modeLoad)
	g.res = slotDir{root: dir}
	g.parser = repositories.SceneParser{}
	g.gs = types.NewGameState()
	g.cycleCache = map[string]*walkCycle{}

	slot := slotRect(2)
	g.updateSlotScreen(mouseState{
		x: slot.Min.X + 4, y: slot.Min.Y + 4, clicked: true, pressed: true,
	})
	if g.slotSel != 2 {
		t.Fatalf("slotSel = %d, want the clicked slot 2", g.slotSel)
	}
	g.updateSlotScreen(mouseState{
		x: saveButton.Min.X + 4, y: saveButton.Min.Y + 4,
		clicked: true, pressed: true,
	})
	if g.mode != modeLoad {
		t.Fatalf("the press alone restored: mode = %d", g.mode)
	}
	g.updateSlotScreen(mouseState{
		x: saveButton.Min.X + 4, y: saveButton.Min.Y + 4, released: true,
	})
	if g.mode != modePlay || !g.started {
		t.Errorf("release: mode = %d started = %v, want a restored run",
			g.mode, g.started)
	}
}

// The slots are ROBY.EXE's table at 0x46d1f0, the openings of the painted
// frames: a thumbnail put anywhere else covers the frame's gold.
func TestSlotRectsAreTheOriginalTable(t *testing.T) {
	want := [slotCount]image.Rectangle{
		image.Rect(16, 50, 144, 146),
		image.Rect(176, 50, 304, 146),
		image.Rect(336, 50, 464, 146),
		image.Rect(496, 50, 624, 146),
		image.Rect(16, 180, 144, 276),
		image.Rect(176, 180, 304, 276),
		image.Rect(336, 180, 464, 276),
		image.Rect(496, 180, 624, 276),
		image.Rect(16, 310, 144, 406),
		image.Rect(176, 310, 304, 406),
		image.Rect(336, 310, 464, 406),
		image.Rect(496, 310, 624, 406),
	}
	for i, r := range want {
		if got := slotRect(i); got != r {
			t.Errorf("slot %d at %v, want %v", i, got, r)
		}
	}
}

// An unchosen slot is shaded at 0.4 of a .FAD table, truncated to a step: the
// scene's 16 steps give the sixth, a little over half the brightness, and no
// table leaves the slot as it is.
func TestSlotShadeIsTheOriginalStep(t *testing.T) {
	if fadeStep(16) != 6 || fadeStep(32) != 12 {
		t.Errorf("steps %d of 16 and %d of 32, want 6 and 12",
			fadeStep(16), fadeStep(32))
	}
	if k := fadeShade(nil); k != 1 {
		t.Errorf("no table: shade %.2f, want 1", k)
	}
	res := repositories.NewResources(testutil.GameRoot(t))
	if k := fadeShade(res.SceneFade("SCENA0")); k < 0.55 || k > 0.65 {
		t.Errorf("SCENA0 shade %.2f, want about 0.60", k)
	}
}

// The empty slot shaded through TEMP.FAD is the same marble, darker by the
// shade step and far from black.
func TestEmptySlotShadeDarkensTheMarble(t *testing.T) {
	res := repositories.NewResources(testutil.GameRoot(t))
	n, pal := res.Screen("OPTIONS", "TEMP")
	fad := res.ScreenFile("OPTIONS", "TEMP.FAD")
	if n == nil || len(fad) != 32*256 {
		t.Fatal("OPTIONS.DAT lacks TEMP.NGB or its 32-step TEMP.FAD")
	}
	mean := func(p types.Palette) float64 {
		sum := 0
		for _, i := range n.Indices {
			c := p[i]
			sum += int(c[0])*3 + int(c[1])*6 + int(c[2])
		}
		return float64(sum) / float64(len(n.Indices))
	}
	if r := mean(fadePalette(pal, fad)) / mean(pal); r < 0.5 || r > 0.8 {
		t.Errorf("shaded marble at %.2f of its brightness, want 0.5..0.8", r)
	}
	if fadePalette(pal, nil) != pal {
		t.Error("no table changed the palette")
	}
}

// The menu is built anew each time it opens, so its slot screens start on the
// first slot whichever was picked last time, shaded for the current scene.
func TestMenuOpensOnTheFirstSlot(t *testing.T) {
	g := &Game{mode: modePlay, slotSel: 5, slotDim: 0.6}
	g.openMenu()
	if g.slotSel != 0 || g.slotDim != 0 {
		t.Errorf("slotSel %d slotDim %.1f, want 0 and 0 (not worked out)",
			g.slotSel, g.slotDim)
	}
}
