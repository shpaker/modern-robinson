package crt

import (
	"math"
	"math/rand/v2"
)

// glitch is a fault of a worn set, played over a moment.
type glitch int

const (
	calm    glitch = iota
	jitter         // the lines tear sideways: horizontal sync slips
	roll           // the picture slips a whole height: vertical sync
	snow           // a burst of snow
	degauss        // the degaussing coil shakes the mask; colours wobble
	ripple         // a change of scene: a little jitter and snow, dying out
)

// lasts is how long each glitch plays, in seconds.
var lasts = [...]float64{
	jitter: 0.4, roll: 0.8, snow: 0.35, degauss: 1.4, ripple: 0.45,
}

// Between two random glitches the tube keeps quiet for a spell around
// Options.Glitches seconds: two thirds of it to four thirds.
const (
	quietMin = 2.0 / 3
	quietMax = 4.0 / 3
)

// Warming up shows a dot, then a line, then the picture; dying folds the
// picture to a line, the line to a dot and fades the dot. Seconds.
const (
	warmUp   = 0.55
	coolDown = 0.7
)

// clock is the tube's time: the seconds it has played, the glitch playing,
// the power coming or going. It runs on the game's ticks, not the wall
// clock, so a scripted run sees the same tube every time.
type clock struct {
	rnd   *rand.Rand
	every float64 // mean seconds between random glitches; 0: none
	now   float64
	ticks int     // the field shown: even or odd (Interlace)
	next  float64 // when the next random glitch is due
	g     glitch
	gt    float64 // how long g has played

	warm     float64 // seconds since power on, at warmRate
	warmRate float64
	dying    bool
	cold     float64 // seconds since power off
}

func newClock(rnd *rand.Rand, every float64) clock {
	c := clock{rnd: rnd, every: every, warmRate: 1}
	c.schedule()
	return c
}

// schedule sets the next random glitch a quiet spell from now, or never.
func (c *clock) schedule() {
	if c.every <= 0 {
		c.next = math.Inf(1)
		return
	}
	k := quietMin + (quietMax-quietMin)*c.rnd.Float64()
	c.next = c.now + c.every*k
}

// tick advances the clock by dt seconds. held (a mouse button down) keeps a
// glitch that falls due waiting, so none lands in the middle of a drag.
func (c *clock) tick(dt float64, held bool) {
	c.now += dt
	c.ticks++
	if c.dying {
		c.cold += dt
	} else {
		c.warm += dt * c.warmRate
	}
	if c.g != calm {
		if c.gt += dt; c.gt >= lasts[c.g] {
			c.g = calm
		}
	}
	if c.g == calm && !held && c.now >= c.next {
		c.start(jitter + glitch(c.rnd.IntN(int(ripple-jitter))))
		c.schedule()
	}
}

// start plays g from its beginning, over whatever was playing.
func (c *clock) start(g glitch) { c.g, c.gt = g, 0 }

// shape is the playing glitch as the shader takes it: line jitter, frame
// roll, snow, degauss, each 0 when quiet.
func (c *clock) shape() [4]float32 {
	var s [4]float32
	if c.g == calm {
		return s
	}
	k := c.gt / lasts[c.g]
	switch c.g {
	case jitter:
		s[0] = float32(1 - k)
	case roll:
		s[1] = float32(0.0001 + easeInOut(k))
	case snow:
		s[2] = float32(math.Sin(math.Pi * k))
	case degauss:
		s[3] = float32((1 - k) * (1 - k))
	case ripple:
		s[0] = float32(0.8 * (1 - k) * (1 - k))
		s[2] = float32(0.3 * (1 - k))
	}
	return s
}

func easeInOut(k float64) float64 {
	if k < 0.5 {
		return 2 * k * k
	}
	return 1 - (2-2*k)*(2-2*k)/2
}

// power is the raster's squeeze as the tube warms up or dies — its width
// and height, 1 when whole — and the flash of the beam.
func (c *clock) power() (w, h, flash float64) {
	if c.dying {
		k := c.cold
		switch {
		case k < 0.18:
			u := k / 0.18
			return 1, math.Max(0.004, 1-u*u), 0.8 * u
		case k < 0.32:
			u := (k - 0.18) / 0.14
			return math.Max(0.004, 1-u), 0.004, 0.9
		case k < coolDown:
			u := (k - 0.32) / (coolDown - 0.32)
			return 0.004, 0.004, 0.9 * (1 - u)
		}
		return 0, 0, 0
	}
	k := c.warm
	switch {
	case k < 0.12:
		return math.Max(0.004, k/0.12), 0.006, 1.2
	case k < warmUp:
		u := (k - 0.12) / (warmUp - 0.12)
		return 1, math.Max(0.006, 1-(1-u)*(1-u)*(1-u)), 1.2 * (1 - u)
	}
	return 1, 1, 0
}

// dark reports the tube dead after a power off.
func (c *clock) dark() bool { return c.dying && c.cold >= coolDown }
