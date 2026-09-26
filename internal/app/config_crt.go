package app

import (
	"math"
	"strconv"
)

// TubeKeys are the CRT's settings in config.yml beyond its switch (crt): the
// dials of its look, each 0..1, then how it misbehaves — the mean seconds
// between its glitches (0: none), the shiver on a change of scene and the
// monitor case in full screen. The browser build takes the same keys from
// the page's address.
var TubeKeys = []string{
	"crt_curvature", "crt_scanlines", "crt_mask", "crt_glow",
	"crt_softness", "crt_convergence", "crt_vignette", "crt_noise",
	"crt_hum", "crt_flicker", "crt_interlace",
	"crt_glitches", "crt_ripple", "crt_case",
}

// dial is the part of the tube's look a crt_ key turns, if it is one.
func (c *Config) dial(key string) *float32 {
	l := &c.Tube.Look
	switch key {
	case "crt_curvature":
		return &l.Curvature
	case "crt_scanlines":
		return &l.Scanlines
	case "crt_mask":
		return &l.Mask
	case "crt_glow":
		return &l.Glow
	case "crt_softness":
		return &l.Softness
	case "crt_convergence":
		return &l.Convergence
	case "crt_vignette":
		return &l.Vignette
	case "crt_noise":
		return &l.Noise
	case "crt_hum":
		return &l.Hum
	case "crt_flicker":
		return &l.Flicker
	case "crt_interlace":
		return &l.Interlace
	}
	return nil
}

// applyTube reads one of TubeKeys, a value it cannot read left alone.
func (c *Config) applyTube(key, val string) {
	f, err := strconv.ParseFloat(val, 64)
	number := err == nil && !math.IsNaN(f)
	if p := c.dial(key); p != nil {
		if number {
			*p = float32(f)
		}
		return
	}
	switch key {
	case "crt_glitches":
		if number {
			c.Tube.Glitches = f
		}
	case "crt_ripple":
		c.Tube.Ripple = truthy(val)
	case "crt_case":
		c.Tube.Case = truthy(val)
	}
}

// clampTube keeps the dials within 0..1 and a glitch at least once an hour.
func (c *Config) clampTube() {
	for _, k := range TubeKeys {
		if p := c.dial(k); p != nil {
			*p = float32(clampF(float64(*p), 0, 1))
		}
	}
	c.Tube.Glitches = clampF(c.Tube.Glitches, 0, 3600)
}
