package app

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// fakeAudio records what it was asked to play instead of opening a device.
type fakeAudio struct {
	played  []string  // keys sent to the effect channels
	ambient []string  // keys sent as one-shots, outside the channels
	vols    []float64 // and their level and placement, shot by shot
	pans    []float64
	stops   int
}

func (a *fakeAudio) Play(key string, _ []byte, _ int) {
	a.played = append(a.played, key)
}

func (a *fakeAudio) PlayAmbient(key string, _ []byte, volScale, pan float64) {
	a.ambient = append(a.ambient, key)
	a.vols = append(a.vols, volScale)
	a.pans = append(a.pans, pan)
}

func (a *fakeAudio) PlayMusic(string, []byte) {}
func (a *fakeAudio) StopMusic()               {}
func (a *fakeAudio) StopEffects()             { a.stops++ }
func (a *fakeAudio) SetVolume(float64)        {}
func (a *fakeAudio) SetMusicVolume(float64)   {}

// silentRes answers every asset lookup with nothing, so a Game can run its
// sound and ambient paths without the game files.
type silentRes struct{ interfaces.IResources }

func (silentRes) Sound(string) []byte { return nil }

// skipGame builds a game playing fs as its current action, with Interrupt on so
// the script allows skipping.
func skipGame(t *testing.T, fs *types.FrameScript) (*Game, *fakeAudio) {
	t.Helper()
	fa := &fakeAudio{}
	g := &Game{
		res:   silentRes{},
		audio: fa,
		gs:    types.NewGameState(),
		sc:    &types.Scene{SoundVars: map[string][2]string{}},
		texts: []string{"", "a line"},
		act: &actionPlay{
			fs:      fs,
			player:  use_cases.NewPlayer(fs, false),
			started: true,
		},
	}
	g.gs.UI["interrupt"] = true
	return g, fa
}

// Skipping a cutscene rushes its remaining frames through in one tick. The
// quest state still has to land, but the sounds and subtitles must not: INT1
// alone would otherwise dump rain, thunder and Robinson's scream on the player
// the moment they click past it.
func TestSkipCutsceneMutesSoundAndText(t *testing.T) {
	fs := repositories.SceneParser{}.ParseFrameScript(
		"MovieName Int1.mv;\nTotalFrames 3;\n" +
			"Frame 0,1;\nDelay 140;\nInterrupt ON;\n" +
			"Frame 1,1;\nDelay 140;\nSound scream,9;\nText 1,1;\nSetVar Seen,1;\n" +
			"Frame 2,1;\nDelay 140;\nGoScene SCENA0,Roby,robawake,Frid,frin0,2,0;\nEnd;",
	)
	g, fa := skipGame(t, fs)

	if !g.skipCutscene() {
		t.Fatal("a script with Interrupt ON must be skippable")
	}
	if len(fa.played) != 0 {
		t.Errorf("played %v, want silence while fast-forwarding", fa.played)
	}
	if g.msg != "" {
		t.Errorf("msg = %q, want the skipped scene's line gone", g.msg)
	}
	if g.gs.Var("Seen") != 1 {
		t.Error("state commands must still apply: Seen was not set")
	}
	if g.pending == nil || g.pending.Scene != "SCENA0" {
		t.Errorf("pending = %+v, want the GoScene to SCENA0", g.pending)
	}
	if g.act != nil {
		t.Error("the action must be over after a skip")
	}
	if fa.stops != 1 {
		t.Errorf("StopEffects called %d times, want 1", fa.stops)
	}
}

// Only the scripts that ask for it (Interrupt ON) may be cut short — the rest
// have to play out, so a click must not end them.
func TestSkipCutsceneNeedsInterrupt(t *testing.T) {
	fs := repositories.SceneParser{}.ParseFrameScript(
		"MovieName Roin2.mv;\nTotalFrames 1;\n" +
			"Frame 0,1;\nDelay 140;\nSetVar Seen,1;\nEnd;",
	)
	g, fa := skipGame(t, fs)
	g.gs.UI["interrupt"] = false

	if g.skipCutscene() {
		t.Error("a script without Interrupt must not be skippable")
	}
	if g.act == nil {
		t.Error("the action must keep playing")
	}
	if g.gs.Var("Seen") != 0 || fa.stops != 0 {
		t.Error("a refused skip must change nothing")
	}
}

// Muting is only for the fast-forward: played at the authored pace, the same
// events reach the speakers and the subtitle line.
func TestApplyEventsPlaysPresentationUnmuted(t *testing.T) {
	fs := repositories.SceneParser{}.ParseFrameScript(
		"MovieName Int1.mv;\nTotalFrames 1;\n" +
			"Frame 0,1;\nDelay 140;\nSound scream,9;\nText 1,1;\nEnd;",
	)
	g, fa := skipGame(t, fs)
	cmds := fs.Frames[0].Events

	g.applyEvents(cmds)
	if len(fa.played) != 1 || fa.played[0] != "scream.wav" {
		t.Errorf("played %v, want scream.wav", fa.played)
	}
	if g.msg == "" {
		t.Error("the subtitle must show")
	}

	fa.played, g.msg = nil, ""
	g.enqueueAction(g.act, cmds, true) // the skip path mutes them
	if len(fa.played) != 0 || g.msg != "" {
		t.Errorf("muted run played %v / msg %q, want neither", fa.played, g.msg)
	}
}

// entryRes serves one .FS as the scene's arrival script and no art at all.
type entryRes struct {
	silentRes
	fs string
}

func (entryRes) MovieFrames(string) ([]*types.NGB, types.Palette) {
	return nil, types.Palette{}
}

func (entryRes) MovieShift(string) [2]int { return [2]int{} }

func (r entryRes) container() interfaces.IContainer {
	return entryContainer{fs: r.fs}
}

type entryContainer struct {
	interfaces.IContainer
	fs string
}

func (c entryContainer) ExtractName(string) ([]byte, error) {
	return []byte(c.fs), nil
}

// A cutscene's opening frame is what declares it skippable, and input runs
// earlier in the tick than the action does. Firing frame 0 as the script starts
// is what keeps the very first press from being swallowed.
func TestStartEntryAppliesFirstFrame(t *testing.T) {
	res := entryRes{fs: "MovieName Int1.mv;\nTotalFrames 2;\n" +
		"Frame 0,1;\nDelay 140;\nShowCursor OFF;\nInterrupt ON;\n" +
		"Frame 1,1;\nDelay 140;\nSetVar Late,1;\nEnd;"}
	g := &Game{
		res:    res,
		parser: repositories.SceneParser{},
		audio:  &fakeAudio{},
		gs:     types.NewGameState(),
		sc:     &types.Scene{SoundVars: map[string][2]string{}},
		sceneC: res.container(),
	}

	g.startEntry("Int1")
	if g.act == nil {
		t.Fatal("the entry script must be playing")
	}
	if !g.gs.UI["interrupt"] {
		t.Error("frame 0's Interrupt ON must be in effect before the next tick")
	}
	if g.gs.Var("Late") != 0 {
		t.Error("only frame 0 may fire: the script has not advanced yet")
	}
	// The player must not owe frame 0 twice.
	g.applyEvents(g.act.player.Update(0))
	if g.gs.Var("Late") != 0 {
		t.Error("frame 0 fired again")
	}
}

// Keys that already do something else must not double as "skip", or F5 would
// save and jump the screen at once, and Esc would open the menu over a cutscene
// it just cut short.
func TestServiceKeysAreNotSkipKeys(t *testing.T) {
	cases := []struct {
		name string
		key  ebiten.Key
		want bool
	}{
		{"F1 toggles debug", ebiten.KeyF1, true},
		{"F5 saves", ebiten.KeyF5, true},
		{"F9 loads", ebiten.KeyF9, true},
		{"Esc opens the menu", ebiten.KeyEscape, true},
		{"Enter skips", ebiten.KeyEnter, false},
		{"Space skips", ebiten.KeySpace, false},
		{"a letter skips", ebiten.KeyA, false},
	}
	for _, c := range cases {
		if got := serviceKey(c.key); got != c.want {
			t.Errorf("serviceKey(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
