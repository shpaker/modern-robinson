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
	dir := t.TempDir()
	cfg := LoadConfig(dir)
	want := DefaultConfig()
	want.Path = filepath.Join(dir, ConfigName) // where the sliders will go
	if cfg != want {
		t.Errorf("LoadConfig = %+v, want %+v", cfg, want)
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

// The sliders go back into the file they came from: only their lines change,
// comments and other keys stay, a repeated key changes everywhere and a
// missing one is added; the next start reads them back.
func TestSaveLevelsKeepsTheRestOfTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigName)
	body := "# player settings\n" +
		"scale: 3\n" +
		"sound: 1.0   # loud\n" +
		"debug: yes\n" +
		"sound: 0.9\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig(dir)
	if cfg.Path != path {
		t.Fatalf("Path = %q, want %q", cfg.Path, path)
	}
	if err := cfg.SaveLevels(0.3, 0.456, 0.5); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := "# player settings\n" +
		"scale: 3\n" +
		"sound: 0.30  # loud\n" +
		"debug: yes\n" +
		"sound: 0.30\n" +
		"music: 0.46\n" +
		"speed: 0.50\n"
	if string(got) != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
	again := LoadConfig(dir)
	if again.Sound != 0.3 || again.Music != 0.46 || again.Speed != 0.5 ||
		again.Scale != 3 || !again.Debug {
		t.Errorf("reloaded %+v", again)
	}
}

// With no file yet, the levels start one beside the saves; a config that
// never came from disk (the browser build) writes nothing.
func TestSaveLevelsCreatesTheFile(t *testing.T) {
	dir := t.TempDir()
	cfg := LoadConfig(dir)
	if err := cfg.SaveLevels(0.1, 0.2, 0.7); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ConfigName))
	if err != nil || string(got) != "sound: 0.10\nmusic: 0.20\nspeed: 0.70\n" {
		t.Errorf("file = %q err = %v", got, err)
	}
	if err := DefaultConfig().SaveLevels(1, 1, 1); err != nil {
		t.Errorf("in-memory config: %v", err)
	}
}

// F writes its switch like a slider: the line changes in place, comment and
// all, or joins the end; the next start reads it back.
func TestSaveSwitch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigName)
	body := "scale: 2\n" +
		"fullscreen: false # the whole screen\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig(dir)
	if err := cfg.SaveSwitch("fullscreen", true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := "scale: 2\n" +
		"fullscreen: true  # the whole screen\n"
	if string(got) != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
	if !LoadConfig(dir).Fullscreen {
		t.Error("the switch did not come back on the next start")
	}

	fresh := LoadConfig(t.TempDir())
	if err := fresh.SaveSwitch("fullscreen", false); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(fresh.Path)
	if string(got) != "fullscreen: false\n" {
		t.Errorf("new file = %q", got)
	}
}
