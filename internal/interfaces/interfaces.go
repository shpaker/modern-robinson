// Package interfaces defines the contracts between layers. No engine or
// runtime imports — only the Domain (types). Dependencies point inward.
package interfaces

import "github.com/shpaker/modern-robinson/internal/types"

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
	Sound(name string) []byte
	SceneContainer(name string) IContainer
	SceneBackground(name string) (*types.NGB, types.Palette, []byte)
	BarBackground() (*types.NGB, types.Palette)
	BarSprites() (map[string]*types.NGB, types.Palette)
	// Screen is a named full-screen image from a top-level pack (LOGO, OPTIONS).
	Screen(pack, name string) (*types.NGB, types.Palette)
	// ScreenPack is every bitmap of a pack plus its shared palette.
	ScreenPack(pack string) (map[string]*types.NGB, types.Palette)
	// SceneFade is the scene's per-step fade brightness curve (1 -> 0).
	SceneFade(name string) []float64
	// Texts is the global string table (TEXT.DAT); Text ids are 0-based lines.
	Texts() []string
	// InitialVisibility is the start-of-game object visibility for a scene
	// (from BEGIN.BGI), keyed by lower-case object name; objects absent from
	// the map default to visible.
	InitialVisibility(objectNames []string) map[string]bool
}

// ISceneParser parses NGI text scripts into Domain entities.
type ISceneParser interface {
	ParseScene(text string) *types.Scene
	ParseObject(text string) *types.SceneObject
	ParseFrameScript(text string) *types.FrameScript
	ParseStartup(text string) (vars map[string]int, charVars map[string]string)
	ParseBar(text string) *types.Bar
	ParseChar(text string) *types.Character
	SceneExits(c IContainer) (left, right types.Exit)
}

// IGrid maps between screen pixels and walk-grid cells and finds paths.
type IGrid interface {
	ToScreen(gx, gy int) (int, int)
	ToCell(px, py int) (int, int)
	Valid(gx, gy int) bool
	Blocked(gx, gy int) bool
	SetVert(gx, gy int, open bool)
	NearestFree(gx, gy int) (int, int, bool)
	Path(start, goal [2]int) [][2]int
	Dims() (int, int)
}
