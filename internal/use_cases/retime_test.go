package use_cases

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

// fs builds a script of frames with the given delays.
func fsWithDelays(delays ...int) *types.FrameScript {
	fs := &types.FrameScript{}
	for i, d := range delays {
		fs.Frames = append(fs.Frames, &types.Frame{Index: i, Delay: d})
	}
	return fs
}

// delays reads the frame delays back.
func delays(fs *types.FrameScript) []int {
	out := make([]int, len(fs.Frames))
	for i, f := range fs.Frames {
		out[i] = f.Delay
	}
	return out
}

// The three-argument Sound respreads the delays of N frames — its own first —
// evenly over the voice line, leaving the frames past the span alone. The
// authored delays are placeholders: rs9011.wav really is 3302 ms over 9 frames
// of authored 372, and the engine plays them at 366 each.
func TestRetimeByVoiceRespreadsDelays(t *testing.T) {
	fs := fsWithDelays(142, 372, 372, 372, 142)
	fs.Frames[1].Events = []types.Command{
		{Kw: "sound", Args: []string{"rs9011.wav", "1", "3"}},
	}
	RetimeByVoice(fs, func(name string) (int, bool) {
		if name != "rs9011.wav" {
			t.Fatalf("resolved %q, want rs9011.wav", name)
		}
		return 3302, true
	})
	want := []int{142, 1100, 1100, 1100, 142}
	for i, w := range want {
		if fs.Frames[i].Delay != w {
			t.Fatalf("delays = %v, want %v", delays(fs), want)
		}
	}
}

// A negative delay marks its frame as unscaled in the engine, and the retimer
// keeps that sign; a zero one is left alone but still spends a slot of N.
func TestRetimeByVoiceKeepsSignAndSkipsZero(t *testing.T) {
	fs := fsWithDelays(100, -100, 0, 100, 100)
	fs.Frames[0].Events = []types.Command{
		{Kw: "sound", Args: []string{"v", "1", "4"}},
	}
	RetimeByVoice(fs, func(string) (int, bool) { return 800, true })
	want := []int{200, -200, 0, 200, 100}
	for i, w := range want {
		if fs.Frames[i].Delay != w {
			t.Fatalf("delays = %v, want %v", delays(fs), want)
		}
	}
}

// A two-argument Sound plays whole and moves no frames; an unresolvable name
// leaves the authored timing too (the engine only retimes what it can find).
func TestRetimeByVoiceLeavesUntimedAlone(t *testing.T) {
	fs := fsWithDelays(142, 142)
	fs.Frames[0].Events = []types.Command{
		{Kw: "sound", Args: []string{"step", "1"}},
		{Kw: "sound", Args: []string{"ghost", "1", "2"}},
	}
	RetimeByVoice(fs, func(string) (int, bool) { return 0, false })
	for i, w := range []int{142, 142} {
		if fs.Frames[i].Delay != w {
			t.Fatalf("delays = %v, want unchanged", delays(fs))
		}
	}
}

// The last Sound of a frame wins, as in the engine: its span replaces the
// earlier one's from that frame on.
func TestRetimeByVoiceLastSoundOfFrameWins(t *testing.T) {
	fs := fsWithDelays(100, 100)
	fs.Frames[0].Events = []types.Command{
		{Kw: "sound", Args: []string{"a", "1", "2"}},
		{Kw: "sound", Args: []string{"b", "1", "1"}},
	}
	RetimeByVoice(fs, func(name string) (int, bool) {
		if name == "a" {
			return 2000, true
		}
		return 600, true
	})
	want := []int{600, 100}
	for i, w := range want {
		if fs.Frames[i].Delay != w {
			t.Fatalf("delays = %v, want %v", delays(fs), want)
		}
	}
}
