// Package types holds the Domain layer: entity data with no dependencies on
// the engine, decoders, or runtimes. See ARCHITECTURE.md.
package types

// Entry is one decrypted directory record of an NL container (32 bytes on disk).
type Entry struct {
	Name   string // "NAME.EXT"
	Method uint16 // ngiUnpack method: 0 raw, 0x40 LZSS, 0x80 LZHUF, 0x100 graphics
	ID     uint16
	USize  uint32 // uncompressed size
	Offset uint32 // absolute byte offset of the data blob
	CSize  uint32 // stored (compressed) size
}

// Compressed reports whether the entry's payload is stored compressed.
func (e Entry) Compressed() bool { return e.USize != e.CSize }

// Palette is 256 RGBA colors, channels already in R,G,B order (source COL is
// Windows RGBQUAD / BGR).
type Palette [256][4]byte

// NGB is a decoded Nikita Graphics Bitmap: palette indices plus an opacity mask.
type NGB struct {
	X0, Y0, X1, Y1 int16
	Transparent    byte
	Sig            uint32
	Width, Height  int
	Indices        []byte // Height*Width palette indices
	Mask           []bool // Height*Width, true = opaque
}

// RGBA renders the bitmap to premultiplied-alpha RGBA bytes (Height*Width*4).
// Transparent pixels are fully zero so they composite cleanly in premultiplied
// pipelines (Ebiten); opaque pixels keep their palette color at alpha 255.
func (n *NGB) RGBA(p Palette) []byte {
	out := make([]byte, n.Width*n.Height*4)
	for i, idx := range n.Indices {
		if !n.Mask[i] {
			continue
		}
		c := p[idx]
		o := i * 4
		out[o], out[o+1], out[o+2], out[o+3] = c[0], c[1], c[2], 255
	}
	return out
}

// ObjectRef places an object on the scene grid.
type ObjectRef struct {
	Name   string
	GX, GY int
	Flag   bool
}

// Scene is a parsed .SCN (screen layout, walk grid, objects, sound bank).
type Scene struct {
	Name, Screen, Bar string
	Size              [2]int
	LeftTopGrid       [2]int
	GridSize          [2]int
	GridLength        [2]int
	GridShift         [2]int
	ZPerGrid          int
	// ScrollPar divides the distance left to scroll and ScrollDesc caps it, so
	// the camera eases toward its target instead of snapping (engine 0x414e90).
	ScrollPar  [2]int
	ScrollDesc [2]int
	ClosedVert [][2]int
	Objects    []ObjectRef
	SoundVars  map[string][2]string // name -> {wav, channel}
	Music      string
}

// SceneObject is a parsed .OB (a clickable object on a scene).
type SceneObject struct {
	Name       string
	FonScript  string
	Z          int
	ActiveZone [4]int // x,y,w,h
	Cursor     int
	Text       int
	// ClosedVert are the cells the object blocks, relative to its own cell:
	// the crab, the bridge logs and the finished hut all stand in the way.
	ClosedVert [][2]int
}

// Command is one event line inside a frame (e.g. Sound, Text, Set, GoScene).
type Command struct {
	Kw   string
	Args []string
}

// Frame is one frame of a .FS frame script: a delay plus its event commands.
type Frame struct {
	Index, Sub, Delay int
	Events            []Command
}

// FrameScript is a parsed .FS (a movie's per-frame timeline).
type FrameScript struct {
	ScriptName string
	MovieName  string
	Shift      [2]int
	Total      int
	Frames     []*Frame
}

// Character is a parsed .CHR: the walk mode and the three idle slots the engine
// cycles through — [0] standing, [1] the acknowledge animation, [2] the long
// idle SetRest swaps out.
type Character struct {
	Name     string
	MoveType string // NumPadGoing (8 directions) or ArrowGoing (4)
	Idle     [3]string
	Items    []string
}

// Exit is a scene transition target reached from an edge / arrow object or a
// GoScene command; Entry names the hero's arrival .FS, EntryFrid the second
// character's (7-arg GoScene moves both).
type Exit struct {
	Scene     string
	Entry     string
	EntryFrid string
	GX, GY    int
	OK        bool
}
