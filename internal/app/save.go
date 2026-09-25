package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/types"
)

// saveStore is where the twelve slots live. On the desktop that is a pair of
// files per slot next to the game data, in the browser it is localStorage;
// either way the game only ever asks for a name and gets bytes back.
type saveStore interface {
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
}

// store is the save store, defaulted on first use so that a Game assembled
// field by field (as the tests do) still saves wherever Root() points.
func (g *Game) store() saveStore {
	if g.saves == nil {
		g.saves = defaultSaveStore(g.res.Root())
	}
	return g.saves
}

// Saves are one record per slot plus a thumbnail, mirroring the original's
// twelve-slot screen with its little screenshots.
func slotName(i int) string { return fmt.Sprintf("robinson%02d.sav", i) }

func slotThumbName(i int) string { return fmt.Sprintf("robinson%02d.png", i) }

// snapshot is the run as a save records it: the quest state, the hero's cell
// and Friday's own place — without her, a load would lose her (see loadSlot).
func (g *Game) snapshot() types.SaveData {
	sd := g.gs.Snapshot(g.sceneName, g.cell)
	sd.Frid = &types.CharSave{
		Cell: g.fridCell, Z: g.fridZ, Hidden: g.fridHidden,
	}
	scroll := g.camX
	sd.Scroll = &scroll
	return sd
}

// saveSlot writes the quest state and the current thumbnail into slot i.
func (g *Game) saveSlot(i int) {
	sd := g.snapshot()
	sd.Saved = time.Now().Format("02.01.2006 15:04")
	b, err := json.MarshalIndent(sd, "", " ")
	if err != nil {
		return
	}
	if g.store().Write(slotName(i), b) != nil {
		return
	}
	g.captureThumb()
	if raw, err := encodePNG(g.thumb); err == nil {
		_ = g.store().Write(slotThumbName(i), raw)
	}
	// Drop the entry so the slot is read again; a stored nil now means
	// "checked, nothing there" and would stick.
	delete(g.slotCache, i)
	g.msg, g.msgT = "Игра сохранена", 2
}

// loadSlot restores slot i; false when the slot is empty or unreadable.
func (g *Game) loadSlot(i int) bool {
	b, err := g.store().Read(slotName(i))
	if err != nil {
		return false
	}
	var sd types.SaveData
	if json.Unmarshal(b, &sd) != nil || sd.Scene == "" {
		return false
	}
	g.gs = types.Restore(sd)
	g.resetRun()
	g.loadScene(sd.Scene, &sd.Cell, "", "")
	// The view comes back where it was saved; a save from before the scroll
	// was kept finds the hero instead.
	if sd.Scroll != nil {
		g.setCamera(*sd.Scroll)
	} else {
		g.centreOnHero()
	}
	// Friday stands where the save left her. Arriving by load runs no entry
	// script to place her, and resetRun hides her, so a party saved together
	// used to come back without her.
	switch {
	case sd.Frid != nil:
		g.fridHidden, g.fridZ = sd.Frid.Hidden, sd.Frid.Z
		g.placeFrid(sd.Frid.Cell)
	case g.gs.Var("FridIs") == 1:
		// An older save knows only that she has joined: bring her back
		// beside the hero.
		g.fridHidden = false
		g.placeFrid(g.besideHero())
	}
	g.started = true // a restored run counts as started
	g.msg, g.msgT = "Игра загружена", 2
	return true
}

// besideHero is a cell next to the hero for Friday when a save does not say
// where she stood: the first walkable neighbour, else his own cell.
func (g *Game) besideHero() [2]int {
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		c := [2]int{g.cell[0] + d[0], g.cell[1] + d[1]}
		if g.grid.Valid(c[0], c[1]) {
			return c
		}
	}
	return g.cell
}

// save/load keep the quick-save keys (F5/F9) on slot 0.
func (g *Game) save() { g.saveSlot(0) }
func (g *Game) load() { g.loadSlot(0) }

// slotThumb returns slot i's thumbnail, reading it from the store once. A
// cached nil is a remembered miss: the save screen redraws every frame, so
// re-reading all twelve slots each time cost about 1400 failed syscalls a
// second.
func (g *Game) slotThumb(i int) *ebiten.Image {
	if img, ok := g.slotCache[i]; ok {
		return img
	}
	raw, err := g.store().Read(slotThumbName(i))
	if err != nil {
		g.slotCache[i] = nil
		return nil
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		g.slotCache[i] = nil
		return nil
	}
	img := ebiten.NewImageFromImage(src)
	if b := src.Bounds(); b.Dx() != slotW || b.Dy() != slotH {
		img = legacyThumb(img)
	}
	g.slotCache[i] = img
	return img
}

// legacyThumb refits a thumbnail saved before the slots took the original's
// size: 129x98 of the whole 640x480 frame, with the bar's place left black at
// the bottom. The scene part is cut out and stretched into the slot.
func legacyThumb(old *ebiten.Image) *ebiten.Image {
	r := legacyThumbScene(old.Bounds())
	if r.Dx() < 1 || r.Dy() < 1 {
		return old
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(
		float64(slotW)/float64(r.Dx()), float64(slotH)/float64(r.Dy()),
	)
	img := ebiten.NewImage(slotW, slotH)
	img.DrawImage(old.SubImage(r).(*ebiten.Image), op)
	return img
}

// legacyThumbScene is the scene's share of an old thumbnail: its top
// PlayH/ViewH, above where the bar was.
func legacyThumbScene(b image.Rectangle) image.Rectangle {
	return image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+b.Dy()*PlayH/ViewH)
}

// captureThumb renders the live scene into an offscreen frame and downscales it
// into the save thumbnail. Rendering on demand (rather than sampling whatever
// was last presented) keeps saving deterministic — including headless runs,
// where ticks advance without producing frames.
func (g *Game) captureThumb() {
	if g.thumb == nil {
		g.thumb = ebiten.NewImage(slotW, slotH)
	}
	if g.scratch == nil {
		g.scratch = ebiten.NewImage(ViewW, ViewH)
	}
	g.scratch.Clear()
	g.drawPlay(g.scratch, false)
	// The picture is the scene alone, as the original takes it (0x405fc0): the
	// scene's own context, 640 wide and as tall as the scene — the bar has a
	// context of its own — stretched whole into the slot.
	h := thumbSourceH(g.h)
	src := g.scratch.SubImage(image.Rect(0, 0, ViewW, h)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(slotW)/float64(ViewW), float64(slotH)/float64(h))
	g.thumb.Clear()
	g.thumb.DrawImage(src, op)
}

// thumbSourceH is how much of the frame a thumbnail takes: the scene's height,
// 400 above the bar and 480 for the intro bridges.
func thumbSourceH(sceneH int) int {
	switch {
	case sceneH <= 0:
		return PlayH
	case sceneH > ViewH:
		return ViewH
	}
	return sceneH
}

// encodePNG serialises an Ebiten image to PNG bytes.
func encodePNG(src *ebiten.Image) ([]byte, error) {
	b := src.Bounds()
	img := image.NewRGBA(b)
	src.ReadPixels(img.Pix)
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+3] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
