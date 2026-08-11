package repositories

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
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
