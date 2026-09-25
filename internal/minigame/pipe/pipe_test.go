package pipe

import (
	"image"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters/mouse"
	"github.com/shpaker/modern-robinson/internal/minigame"
)

// recorder is a Host with a backdrop and no other asset, which notes every
// sound the organ plays.
type recorder struct {
	minigame.Host
	played []string
}

func (*recorder) Images(string, func(string) int) map[string]*ebiten.Image {
	return map[string]*ebiten.Image{"BACK": ebiten.NewImage(1, 1)}
}

func (r *recorder) PlaySound(file string, _ int) {
	r.played = append(r.played, file)
}

const tick = 1.0 / 60

// click is the player's click in the middle of r, a tick a step.
func click(p *pipeGame, r image.Rectangle) {
	x, y := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	for _, down := range []bool{false, true, false} {
		mouse.Hold(x, y, down, false)
		p.Update(tick)
	}
}

// The drummer's phrase sounds each mouth by the tube that stands in it, as
// the engine loads the notes: by their place in the sound bank, which is not
// the order of their names. Both arrangements the engine accepts play the
// phrase through and solve the organ; tubes 0 and 7, which they swap, are
// PIPE02 and PIPE08 — one note twice over.
func TestTheOrganPlaysTheTubesInItsMouths(t *testing.T) {
	defer mouse.Release()
	want := [2]string{
		"pipe01 pipe02 pipe01 pipe02 pipe03 pipe01 pipe04 pipe05 pipe05 " +
			"pipe05 pipe06 pipe07 pipe02 pipe02 pipe08",
		"pipe01 pipe08 pipe01 pipe08 pipe03 pipe01 pipe04 pipe05 pipe05 " +
			"pipe05 pipe06 pipe07 pipe08 pipe08 pipe02",
	}
	for i, win := range pipeWins {
		h := &recorder{}
		g := New(h, 7)
		if g == nil {
			t.Fatal("no organ")
		}
		p := g.(*pipeGame)
		for m, tube := range win {
			click(p, p.tubeRect(tube))
			click(p, mouthRect(m))
			if p.inMouth[m] != tube {
				t.Fatalf("arrangement %d: tube %d did not go into mouth %d",
					i, tube, m)
			}
		}
		seated := strings.Join(h.played, " ")
		h.played = nil
		click(p, pipeListen)
		done, result := false, 0
		for n := 0; n < 60*60 && !done; n++ {
			done, result = p.Update(tick)
		}
		got := strings.ReplaceAll(strings.Join(h.played, " "), ".wav", "")
		if got != want[i] {
			t.Errorf("arrangement %d plays %q, want %q", i, got, want[i])
		}
		if !done || result != 1 {
			t.Errorf("arrangement %d: done = %v result = %d, want solved",
				i, done, result)
		}
		if strings.Contains(seated, pipeDud) {
			t.Errorf("arrangement %d: a seated tube sounded the dud: %s",
				i, seated)
		}
	}
}
