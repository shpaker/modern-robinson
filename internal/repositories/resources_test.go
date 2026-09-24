package repositories

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

func TestResources(t *testing.T) {
	root := testutil.GameRoot(t)
	r := NewResources(root)

	if mv := r.Movie("Roby1.mv"); mv == nil {
		t.Error("Movie(Roby1.mv) not found")
	}
	if r.Movie("no-such.mv") != nil {
		t.Error("Movie of missing name should be nil")
	}
	if r.Sound("A01A.WAV") == nil {
		t.Error("Sound(A01A.WAV) not found")
	}
	if r.Sound("no-such.wav") != nil {
		t.Error("Sound of missing name should be nil")
	}
	if r.SceneContainer("SCENA0") == nil {
		t.Error("SceneContainer(SCENA0) not found")
	}

	ngb, pal, _ := r.SceneBackground("SCENA0")
	if ngb == nil || ngb.Width != 1024 || ngb.Height != 400 {
		t.Fatalf("SceneBackground(SCENA0) = %v", ngb)
	}
	if pal[27] == [4]byte{} {
		t.Error("palette not loaded")
	}

	frames, fpal := r.MovieFrames("Roby1.mv")
	if len(frames) < 10 {
		t.Errorf("MovieFrames(Roby1.mv) = %d frames, want >= 10", len(frames))
	}
	if fpal[1] == [4]byte{} && fpal[2] == [4]byte{} {
		t.Error("movie palette not loaded")
	}
}

// Every bitmap and palette of the six minigame packs decodes. The packs' last
// entries run a byte past the end of the file, and dropping them cost the hut
// and the map puzzles their palettes (both drew black) and the draughts game
// its board (so it never opened and the quest counted a win).
func TestMinigamePacksDecodeWhole(t *testing.T) {
	r := NewResources(testutil.GameRoot(t))
	for pack, backdrop := range map[string]string{
		"MAP": "DESK", "HOUSE": "BACK", "CHESS": "BACK",
		"BALOON": "SKY", "PIPE": "BACK", "CRYPT": "CRYPT",
	} {
		c := r.container(r.screenDat[pack])
		if c == nil {
			t.Fatalf("%s: no container", pack)
		}
		bitmaps, palettes := 0, 0
		for _, e := range c.Entries() {
			switch up := strings.ToUpper(e.Name); {
			case strings.HasSuffix(up, ".NGB"):
				bitmaps++
			case strings.HasSuffix(up, ".COL"):
				palettes++
			}
		}
		sprites, pal := r.ScreenPack(pack)
		if len(sprites) != bitmaps {
			t.Errorf("%s: %d sprites decoded, the pack has %d", pack,
				len(sprites), bitmaps)
		}
		if n := sprites[backdrop]; n == nil || n.Width != 640 {
			t.Errorf("%s: backdrop %s missing", pack, backdrop)
		}
		if pal == (types.Palette{}) {
			t.Errorf("%s: no palette", pack)
		}
		if got := len(r.ScreenPalettes(pack)); got != palettes {
			t.Errorf("%s: %d palettes decoded, the pack has %d", pack, got,
				palettes)
		}
	}
}

// The translator's whole game is one CP1251 text: its first line is the
// 30-letter lowercase alphabet the pictograms stand for.
func TestScreenTextDecodesCP1251(t *testing.T) {
	r := NewResources(testutil.GameRoot(t))
	txt := r.ScreenText("CRYPT", "CRYPT.TXT")
	first, _, _ := strings.Cut(txt, "\n")
	if got := []rune(strings.TrimSpace(first)); len(got) != 30 || got[0] != 'а' {
		t.Errorf("alphabet line = %q, want 30 letters from «а»", first)
	}
	if r.ScreenText("CRYPT", "NO.TXT") != "" || r.ScreenText("NO", "X") != "" {
		t.Error("a missing pack or entry must read as empty text")
	}
}
