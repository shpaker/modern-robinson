package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/types"
)

// Saves live next to the game data, one file per slot plus a thumbnail, mirroring
// the original's twelve-slot screen with its little screenshots.
func (g *Game) slotPath(i int) string {
	return filepath.Join(g.res.Root(), fmt.Sprintf("robinson%02d.sav", i))
}

func (g *Game) slotThumbPath(i int) string {
	return filepath.Join(g.res.Root(), fmt.Sprintf("robinson%02d.png", i))
}

// saveSlot writes the quest state and the current thumbnail into slot i.
func (g *Game) saveSlot(i int) {
	sd := g.gs.Snapshot(g.sceneName, g.cell)
	sd.Saved = time.Now().Format("02.01.2006 15:04")
	b, err := json.MarshalIndent(sd, "", " ")
	if err != nil {
		return
	}
	if os.WriteFile(g.slotPath(i), b, 0o644) != nil {
		return
	}
	g.captureThumb()
	if raw, err := encodePNG(g.thumb); err == nil {
		_ = os.WriteFile(g.slotThumbPath(i), raw, 0o644)
	}
	g.slotCache[i] = nil // force a re-read of the thumbnail
	g.slotInfo[i] = ""
	g.msg, g.msgT = "Игра сохранена", 2
}

// loadSlot restores slot i; false when the slot is empty or unreadable.
func (g *Game) loadSlot(i int) bool {
	b, err := os.ReadFile(g.slotPath(i))
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
	g.msg, g.msgT = "Игра загружена", 2
	return true
}

// save/load keep the quick-save keys (F5/F9) on slot 0.
func (g *Game) save() { g.saveSlot(0) }
func (g *Game) load() { g.loadSlot(0) }

// slotThumb returns slot i's thumbnail, reading it from disk once.
func (g *Game) slotThumb(i int) *ebiten.Image {
	if img, ok := g.slotCache[i]; ok && img != nil {
		return img
	}
	raw, err := os.ReadFile(g.slotThumbPath(i))
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
	g.slotCache[i] = img
	return img
}

// slotMeta returns slot i's caption (scene and save time), cached.
func (g *Game) slotMeta(i int) string {
	if s, ok := g.slotInfo[i]; ok && s != "" {
		return s
	}
	b, err := os.ReadFile(g.slotPath(i))
	if err != nil {
		g.slotInfo[i] = ""
		return ""
	}
	var sd types.SaveData
	if json.Unmarshal(b, &sd) != nil {
		g.slotInfo[i] = ""
		return ""
	}
	s := sd.Scene
	if sd.Saved != "" {
		s += "  " + sd.Saved
	}
	g.slotInfo[i] = s
	return s
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
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(slotW)/float64(ViewW), float64(slotH)/float64(ViewH))
	g.thumb.Clear()
	g.thumb.DrawImage(g.scratch, op)
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
