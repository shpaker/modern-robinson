"""
ngi.py — reader/unpacker for the NGI ("Nikita Game Interface") resource
containers used by the 1999 quest game *Новый Робинзон* (Nikita).

Reverse-engineered from NGI32.DLL (the engine's resource loader / `ngiUnpack`).
See ../docs/ for the format specification.

Supported so far:
  * NL container parsing (header + encrypted 32-byte directory)
  * directory decryption (8-bit stream cipher, key = dword @ file+0x14)
  * method 0    : stored (raw) — e.g. WAVE.DAN  -> *.WAV (RIFF PCM)
  * method 0x40 : classic LZSS (Okumura, N=4096 F=18) — *.FS/*.OB/*.SCN/*.CHR
  * method 0x80 : LZHUF (LZSS+adaptive Huffman) — *.NGB/*.COL/*.SCR/*.FAD (~75%)
  * method 0x100: graphics variant (227 entries): NOT yet decoded (see docs/03)

Everything here is pure Python, no third-party deps.
"""
from __future__ import annotations
import struct
from dataclasses import dataclass

from .lzhuf import decompress as lzhuf_decompress


NL_MAGIC = b"NL\x00\x01"      # file[0:4]
ABBA = 0xABBA                  # marker at file+0x0E on "encrypted-directory" files


@dataclass
class Entry:
    name: str        # "NAME.EXT", up to 12 bytes, nul padded
    method: int      # ngiUnpack method/flags (0=raw, 0x40=LZSS, 0x100=graphics)
    id: int          # per-archive sequential id
    usize: int       # uncompressed size
    offset: int      # absolute byte offset of the data blob in the file
    csize: int       # stored (compressed) size

    @property
    def compressed(self) -> bool:
        return self.usize != self.csize


class Container:
    """A parsed NL resource file (.DAN / .DAT / .MV all share this format)."""

    def __init__(self, data: bytes):
        if data[0:2] != b"NL":
            raise ValueError("not an NL container (bad magic)")
        self.data = data
        self.version = data[2] | (data[3] << 8)          # 0x0100
        self.count = struct.unpack_from("<H", data, 4)[0]
        self.flag = struct.unpack_from("<H", data, 0x0C)[0]
        self.marker = struct.unpack_from("<H", data, 0x0E)[0]
        self.usize_total = struct.unpack_from("<I", data, 0x10)[0]
        self.csize_total = struct.unpack_from("<I", data, 0x14)[0]
        self.key = self.csize_total  # the cipher key IS the dword at 0x14
        self.entries = self._read_directory()

    # -- directory ---------------------------------------------------------
    def _decrypt_directory(self) -> bytes:
        """8-bit stream cipher lifted verbatim from NGI32.DLL @0x2333b.

        state = (al, dl); al=key&0xFF, dl=(key>>8)&0xFF
        per byte:  al=((al<<1)&0xFF)^dl; dl=(dl>>1)&0xFF
                   plain = cipher ^ al;  dl=(dl^al)&0xFF
        """
        n = self.count
        ct = self.data[0x20:0x20 + n * 32]
        key = self.key
        al = key & 0xFF
        dl = (key >> 8) & 0xFF
        out = bytearray(len(ct))
        for i, c in enumerate(ct):
            al = ((al << 1) & 0xFF) ^ dl
            dl = (dl >> 1) & 0xFF
            out[i] = c ^ al
            dl = (dl ^ al) & 0xFF
        return bytes(out)

    def _read_directory(self) -> list[Entry]:
        dec = self._decrypt_directory()
        out = []
        for i in range(self.count):
            e = dec[i * 32:(i + 1) * 32]
            name = e[:12].split(b"\x00", 1)[0].decode("latin1")
            method, eid = struct.unpack_from("<HH", e, 0x10)
            usize, offset, csize = struct.unpack_from("<III", e, 0x14)
            out.append(Entry(name, method, eid, usize, offset, csize))
        return out

    # -- extraction --------------------------------------------------------
    def raw(self, e: Entry) -> bytes:
        return self.data[e.offset:e.offset + e.csize]

    def extract(self, e: Entry) -> bytes:
        """Return the fully decoded (decompressed) bytes for an entry."""
        blob = self.raw(e)
        m = e.method
        if e.usize == e.csize:                 # stored
            return blob
        if m & 0x80:                           # LZHUF (LZSS + adaptive Huffman)
            return lzhuf_decompress(blob, e.usize)
        if (m & 0x1E0) == 0x40:                # classic LZSS
            return lzss_decompress(blob, e.usize)
        # method 0x100 graphics variant not decoded yet
        raise NotImplementedError(f"method {m:#06x} for {e.name!r} not implemented")

    def __iter__(self):
        return iter(self.entries)

    @classmethod
    def open(cls, path: str) -> "Container":
        with open(path, "rb") as f:
            return cls(f.read())


def lzss_decompress(src: bytes, out_size: int) -> bytes:
    """Okumura LZSS as implemented in NGI32.DLL sub_26f4e (bl & 0x80 == 0).

    4096-byte ring buffer prefilled with 0x20, write pos starts at 0xFEE.
    Flag byte consumed LSB-first: bit 1 => literal, bit 0 => (pos12, len4+3).
    The match position is an *absolute* index into the ring buffer.
    """
    N = 4096
    ring = bytearray([0x20] * N)
    pos = 0xFEE
    out = bytearray()
    si = 0
    flags = 0
    n = len(src)
    while len(out) < out_size and si < n:
        flags >>= 1
        if not (flags & 0x100):
            if si >= n:
                break
            flags = src[si] | 0xFF00
            si += 1
        if flags & 1:                         # literal
            if si >= n:
                break
            b = src[si]; si += 1
            out.append(b); ring[pos] = b; pos = (pos + 1) & (N - 1)
        else:                                 # back-reference
            if si + 1 >= n:
                break
            lo = src[si]; hi = src[si + 1]; si += 2
            p = ((hi >> 4) << 8) | lo
            length = (hi & 0x0F) + 3
            for _ in range(length):
                b = ring[p & (N - 1)]; p += 1
                out.append(b); ring[pos] = b; pos = (pos + 1) & (N - 1)
                if len(out) >= out_size:
                    break
    return bytes(out)
