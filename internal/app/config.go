package app

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
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
	// Path is the file the settings live in, where SaveLevels writes the
	// sliders back; "" keeps them for this run only (the browser build).
	Path string
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
	cfg.Path = filepath.Join(root, ConfigName) // beside the saves, when none yet
	cands := []string{cfg.Path}
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Join(filepath.Dir(exe), ConfigName))
	}
	for _, p := range cands {
		if f, err := os.Open(p); err == nil {
			cfg.Path = p
			cfg.apply(f)
			_ = f.Close()
			break
		}
	}
	cfg.clamp()
	return cfg
}

// LoadConfigFrom is LoadConfig for settings that do not come from a file on
// disk — the browser build hands it the query string turned into the same
// "key: value" lines.
func LoadConfigFrom(r io.Reader) Config {
	cfg := DefaultConfig()
	if r != nil {
		cfg.apply(r)
	}
	cfg.clamp()
	return cfg
}

// apply reads "key: value" lines, ignoring blanks, comments and anything it
// does not recognise — an unknown key is a note to a future version, not an
// error worth refusing to start over.
func (c *Config) apply(r io.Reader) {
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

// levelKeys are the settings the options screen changes, in the order a file
// that lacks them gets them appended.
var levelKeys = []string{"sound", "music", "speed"}

// SaveLevels writes the sliders into the settings file, so the next start
// begins where the player left them. Only the sound, music and speed lines
// change — every one of them, should a key repeat — and the rest of the file,
// comments included, stays as written; a key the file lacks goes at the end.
func (c Config) SaveLevels(sound, music, speed float64) error {
	if c.Path == "" {
		return nil
	}
	vals := map[string]string{
		"sound": level(sound), "music": level(music), "speed": level(speed),
	}
	raw, err := os.ReadFile(c.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var lines []string
	if s := strings.TrimRight(string(raw), "\n"); s != "" {
		lines = strings.Split(s, "\n")
	}
	seen := map[string]bool{}
	for i, line := range lines {
		body, note := line, ""
		if j := strings.IndexByte(line, '#'); j >= 0 {
			body, note = line[:j], "  "+line[j:]
		}
		key, _, ok := strings.Cut(body, ":")
		k := strings.ToLower(strings.TrimSpace(key))
		if v, known := vals[k]; ok && known {
			lines[i] = strings.TrimSpace(key) + ": " + v + note
			seen[k] = true
		}
	}
	for _, k := range levelKeys {
		if !seen[k] {
			lines = append(lines, k+": "+vals[k])
		}
	}
	tmp := c.Path + ".tmp"
	body := []byte(strings.Join(lines, "\n") + "\n")
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.Path)
}

// level is a slider's value as the file keeps it: two decimals are finer
// than a pixel of the track.
func level(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func truthy(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
