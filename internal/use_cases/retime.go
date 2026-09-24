package use_cases

import (
	"strconv"
	"strings"

	"github.com/shpaker/modern-robinson/internal/types"
)

// RetimeByVoice fits a script's frame delays to its voice lines, the way the
// engine does to every script it loads (RetimeByVoice, ROBY.EXE 0x416F60).
//
// The third argument of Sound name,ch,N is not about the sound at all — the
// sound always plays whole. N counts script frames, starting at the one the
// Sound stands in, whose Delay is rewritten to an equal share of the wav's
// duration: the animation is stretched or squeezed under the line, which is
// what keeps the lips in sync. The authored delays under a voice line are
// rough placeholders (they run from 0.17x to 6.2x of the wav), so skipping
// this pass plays half the dialogue at a visibly wrong pace.
//
// durMS resolves a Sound event's name (a scene SoundVariable or a wav file)
// to its duration in milliseconds; a name it cannot resolve leaves the frames
// as authored. A positive Delay takes the share as is, a negative one keeps
// its sign, a zero one is left alone but still consumes a slot. The last
// Sound of a frame wins, as in the engine.
func RetimeByVoice(
	fs *types.FrameScript,
	durMS func(name string) (int, bool),
) {
	perFrame, remain := 0, 0
	for _, f := range fs.Frames {
		for _, ev := range f.Events {
			if !strings.EqualFold(ev.Kw, "sound") || len(ev.Args) < 3 {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(ev.Args[2]))
			if err != nil || n <= 0 {
				continue
			}
			ms, ok := durMS(ev.Args[0])
			if !ok {
				continue
			}
			perFrame, remain = ms/n, n
		}
		if remain > 0 {
			switch {
			case f.Delay > 0:
				f.Delay = perFrame
			case f.Delay < 0:
				f.Delay = -perFrame
			}
			remain--
		}
	}
}
