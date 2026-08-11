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
// inventory bar. Object/character sprites are authored on the full 640x480
// canvas, so their lower parts fall behind the bar.
const (
	ViewW = 640 // viewport width
	ViewH = 480 // window height (play area + bar)
	PlayH = 400 // scene play-area height
	BarH  = ViewH - PlayH
)
