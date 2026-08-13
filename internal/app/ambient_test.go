package app

import (
	"math"
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

// ambientGame builds a game whose ambient pool is pool and whose random rolls
// come from rolls, cycling. Like rand.Intn, a roll stays inside the bound it was
// asked for.
func ambientGame(pool []string, rolls ...int) (*Game, *fakeAudio) {
	fa := &fakeAudio{}
	var entries []types.SoundVar
	for _, w := range pool {
		entries = append(
			entries,
			types.SoundVar{Name: "fon1", Wav: w, Ambient: true},
		)
	}
	i := 0
	g := &Game{
		res:         silentRes{},
		audio:       fa,
		ambientPool: entries,
		randn: func(n int) int {
			r := rolls[i%len(rolls)]
			i++
			return r % n
		},
	}
	return g, fa
}

// A scene arms its ambient timer already expired, so the place speaks up on the
// first tick rather than opening with silence, and then waits out the rolled gap.
func TestAmbientFiresThenWaits(t *testing.T) {
	// rolls: pick 1, no fade, hard left pan, then a 2 s gap
	g, fa := ambientGame([]string{"bird01.wav", "bird02.wav"}, 1, 0, 0, 2000)

	g.updateAmbient(1.0 / 60)
	if len(fa.ambient) != 1 || fa.ambient[0] != "bird02.wav" {
		t.Fatalf("ambient = %v, want bird02.wav on the first tick", fa.ambient)
	}
	if fa.vols[0] != 1 {
		t.Errorf("volScale = %v, want 1 for a zero fade roll", fa.vols[0])
	}
	if fa.pans[0] != -ambientMaxPan {
		t.Errorf(
			"pan = %v, want %v for a zero pan roll",
			fa.pans[0],
			-ambientMaxPan,
		)
	}
	if math.Abs(g.ambientT-2) > 1e-9 {
		t.Errorf("next shot in %v s, want 2", g.ambientT)
	}

	// Nothing more until the gap runs out.
	for i := 0; i < 60; i++ {
		g.updateAmbient(1.0 / 60)
	}
	if len(fa.ambient) != 1 {
		t.Errorf("fired %d shots inside the gap, want 1", len(fa.ambient))
	}
	for i := 0; i < 61; i++ {
		g.updateAmbient(1.0 / 60)
	}
	if len(fa.ambient) != 2 {
		t.Errorf("fired %d shots after the gap, want 2", len(fa.ambient))
	}
}

// The rolled fade is in hundredths of a decibel, so the deepest roll has to come
// out quiet but audible, not silent and not full volume.
func TestAmbientFadeRollAttenuates(t *testing.T) {
	g, fa := ambientGame([]string{"chaiki.wav"}, 0, ambientMaxFade-1, 4000, 0)

	g.updateAmbient(1)
	if len(fa.vols) != 1 {
		t.Fatalf("shots = %d, want 1", len(fa.vols))
	}
	if v := fa.vols[0]; v < 0.03 || v > 0.04 {
		t.Errorf("volScale = %v, want ~0.032 (-30 dB)", v)
	}
	if p := fa.pans[0]; p != 0 {
		t.Errorf("pan = %v, want centre for a mid roll", p)
	}
}

// Most scenes carry no ambience at all, and those must stay quiet without
// touching the random source.
func TestAmbientEmptyPoolIsSilent(t *testing.T) {
	g, fa := ambientGame(nil)
	g.randn = func(int) int { panic("an empty pool must not roll the dice") }

	g.updateAmbient(1)
	if len(fa.ambient) != 0 {
		t.Errorf("ambient = %v, want silence", fa.ambient)
	}
}
