// Package webdata downloads the game's resources into memory before the game
// starts.
//
// Why all of it, up front: IResources is synchronous — Sound, MovieFrames and
// SceneBackground are called from inside Game.Update, and the sound bank is
// read entry by entry while the game runs. A browser cannot block on a fetch
// from inside a frame callback, so there is no lazy option; everything the
// engine may ask for has to be resident before ebiten.RunGame is reached.
//
// It runs on plain net/http, which on js/wasm is implemented over fetch. That
// keeps it ordinary Go: it builds and is tested on every platform.
package webdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// workers is how many files are in flight at once. The files are small (a few
// hundred KB on average) and served over HTTP/2, so this is about hiding
// round-trip latency, not saturating the link.
const workers = 8

// Entry is one file in the manifest written by tools/packweb.
type Entry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// Manifest is the index of the published resource set.
type Manifest struct {
	Version    string  `json:"version"`
	TotalBytes int64   `json:"totalBytes"`
	Files      []Entry `json:"files"`
}

// ResolveBase turns the data base into an absolute URL against the page it is
// read from, dropping any query or fragment. The default is relative ("v1/"):
// the shell and the resource set are published side by side on one host, so
// the build names no host at all, and ?data= may still point anywhere,
// relative or absolute.
func ResolveBase(page, base string) (string, error) {
	p, err := url.Parse(page)
	if err != nil {
		return "", fmt.Errorf("page url: %w", err)
	}
	b, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("data url: %w", err)
	}
	out := p.ResolveReference(b)
	out.RawQuery, out.Fragment = "", ""
	return strings.TrimSuffix(out.String(), "/") + "/", nil
}

// Paths is the manifest's file list, in the order the packer wrote it, ready to
// hand to repositories.NewResourcesFS as its index.
func (m *Manifest) Paths() []string {
	out := make([]string, len(m.Files))
	for i, e := range m.Files {
		out[i] = e.Path
	}
	return out
}

// LoadManifest fetches and parses the manifest at base.
func LoadManifest(base string) (*Manifest, error) {
	b, err := get(strings.TrimSuffix(base, "/") + "/manifest.json")
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if len(m.Files) == 0 {
		return nil, fmt.Errorf("manifest lists no files")
	}
	return &m, nil
}

// Fetch downloads every file the manifest lists and returns them keyed by path,
// ready for webfs.New. progress is called as bytes arrive, throttled so that
// painting it stays cheap; it is always called once more with done == total
// before Fetch returns.
func Fetch(
	base string,
	m *Manifest,
	progress func(done, total int64),
) (map[string][]byte, error) {
	base = strings.TrimSuffix(base, "/") + "/"

	var (
		mu       sync.Mutex
		files    = make(map[string][]byte, len(m.Files))
		done     int64
		reported int64
		failed   error
	)
	// The progress bar only needs to look alive; a call per megabyte keeps the
	// DOM work off the download's critical path.
	const reportEvery = 1 << 20
	advance := func(n int64) {
		mu.Lock()
		done += n
		show := done-reported >= reportEvery
		if show {
			reported = done
		}
		d := done
		mu.Unlock()
		if show && progress != nil {
			progress(d, m.TotalBytes)
		}
	}

	work := make(chan Entry)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range work {
				b, err := fetchOne(base+e.Path, e.Size, advance)
				mu.Lock()
				if err != nil && failed == nil {
					failed = err
				}
				if err == nil {
					files[e.Path] = b
				}
				mu.Unlock()
			}
		}()
	}
	for _, e := range m.Files {
		work <- e
	}
	close(work)
	wg.Wait()

	if failed != nil {
		return nil, failed
	}
	if progress != nil {
		progress(done, m.TotalBytes)
	}
	return files, nil
}

// fetchOne reads a file into a slice sized from the manifest. Sizing it up
// front matters: letting a buffer grow would peak at twice the resource set,
// which the browser does not have room for.
func fetchOne(url string, size int64, advance func(int64)) ([]byte, error) {
	resp, err := http.Get(
		url,
	) //nolint:noctx // no deadline: the user's loading screen is the timeout
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: %s", url, resp.Status)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(&counter{r: resp.Body, on: advance}, buf); err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	return buf, nil
}

func get(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:noctx // see fetchOne
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// counter reports bytes as they are read.
type counter struct {
	r  io.Reader
	on func(int64)
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.on(int64(n))
	}
	return n, err
}
