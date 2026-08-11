"""
lzhuf.py — NGI LZHUF (method 0x80) decompressor.

Register-faithful port of NGI32.DLL sub_26F4E (bl & 0x80 branch) and its helpers
sub_26D00 (StartHuff), sub_26DCE (DecodeChar/update). This is LZSS + adaptive
Huffman, but with NGI's OWN position tables (d_code/d_len built by StartHuff),
which are NOT the textbook lzhuf tables — that difference is why a naive port
diverges on long streams.

Confirmed constants: N=4096, F=60, THRESHOLD=2, T=627, R=626, ring start=0xFC4,
match length = symbol-253.  Shared 32-bit bit accumulator: tree navigation
consumes bit 15 (`shl bp,1`); position decode reads a byte from bits 16-23
(`shl ebp,8`).  d_code/d_len below were emulated from StartHuff's build loops.
"""
from __future__ import annotations

N, F, THRESHOLD = 4096, 60, 2
N_CHAR = 256 - THRESHOLD + F      # 314
T = N_CHAR * 2 - 1                # 627
R = T - 1                         # 626
MAX_FREQ = 0x8000
MASK = 0xFFFFFFFF

# NGI position tables (from StartHuff build, sub_26D00). d_code values 0..63,
# d_len bit-lengths 1..6.  Built from count tables [1,3,8,12,24,16]/[32,48,64,48,48,16].
_C210 = (1, 3, 8, 12, 24, 16)
_C216 = (32, 48, 64, 48, 48, 16)


def _build_tables():
    d_code = [0] * 256
    eax = 0
    bit = 0x20
    idx = 0
    ci = 0
    while bit:
        ch = _C210[ci]; ci += 1
        while ch > 0:
            for _ in range(bit):
                d_code[idx] = eax >> 22
                idx += 1
            eax += 0x400000
            ch -= 1
        bit >>= 1
    d_len = []
    val = 1
    for cnt in _C216:
        d_len += [val] * cnt
        val += 1
    return d_code, d_len


D_CODE, D_LEN = _build_tables()


class _Huff:
    """Adaptive Huffman tree — StartHuff/update/reconst (sub_26D00 / sub_26DCE)."""

    def __init__(self):
        self.freq = [0] * (T + 1)
        self.son = [0] * T
        self.prnt = [0] * (T + N_CHAR)
        for i in range(N_CHAR):
            self.freq[i] = 1
            self.son[i] = i + T
            self.prnt[i + T] = i
        i, j = 0, N_CHAR
        while j <= R:
            self.freq[j] = self.freq[i] + self.freq[i + 1]
            self.son[j] = i
            self.prnt[i] = self.prnt[i + 1] = j
            i += 2
            j += 1
        self.freq[T] = 0xFFFF
        self.prnt[R] = 0

    def reconst(self):
        freq, son, prnt = self.freq, self.son, self.prnt
        j = 0
        for i in range(T):
            if son[i] >= T:
                freq[j] = (freq[i] + 1) // 2
                son[j] = son[i]
                j += 1
        i = 0
        j = N_CHAR
        while j < T:
            f = freq[j] = freq[i] + freq[i + 1]
            k = j - 1
            while f < freq[k]:
                k -= 1
            k += 1
            for m in range(j, k, -1):
                freq[m] = freq[m - 1]
                son[m] = son[m - 1]
            freq[k] = f
            son[k] = i
            i += 2
            j += 1
        for i in range(T):
            k = son[i]
            if k >= T:
                prnt[k] = i
            else:
                prnt[k] = prnt[k + 1] = i

    def update(self, c):
        freq, son, prnt = self.freq, self.son, self.prnt
        if freq[R] == MAX_FREQ:
            self.reconst()
        c = prnt[c + T]
        while True:
            freq[c] += 1
            k = freq[c]
            l = c + 1
            if k > freq[l]:
                while k > freq[l + 1]:
                    l += 1
                freq[c] = freq[l]
                freq[l] = k
                i = son[c]
                prnt[i] = l
                if i < T:
                    prnt[i + 1] = l
                j2 = son[l]
                son[l] = i
                prnt[j2] = c
                if j2 < T:
                    prnt[j2 + 1] = c
                son[c] = j2
                c = l
            c = prnt[c]
            if c == 0:
                break


def decompress(src: bytes, out_size: int) -> bytes:
    huff = _Huff()
    son = huff.son
    ring = bytearray([0x20] * N)
    ringpos = 0xFC4
    out = bytearray()
    n = len(src)
    si = 0
    ebp = 0        # 32-bit bit accumulator
    cl = 8

    while len(out) < out_size:
        # --- DecodeChar: navigate tree (entry 0x26FAD) ---
        c = son[R]
        cl -= 1
        while c < T:
            cl += 1
            if cl >= 0:                       # refill (0x26FB8)
                while True:
                    b = src[si] if si < n else 0
                    si += 1
                    ebp = (ebp | (b << cl)) & MASK
                    cl -= 8
                    if cl < 0:
                        break
            # getbit from bit 15 (0x26FCA: shl bp,1)
            bit = (ebp >> 15) & 1
            ebp = (ebp & 0xFFFF0000) | ((ebp << 1) & 0xFFFF)
            c = son[c + bit]
        cl += 1                               # 0x26FE6
        c -= T
        huff.update(c)

        if c < 256:                           # literal
            out.append(c)
            ring[ringpos] = c
            ringpos = (ringpos + 1) & (N - 1)
            continue

        # --- match: decode position (0x27022) ---
        if cl >= 0:                           # top-up (0x2702B)
            while True:
                b = src[si] if si < n else 0
                si += 1
                ebp = (ebp | ((b & 0xFF) << cl)) & MASK
                cl -= 8
                if cl < 0:
                    break
        ebp = (ebp << 8) & MASK               # 0x27040
        i = (ebp >> 16) & 0xFF
        ch = D_LEN[i]
        base = D_CODE[i]
        cl += 8                               # 0x2705A
        state_refill = (cl >= 0)
        while True:
            if state_refill:                  # L_refill (0x27069)
                while cl >= 0:
                    b = src[si] if si < n else 0
                    si += 1
                    ebp = (ebp | ((b & 0xFF) << cl)) & MASK
                    cl -= 8
                state_refill = False
            ebp = (ebp << 1) & MASK           # L_shift (0x2705F)
            ch -= 1
            if ch == 0:
                break
            cl += 1
            state_refill = cl >= 0
        cl += 1                               # 0x2707E inc cl
        pos = (((ebp & 0x3F0000) | (base << 22)) >> 16) & 0xFFFF

        length = c - 253                      # symbol-253  (0x2709A lea ebp,[ebx-0xFD])
        srcidx = (ringpos - 1 - pos) & (N - 1)
        for _ in range(length):
            b = ring[srcidx & (N - 1)]
            srcidx += 1
            out.append(b)
            ring[ringpos] = b
            ringpos = (ringpos + 1) & (N - 1)
            if len(out) >= out_size:
                break

    return bytes(out)
