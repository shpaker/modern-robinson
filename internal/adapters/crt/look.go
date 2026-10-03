package crt

import "github.com/shpaker/kinescope"

// Look is how the tube shows the frame, each part from 0 (none) to 1.
type Look struct {
	Curvature   float32 // the bulge of the glass
	Scanlines   float32 // dark gaps between the beam's lines
	Mask        float32 // the slot mask's phosphor stripes
	Glow        float32 // light bleeding around the bright areas
	Softness    float32 // the beam's spread between neighbouring pixels
	Convergence float32 // the red and blue guns out of register
	Vignette    float32 // the picture dimming towards the corners
	Noise       float32 // grain
	Hum         float32 // the mains hum: a slow dark bar rolling by
	Flicker     float32 // the brightness trembling
	Interlace   float32 // the two fields of lines taking turns, a shimmer
}

// Options is a tube: its look and its manners.
type Options struct {
	Look
	Glitches float64 // mean seconds between random glitches; 0: none
	Ripple   bool    // a shiver on a change of scene
}

// Defaults is the set the game plays on: a well-worn one, bulging, grainy
// and humming, that glitches every minute or two.
var Defaults = Options{
	Look: Look{
		Curvature:   0.75,
		Scanlines:   0.65,
		Mask:        0.35,
		Glow:        0.5,
		Softness:    0.35,
		Convergence: 0.65,
		Vignette:    0.95,
		Noise:       0.8,
		Hum:         0.75,
		Flicker:     0.25,
	},
	Glitches: 90,
	Ripple:   true,
}

// halfHeight is half the frame's height in pixels: the game's frame is
// 640×480 (app.ViewH), and the glass's rounding is measured in it.
const halfHeight = 240

// values are the tube's dials as kinescope params: each dial at 1 is the
// strongest the game's tube goes.
func (o Options) values() map[kinescope.ParamKey]float32 {
	l := o.Look
	corner := (0.02 + 0.06*l.Curvature) * halfHeight
	return map[kinescope.ParamKey]float32{
		kinescope.CornersRadius:     corner,
		kinescope.CurvatureX:        0.06 * l.Curvature,
		kinescope.CurvatureY:        0.08 * l.Curvature,
		kinescope.ScanlinesDepth:    0.7 * l.Scanlines,
		kinescope.SlotMaskStrength:  l.Mask,
		kinescope.GlowStrength:      0.5 * l.Glow,
		kinescope.SoftnessAmount:    l.Softness,
		kinescope.ConvergenceAmount: 0,
		kinescope.ConvergenceOffset: 1.12 * l.Convergence,
		kinescope.VignetteStrength:  0.74 * l.Vignette,
		kinescope.GrainStrength:     0.0625 * l.Noise,
		kinescope.HumStrength:       -0.17 * l.Hum,
		kinescope.FlickerStrength:   0.03 * l.Flicker,
		kinescope.InterlaceStrength: l.Interlace,
	}
}
