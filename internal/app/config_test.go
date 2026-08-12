package app

import (
	"os"
	"path/filepath"
	"testing"
)

// A hand-edited settings file must never stop the game from starting: unknown
// keys, junk values and out-of-range numbers all fall back to the defaults.
func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	body := "" +
		"# player settings\n" +
		"scale: 3\n" +
		"sound: 0.25\n" +
		"music: bogus\n" + // unparsable: keeps the default
		"speed: 9\n" + // out of range: clamped
		"debug: yes\n" +
		"colour: purple\n" + // unknown key: ignored
		"fullscreen\n" // no colon: ignored
	if err := os.WriteFile(
		filepath.Join(dir, ConfigName), []byte(body), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	def := DefaultConfig()
	cfg := LoadConfig(dir)
	if cfg.Scale != 3 {
		t.Errorf("Scale = %d, want 3", cfg.Scale)
	}
	if cfg.Sound != 0.25 {
		t.Errorf("Sound = %v, want 0.25", cfg.Sound)
	}
	if cfg.Music != def.Music {
		t.Errorf("Music = %v, want the default %v", cfg.Music, def.Music)
	}
	if cfg.Speed != 1 {
		t.Errorf("Speed = %v, want it clamped to 1", cfg.Speed)
	}
	if !cfg.Debug {
		t.Error("debug: yes should turn the overlay on")
	}
	if cfg.Fullscreen {
		t.Error("a line without a colon must not set anything")
	}
}

// No file at all is the normal case, and it must give exactly the defaults.
func TestLoadConfigMissing(t *testing.T) {
	cfg := LoadConfig(t.TempDir())
	if cfg != DefaultConfig() {
		t.Errorf("LoadConfig = %+v, want %+v", cfg, DefaultConfig())
	}
}

// The scale bounds a window we can actually show.
func TestConfigClampsScale(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, ConfigName), []byte("scale: 99\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	if got := LoadConfig(dir).Scale; got != 4 {
		t.Errorf("Scale = %d, want it clamped to 4", got)
	}
}
