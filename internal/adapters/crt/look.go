package crt

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
	Case     bool    // the monitor case around the picture in full screen
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
	Case:     true,
}
