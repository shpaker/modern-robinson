// Package minigame is what the six minigames share: the contract between a
// game and the adventure that launches it, and the few drawing and input
// helpers they all use. Each game lives in a package of its own below this one;
// catalog lists them by the id StartGame passes. In the original all six sit in
// MINIGAME.DLL behind a single export, MiniGame(gameId, ...).
//
// A game may use the engine, but reaches the game's files only through Host.
package minigame

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters/pointer"
	"github.com/shpaker/modern-robinson/internal/interfaces"
)

// ScreenW and ScreenH are the window a minigame takes over whole.
const (
	ScreenW = 640
	ScreenH = 480
)

// Game is a running minigame. The adventure waits while it plays.
type Game interface {
	// Update advances the game by dt seconds; done reports it finished, and
	// result is what goes into the quest variable (1 solved, 0 given up).
	Update(dt float64) (done bool, result int)
	Draw(screen *ebiten.Image)
}

// Host is what a minigame gets from the adventure: its pack's assets and the
// sound channels.
type Host interface {
	// Images decodes every bitmap of a top-level pack. palOf names the palette
	// index a sprite belongs to for packs that ship several; nil, or an index it
	// does not answer for, means the pack's default palette.
	Images(pack string, palOf func(name string) int) map[string]*ebiten.Image
	// Text is a text entry of a pack, "" when it is missing.
	Text(pack, name string) string
	// PlaySound plays a .wav from the minigame sound bank on a channel.
	PlaySound(file string, ch int)
}

// New starts a game. param is the value of the quest variable StartGame names
// as its third argument. A game whose assets are missing returns nil.
type New func(h Host, param int) Game

// host serves a minigame from the game's resources and audio.
type host struct {
	res   interfaces.IResources
	audio interfaces.IAudio
}

var _ Host = host{}

// NewHost is the Host every build uses: the resource layer and the audio.
func NewHost(res interfaces.IResources, audio interfaces.IAudio) Host {
	return host{res: res, audio: audio}
}

// Images decodes a pack into Ebiten images, forcing full-screen backdrops
// opaque.
func (h host) Images(
	pack string, palOf func(name string) int,
) map[string]*ebiten.Image {
	sprites, pal := h.res.ScreenPack(pack)
	pals := h.res.ScreenPalettes(pack)
	out := make(map[string]*ebiten.Image, len(sprites))
	for name, n := range sprites {
		if n == nil {
			continue
		}
		use := pal
		if palOf != nil {
			if i := palOf(name); i >= 0 && i < len(pals) {
				use = pals[i]
			}
		}
		rgba := n.RGBA(use)
		if n.Width == ScreenW && n.Height == ScreenH {
			for i := 0; i < n.Width*n.Height; i++ {
				rgba[i*4+3] = 255
			}
		}
		img := ebiten.NewImage(n.Width, n.Height)
		img.WritePixels(rgba)
		out[name] = img
	}
	return out
}

func (h host) Text(pack, name string) string {
	return h.res.ScreenText(pack, name)
}

func (h host) PlaySound(file string, ch int) {
	h.audio.Play(file, h.res.Sound(file), ch)
}

// Blit draws an image at (x, y); a missing one draws nothing.
func Blit(screen *ebiten.Image, img *ebiten.Image, x, y int) {
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

// Opaque reports whether the image has an opaque pixel at (x, y) — the engine
// picks jigsaw pieces by their pixels, not their rectangles.
func Opaque(img *ebiten.Image, x, y int) bool {
	if img == nil {
		return false
	}
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return false
	}
	_, _, _, a := img.At(x, y).RGBA()
	return a > 0
}

// In reports whether (x, y) lies inside r.
func In(r image.Rectangle, x, y int) bool {
	return x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y
}

// Clicked reports a click: the left button going down, or a tap.
func Clicked() bool { return pointer.Clicked() }

// Cursor is where the pointer is: the mouse, or the finger last seen.
func Cursor() (int, int) { return pointer.Pos() }

// RightClicked reports a right click: the right button, or a finger held
// still (it then lifts without clicking).
func RightClicked() bool { return pointer.TakeRightClick() }
