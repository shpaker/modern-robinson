package webdata

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// site serves a manifest and its files the way the data host does.
func site(t *testing.T, files map[string][]byte) string {
	t.Helper()
	var entries []string
	var total int
	for name, b := range files {
		entries = append(entries, fmt.Sprintf(
			`{"path":%q,"size":%d}`, name, len(b),
		))
		total += len(b)
	}
	manifest := fmt.Sprintf(
		`{"version":"1","totalBytes":%d,"files":[%s]}`,
		total, strings.Join(entries, ","),
	)

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/manifest.json",
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(manifest))
		},
	)
	for name, b := range files {
		mux.HandleFunc("/"+name, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(b)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFetchDownloadsEveryFile(t *testing.T) {
	want := map[string][]byte{
		"LOGO.DAT":           []byte("logo bytes"),
		"DATA/WAVE/WAVE.DAN": []byte("a much longer sound bank payload"),
		"DATA/SCEN/X.DAN":    []byte("scene"),
	}
	base := site(t, want)

	m, err := LoadManifest(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Paths()) != len(want) {
		t.Fatalf("Paths() = %d entries, want %d", len(m.Paths()), len(want))
	}

	got, err := Fetch(base, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range want {
		if string(got[name]) != string(b) {
			t.Errorf("%s = %q, want %q", name, got[name], b)
		}
	}
}

// The loading screen is the only feedback for a quarter-gigabyte download, so
// progress has to reach the total rather than stop somewhere short of it.
func TestFetchReportsProgressToCompletion(t *testing.T) {
	base := site(t, map[string][]byte{
		"a": []byte(strings.Repeat("x", 1000)),
		"b": []byte(strings.Repeat("y", 2000)),
	})
	m, err := LoadManifest(base)
	if err != nil {
		t.Fatal(err)
	}

	var lastDone, lastTotal int64
	if _, err := Fetch(base, m, func(done, total int64) {
		lastDone, lastTotal = done, total
	}); err != nil {
		t.Fatal(err)
	}
	if lastDone != 3000 || lastTotal != 3000 {
		t.Errorf("final progress = %d/%d, want 3000/3000", lastDone, lastTotal)
	}
}

// A missing or truncated resource must fail the boot loudly. Starting the game
// with a hole in the resource set would surface much later as a blank scene.
func TestFetchFailsOnMissingFile(t *testing.T) {
	base := site(t, map[string][]byte{"present": []byte("ok")})
	m, err := LoadManifest(base)
	if err != nil {
		t.Fatal(err)
	}
	m.Files = append(m.Files, Entry{Path: "absent", Size: 5})

	if _, err := Fetch(base, m, nil); err == nil {
		t.Fatal("Fetch must fail when a manifest entry is not served")
	}
}

func TestFetchFailsOnShortFile(t *testing.T) {
	base := site(t, map[string][]byte{"a": []byte("short")})
	m, err := LoadManifest(base)
	if err != nil {
		t.Fatal(err)
	}
	m.Files[0].Size = 500 // manifest disagrees with what the host serves

	if _, err := Fetch(base, m, nil); err == nil {
		t.Fatal("Fetch must fail when a file is shorter than the manifest says")
	}
}

func TestLoadManifestRejectsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version":"1","files":[]}`))
		}))
	defer srv.Close()

	if _, err := LoadManifest(srv.URL); err == nil {
		t.Fatal("an empty manifest must be an error, not an empty game")
	}
}

// The build names no host: the default base is relative to the page, and no
// query or fragment — the page's (?data=..., ?scene=...) or the base's own —
// may leak into the data URLs, which get paths appended.
func TestResolveBase(t *testing.T) {
	for _, tc := range []struct{ page, base, want string }{
		{"https://example.org/?scene=SCENA5", "v1/", "https://example.org/v1/"},
		{"https://example.org/play/index.html", "v1", "https://example.org/play/v1/"},
		{"http://localhost:8080/?data=x", "/data/v1/", "http://localhost:8080/data/v1/"},
		{"http://localhost:8080/", "https://cdn.example.net/v2", "https://cdn.example.net/v2/"},
		{"https://example.org/?scene=X#f", "v1/?sig=1#y", "https://example.org/v1/"},
	} {
		got, err := ResolveBase(tc.page, tc.base)
		if err != nil {
			t.Fatalf("ResolveBase(%q, %q): %v", tc.page, tc.base, err)
		}
		if got != tc.want {
			t.Errorf("ResolveBase(%q, %q) = %q, want %q", tc.page, tc.base,
				got, tc.want)
		}
	}
}
