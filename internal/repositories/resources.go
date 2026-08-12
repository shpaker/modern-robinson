// Package repositories is the Infrastructure layer: the only place that reads
// game files. Higher layers use it through interfaces (ARCHITECTURE.md).
package repositories

import (
	"encoding/binary"
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
	root        string
	movies      map[string]string
	sceneDirs   map[string]string
	sceneDat    map[string]string
	screenDat   map[string]string
	cache       map[string]*codec.Container
	wave        *codec.Container
	waveIndex   map[string]types.Entry
	mgWave      *codec.Container
	mgWaveIndex map[string]types.Entry
	bgi         [][]bgiRecord // BEGIN.BGI initial object states, lazily parsed
}

// readFileUpper reads a file under root joining path elements, trying the exact
// name (game files ship upper-case on the ISO).
func readFileUpper(root string, parts ...string) ([]byte, error) {
	p := filepath.Join(append([]string{root}, parts...)...)
	return os.ReadFile(p)
}

var _ interfaces.IResources = (*Resources)(nil)

// NewResources indexes the game folder at root.
func NewResources(root string) *Resources {
	r := &Resources{
		root:      root,
		movies:    map[string]string{},
		sceneDirs: map[string]string{},
		sceneDat:  map[string]string{},
		screenDat: map[string]string{},
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
			// inventory bar); top-level .DAT (LOGO, OPTIONS, ...) are screen
			// packs; the rest (CONFIG, LANG, ...) fail codec.Open harmlessly.
			dir := strings.ToUpper(p)
			sep := string(os.PathSeparator)
			if strings.Contains(dir, sep+"SCEN"+sep) ||
				strings.Contains(dir, sep+"BAR"+sep) {
				r.sceneDat[up[:len(up)-4]] = p
			} else {
				r.screenDat[up[:len(up)-4]] = p
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

// Root is the game folder this Resources indexes.
func (r *Resources) Root() string { return r.root }

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

// MovieShift returns a movie's authored hotspot — the origin field of its .SCR
// header (offset 0x18/0x1C). The engine places every sprite so that this point
// lands on the cell anchor; a FonScript may override it (only wave.mv does).
// Returns (0,0) when the movie or its header is absent.
func (r *Resources) MovieShift(name string) [2]int {
	mv := r.Movie(name)
	if mv == nil {
		return [2]int{}
	}
	e, ok := mv.FindExt(".SCR")
	if !ok {
		return [2]int{}
	}
	d, err := mv.Extract(e)
	if err != nil || len(d) < 0x20 {
		return [2]int{}
	}
	return [2]int{
		int(int32(binary.LittleEndian.Uint32(d[0x18:]))),
		int(int32(binary.LittleEndian.Uint32(d[0x1c:]))),
	}
}

// Sound returns the raw WAV bytes for a sound by name. It searches the main
// bank (DATA/WAVE/WAVE.DAN) first and then the minigame bank (MINIGAME.WDT),
// which holds the organ notes, puzzle clicks and the victory jingles.
func (r *Resources) Sound(name string) []byte {
	r.indexWaves()
	up := strings.ToUpper(name)
	if e, ok := r.waveIndex[up]; ok {
		b, _ := r.wave.Extract(e)
		return b
	}
	if e, ok := r.mgWaveIndex[up]; ok {
		b, _ := r.mgWave.Extract(e)
		return b
	}
	return nil
}

// indexWaves opens and indexes both sound banks once.
func (r *Resources) indexWaves() {
	if r.waveIndex == nil {
		r.waveIndex = map[string]types.Entry{}
		r.wave = r.container(
			filepath.Join(r.root, "DATA", "WAVE", "WAVE.DAN"),
		)
		if r.wave != nil {
			for _, e := range r.wave.Entries() {
				r.waveIndex[strings.ToUpper(e.Name)] = e
			}
		}
	}
	if r.mgWaveIndex == nil {
		r.mgWaveIndex = map[string]types.Entry{}
		r.mgWave = r.container(filepath.Join(r.root, "MINIGAME.WDT"))
		if r.mgWave != nil {
			for _, e := range r.mgWave.Entries() {
				r.mgWaveIndex[strings.ToUpper(e.Name)] = e
			}
		}
	}
}

// BarBackground returns the inventory bar's 640x80 background bitmap (BAR0.NGB)
// and its palette from DATA/BAR/BAR.DAT.
func (r *Resources) BarBackground() (*types.NGB, types.Palette) {
	sp, pal := r.BarSprites()
	return sp["BAR0"], pal
}

// BarSprites returns every bitmap of DATA/BAR/BAR.DAT keyed by upper-case base
// name (BAR0 = strip background, BAR1-3 = character portraits, BAR4-5 = text
// boxes, BAR6-19 = normal/selected icon pairs for the first seven items).
func (r *Resources) BarSprites() (map[string]*types.NGB, types.Palette) {
	var pal types.Palette
	out := map[string]*types.NGB{}
	p, ok := r.sceneDat["BAR"]
	if !ok {
		return out, pal
	}
	c := r.container(p)
	if c == nil {
		return out, pal
	}
	for _, e := range c.Entries() {
		up := strings.ToUpper(e.Name)
		d, err := c.Extract(e)
		if err != nil {
			continue
		}
		switch {
		case strings.HasSuffix(up, ".NGB"):
			out[strings.TrimSuffix(up, ".NGB")] = codec.DecodeNGB(d)
		case strings.HasSuffix(up, ".COL"):
			pal = codec.LoadPalette(d)
		}
	}
	return out, pal
}

// SceneFade returns the scene's fade curve (per-step brightness, 1 -> 0) built
// from its .FAD table, or nil when the scene ships none.
func (r *Resources) SceneFade(name string) []float64 {
	_, pal, fad := r.SceneBackground(name)
	if fad == nil {
		return nil
	}
	return codec.FadeCurve(fad, pal)
}

// ScreenPack returns every bitmap of a top-level pack keyed by upper-case base
// name, plus the pack's palette (its <pack>.COL, else the first one found).
// Widgets inside a pack (OPTS*, BUT*) share that palette.
func (r *Resources) ScreenPack(pack string) (
	map[string]*types.NGB, types.Palette,
) {
	var pal types.Palette
	out := map[string]*types.NGB{}
	p, ok := r.screenDat[strings.ToUpper(pack)]
	if !ok {
		return out, pal
	}
	c := r.container(p)
	if c == nil {
		return out, pal
	}
	want := strings.ToUpper(pack) + ".COL"
	var first types.Palette
	haveFirst, havePack := false, false
	for _, e := range c.Entries() {
		up := strings.ToUpper(e.Name)
		d, err := c.Extract(e)
		if err != nil {
			continue
		}
		switch {
		case strings.HasSuffix(up, ".NGB"):
			out[strings.TrimSuffix(up, ".NGB")] = codec.DecodeNGB(d)
		case up == want:
			pal, havePack = codec.LoadPalette(d), true
		case strings.HasSuffix(up, ".COL") && !haveFirst:
			first, haveFirst = codec.LoadPalette(d), true
		}
	}
	if !havePack && haveFirst {
		pal = first
	}
	return out, pal
}

// ScreenFile returns a raw entry of a top-level pack (e.g. CRYPT.DAT's
// CRYPT.TXT), or nil when the pack or the entry is missing.
func (r *Resources) ScreenFile(pack, name string) []byte {
	p, ok := r.screenDat[strings.ToUpper(pack)]
	if !ok {
		return nil
	}
	c := r.container(p)
	if c == nil {
		return nil
	}
	d, err := c.ExtractName(strings.ToUpper(name))
	if err != nil {
		return nil
	}
	return d
}

// Screen returns a named full-screen bitmap from a top-level screen pack:
// Screen("LOGO", "ROBINSON") is the title image of LOGO.DAT (pairs NAME.NGB +
// NAME.COL). Returns nil if the pack or the image is absent.
func (r *Resources) Screen(pack, name string) (*types.NGB, types.Palette) {
	var pal types.Palette
	p, ok := r.screenDat[strings.ToUpper(pack)]
	if !ok {
		return nil, pal
	}
	c := r.container(p)
	if c == nil {
		return nil, pal
	}
	up := strings.ToUpper(name)
	var ngb *types.NGB
	for _, e := range c.Entries() {
		eu := strings.ToUpper(e.Name)
		if eu != up+".NGB" && eu != up+".COL" {
			continue
		}
		d, err := c.Extract(e)
		if err != nil {
			continue
		}
		if strings.HasSuffix(eu, ".NGB") {
			ngb = codec.DecodeNGB(d)
		} else {
			pal = codec.LoadPalette(d)
		}
	}
	return ngb, pal
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
func (r *Resources) SceneBackground(
	name string,
) (*types.NGB, types.Palette, []byte) {
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
