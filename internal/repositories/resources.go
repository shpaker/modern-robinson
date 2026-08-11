// Package repositories is the Infrastructure layer: the only place that reads
// game files. Higher layers use it through interfaces (ARCHITECTURE.md).
package repositories

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories/codec"
	"github.com/shpaker/modern-robinson/internal/types"
)

// Resources indexes every NL container under the game root and resolves assets
// by name. It implements interfaces.IResources.
type Resources struct {
	root      string
	movies    map[string]string
	sceneDirs map[string]string
	sceneDat  map[string]string
	cache     map[string]*codec.Container
	wave      *codec.Container
	waveIndex map[string]types.Entry
}

var _ interfaces.IResources = (*Resources)(nil)

// NewResources indexes the game folder at root.
func NewResources(root string) *Resources {
	r := &Resources{
		root:      root,
		movies:    map[string]string{},
		sceneDirs: map[string]string{},
		sceneDat:  map[string]string{},
		cache:     map[string]*codec.Container{},
	}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		up := strings.ToUpper(info.Name())
		switch {
		case strings.HasSuffix(up, ".MV"):
			r.movies[up] = p
		case strings.HasSuffix(up, ".DAN"):
			// Every scene/interior/global script container (SCENA*, CAB_*,
			// INT*, PALACE, STARTUP, BAR, ...) keyed by base name.
			r.sceneDirs[up[:len(up)-4]] = p
		case strings.HasSuffix(up, ".DAT"):
			// Background bitmaps live in DATA/SCEN (scenes) and DATA/BAR (the
			// inventory bar); ignore the top-level data blobs (CONFIG, LANG,
			// CHESS, ...) that share the extension.
			dir := strings.ToUpper(p)
			sep := string(os.PathSeparator)
			if strings.Contains(dir, sep+"SCEN"+sep) || strings.Contains(dir, sep+"BAR"+sep) {
				r.sceneDat[up[:len(up)-4]] = p
			}
		}
		return nil
	})
	return r
}

func (r *Resources) container(path string) *codec.Container {
	if c, ok := r.cache[path]; ok {
		return c
	}
	c, err := codec.Open(path)
	if err != nil {
		return nil
	}
	r.cache[path] = c
	return c
}

// Movie returns the container for a movie by name (e.g. "Roby1.mv").
func (r *Resources) Movie(name string) interfaces.IContainer {
	p, ok := r.movies[strings.ToUpper(name)]
	if !ok {
		return nil
	}
	if c := r.container(p); c != nil {
		return c
	}
	return nil
}

// MovieFrames decodes all NGB frames of a movie plus its palette.
func (r *Resources) MovieFrames(name string) ([]*types.NGB, types.Palette) {
	var pal types.Palette
	mv := r.Movie(name)
	if mv == nil {
		return nil, pal
	}
	if e, ok := mv.FindExt(".COL"); ok {
		if col, err := mv.Extract(e); err == nil {
			pal = codec.LoadPalette(col)
		}
	}
	var frames []*types.NGB
	for _, e := range mv.Entries() {
		if !strings.HasSuffix(strings.ToUpper(e.Name), ".NGB") {
			continue
		}
		if d, err := mv.Extract(e); err == nil {
			frames = append(frames, codec.DecodeNGB(d))
		}
	}
	return frames, pal
}

// Sound returns the raw WAV bytes for a sound by name.
func (r *Resources) Sound(name string) []byte {
	if r.wave == nil {
		r.wave = r.container(filepath.Join(r.root, "DATA", "WAVE", "WAVE.DAN"))
		if r.wave == nil {
			return nil
		}
		r.waveIndex = map[string]types.Entry{}
		for _, e := range r.wave.Entries() {
			r.waveIndex[strings.ToUpper(e.Name)] = e
		}
	}
	e, ok := r.waveIndex[strings.ToUpper(name)]
	if !ok {
		return nil
	}
	b, _ := r.wave.Extract(e)
	return b
}

// BarBackground returns the inventory bar's 640x80 background bitmap (BAR0.NGB)
// and its palette from DATA/BAR/BAR.DAT.
func (r *Resources) BarBackground() (*types.NGB, types.Palette) {
	var pal types.Palette
	p, ok := r.sceneDat["BAR"]
	if !ok {
		return nil, pal
	}
	c := r.container(p)
	if c == nil {
		return nil, pal
	}
	var bg *types.NGB
	for _, e := range c.Entries() {
		up := strings.ToUpper(e.Name)
		d, err := c.Extract(e)
		if err != nil {
			continue
		}
		switch {
		case up == "BAR0.NGB":
			bg = codec.DecodeNGB(d)
		case strings.HasSuffix(up, ".COL"):
			pal = codec.LoadPalette(d)
		}
	}
	return bg, pal
}

// SceneContainer returns the .DAN container for a scene (its scripts/objects).
func (r *Resources) SceneContainer(name string) interfaces.IContainer {
	p, ok := r.sceneDirs[strings.ToUpper(name)]
	if !ok {
		return nil
	}
	if c := r.container(p); c != nil {
		return c
	}
	return nil
}

// SceneBackground returns the scene's background bitmap, palette, and fade table.
func (r *Resources) SceneBackground(name string) (*types.NGB, types.Palette, []byte) {
	var pal types.Palette
	p, ok := r.sceneDat[strings.ToUpper(name)]
	if !ok {
		return nil, pal, nil
	}
	c := r.container(p)
	if c == nil {
		return nil, pal, nil
	}
	var ngb *types.NGB
	var fad []byte
	for _, e := range c.Entries() {
		up := strings.ToUpper(e.Name)
		d, err := c.Extract(e)
		if err != nil {
			continue
		}
		switch {
		case strings.HasSuffix(up, ".NGB"):
			ngb = codec.DecodeNGB(d)
		case strings.HasSuffix(up, ".COL"):
			pal = codec.LoadPalette(d)
		case strings.HasSuffix(up, ".FAD"):
			fad = d
		}
	}
	return ngb, pal, fad
}
