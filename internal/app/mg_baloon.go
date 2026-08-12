package app

import (
	"image"
	"math"
	"math/rand"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The balloon flight (StartGame 3 -> LandOK). The whole navigation puzzle is
// that the heading depends only on the altitude — 540 degrees of turn across
// the 50..550 range — so the player steers by climbing (the Roby button) and
// descending (the Frid button) until the neighbouring island drifts under the
// basket, then drops below the landing height.
type baloonGame struct {
	sprites map[string]*ebiten.Image
	x, y    float64 // world position, [30..970]
	alt     float64 // displayed altitude
	target  float64 // altitude the balloon eases towards
	wind    float64 // heading base angle
	landing bool
	won     bool
	finishT float64
}

// World constants from the engine.
const (
	balWorldMin = 30
	balWorldMax = 970
	balHomeX    = 763
	balHomeY    = 738
	balLandX    = 251
	balLandY    = 246
	balAltMin   = 50
	balAltMax   = 550
	balLandAlt  = 175
	balLandDist = 50
)

// The panel buttons.
var (
	balRoby = image.Rect(415, 408, 483, 465)
	balFrid = image.Rect(490, 408, 558, 465)
	balExit = image.Rect(565, 408, 633, 465)
)

// newBaloonGame starts at the home island, high up.
func newBaloonGame(g *Game) minigame {
	b := &baloonGame{
		x: balHomeX, y: balHomeY,
		alt: 412, target: 412,
	}
	b.sprites = g.packImages("BALOON")
	if b.sprites["SKY"] == nil {
		return nil
	}
	return b
}

// update flies the balloon: ease the altitude, turn with it, drift, land.
func (b *baloonGame) update(g *Game, dt float64) (bool, int) {
	if b.won {
		b.finishT += dt
		return b.finishT > 3 || clickedThisTick(), 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true, 0
	}
	if clickedThisTick() {
		mx, my := ebiten.CursorPosition()
		switch {
		case pointIn(balExit, mx, my):
			return true, 0
		case pointIn(balRoby, mx, my) && !b.landing:
			b.target = math.Min(balAltMax, b.target+32)
		case pointIn(balFrid, mx, my) && !b.landing:
			b.target = math.Max(balAltMin, b.target-16)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) && !b.landing {
		b.target = math.Min(balAltMax, b.target+32)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && !b.landing {
		b.target = math.Max(balAltMin, b.target-16)
	}

	// The engine runs this at 30 fps; scale to our tick.
	steps := dt * 30
	switch {
	case b.alt < b.target:
		b.alt = math.Min(b.target, b.alt+2*steps)
	case b.alt > b.target:
		b.alt = math.Max(b.target, b.alt-2*steps)
	}
	if b.landing {
		b.alt -= 2 * steps
		if b.alt < -60 {
			b.won = true
			g.playSound([]string{"final3.wav", "1"})
		}
		return false, 0
	}
	// Heading from altitude, drift with a jittered step.
	theta := b.wind + 3*math.Pi*b.alt/550
	vx := math.Sin(theta) + math.Cos(theta)
	vy := math.Sin(theta) - math.Cos(theta)
	k := (1 - 0.01*float64(rand.Intn(20))) * 0.3 * steps
	b.x += k * vx
	b.y += k * vy
	// Crossing an edge swings the wind to push the balloon back in.
	if b.x < balWorldMin || b.x > balWorldMax ||
		b.y < balWorldMin || b.y > balWorldMax {
		b.wind += 2 * math.Pi / 3
		b.x = clampF(b.x, balWorldMin, balWorldMax)
		b.y = clampF(b.y, balWorldMin, balWorldMax)
	}
	// The landing test.
	if b.alt <= balLandAlt &&
		math.Abs(b.x-balLandX) < balLandDist &&
		math.Abs(b.y-balLandY) < balLandDist {
		b.landing = true
		g.playSound([]string{"stnbalon.wav", "1"})
	}
	return false, 0
}

// island picks the closest authored scale of an island for a distance.
func (b *baloonGame) island(base string, d float64) *ebiten.Image {
	// The engine projects a continuous scale and takes the nearest of the
	// eight prescaled bitmaps (100/90/80/65/50/30/15/10 %).
	s := 1 - math.Min(0.85, (d-50)/(math.Sqrt(500000)-50))
	scales := [8]float64{1, .9, .8, .65, .5, .3, .15, .1}
	best, bestD := 0, math.Inf(1)
	for i, sc := range scales {
		if diff := math.Abs(sc - s); diff < bestD {
			best, bestD = i, diff
		}
	}
	if best == 0 {
		return b.sprites[base]
	}
	return b.sprites[base+strconv.Itoa(best)]
}

// draw paints the sky, the islands sliding under the basket, and the panel
// with its course strip, altitude ruler and radar blip.
func (b *baloonGame) draw(_ *Game, screen *ebiten.Image) {
	// Sky scroll: high altitude shows the clouds, low the sea.
	skyY := 120*b.alt/550 - 130
	blitAt(screen, b.sprites["SKY"], 0, int(skyY))

	// The two islands drift relative to the balloon (screen centre).
	for _, is := range [2]struct {
		name   string
		wx, wy float64
	}{{"ISLAND", balHomeX, balHomeY}, {"LAND", balLandX, balLandY}} {
		d := math.Hypot(b.x-is.wx, b.y-is.wy)
		img := b.island(is.name, d+b.alt)
		if img == nil {
			continue
		}
		bd := img.Bounds()
		x := 320 + int(is.wx-b.x) - bd.Dx()/2
		y := 200 + int(is.wy-b.y) - bd.Dy()/2
		blitAt(screen, img, x, y)
	}

	// The instrument panel.
	blitAt(screen, b.sprites["BAR"], 0, 400)
	// Course strip in its window: 360 degrees = 240 px, zero at x 83.
	theta := b.wind + 3*math.Pi*b.alt/550
	vx := math.Sin(theta) + math.Cos(theta)
	vy := math.Sin(theta) - math.Cos(theta)
	cx := 83 - (math.Atan2(vx, -vy)+math.Pi)*(240/(2*math.Pi))
	for cx < 140-243 {
		cx += 243
	}
	sub := screen.SubImage(image.Rect(148, 449, 259, 461)).(*ebiten.Image)
	blitAt(sub, b.sprites["COURSE"], int(cx), 448)
	// Altitude ruler in its window.
	subA := screen.SubImage(image.Rect(355, 416, 390, 464)).(*ebiten.Image)
	blitAt(subA, b.sprites["ALT"], 358, int(0.21*(b.alt-550)+37)+400)
	// Radar blip.
	blitAt(screen, b.sprites["BALL"],
		3+int(0.127*b.x), 406+int(0.065*b.y))
}
