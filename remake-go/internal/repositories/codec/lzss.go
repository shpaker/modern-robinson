package codec

// LZSS (method 0x40) — Okumura LZSS, port of NGI32.DLL sub_26F4E (classic
// branch). 4096-byte ring prefilled with 0x20, write pos starts at 0xFEE;
// flag byte LSB-first, bit 1 => literal, bit 0 => (pos12, len4+3).

func lzssDecompress(src []byte, outSize int) []byte {
	const n = 4096
	ring := make([]byte, n)
	for i := range ring {
		ring[i] = 0x20
	}
	pos := 0xFEE
	out := make([]byte, 0, outSize)
	si := 0
	flags := 0
	sl := len(src)
	for len(out) < outSize && si < sl {
		flags >>= 1
		if flags&0x100 == 0 {
			if si >= sl {
				break
			}
			flags = int(src[si]) | 0xFF00
			si++
		}
		if flags&1 != 0 { // literal
			if si >= sl {
				break
			}
			b := src[si]
			si++
			out = append(out, b)
			ring[pos] = b
			pos = (pos + 1) & (n - 1)
		} else { // back-reference
			if si+1 >= sl {
				break
			}
			lo := int(src[si])
			hi := int(src[si+1])
			si += 2
			p := ((hi >> 4) << 8) | lo
			length := (hi & 0x0F) + 3
			for k := 0; k < length; k++ {
				b := ring[p&(n-1)]
				p++
				out = append(out, b)
				ring[pos] = b
				pos = (pos + 1) & (n - 1)
				if len(out) >= outSize {
					break
				}
			}
		}
	}
	return out
}
