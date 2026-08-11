package codec

import (
	"encoding/binary"

	"github.com/shpaker/modern-robinson/internal/types"
)

// NGB signatures.
const (
	sigRaw = 0x01F0CDAB // subtype A: raw width*height indices
	sigRLE = 0x01EFCDAB // subtype B: row-offset table + RLE runs
)

// LoadPalette parses a .COL: 256 Windows RGBQUAD entries (B,G,R,reserved),
// returning them channel-swapped to R,G,B with full alpha.
func LoadPalette(col []byte) types.Palette {
	var p types.Palette
	for i := 0; i < 256 && i*4+2 < len(col); i++ {
		b, g, r := col[i*4+0], col[i*4+1], col[i*4+2]
		p[i] = [4]byte{r, g, b, 255} // BGR -> RGB
	}
	return p
}

// DecodeNGB parses NGB bytes into a Domain bitmap. See ngiGetNgbPixel (0xF7F0).
func DecodeNGB(d []byte) *types.NGB {
	n := &types.NGB{
		X0:          int16(binary.LittleEndian.Uint16(d[0:2])),
		Y0:          int16(binary.LittleEndian.Uint16(d[2:4])),
		X1:          int16(binary.LittleEndian.Uint16(d[4:6])),
		Y1:          int16(binary.LittleEndian.Uint16(d[6:8])),
		Transparent: d[8],
		Sig:         binary.LittleEndian.Uint32(d[0x0C:0x10]),
	}
	n.Width = int(n.X1-n.X0) + 1
	n.Height = int(n.Y1-n.Y0) + 1
	if n.Width < 0 {
		n.Width = 0
	}
	if n.Height < 0 {
		n.Height = 0
	}
	n.Indices = make([]byte, n.Width*n.Height)
	n.Mask = make([]bool, n.Width*n.Height)
	for i := range n.Indices {
		n.Indices[i] = n.Transparent
	}
	if n.Sig == sigRaw {
		decodeRaw(n, d)
	} else {
		decodeRLE(n, d)
	}
	return n
}

func decodeRaw(n *types.NGB, d []byte) {
	body := d[0x10:]
	sz := n.Width * n.Height
	if sz > len(body) {
		sz = len(body)
	}
	copy(n.Indices, body[:sz])
	for i := 0; i < sz; i++ {
		n.Mask[i] = body[i] != n.Transparent
	}
}

func decodeRLE(n *types.NGB, d []byte) {
	nn := len(d)
	w, h := n.Width, n.Height
	for y := 0; y < h; y++ {
		toff := 0x10 + y*4
		if toff+4 > nn {
			break
		}
		p := int(binary.LittleEndian.Uint32(d[toff : toff+4]))
		x := 0
		for p+4 <= nn {
			header := binary.LittleEndian.Uint32(d[p : p+4])
			if header&0x8000 != 0 { // end of row
				break
			}
			skip := int(header & 0xFFFF)
			count := int(header >> 16)
			x += skip
			if count > 0 {
				if p+4+count > nn || x+count > w {
					break
				}
				row := y * w
				copy(n.Indices[row+x:row+x+count], d[p+4:p+4+count])
				for k := 0; k < count; k++ {
					n.Mask[row+x+k] = true
				}
				x += count
			}
			p += 4 + count
		}
	}
}
