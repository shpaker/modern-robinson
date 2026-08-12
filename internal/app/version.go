// Package app is the Composition Root: it wires the dependency graph and runs
// the game loop. Only this layer may see all other layers. See ARCHITECTURE.md.
package app

// Version is stamped at build time via -ldflags. Defaults for `go run`.
var Version = "dev"

// DebugFlag ("true"/"false") is stamped via -ldflags to start in debug mode.
var DebugFlag = "false"

// SampleRate for the audio context (all game WAVs are 22050 Hz).
const SampleRate = 22050

// The original runs in a fixed 640x480 window: a 640x400 scene viewport that
// scrolls horizontally across the (up to 1024-wide) scene, above an 80px
// inventory bar. Every sprite is placed at anchor(cell) - FonScript.Shift and
// blitted as a full canvas (see use_cases.Grid and docs/08), so tall props such
// as the palm are clipped by the top of the play area exactly as in the
// original — there is no per-sprite vertical correction.
const (
	ViewW = 640 // viewport width
	ViewH = 480 // window height (play area + bar)
	PlayH = 400 // scene play-area height
	BarH  = ViewH - PlayH
)

// enginePace is the frame rate the authored per-frame constants assume (the
// original ran its scroll and flight steps about thirty times a second), used
// to scale them onto our tick.
const enginePace = 30.0
