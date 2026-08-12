package app

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the player's own settings, read from config.yml beside the binary
// (or in the game folder). Everything in it is optional: a missing file, a
// missing key or an unreadable value leaves the built-in default alone, so the
// game always starts.
//
// The file is deliberately a flat list of "key: value" lines — the settings are
// a handful of scalars, and reading them needs no dependency:
//
//	scale: 2          # window magnification, 1..4
//	sound: 1.0        # 0..1
//	music: 0.7        # 0..1
//	speed: 0.5        # 0..1, 0.5 is the original pace
//	debug: false      # start with the F1 overlay on
//	fullscreen: false
type Config struct {
	Scale      int
	Sound      float64
	Music      float64
	Speed      float64
	Debug      bool
	Fullscreen bool
}

// DefaultConfig is what the game uses when config.yml says nothing.
func DefaultConfig() Config {
	return Config{
		Scale: 2, Sound: 1, Music: 0.7, Speed: 0.5,
		Debug: DebugFlag == "true",
	}
}

// ConfigName is the settings file's name.
const ConfigName = "config.yml"

// LoadConfig reads config.yml from the game folder, falling back to the
// binary's own directory, and returns the defaults with whatever it found
// applied on top.
func LoadConfig(root string) Config {
	cfg := DefaultConfig()
	cands := []string{filepath.Join(root, ConfigName)}
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Join(filepath.Dir(exe), ConfigName))
	}
	for _, p := range cands {
		if f, err := os.Open(p); err == nil {
			cfg.apply(f)
			_ = f.Close()
			break
		}
	}
	cfg.clamp()
	return cfg
}

// apply reads "key: value" lines, ignoring blanks, comments and anything it
// does not recognise — an unknown key is a note to a future version, not an
// error worth refusing to start over.
func (c *Config) apply(r *os.File) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		switch key {
		case "scale":
			if n, err := strconv.Atoi(val); err == nil {
				c.Scale = n
			}
		case "sound":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				c.Sound = f
			}
		case "music":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				c.Music = f
			}
		case "speed":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				c.Speed = f
			}
		case "debug":
			c.Debug = truthy(val)
		case "fullscreen":
			c.Fullscreen = truthy(val)
		}
	}
}

// clamp keeps a hand-edited file from producing an unusable window or a silent
// game.
func (c *Config) clamp() {
	c.Scale = clampInt(c.Scale, 1, 4)
	c.Sound = clampF(c.Sound, 0, 1)
	c.Music = clampF(c.Music, 0, 1)
	c.Speed = clampF(c.Speed, 0, 1)
}

func truthy(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
