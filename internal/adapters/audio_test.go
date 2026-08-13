package adapters

import (
	"encoding/binary"
	"math"
	"testing"
)

// framesF32 builds interleaved stereo float32 PCM from (left, right) pairs.
func framesF32(pairs ...[2]float32) []byte {
	b := make([]byte, 0, len(pairs)*8)
	for _, p := range pairs {
		for _, s := range p {
			var w [4]byte
			binary.LittleEndian.PutUint32(w[:], math.Float32bits(s))
			b = append(b, w[:]...)
		}
	}
	return b
}

// readF32 reads the sample pairs back out.
func readF32(b []byte) [][2]float32 {
	var out [][2]float32
	for i := 0; i+8 <= len(b); i += 8 {
		out = append(out, [2]float32{
			math.Float32frombits(binary.LittleEndian.Uint32(b[i : i+4])),
			math.Float32frombits(binary.LittleEndian.Uint32(b[i+4 : i+8])),
		})
	}
	return out
}

// Panning fades the far side and leaves the near one alone, so a shot moves
// across the stereo field without getting louder than it was authored.
func TestPanPCM(t *testing.T) {
	src := framesF32([2]float32{1, 1}, [2]float32{0.5, -0.5})

	for _, c := range []struct {
		name           string
		pan            float64
		wantL, wantR   float32
		want2L, want2R float32
	}{
		{"right fades the left", 0.4, 0.6, 1, 0.3, -0.5},
		{"left fades the right", -0.4, 1, 0.6, 0.5, -0.3},
	} {
		got := readF32(panPCM(src, c.pan))
		if len(got) != 2 {
			t.Fatalf("%s: frames = %d, want 2", c.name, len(got))
		}
		if !closeF32(got[0][0], c.wantL) || !closeF32(got[0][1], c.wantR) {
			t.Errorf(
				"%s: frame 0 = %v, want [%v %v]",
				c.name,
				got[0],
				c.wantL,
				c.wantR,
			)
		}
		if !closeF32(got[1][0], c.want2L) || !closeF32(got[1][1], c.want2R) {
			t.Errorf(
				"%s: frame 1 = %v, want [%v %v]",
				c.name,
				got[1],
				c.want2L,
				c.want2R,
			)
		}
	}

	// The decoded PCM is shared and cached, so panning must not write into it.
	if src0 := readF32(src)[0]; src0 != [2]float32{1, 1} {
		t.Errorf("source PCM changed to %v: the cache would keep the pan", src0)
	}
}

// A truncated tail (an odd trailing sample) must not send the loop off the end.
func TestPanPCMIgnoresPartialFrame(t *testing.T) {
	src := append(framesF32([2]float32{1, 1}), 0, 0, 0)
	got := panPCM(src, 0.5)
	if len(got) != len(src) {
		t.Errorf("length = %d, want %d", len(got), len(src))
	}
	if f := readF32(got)[0]; !closeF32(f[0], 0.5) || !closeF32(f[1], 1) {
		t.Errorf("frame 0 = %v, want [0.5 1]", f)
	}
}

func closeF32(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-6 }
