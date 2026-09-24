// Command packweb prepares the game's resources for the browser build.
//
// It copies everything the engine actually reads out of a game folder into a
// flat, web-servable tree, writes the manifest the browser loads first, and
// drops a .gz next to any file gzip meaningfully shrinks. Serving those
// sidecars is the data host's job (see packaging/Caddyfile.example); the
// browser decompresses them transparently, so nothing in the game has to know.
//
//	go run ./tools/packweb -out dist/webdata/v1 extracted/ROBINSON_ISO/ROBINSON
//
// The output is what gets rsynced to the data host. See docs/11-web-build.md.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// wanted are the extensions repositories.Resources indexes, plus the two loose
// files it opens by name (MINIGAME.WDT and DATA/BEGIN.BGI). Nothing else in the
// game folder is ever read: the installer, the DirectX redistributable and the
// manual are all dead weight in a browser.
var wanted = map[string]bool{
	".MV":  true, // sprites, animations, cutscenes
	".DAN": true, // script containers and the sound bank
	".DAT": true, // backgrounds, screen packs, minigame assets
	".WDT": true, // MINIGAME.WDT sound bank
	".BGI": true, // BEGIN.BGI initial object visibility
}

// skipDirs hold no game resources — only the DirectX 7 redistributable and the
// scanned manual, together 55 MB of the disc.
var skipDirs = map[string]bool{"DIRECTX": true, "DOCUMENT": true}

// gzipGain is how much a file has to shrink before a sidecar earns its keep.
// In practice it selects the movies and the script containers (.MV, .DAN);
// the PCM sound bank WAVE.DAN shrinks by under 8% and is served as it is.
const gzipGain = 0.10

type entry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`

	wire int64 // bytes actually served: the .gz when there is one
}

type manifest struct {
	Version    string  `json:"version"`
	TotalBytes int64   `json:"totalBytes"`
	Files      []entry `json:"files"`
}

func main() {
	out := flag.String("out", "dist/webdata/v1", "output directory")
	version := flag.String("version", "1", "manifest version string")
	jobs := flag.Int("jobs", runtime.NumCPU(), "parallel workers")
	level := flag.Int(
		"gzip",
		gzip.DefaultCompression,
		"gzip level for sidecars",
	)
	force := flag.Bool(
		"force",
		false,
		"rewrite files that already look current",
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: packweb [flags] <game-dir>\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	root := flag.Arg(0)

	src, err := collect(root)
	if err != nil {
		fatal(err)
	}
	if len(src) == 0 {
		fatal(fmt.Errorf("no game resources under %s", root))
	}

	entries, err := pack(root, *out, src, *jobs, *level, *force)
	if err != nil {
		fatal(err)
	}

	m := manifest{Version: *version, Files: entries}
	for _, e := range entries {
		m.TotalBytes += e.Size
	}
	if err := writeManifest(*out, m); err != nil {
		fatal(err)
	}

	var wire int64
	for _, e := range entries {
		wire += e.wire
	}
	fmt.Printf(
		"%d files -> %s\n  %.1f MiB resident, %.1f MiB over the wire\n",
		len(entries), *out, mib(m.TotalBytes), mib(wire),
	)
}

// collect lists the resources under root, as slash-separated relative paths in
// a stable order so that reruns produce an identical manifest.
func collect(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(
		root,
		func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[strings.ToUpper(d.Name())] {
					return fs.SkipDir
				}
				return nil
			}
			if !wanted[strings.ToUpper(filepath.Ext(d.Name()))] {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
			return nil
		},
	)
	sort.Strings(out)
	return out, err
}

// pack copies every file and returns the manifest entries, in the same order.
func pack(
	root, out string,
	src []string,
	jobs, level int,
	force bool,
) ([]entry, error) {
	entries := make([]entry, len(src))
	errs := make([]error, len(src))

	work := make(chan int)
	var wg sync.WaitGroup
	for range max(jobs, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				e, err := one(root, out, src[i], level, force)
				entries[i], errs[i] = e, err
			}
		}()
	}

	var done int
	for i := range src {
		work <- i
		done++
		if done%50 == 0 {
			fmt.Fprintf(os.Stderr, "\r%d/%d…", done, len(src))
		}
	}
	close(work)
	wg.Wait()
	fmt.Fprintf(os.Stderr, "\r%d/%d files\n", len(src), len(src))

	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("%s: %w", src[i], err)
		}
	}
	return entries, nil
}

// one copies a single resource, hashes it, and writes a .gz beside it when that
// is worth serving.
func one(root, out, rel string, level int, force bool) (entry, error) {
	srcPath := filepath.Join(root, filepath.FromSlash(rel))
	dstPath := filepath.Join(out, filepath.FromSlash(rel))

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return entry{}, err
	}
	sum := sha256.Sum256(data)
	e := entry{
		Path:   rel,
		Size:   int64(len(data)),
		SHA256: hex.EncodeToString(sum[:]),
		wire:   int64(len(data)),
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return entry{}, err
	}
	if force || !current(dstPath, e.Size) {
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return entry{}, err
		}
	}

	gzPath := dstPath + ".gz"
	gz, err := squeeze(data, level)
	if err != nil {
		return entry{}, err
	}
	if float64(len(gz)) <= float64(len(data))*(1-gzipGain) {
		if force || !current(gzPath, int64(len(gz))) {
			if err := os.WriteFile(gzPath, gz, 0o644); err != nil {
				return entry{}, err
			}
		}
		e.wire = int64(len(gz))
	} else if err := os.Remove(gzPath); err != nil && !os.IsNotExist(err) {
		// A sidecar left over from an earlier run would be served in place of
		// a file that no longer benefits from it.
		return entry{}, err
	}
	return e, nil
}

// current reports whether a destination already holds exactly this many bytes,
// which for immutable game resources is enough to skip rewriting it.
func current(path string, size int64) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() == size
}

func squeeze(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(len(data) / 2)
	zw, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeManifest(out string, m manifest) error {
	f, err := os.Create(filepath.Join(out, "manifest.json"))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return f.Close()
}

func mib(n int64) float64 { return float64(n) / (1 << 20) }

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "packweb:", err)
	os.Exit(1)
}
