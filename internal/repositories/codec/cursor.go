package codec

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/shpaker/modern-robinson/internal/types"
)

// The game's mouse cursors are not in its packs: they are Win32 resources of
// ROBY.EXE, one group per inventory item plus the exit arrows and the waiting
// clock. Only what they need is read here: the section table that maps RVAs to
// the file, the resource tree, RT_GROUP_CURSOR by name and the RT_CURSOR image
// each group points at.
const (
	rtCursor      = 1
	rtGroupCursor = 12
)

var errNotPE = errors.New("codec: not a PE image with resources")

// ExeCursors decodes every cursor group of a Win32 executable, keyed by the
// group's name in upper case ("HAND", "ADV1") or by its number ("247"). A group
// shows its first image — the game ships a single 32x32 one-bit image in each.
// Images in any other format are left out.
func ExeCursors(exe []byte) (map[string]types.Cursor, error) {
	p, ok := parsePE(exe)
	if !ok {
		return nil, errNotPE
	}
	images := p.resources(rtCursor)
	out := map[string]types.Cursor{}
	for name, grp := range p.resources(rtGroupCursor) {
		// NEWHEADER (6 bytes), then 14-byte entries ending in the image id.
		if len(grp) < 6+14 || binary.LittleEndian.Uint16(grp[4:]) == 0 {
			continue
		}
		id := binary.LittleEndian.Uint16(grp[6+12:])
		img, ok := images[strconv.Itoa(int(id))]
		if !ok {
			continue
		}
		if c, ok := decodeCursor(img); ok {
			out[name] = c
		}
	}
	return out, nil
}

// decodeCursor turns one RT_CURSOR image into a bitmap: a 4-byte hotspot, then
// a 1-bit DIB whose height counts the XOR mask and the AND mask stacked, both
// stored bottom-up. AND set and XOR clear is see-through; AND clear paints the
// XOR bit's palette colour. AND and XOR both set inverts the screen under it —
// no cursor of the game uses that, and it is drawn black.
func decodeCursor(b []byte) (types.Cursor, bool) {
	if len(b) < 4+40 {
		return types.Cursor{}, false
	}
	le16 := binary.LittleEndian.Uint16
	le32 := binary.LittleEndian.Uint32
	hx, hy := int(le16(b[0:])), int(le16(b[2:]))
	dib := b[4:]
	hdr := int(le32(dib[0:]))
	w, h2 := int(int32(le32(dib[4:]))), int(int32(le32(dib[8:])))
	bpp, colors := le16(dib[14:]), int(le32(dib[32:]))
	if bpp != 1 || w <= 0 || h2 <= 0 || h2%2 != 0 || hdr < 40 {
		return types.Cursor{}, false
	}
	if colors == 0 {
		colors = 2
	}
	h := h2 / 2
	stride := (w + 31) / 32 * 4
	pal := hdr
	xor := pal + colors*4
	and := xor + stride*h
	if colors < 2 || len(dib) < and+stride*h {
		return types.Cursor{}, false
	}
	pix := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		row := (h - 1 - y) * stride // bottom-up
		for x := 0; x < w; x++ {
			bit := byte(0x80) >> (x % 8)
			xi := dib[xor+row+x/8]&bit != 0
			ai := dib[and+row+x/8]&bit != 0
			if ai && !xi {
				continue // see-through
			}
			var r, g, bl byte // the inverting pixel stays black
			if !ai {
				po := pal
				if xi {
					po += 4
				}
				bl, g, r = dib[po], dib[po+1], dib[po+2] // RGBQUAD is B,G,R
			}
			o := (y*w + x) * 4
			pix[o], pix[o+1], pix[o+2], pix[o+3] = r, g, bl, 255
		}
	}
	return types.Cursor{W: w, H: h, HotX: hx, HotY: hy, Pix: pix}, true
}

// peImage is the part of a PE file the resource lookup needs.
type peImage struct {
	d        []byte
	sections [][4]uint32 // virtual address, virtual size, raw offset, raw size
	rsrc     int         // file offset of the resource directory
}

func parsePE(d []byte) (*peImage, bool) {
	le16 := func(o int) int { return int(binary.LittleEndian.Uint16(d[o:])) }
	le32 := func(o int) uint32 { return binary.LittleEndian.Uint32(d[o:]) }
	if len(d) < 0x40 || d[0] != 'M' || d[1] != 'Z' {
		return nil, false
	}
	pe := int(le32(0x3C))
	if pe < 0 || pe+24 > len(d) || string(d[pe:pe+4]) != "PE\x00\x00" {
		return nil, false
	}
	nsec, optSize := le16(pe+6), le16(pe+20)
	opt := pe + 24
	if opt+optSize > len(d) || optSize < 2 {
		return nil, false
	}
	dirs := opt + 96 // PE32
	if le16(opt) == 0x20b {
		dirs = opt + 112 // PE32+
	}
	if dirs+3*8 > opt+optSize {
		return nil, false
	}
	p := &peImage{d: d}
	sec := opt + optSize
	for i := 0; i < nsec; i++ {
		o := sec + i*40
		if o+40 > len(d) {
			return nil, false
		}
		p.sections = append(p.sections, [4]uint32{
			le32(o + 12), le32(o + 8), le32(o + 20), le32(o + 16),
		})
	}
	off, ok := p.offset(le32(dirs + 2*8))
	if !ok || off+16 > len(d) {
		return nil, false
	}
	p.rsrc = off
	return p, true
}

// offset maps an RVA to its position in the file.
func (p *peImage) offset(rva uint32) (int, bool) {
	for _, s := range p.sections {
		size := max(s[1], s[3])
		if rva >= s[0] && rva < s[0]+size {
			return int(s[2] + rva - s[0]), true
		}
	}
	return 0, false
}

// resEntry is one entry of a resource directory: its key and where it leads.
type resEntry struct {
	key string
	off int  // relative to the resource directory
	dir bool // off is a subdirectory, not a data entry
}

// entries lists a resource directory at off (relative to the resource base).
func (p *peImage) entries(off int) []resEntry {
	d, base := p.d, p.rsrc
	at := base + off
	if off < 0 || at+16 > len(d) {
		return nil
	}
	n := int(binary.LittleEndian.Uint16(d[at+12:])) +
		int(binary.LittleEndian.Uint16(d[at+14:]))
	var out []resEntry
	for i := 0; i < n; i++ {
		e := at + 16 + i*8
		if e+8 > len(d) {
			break
		}
		name := binary.LittleEndian.Uint32(d[e:])
		to := binary.LittleEndian.Uint32(d[e+4:])
		key := strconv.Itoa(int(name))
		if name&0x80000000 != 0 {
			key = p.name(int(name & 0x7fffffff))
		}
		out = append(out, resEntry{
			key: key, off: int(to & 0x7fffffff), dir: to&0x80000000 != 0,
		})
	}
	return out
}

// name reads a length-prefixed UTF-16 resource name, upper-cased.
func (p *peImage) name(off int) string {
	at := p.rsrc + off
	if at+2 > len(p.d) {
		return ""
	}
	n := int(binary.LittleEndian.Uint16(p.d[at:]))
	if at+2+n*2 > len(p.d) {
		return ""
	}
	u := make([]uint16, n)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(p.d[at+2+i*2:])
	}
	return strings.ToUpper(string(utf16.Decode(u)))
}

// resources returns every resource of a type, keyed by name or number, each
// in its first language.
func (p *peImage) resources(typ int) map[string][]byte {
	out := map[string][]byte{}
	for _, t := range p.entries(0) {
		if !t.dir || t.key != strconv.Itoa(typ) {
			continue
		}
		for _, r := range p.entries(t.off) {
			if !r.dir {
				continue
			}
			langs := p.entries(r.off)
			if len(langs) == 0 || langs[0].dir {
				continue
			}
			if b, ok := p.data(langs[0].off); ok {
				out[r.key] = b
			}
		}
	}
	return out
}

// data reads the bytes an IMAGE_RESOURCE_DATA_ENTRY points at.
func (p *peImage) data(off int) ([]byte, bool) {
	at := p.rsrc + off
	if off < 0 || at+16 > len(p.d) {
		return nil, false
	}
	rva := binary.LittleEndian.Uint32(p.d[at:])
	size := int(binary.LittleEndian.Uint32(p.d[at+4:]))
	o, ok := p.offset(rva)
	if !ok || o+size > len(p.d) {
		return nil, false
	}
	return p.d[o : o+size], true
}
