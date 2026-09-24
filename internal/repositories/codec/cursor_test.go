package codec

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

// monoCursor builds one RT_CURSOR image: a 32x32 one-bit DIB with a black and
// a white palette entry. set(x, y) returns the pixel's AND and XOR bits, with y
// counted from the top as the screen sees it.
func monoCursor(hx, hy int, set func(x, y int) (and, xor bool)) []byte {
	const w, h, stride = 32, 32, 4
	b := make([]byte, 4+40+8+2*stride*h)
	binary.LittleEndian.PutUint16(b[0:], uint16(hx))
	binary.LittleEndian.PutUint16(b[2:], uint16(hy))
	dib := b[4:]
	binary.LittleEndian.PutUint32(dib[0:], 40)
	binary.LittleEndian.PutUint32(dib[4:], w)
	binary.LittleEndian.PutUint32(dib[8:], 2*h)
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 1)
	copy(dib[40:], []byte{0, 0, 0, 0, 255, 255, 255, 0}) // black, white
	xor, and := 48, 48+stride*h
	for y := 0; y < h; y++ {
		row := (h - 1 - y) * stride
		for x := 0; x < w; x++ {
			a, xo := set(x, y)
			bit := byte(0x80) >> (x % 8)
			if a {
				dib[and+row+x/8] |= bit
			}
			if xo {
				dib[xor+row+x/8] |= bit
			}
		}
	}
	return b
}

// resNode is a node of a resource tree for the synthetic PE: a directory when
// it has kids, a data leaf otherwise.
type resNode struct {
	name string // named entry when set, otherwise id
	id   uint32
	kids []*resNode
	data []byte
}

// buildPE lays out a minimal PE32 image with a single .rsrc section holding
// the tree: directories first, then data entries, names and the data itself.
func buildPE(root *resNode) []byte {
	const peOff, optSize, secRVA, secRaw = 0x40, 224, 0x1000, 0x200
	var dirs, leaves, named []*resNode
	var walk func(n *resNode)
	walk = func(n *resNode) {
		if n.kids == nil {
			leaves = append(leaves, n)
			return
		}
		dirs = append(dirs, n)
		for _, k := range n.kids {
			if k.name != "" {
				named = append(named, k)
			}
		}
		for _, k := range n.kids {
			walk(k)
		}
	}
	walk(root)
	off := map[*resNode]int{}
	at := 0
	for _, d := range dirs {
		off[d] = at
		at += 16 + 8*len(d.kids)
	}
	for _, l := range leaves {
		off[l] = at
		at += 16
	}
	nameAt := map[*resNode]int{}
	for _, n := range named {
		nameAt[n] = at
		at += 2 + 2*len(utf16.Encode([]rune(n.name)))
	}
	dataAt := map[*resNode]int{}
	for _, l := range leaves {
		at = (at + 3) &^ 3
		dataAt[l] = at
		at += len(l.data)
	}
	rs := make([]byte, at)
	put16 := func(o, v int) { binary.LittleEndian.PutUint16(rs[o:], uint16(v)) }
	put32 := func(o int, v uint32) { binary.LittleEndian.PutUint32(rs[o:], v) }
	for _, d := range dirs {
		o, nNamed := off[d], 0
		for _, k := range d.kids {
			if k.name != "" {
				nNamed++
			}
		}
		put16(o+12, nNamed)
		put16(o+14, len(d.kids)-nNamed)
		for i, k := range d.kids {
			e := o + 16 + i*8
			if k.name != "" {
				put32(e, 0x80000000|uint32(nameAt[k]))
			} else {
				put32(e, k.id)
			}
			if k.kids != nil {
				put32(e+4, 0x80000000|uint32(off[k]))
			} else {
				put32(e+4, uint32(off[k]))
			}
		}
	}
	for _, n := range named {
		u := utf16.Encode([]rune(n.name))
		put16(nameAt[n], len(u))
		for i, c := range u {
			put16(nameAt[n]+2+2*i, int(c))
		}
	}
	for _, l := range leaves {
		put32(off[l], uint32(secRVA+dataAt[l]))
		put32(off[l]+4, uint32(len(l.data)))
		copy(rs[dataAt[l]:], l.data)
	}

	img := make([]byte, secRaw+len(rs))
	img[0], img[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(img[0x3C:], peOff)
	copy(img[peOff:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(img[peOff+4:], 0x14c)
	binary.LittleEndian.PutUint16(img[peOff+6:], 1)
	binary.LittleEndian.PutUint16(img[peOff+20:], optSize)
	opt := peOff + 24
	binary.LittleEndian.PutUint16(img[opt:], 0x10b)
	binary.LittleEndian.PutUint32(img[opt+96+16:], secRVA)
	binary.LittleEndian.PutUint32(img[opt+96+20:], uint32(len(rs)))
	sec := opt + optSize
	copy(img[sec:], ".rsrc")
	binary.LittleEndian.PutUint32(img[sec+8:], uint32(len(rs)))
	binary.LittleEndian.PutUint32(img[sec+12:], secRVA)
	binary.LittleEndian.PutUint32(img[sec+16:], uint32(len(rs)))
	binary.LittleEndian.PutUint32(img[sec+20:], secRaw)
	copy(img[secRaw:], rs)
	return img
}

// group is an RT_GROUP_CURSOR body naming a single image.
func group(id uint16) []byte {
	b := make([]byte, 6+14)
	binary.LittleEndian.PutUint16(b[2:], 2) // cursors
	binary.LittleEndian.PutUint16(b[4:], 1)
	binary.LittleEndian.PutUint16(b[6+12:], id)
	return b
}

// A cursor group resolves by its name (or number) to the image it lists, and
// the image's masks give see-through, black and white pixels, with the rows
// turned right side up and the hotspot carried along.
func TestExeCursorsReadsGroupsAndMasks(t *testing.T) {
	img := monoCursor(3, 5, func(x, y int) (bool, bool) {
		switch {
		case x == 0 && y == 0:
			return false, true // white
		case x == 1 && y == 0:
			return false, false // black
		case x == 0 && y == 31:
			return false, true // white, bottom row
		}
		return true, false // see-through
	})
	// type -> name -> language -> data: the name's directory lists languages.
	leaf := func(id uint32, data []byte) *resNode {
		return &resNode{id: id, kids: []*resNode{{id: 1049, data: data}}}
	}
	named := func(name string, data []byte) *resNode {
		n := leaf(0, data)
		n.name = name
		return n
	}
	exe := buildPE(&resNode{kids: []*resNode{
		{id: rtCursor, kids: []*resNode{leaf(7, img)}},
		{id: rtGroupCursor, kids: []*resNode{
			named("hand", group(7)),
			leaf(247, group(7)),
			named("LOST", group(9)), // points at no image
		}},
	}})

	got, err := ExeCursors(exe)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("cursors = %d, want HAND and 247 (LOST has no image)", len(got))
	}
	c, ok := got["HAND"]
	if !ok {
		t.Fatalf("no HAND in %v", keys(got))
	}
	if _, ok := got["247"]; !ok {
		t.Errorf("a numbered group must answer to its number: %v", keys(got))
	}
	if c.W != 32 || c.H != 32 || c.HotX != 3 || c.HotY != 5 {
		t.Errorf("cursor = %dx%d hot %d,%d, want 32x32 hot 3,5",
			c.W, c.H, c.HotX, c.HotY)
	}
	px := func(x, y int) [4]byte {
		o := (y*c.W + x) * 4
		return [4]byte{c.Pix[o], c.Pix[o+1], c.Pix[o+2], c.Pix[o+3]}
	}
	if p := px(0, 0); p != [4]byte{255, 255, 255, 255} {
		t.Errorf("top-left = %v, want white", p)
	}
	if p := px(1, 0); p != [4]byte{0, 0, 0, 255} {
		t.Errorf("(1,0) = %v, want black", p)
	}
	if p := px(0, 31); p != [4]byte{255, 255, 255, 255} {
		t.Errorf("bottom-left = %v, want white: rows are stored bottom-up", p)
	}
	if p := px(5, 5); p != [4]byte{} {
		t.Errorf("(5,5) = %v, want see-through", p)
	}
}

func keys(m map[string]types.Cursor) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Not an executable, or one without resources, is an error, not a panic.
func TestExeCursorsRejectsNonPE(t *testing.T) {
	for _, b := range [][]byte{nil, []byte("MZ"), make([]byte, 0x100)} {
		if _, err := ExeCursors(b); err == nil {
			t.Errorf("ExeCursors(%d bytes) succeeded", len(b))
		}
	}
}

// The shipped ROBY.EXE: a cursor for each of the 33 items, the four exit
// arrows and the waiting clock, with their hotspots on the arrow tips.
func TestExeCursorsOnShippedExe(t *testing.T) {
	exe, err := os.ReadFile(filepath.Join(testutil.GameRoot(t), "ROBY.EXE"))
	if err != nil {
		t.Skip("ROBY.EXE not present")
	}
	got, err := ExeCursors(exe)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 38 {
		t.Errorf("cursors = %d, want 38", len(got))
	}
	for name, hot := range map[string][2]int{
		"ADV1": {0, 15}, "ADV2": {31, 15}, "ADV3": {15, 0}, "ADV4": {15, 31},
		"247": {15, 0}, "HAND": {0, 0}, "HAT": {0, 26}, "HANDFR": {0, 0},
	} {
		c, ok := got[name]
		if !ok {
			t.Errorf("no %s cursor", name)
			continue
		}
		if c.W != 32 || c.H != 32 || [2]int{c.HotX, c.HotY} != hot {
			t.Errorf("%s = %dx%d hot %d,%d, want 32x32 hot %v",
				name, c.W, c.H, c.HotX, c.HotY, hot)
		}
	}
}
