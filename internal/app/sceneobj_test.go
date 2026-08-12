package app

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

// frameOf is one script frame carrying the given event keywords.
func frameOf(kws ...string) *types.Frame {
	f := &types.Frame{}
	for _, kw := range kws {
		f.Events = append(f.Events, types.Command{Kw: kw})
	}
	return f
}

// An object's FonScript is ambient scenery and loops by default, so the decision
// is whether its last frame closes the script out with a world change. Get it
// wrong the other way and a driver like START.FS fires its GoScene on every
// loop, sending the intro round in circles.
func TestEndsScriptOnlyForTerminalLastFrame(t *testing.T) {
	cases := []struct {
		name string
		fs   *types.FrameScript
		want bool
	}{
		{"empty script", &types.FrameScript{}, false},
		{"ambient sound and text", &types.FrameScript{
			Frames: []*types.Frame{frameOf("sound"), frameOf("text")},
		}, false},
		{"no events at all", &types.FrameScript{
			Frames: []*types.Frame{frameOf(), frameOf()},
		}, false},
		{"goscene", &types.FrameScript{
			Frames: []*types.Frame{frameOf("sound"), frameOf("goscene")},
		}, true},
		// The scripts are written in mixed case, so the match must fold.
		{"mixed case GoScene", &types.FrameScript{
			Frames: []*types.Frame{frameOf("GoScene")},
		}, true},
		{"delobject", &types.FrameScript{
			Frames: []*types.Frame{frameOf("DelObject")},
		}, true},
		{"deleteobject", &types.FrameScript{
			Frames: []*types.Frame{frameOf("DeleteObject")},
		}, true},
		{"showchar", &types.FrameScript{
			Frames: []*types.Frame{frameOf("ShowChar")},
		}, true},
		{"startgame", &types.FrameScript{
			Frames: []*types.Frame{frameOf("StartGame")},
		}, true},
		{"terminal command among others", &types.FrameScript{
			Frames: []*types.Frame{frameOf("sound", "goscene", "text")},
		}, true},
		// Only the *last* frame ends a script. An ambient loop may well delete
		// something mid-way and keep running afterwards.
		{"terminal command earlier on", &types.FrameScript{
			Frames: []*types.Frame{frameOf("delobject"), frameOf("sound")},
		}, false},
	}
	for _, c := range cases {
		if got := endsScript(c.fs); got != c.want {
			t.Errorf("%s: endsScript = %v, want %v", c.name, got, c.want)
		}
	}
}
