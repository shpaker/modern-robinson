// Package crt shows the frame on a CRT tube of the day: the glass bulges it,
// the beam draws it in soft scanlines through a slot mask, the bright areas
// glow, the red and blue guns drift out of register towards the edges, and
// the set hums and snows — with a glitch every minute or two and a ripple on
// a change of scene. It is the frame's last pass, onto the screen itself, so
// whatever reads the frame (save thumbnails, the MCP driver's picture) never
// sees it.
package crt

import (
	_ "embed"
	"image"
	"math"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	//go:embed crt.kage
	crtKage []byte
	//go:embed glow.kage
	glowKage []byte
)

// caseShrink is the picture's size in full screen: the monitor case adds
// 0.15 of the picture's half-height on every side (crt.kage, monitor), and
// case and all have to fit the screen's height.
const caseShrink = 1 / 1.15

// quad is the two triangles over the whole screen.
var quad = []uint32{0, 1, 2, 1, 2, 3}

// Tube is the CRT the frame is shown on. A nil Tube is one switched off.
type Tube struct {
	opts   Options
	on     bool
	shader *ebiten.Shader // crt.kage
	blur   *ebiten.Shader // glow.kage
	glow   *ebiten.Image  // the frame blurred for the glow
	clock

	vs  [4]ebiten.Vertex
	uni map[string]any
}

// New builds the tube, on or off. Should its shaders not compile, it stays
// off for good and the error says why.
func New(on bool, o Options) (*Tube, error) {
	rnd := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	t := &Tube{opts: o, clock: newClock(rnd, o.Glitches), uni: map[string]any{}}
	sh, err := ebiten.NewShader(crtKage)
	if err != nil {
		return t, err
	}
	blur, err := ebiten.NewShader(glowKage)
	if err != nil {
		return t, err
	}
	t.shader, t.blur, t.on = sh, blur, on
	return t, nil
}

// On reports whether the frame goes through the tube.
func (t *Tube) On() bool { return t != nil && t.on }

// Toggle is the player's switch: off shows the bare frame at once; on again,
// the tube warms up, quicker than from cold.
func (t *Tube) Toggle() {
	if t == nil || t.shader == nil {
		return
	}
	t.on = !t.on
	if t.on {
		t.warm, t.warmRate, t.dying = 0, 1.6, false
	}
}

// Update advances the tube by a tick of dt seconds. held says a mouse
// button is down: a glitch falling due then waits for its release.
func (t *Tube) Update(dt float64, held bool) {
	if t.On() {
		t.tick(dt, held)
	}
}

// Ripple is a change of scene: the signal shivers for a moment, unless the
// tube is set not to (Options.Ripple).
func (t *Tube) Ripple() {
	if t != nil && t.opts.Ripple {
		t.start(ripple)
	}
}

// PowerOff starts the picture folding away; Dark reports it gone.
func (t *Tube) PowerOff() {
	if t != nil && !t.dying {
		t.dying, t.cold = true, 0
	}
}

// Dark reports the tube dead after PowerOff.
func (t *Tube) Dark() bool { return t != nil && t.dark() }

// Warp is the bend of the glass, for the pointer. x,y is a point on a w×h
// frame as Ebiten maps the mouse, in a straight line from the screen; Warp
// gives the frame pixel the tube shows there, the one the shader samples, so
// a click lands on what is under the pointer. full is full screen, where the
// picture is smaller for the case around it (Options.Case).
func (t *Tube) Warp(x, y float64, w, h int, full bool) (float64, float64) {
	shrink := t.shrink(full)
	cx := (x/float64(w) - 0.5) / shrink * 2
	cy := (y/float64(h) - 0.5) / shrink * 2
	k := float64(t.opts.Curvature)
	bx := cx * (1 + cy*cy*k*0.06)
	by := cy * (1 + cx*cx*k*0.08)
	return (bx*0.5 + 0.5) * float64(w), (by*0.5 + 0.5) * float64(h)
}

// shrink is the picture's size within the frame's place: smaller in full
// screen, where the monitor case goes around it.
func (t *Tube) shrink(full bool) float64 {
	if full && t.opts.Case {
		return caseShrink
	}
	return 1
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
	fb := frame.Bounds()
	if t.glow == nil || t.glow.Bounds().Size() != fb.Size() {
		if t.glow != nil {
			t.glow.Deallocate()
		}
		t.glow = ebiten.NewImageWithOptions(
			image.Rect(0, 0, fb.Dx(), fb.Dy()),
			&ebiten.NewImageOptions{Unmanaged: true},
		)
	}
	if t.opts.Glow > 0 {
		op := &ebiten.DrawRectShaderOptions{Blend: ebiten.BlendCopy}
		op.Images[0] = frame
		t.glow.DrawRectShader(fb.Dx(), fb.Dy(), t.blur, op)
	}

	// The quad covers the screen; its source corners are where the screen's
	// corners fall on the frame, off it beside the picture.
	inv := geoM
	inv.Invert()
	sb := screen.Bounds()
	corners := [4]image.Point{
		sb.Min, {sb.Max.X, sb.Min.Y}, {sb.Min.X, sb.Max.Y}, sb.Max,
	}
	for i, p := range corners {
		sx, sy := inv.Apply(float64(p.X), float64(p.Y))
		t.vs[i] = ebiten.Vertex{
			DstX: float32(p.X), DstY: float32(p.Y),
			SrcX:   float32(sx) + float32(fb.Min.X),
			SrcY:   float32(sy) + float32(fb.Min.Y),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		}
	}
	op := &ebiten.DrawTrianglesShaderOptions{
		Uniforms: t.uniforms(geoM.Element(0, 0), t.shrink(full)),
		Blend:    ebiten.BlendCopy,
	}
	op.Images[0] = frame
	op.Images[1] = t.glow
	screen.DrawTrianglesShader32(t.vs[:], quad, t.shader, op)
}

// uniforms are the shader's inputs for a frame drawn scale screen pixels to
// a frame pixel, the picture shrunk into its case by shrink.
func (t *Tube) uniforms(scale, shrink float64) map[string]any {
	box := float32(0)
	if shrink < 1 {
		box = 1
	}
	fine := scale * shrink
	w, h, flash := t.power()
	g := t.shape()
	l := t.opts.Look
	u := t.uni
	u["Time"] = float32(math.Mod(t.now, 600))
	u["Curvature"] = l.Curvature
	u["Scanlines"] = l.Scanlines
	u["Mask"] = l.Mask
	u["Glow"] = l.Glow
	u["Softness"] = l.Softness
	u["Convergence"] = l.Convergence
	u["Vignette"] = l.Vignette
	u["Noise"] = l.Noise
	u["Hum"] = l.Hum
	u["Flicker"] = l.Flicker
	u["Interlace"] = l.Interlace
	u["Field"] = float32(t.ticks % 2)
	u["MaskFine"] = float32(smoothstep(2, 3, fine))
	u["ScanFine"] = float32(smoothstep(1, 2, fine))
	u["Glitch"] = g[:]
	u["Power"] = []float32{float32(w), float32(h), float32(flash)}
	u["Shrink"] = float32(shrink)
	u["Case"] = box
	return u
}

func smoothstep(e0, e1, x float64) float64 {
	k := math.Min(math.Max((x-e0)/(e1-e0), 0), 1)
	return k * k * (3 - 2*k)
}
