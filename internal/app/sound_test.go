package app

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// fakeAudio records what it was asked to play instead of opening a device.
type fakeAudio struct {
	played []string  // every key asked for, on a channel or a voice
	voiced []string  // the keys sent to a sound variable's voices
	counts []int     // and how many voices each variable has,
	vols   []float64 // its level and placement, shot by shot
	pans   []float64
	stops  int
	sound  float64 // the effects and music levels last set
	music  float64
}

func (a *fakeAudio) Play(key string, _ []byte, _ int) {
	a.played = append(a.played, key)
}

func (a *fakeAudio) PlayVoice(
	key string,
	_ []byte,
	voices int,
	volScale, pan float64,
) {
	a.played = append(a.played, key)
	a.voiced = append(a.voiced, key)
	a.counts = append(a.counts, voices)
	a.vols = append(a.vols, volScale)
	a.pans = append(a.pans, pan)
}

func (a *fakeAudio) PlayMusic(string, []byte) {}
func (a *fakeAudio) StopMusic()               {}
func (a *fakeAudio) StopEffects()             { a.stops++ }
func (a *fakeAudio) SetVolume(v float64)      { a.sound = v }
func (a *fakeAudio) SetMusicVolume(v float64) { a.music = v }

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

// voiceRes serves every sound as a wav whose length the engine's arithmetic
// puts at exactly one second: 44140 bytes = the 40000-byte header allowance
// plus 44100 (22050 Hz, 16-bit mono).
type voiceRes struct{ interfaces.IResources }

func (voiceRes) Sound(string) []byte { return make([]byte, 44140) }

// parseFS retimes the frames under a voice line the way the engine does on
// load: Sound int1b,5,2 respreads the wav's second over its two frames, and
// the name resolves through the scene's SoundVariables.
func TestParseFSRetimesVoicedFrames(t *testing.T) {
	g := &Game{
		res:    voiceRes{},
		parser: repositories.SceneParser{},
		sc: &types.Scene{SoundVars: map[string][2]string{
			"int1b": {"int1b.wav", "5"},
		}},
	}
	fs := g.parseFS([]byte(
		"MovieName Int1.mv;\nTotalFrames 3;\n" +
			"Frame 0,1;\nDelay 142;\nSound int1b,5,2;\n" +
			"Frame 1,1;\nDelay 339;\n" +
			"Frame 2,1;\nDelay 142;\nEnd;",
	))
	want := []int{500, 500, 142}
	for i, w := range want {
		if fs.Frames[i].Delay != w {
			t.Fatalf(
				"frame %d Delay = %d, want %d", i, fs.Frames[i].Delay, w,
			)
		}
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
		{"F toggles full screen", ebiten.KeyF, true},
		{"F3 switches the CRT", ebiten.KeyF3, true},
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

// A Sound event names either a scene's sound variable or a quoted file, and the
// engine treats them apart (0x41B4D4): the file goes to the numbered channel,
// the variable to its own voices whatever channel the event names.
func TestPlaySoundVariableIgnoresChannel(t *testing.T) {
	fa := &fakeAudio{}
	g := &Game{
		res:   silentRes{},
		audio: fa,
		sc: &types.Scene{SoundVars: map[string][2]string{
			"wave0": {"Wave.wav", "1"},
			"fon1":  {"sound26.wav", "7"},
		}},
	}

	g.playSound([]string{"wave0", "8"})
	g.playSound([]string{"fon1", "7"})
	g.playSound([]string{"rr088.wav", "1"})

	if len(fa.voiced) != 2 || fa.voiced[0] != "wave.wav" ||
		fa.voiced[1] != "sound26.wav" {
		t.Fatalf("voiced %v, want the two variables", fa.voiced)
	}
	if fa.counts[0] != 1 || fa.counts[1] != 7 {
		t.Errorf("voices %v, want the SoundVariables counts [1 7]", fa.counts)
	}
	if fa.vols[0] != 1 || fa.pans[0] != 0 {
		t.Errorf("a script's sound plays at %v pan %v, want 1 and 0",
			fa.vols[0], fa.pans[0])
	}
	if len(fa.played) != 3 || fa.played[2] != "rr088.wav" {
		t.Errorf("played %v, want the quoted file on its channel last",
			fa.played)
	}
}

// A direct scene entry (ROBINSON_SCENE, ?scene= on the web) skips INT0, the
// script that hides Friday before she joins. Leaving SCENA0 by the oak aims an
// Aproach at her, and with her left shown she walked there unseen, her steps
// (at2, at, step_pp) sounding over Robinson's exit.
func TestLeavingScena0DirectEntryKeepsFridaySilent(t *testing.T) {
	t.Setenv("ROBINSON_SCENE", "SCENA0")
	res := repositories.NewResources(testutil.GameRoot(t))
	g := NewGameWith(res, DefaultConfig())
	fa := &fakeAudio{}
	g.audio = fa
	if !g.fridHidden {
		t.Fatal("a run must open with Friday hidden")
	}

	if !g.startObjectAction("goleft") {
		t.Fatal("no ROHANGOL")
	}
	for i := 0; i < 60*30 && g.sceneName == "SCENA0"; i++ {
		_ = g.Update()
	}
	if g.sceneName != "SCENA1" {
		t.Fatalf("scene = %s, want the walk to reach SCENA1", g.sceneName)
	}
	for _, k := range fa.played {
		switch strings.ToLower(k) {
		case "step_pp.wav", "at.wav", "at2.wav", "left.wav", "right.wav":
			t.Errorf("Friday's %s sounded, want her absent: %v", k, fa.played)
		}
	}
}

// The levels config.yml sets reach the sounds from the start, not only once
// a slider is touched: until then the voice played at the adapter's default.
func TestConfigVolumesReachTheAudio(t *testing.T) {
	t.Setenv("ROBINSON_SCENE", "SCENA0")
	cfg := DefaultConfig()
	cfg.Sound, cfg.Music = 0.25, 0.4
	fa := &fakeAudio{}
	newGame(repositories.NewResources(testutil.GameRoot(t)), cfg, fa)
	if fa.sound != 0.25 || fa.music != 0.4 {
		t.Errorf("sound = %v music = %v, want 0.25 and 0.4", fa.sound, fa.music)
	}
}
