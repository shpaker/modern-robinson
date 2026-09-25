// Package interfaces defines the contracts between layers. No engine or
// runtime imports — only the Domain (types). Dependencies point inward.
package interfaces

import (
	"context"

	"github.com/shpaker/modern-robinson/internal/types"
)

// IContainer is a parsed NL resource file (.DAN / .DAT / .MV).
type IContainer interface {
	Entries() []types.Entry
	Find(name string) (types.Entry, bool)
	FindExt(ext string) (types.Entry, bool)
	Extract(e types.Entry) ([]byte, error)
	ExtractName(name string) ([]byte, error)
}

// IResources indexes the game's resources and resolves assets by name. It is
// the only door to raw game files (ARCHITECTURE.md: no hardcoded paths elsewhere).
type IResources interface {
	Root() string
	Movie(name string) IContainer
	MovieFrames(name string) ([]*types.NGB, types.Palette)
	// MovieShift is a movie's authored canvas hotspot (.SCR origin): the point
	// the engine lands on the cell anchor. (0,0) when absent.
	MovieShift(name string) [2]int
	Sound(name string) []byte
	SceneContainer(name string) IContainer
	SceneBackground(name string) (*types.NGB, types.Palette, []byte)
	BarBackground() (*types.NGB, types.Palette)
	BarSprites() (map[string]*types.NGB, types.Palette)
	// Screen is a named full-screen image from a top-level pack (LOGO, OPTIONS).
	Screen(pack, name string) (*types.NGB, types.Palette)
	// ScreenPack is every bitmap of a pack plus its shared palette.
	ScreenPack(pack string) (map[string]*types.NGB, types.Palette)
	// ScreenPalettes is every palette of a pack, in directory order, for the
	// packs that ship more than one.
	ScreenPalettes(pack string) []types.Palette
	// ScreenFile is a raw entry of a top-level pack (CRYPT.TXT, ...).
	ScreenFile(pack, name string) []byte
	// ScreenText is a text entry of a top-level pack, decoded from CP1251;
	// empty when the pack or the entry is missing.
	ScreenText(pack, name string) string
	// SceneFade is the scene's per-step fade brightness curve (1 -> 0).
	SceneFade(name string) []float64
	// Texts is the global string table (TEXT.DAT); Text ids are 0-based lines.
	Texts() []string
	// InitialVisibility is the start-of-game object visibility for a scene
	// (from BEGIN.BGI), keyed by lower-case object name; objects absent from
	// the map default to visible.
	InitialVisibility(objectNames []string) map[string]bool
	// Cursors is the game's mouse cursors (resources of ROBY.EXE), keyed by
	// name in upper case — an item's ("HAND", "HAT"), the exit arrows
	// ADV1..ADV4 and the waiting clock "247"; empty without the executable.
	Cursors() map[string]types.Cursor
}

// IAudio plays the game's sounds: numbered effect channels, a looping music
// channel, and the scene's sound variables, each on its own set of voices.
type IAudio interface {
	Play(key string, wavBytes []byte, channel int)
	// PlayVoice sounds a sound variable on one of its voices (how many copies
	// may play at once), attenuated by volScale (0..1) and panned by pan (-1
	// left .. +1 right); with every voice busy a lone one drops the trigger and
	// several take turns.
	PlayVoice(
		key string,
		wavBytes []byte,
		voices int,
		volScale, pan float64,
	)
	PlayMusic(key string, wavBytes []byte)
	StopMusic()
	// StopEffects silences everything but the music channel.
	StopEffects()
	SetVolume(v float64)
	SetMusicVolume(v float64)
}

// ISceneParser parses NGI text scripts into Domain entities.
type ISceneParser interface {
	ParseScene(text string) *types.Scene
	ParseObject(text string) *types.SceneObject
	ParseFrameScript(text string) *types.FrameScript
	ParseStartup(text string) types.Startup
	ParseBar(text string) *types.Bar
	ParseChar(text string) *types.Character
	SceneExits(c IContainer) (left, right types.Exit)
}

// IGrid maps between screen pixels and walk-grid cells and finds paths.
type IGrid interface {
	ToScreen(gx, gy int) (int, int)
	Corner(gx, gy int) (int, int)
	ToCell(px, py int) (int, int)
	Valid(gx, gy int) bool
	Blocked(gx, gy int) bool
	SetVert(gx, gy int, open bool)
	// SetDir toggles a single step fence: leaving (gx,gy) in numpad direction d.
	SetDir(gx, gy, d int, open bool)
	// Fenced reports whether leaving (gx,gy) in numpad direction d is fenced.
	Fenced(gx, gy, d int) bool
	NearestFree(gx, gy int) (int, int, bool)
	Path(start, goal [2]int) [][2]int
	// PathStraight avoids diagonals, for ArrowGoing characters.
	PathStraight(start, goal [2]int) [][2]int
	Dims() (int, int)
}

// IControl is the hero played from outside the game: he looks around and
// acts the way a player would, through the same clicks, and learns only what
// the player sees and hears — as data; the words are the driver's own. Each
// call blocks until the game has taken it in; an error is a refusal worth
// telling the driver ("занято: сцена").
type IControl interface {
	// Look is the world at a glance.
	Look(ctx context.Context) (types.Percept, error)
	// Sight is a PNG of what is in view: the scene without the bar and the
	// cursor, or a whole puzzle screen.
	Sight(ctx context.Context) ([]byte, error)
	// Use aims an item at a thing around, or at the hero himself ("себя");
	// an empty item is whatever is in hand.
	Use(ctx context.Context, target, item string) (types.Outcome, error)
	// Go takes an exit, or a place on the island map.
	Go(ctx context.Context, to string) (types.Outcome, error)
	// OpenMap unfolds the island map.
	OpenMap(ctx context.Context) (types.Outcome, error)
	// AskFriday has Friday aim one of her items at a thing, or at herself;
	// control comes back to the hero afterwards.
	AskFriday(ctx context.Context, target, item string) (types.Outcome, error)
	// Wait lets the game run: until the hero may act, or for the seconds
	// given. A puzzle stands between calls, waiting for the next move, and
	// a wait is its time.
	Wait(ctx context.Context, seconds float64) (types.Outcome, error)
	// PuzzleClick clicks a puzzle screen (640x480) at x,y. It answers once
	// the puzzle has played out what the click set going, and after the win
	// once the puzzle has closed.
	PuzzleClick(
		ctx context.Context,
		x, y int,
		right bool,
	) (types.Outcome, error)
	// PuzzleMove carries a piece across a puzzle screen with the player's
	// clicks: one at from takes it, turns right clicks at to turn it there
	// (0..3), and one at to puts it down. A first click that takes nothing
	// up of its own ends the move, and so does a press on the real mouse.
	PuzzleMove(
		ctx context.Context,
		fromX, fromY, toX, toY, turns int,
	) (types.Outcome, error)
	// PuzzleGiveUp leaves a puzzle unsolved, as Esc does, once it has played
	// out what it was playing out: a win it shows closes it solved.
	PuzzleGiveUp(ctx context.Context) (types.Outcome, error)
	// Save writes the run into a slot (0..11).
	Save(ctx context.Context, slot int) error
	// Load restores the run from a slot.
	Load(ctx context.Context, slot int) (types.Outcome, error)
}
