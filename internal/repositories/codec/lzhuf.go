package codec

// LZHUF (method 0x80) — register-faithful port of NGI32.DLL sub_26F4E (0x80
// branch). LZSS + adaptive Huffman with NGI's own (non-canonical) position
// tables. Mirrors ../../../tools/lzhuf.py exactly. See ../../docs/02-compression.md.

const (
	lzN      = 4096
	lzF      = 60
	lzThresh = 2
	nChar    = 256 - lzThresh + lzF // 314
	tNode    = nChar*2 - 1          // 627
	rNode    = tNode - 1            // 626
	maxFreq  = 0x8000
)

var (
	dCode [256]int
	dLen  [256]int
)

func init() {
	// build d_code / d_len from StartHuff's count tables
	c210 := [6]int{1, 3, 8, 12, 24, 16}
	c216 := [6]int{32, 48, 64, 48, 48, 16}
	eax, bit, idx, ci := 0, 0x20, 0, 0
	for bit != 0 {
		ch := c210[ci]
		ci++
		for ch > 0 {
			for k := 0; k < bit; k++ {
				dCode[idx] = eax >> 22
				idx++
			}
			eax += 0x400000
			ch--
		}
		bit >>= 1
	}
	idx = 0
	for v, cnt := range c216 {
		for k := 0; k < cnt; k++ {
			dLen[idx] = v + 1
			idx++
		}
	}
}

type huff struct {
	freq [tNode + 1]int
	son  [tNode]int
	prnt [tNode + nChar]int
}

func newHuff() *huff {
	h := &huff{}
	for i := 0; i < nChar; i++ {
		h.freq[i] = 1
		h.son[i] = i + tNode
		h.prnt[i+tNode] = i
	}
	i, j := 0, nChar
	for j <= rNode {
		h.freq[j] = h.freq[i] + h.freq[i+1]
		h.son[j] = i
		h.prnt[i] = j
		h.prnt[i+1] = j
		i += 2
		j++
	}
	h.freq[tNode] = 0xFFFF
	h.prnt[rNode] = 0
	return h
}

func (h *huff) reconst() {
	j := 0
	for i := 0; i < tNode; i++ {
		if h.son[i] >= tNode {
			h.freq[j] = (h.freq[i] + 1) / 2
			h.son[j] = h.son[i]
			j++
		}
	}
	i := 0
	j = nChar
	for j < tNode {
		f := h.freq[i] + h.freq[i+1]
		h.freq[j] = f
		k := j - 1
		for f < h.freq[k] {
			k--
		}
		k++
		for m := j; m > k; m-- {
			h.freq[m] = h.freq[m-1]
			h.son[m] = h.son[m-1]
		}
		h.freq[k] = f
		h.son[k] = i
		i += 2
		j++
	}
	for i := 0; i < tNode; i++ {
		k := h.son[i]
		if k >= tNode {
			h.prnt[k] = i
		} else {
			h.prnt[k] = i
			h.prnt[k+1] = i
		}
	}
}

func (h *huff) update(c int) {
	if h.freq[rNode] == maxFreq {
		h.reconst()
	}
	c = h.prnt[c+tNode]
	for {
		h.freq[c]++
		k := h.freq[c]
		l := c + 1
		if k > h.freq[l] {
			for k > h.freq[l+1] {
				l++
			}
			h.freq[c] = h.freq[l]
			h.freq[l] = k
			i := h.son[c]
			h.prnt[i] = l
			if i < tNode {
				h.prnt[i+1] = l
			}
			j := h.son[l]
			h.son[l] = i
			h.prnt[j] = c
			if j < tNode {
				h.prnt[j+1] = c
			}
			h.son[c] = j
			c = l
		}
		c = h.prnt[c]
		if c == 0 {
			break
		}
	}
}

// lzhufDecompress decodes an LZHUF stream to exactly outSize bytes.
func lzhufDecompress(src []byte, outSize int) []byte {
	h := newHuff()
	ring := make([]byte, lzN)
	for i := range ring {
		ring[i] = 0x20
	}
	ringpos := 0xFC4
	out := make([]byte, 0, outSize)
	n := len(src)
	si := 0
	var ebp uint32
	cl := 8

	rd := func() uint32 {
		var b uint32
		if si < n {
			b = uint32(src[si])
		}
		si++
		return b
	}

	for len(out) < outSize {
		c := h.son[rNode]
		cl--
		for c < tNode {
			cl++
			if cl >= 0 {
				for {
					ebp |= rd() << uint(cl)
					cl -= 8
					if cl < 0 {
						break
					}
				}
			}
			bit := int((ebp >> 15) & 1)
			ebp = (ebp & 0xFFFF0000) | ((ebp << 1) & 0xFFFF)
			c = h.son[c+bit]
		}
		cl++
		c -= tNode
		h.update(c)

		if c < 256 {
			out = append(out, byte(c))
			ring[ringpos] = byte(c)
			ringpos = (ringpos + 1) & (lzN - 1)
			continue
		}
		// match: decode position
		if cl >= 0 {
			for {
				ebp |= (rd() & 0xFF) << uint(cl)
				cl -= 8
				if cl < 0 {
					break
				}
			}
		}
		ebp <<= 8
		i := int((ebp >> 16) & 0xFF)
		ch := dLen[i]
		base := dCode[i]
		cl += 8
		stateRefill := cl >= 0
		for {
			if stateRefill {
				for cl >= 0 {
					ebp |= (rd() & 0xFF) << uint(cl)
					cl -= 8
				}
			}
			ebp <<= 1
			ch--
			if ch == 0 {
				break
			}
			cl++
			stateRefill = cl >= 0
		}
		cl++
		pos := int((((ebp & 0x3F0000) | (uint32(base) << 22)) >> 16) & 0xFFFF)
		length := c - 253
		srcidx := (ringpos - 1 - pos) & (lzN - 1)
		for k := 0; k < length; k++ {
			b := ring[srcidx&(lzN-1)]
			srcidx++
			out = append(out, b)
			ring[ringpos] = b
			ringpos = (ringpos + 1) & (lzN - 1)
			if len(out) >= outSize {
				break
			}
		}
	}
	return out
}
