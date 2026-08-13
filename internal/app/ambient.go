package app

import (
	"math"
	"strings"
)

// Scene ambience: the entries a .SCN marks with a trailing "*" form a pool the
// engine plays on its own, with no script involved (birds on the island paths,
// gulls at the shore, crickets in the jungle). Every so often it picks one at
// random and fires it at a random level and a random pan, so a handful of short
// clips reads as a living place. Engine: the per-scene tick at 0x414230.
const (
	// ambientMaxGap is the longest wait between shots; the engine rolls
	// rand()%3000 ms fresh each time, so shots cluster and thin out on their own.
	ambientMaxGap = 3.0
	// ambientMaxFade is the deepest attenuation of a shot, in hundredths of a
	// decibel (the engine's DirectSound units): a roll of 3000 means -30 dB.
	ambientMaxFade = 3000
	// ambientMaxPan is how far off-centre a shot can sit, as a fraction of the
	// full width (the engine rolls +-4000 of DirectSound's +-10000).
	ambientMaxPan = 0.4
)

// updateAmbient fires the scene's next ambient shot when its timer runs out.
// The timer runs on game time, like every other timer here, so the speed slider
// thins the ambience along with the rest of the simulation.
func (g *Game) updateAmbient(dt float64) {
	if len(g.ambientPool) == 0 {
		return
	}
	if g.ambientT -= dt; g.ambientT > 0 {
		return
	}
	e := g.ambientPool[g.randn(len(g.ambientPool))]
	// Hundredths of a decibel to a linear factor: 0 -> 1, 3000 -> ~0.03.
	vol := math.Pow(10, -float64(g.randn(ambientMaxFade))/2000)
	pan := float64(g.randn(2*ambientMaxPanUnits)-ambientMaxPanUnits) / panScale
	g.audio.PlayAmbient(strings.ToLower(e.Wav), g.res.Sound(e.Wav), vol, pan)
	g.ambientT = float64(g.randn(int(ambientMaxGap*1000))) / 1000
}

// The pan roll in the engine's own units, kept whole so randn stays integral.
const (
	panScale           = 10000 // DirectSound's full pan half-width
	ambientMaxPanUnits = int(ambientMaxPan * panScale)
)
