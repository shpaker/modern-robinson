package use_cases

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

func TestPlayerLoop(t *testing.T) {
	fs := &types.FrameScript{
		Looping: true,
		Frames: []*types.Frame{
			{
				Index: 0,
				Delay: 100,
				Events: []types.Command{
					{Kw: "sound", Args: []string{"fire", "4"}},
				},
			},
			{Index: 1, Delay: 100},
			{Index: 2, Delay: 100},
		},
	}
	p := NewPlayer(fs)
	ev := p.Update(0) // enters frame 0
	if len(ev) != 1 || ev[0].Kw != "sound" || p.FrameIndex() != 0 {
		t.Fatalf("frame0: ev=%v idx=%d", ev, p.FrameIndex())
	}
	for _, want := range []int{1, 2, 0, 1} {
		if p.Update(0.1); p.FrameIndex() != want {
			t.Fatalf("after step got idx=%d, want %d", p.FrameIndex(), want)
		}
	}
	if p.Done() {
		t.Error("looping player must never finish")
	}
}

func TestPlayerOneShot(t *testing.T) {
	fs := &types.FrameScript{
		Looping: false,
		Frames: []*types.Frame{
			{Index: 0, Delay: 100},
			{Index: 1, Delay: 100, Events: []types.Command{{Kw: "delobject"}}},
		},
	}
	p := NewPlayer(fs)
	p.Update(0)
	p.Update(0.1) // -> frame 1
	if p.FrameIndex() != 1 || p.Done() {
		t.Fatalf("at frame1: idx=%d done=%v", p.FrameIndex(), p.Done())
	}
	p.Update(0.1) // past last -> done
	if !p.Done() {
		t.Error("one-shot player must finish after last frame")
	}
}

func TestPlayerNegativeDelay(t *testing.T) {
	// negative Delay (ambient) uses |Delay|
	fs := &types.FrameScript{
		Looping: true,
		Frames: []*types.Frame{
			{Index: 0, Delay: -457},
			{Index: 1, Delay: -457},
		},
	}
	p := NewPlayer(fs)
	p.Update(0)
	p.Update(0.4) // < 0.457 -> still frame 0
	if p.FrameIndex() != 0 {
		t.Fatalf("idx=%d, want 0", p.FrameIndex())
	}
	p.Update(0.1) // crosses 0.457 -> frame 1
	if p.FrameIndex() != 1 {
		t.Fatalf("idx=%d, want 1", p.FrameIndex())
	}
}
