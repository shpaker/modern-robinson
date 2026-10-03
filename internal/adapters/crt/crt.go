// Package crt shows the frame on a CRT tube of the day: the glass bulges it,
// the beam draws it in soft scanlines through a slot mask, the bright areas
// glow, the red and blue guns drift out of register towards the edges, and
// the set hums and snows — with a glitch every minute or two and a ripple on
// a change of scene. It is the frame's last pass, onto the screen itself, so
// whatever reads the frame (save thumbnails, the MCP driver's picture) never
// sees it. The tube is a kinescope TV: its Rubin, tuned by the game's dials.
package crt

import (
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/shpaker/kinescope"
	"github.com/shpaker/kinescope/ebitengine"
)

// fullScreen is the tube's signal of full screen: there the monitor case
// goes around the picture (Options.Case).
const fullScreen = "fullscreen"

// caseMargin is the case's width in full screen: 0.15 of the picture's
// half-height on every side, case and all fitting the screen's height.
const caseMargin = 0.15

// Tube is the CRT the frame is shown on. A nil Tube is one switched off.
type Tube struct {
	opts     Options
	on       bool
	tv       *kinescope.TV
	renderer *ebitengine.Renderer
	full     *kinescope.Level
}

// New builds the tube, on or off; switched on, it warms up. Should its
// shaders not compile, it stays off for good and the error says why.
func New(on bool, o Options) (*Tube, error) {
	t := &Tube{opts: o}
	tv, err := kinescope.NewTV(setup(o, rand.Uint64()))
	if err != nil {
		return t, err
	}
	tv.Apply(o.values())
	full, err := tv.Signal(fullScreen)
	if err != nil {
		return t, err
	}
	renderer, err := ebitengine.NewRenderer()
	if err != nil {
		return t, err
	}
	if err := renderer.Prepare(tv); err != nil {
		return t, err
	}
	t.tv, t.renderer, t.full, t.on = tv, renderer, full, on
	if on {
		tv.PowerOn()
	}
	return t, nil
}

// setup is the tube's kinescope setup: Rubin, its glitches as often as the
// options say, its case only in full screen and only if the options want
// it.
func setup(o Options, seed uint64) kinescope.Setup {
	s := kinescope.Rubin()
	s.Seed = seed
	s.Sources = map[string]kinescope.Source{fullScreen: kinescope.Signal{}}
	if o.Case {
		s.Drives = append(s.Drives, kinescope.Drive{
			From: fullScreen, To: kinescope.CabinetMargin, Weight: caseMargin,
		})
	}
	if o.Glitches > 0 {
		every := s.Schedules["glitches"]
		every.Mean = float32(o.Glitches)
		every.Spread = float32(o.Glitches) / 3
		s.Schedules["glitches"] = every
	} else {
		delete(s.Schedules, "glitches")
	}
	return s
}

// On reports whether the frame goes through the tube.
func (t *Tube) On() bool { return t != nil && t.on }

// Toggle is the player's switch: off shows the bare frame at once; on again,
// the tube warms up.
func (t *Tube) Toggle() {
	if t == nil || t.tv == nil {
		return
	}
	t.on = !t.on
	if t.on {
		t.tv.Reset()
		t.tv.PowerOn()
	}
}

// Update advances the tube by a tick of dt seconds. held says a mouse
// button is down: a glitch falling due then waits for its release.
func (t *Tube) Update(dt float64, held bool) {
	if t.On() {
		t.tv.Hold(held)
		t.tv.Update(dt)
	}
}

// Ripple is a change of scene: the signal shivers for a moment, unless the
// tube is set not to (Options.Ripple).
func (t *Tube) Ripple() {
	if t.On() && t.opts.Ripple {
		t.tv.Play(kinescope.Ripple())
	}
}

// PowerOff starts the picture folding away; Dark reports it gone.
func (t *Tube) PowerOff() {
	if t.On() {
		t.tv.PowerOff()
	}
}

// Dark reports the tube dead after PowerOff.
func (t *Tube) Dark() bool { return t.On() && t.tv.Dark() }

// Warp is the bend of the glass, for the pointer. x,y is a point on a w×h
// frame as Ebiten maps the mouse, in a straight line from the screen; Warp
// gives the frame pixel the tube shows there, the one the shader samples, so
// a click lands on what is under the pointer. full is full screen, where the
// picture is smaller for the case around it (Options.Case).
func (t *Tube) Warp(x, y float64, w, h int, full bool) (float64, float64) {
	if !t.On() {
		return x, y
	}
	t.setFull(full)
	return t.tv.Map(x, y, w, h)
}

// Draw shows the frame on the tube over the whole final screen: the picture
// where geoM puts the frame — in full screen smaller, within a monitor case
// (Options.Case) — and the black, or the case and the room, around it.
func (t *Tube) Draw(
	screen ebiten.FinalScreen,
	frame *ebiten.Image,
	geoM ebiten.GeoM,
	full bool,
) {
	t.setFull(full)
	if err := t.renderer.Draw(screen, frame, t.tv, geoM); err != nil {
		ebiten.DefaultDrawFinalScreen(screen, frame, geoM)
	}
}

// setFull tells the tube whether it fills the screen.
func (t *Tube) setFull(full bool) {
	level := float32(0)
	if full {
		level = 1
	}
	t.full.Set(level)
}
