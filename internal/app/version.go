// Package app is the Composition Root: it wires the dependency graph and runs
// the game loop. Only this layer may see all other layers. See ARCHITECTURE.md.
package app

// Version is stamped at build time via -ldflags. Defaults for `go run`.
var Version = "dev"

// DebugFlag ("true"/"false") is stamped via -ldflags to start in debug mode.
var DebugFlag = "false"

// SampleRate for the audio context (all game WAVs are 22050 Hz).
const SampleRate = 22050
