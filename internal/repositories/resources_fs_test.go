package repositories

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories/webfs"
	"github.com/shpaker/modern-robinson/internal/testutil"
)

// The browser build cannot walk a directory tree, so it indexes from the file
// list in its manifest instead. That is a second code path into the same name
// maps, and if it drifted from the walk the web build would quietly resolve
// different containers than the desktop one.
func TestIndexFromNamesMatchesWalkingTheTree(t *testing.T) {
	root := testutil.GameRoot(t)

	walked := NewResources(root)
	listed := NewResourcesFS(os.DirFS(root), gameFiles(t, root))

	for _, m := range []struct {
		name        string
		from, other map[string]string
	}{
		{"movies", walked.movies, listed.movies},
		{"sceneDirs", walked.sceneDirs, listed.sceneDirs},
		{"sceneDat", walked.sceneDat, listed.sceneDat},
		{"screenDat", walked.screenDat, listed.screenDat},
	} {
		if len(m.from) != len(m.other) {
			t.Errorf("%s: walk indexed %d, name list indexed %d",
				m.name, len(m.from), len(m.other))
		}
		for k, v := range m.from {
			if got := m.other[k]; got != v {
				t.Errorf("%s[%q] = %q, want %q", m.name, k, got, v)
			}
		}
	}
}

// The whole browser stack in one go: resources held in memory, resolved through
// the same repository the desktop build uses. The sound bank is the case that
// matters — at 115 MB it takes codec's streaming path, so it proves an
// in-memory file still supports the random access that path needs.
func TestResourcesOverAnInMemoryFS(t *testing.T) {
	root := testutil.GameRoot(t)

	// A handful of real containers is enough; loading all 278 MB into the test
	// would cost more than it proves.
	names := []string{
		"DATA/SCEN/SCENA0.DAN",
		"DATA/SCEN/SCENA0.DAT",
		"DATA/WAVE/WAVE.DAN",
		"DATA/BAR/BAR.DAT",
	}
	files := map[string][]byte{}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n)))
		if err != nil {
			t.Skipf("resource %s missing: %v", n, err)
		}
		files[n] = b
	}

	r := NewResourcesFS(webfs.New(files), names)

	if r.Root() != "" {
		t.Errorf(
			"Root() = %q, want empty: there is no folder in a browser",
			r.Root(),
		)
	}
	if r.SceneContainer("SCENA0") == nil {
		t.Error("SceneContainer(SCENA0) not found over the in-memory FS")
	}
	ngb, pal, _ := r.SceneBackground("SCENA0")
	if ngb == nil || ngb.Width != 1024 || ngb.Height != 400 {
		t.Fatalf("SceneBackground(SCENA0) = %v", ngb)
	}
	if pal[27] == [4]byte{} {
		t.Error("palette not loaded")
	}
	// Reading one entry out of the 115 MB bank is the streaming path.
	wav := r.Sound("A01A.WAV")
	if len(wav) < 4 || string(wav[:4]) != "RIFF" {
		t.Errorf("Sound(A01A.WAV) = %d bytes, want a RIFF payload", len(wav))
	}
	if sp, _ := r.BarSprites(); sp["BAR0"] == nil {
		t.Error("BarSprites lost BAR0 over the in-memory FS")
	}
}

// gameFiles lists the resource files under root the way packweb's manifest
// does, as slash-separated relative paths.
func gameFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(
		root,
		func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err //nolint:nilerr // a nil error keeps the walk going
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("no files under %s", root)
	}
	return out
}
