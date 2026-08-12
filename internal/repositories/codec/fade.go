package codec

import "github.com/shpaker/modern-robinson/internal/types"

// FadeCurve turns a .FAD table into a per-step brightness ratio (1 = untouched,
// 0 = black). A .FAD holds N 256-byte palette-remap steps: step k maps every
// palette index to a darker one, which is how the original fades an 8-bit
// screen. Since we composite in RGBA, we reduce each step to the mean luminance
// it produces over the palette and use that as the step's brightness — same
// curve and step count as the original, applied as a uniform darkening.
func FadeCurve(fad []byte, pal types.Palette) []float64 {
	if len(fad) < 256 {
		return nil
	}
	lum := func(i byte) float64 {
		c := pal[i]
		return 0.299*float64(c[0]) + 0.587*float64(c[1]) + 0.114*float64(c[2])
	}
	steps := len(fad) / 256
	out := make([]float64, steps)
	var base float64
	for k := 0; k < steps; k++ {
		t := fad[k*256 : (k+1)*256]
		var sum float64
		for i := 0; i < 256; i++ {
			sum += lum(t[i])
		}
		if k == 0 {
			base = sum
		}
		switch {
		case base <= 0:
			out[k] = 0
		default:
			out[k] = sum / base
		}
	}
	return out
}
