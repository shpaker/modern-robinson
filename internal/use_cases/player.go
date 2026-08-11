package use_cases

import (
	"math"

	"github.com/shpaker/modern-robinson/internal/types"
)

// Player advances a FrameScript over time: it tracks the current frame, wraps
// (looping FonScripts) or stops (one-shot actions), and reports the event
// commands of every frame it enters so the caller can fire Sound/Text/logic.
// Stateless w.r.t. the engine — pure timing over Domain data.
type Player struct {
	fs      *types.FrameScript
	idx     int
	acc     float64
	started bool
	done    bool
}

// NewPlayer starts a player at frame 0 of fs.
func NewPlayer(fs *types.FrameScript) *Player { return &Player{fs: fs} }

// FrameIndex returns the current frame's NGB index (0-based).
func (p *Player) FrameIndex() int {
	if p.fs == nil || len(p.fs.Frames) == 0 {
		return 0
	}
	return p.fs.Frames[p.idx].Index
}

// Done reports whether a one-shot script has finished.
func (p *Player) Done() bool { return p.done }

// Update advances by dt seconds and returns the events of every frame entered
// during this call (including frame 0 on the first call).
func (p *Player) Update(dt float64) []types.Command {
	if p.fs == nil || len(p.fs.Frames) == 0 || p.done {
		return nil
	}
	var fired []types.Command
	if !p.started {
		p.started = true
		fired = append(fired, p.fs.Frames[0].Events...)
	}
	p.acc += dt
	for !p.done {
		d := math.Abs(float64(p.fs.Frames[p.idx].Delay)) / 1000.0
		if d <= 0 {
			d = 0.001
		}
		if p.acc < d {
			break
		}
		p.acc -= d
		switch {
		case p.idx+1 < len(p.fs.Frames):
			p.idx++
		case p.fs.Looping:
			p.idx = 0
		default:
			p.done = true
		}
		if !p.done {
			fired = append(fired, p.fs.Frames[p.idx].Events...)
		}
	}
	return fired
}
