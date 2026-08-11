"""
ngb.py — decode NGB (Nikita Graphics Bitmap) into an indexed image + mask.

Reverse-engineered from NGI32.DLL `ngiGetNgbPixel` (0xF7F0) and
`ngiGetNgbWidth/Height`. Two subtypes, distinguished by the signature dword.

Header (16 bytes):
  +0x00 int16 x0     bounding-box left   (origin X)
  +0x02 int16 y0     bounding-box top    (origin Y)
  +0x04 int16 x1     bounding-box right
  +0x06 int16 y1     bounding-box bottom
  +0x08 uint8 transparent   palette index treated as transparent
  +0x0C uint32 signature
        0x01F0CDAB -> subtype A: raw width*height indices at +0x10
        0x01EFCDAB -> subtype B: row-offset table (height u32 @+0x10) + RLE runs

width  = x1 - x0 + 1
height = y1 - y0 + 1

Subtype B row = sequence of runs, each: uint32 header then `count` pixel bytes.
  skip  = header & 0xFFFF      (transparent pixels before the run)
  count = header >> 16         (opaque pixels, bytes follow the header)
  end-of-row when (header & 0x8000) is set.
"""
from __future__ import annotations
import struct
import numpy as np

SIG_RAW = 0x01F0CDAB      # subtype A
SIG_RLE = 0x01EFCDAB      # subtype B


class Ngb:
    __slots__ = ("x0", "y0", "x1", "y1", "transparent", "sig",
                 "width", "height", "indices", "mask", "_d")

    def __init__(self, data: bytes):
        self._d = data
        self.x0, self.y0, self.x1, self.y1 = struct.unpack_from("<4h", data, 0)
        self.transparent = data[8]
        self.sig = struct.unpack_from("<I", data, 0x0C)[0]
        self.width = self.x1 - self.x0 + 1
        self.height = self.y1 - self.y0 + 1
        # indices: uint8 palette indices; mask: bool True=opaque
        self.indices = np.full((self.height, self.width), self.transparent, np.uint8)
        self.mask = np.zeros((self.height, self.width), bool)
        if self.sig == SIG_RAW:
            self._decode_raw()
        else:
            self._decode_rle()

    def _decode_raw(self):
        w, h = self.width, self.height
        body = np.frombuffer(self._d, np.uint8, w * h, 0x10).reshape(h, w)
        self.indices = body.copy()
        self.mask = body != self.transparent

    def _decode_rle(self):
        d = self._d
        n = len(d)
        w, h = self.width, self.height
        tbl_end = 0x10 + h * 4
        for y in range(h):
            p = struct.unpack_from("<I", d, 0x10 + y * 4)[0]
            x = 0
            while p + 4 <= n:
                header = struct.unpack_from("<I", d, p)[0]
                if header & 0x8000:            # end of row
                    break
                skip = header & 0xFFFF
                count = header >> 16
                x += skip
                if count:
                    if p + 4 + count > n or x + count > w:
                        break                  # bad offset / overrun -> stop row
                    seg = np.frombuffer(d, np.uint8, count, p + 4)
                    self.indices[y, x:x + count] = seg
                    self.mask[y, x:x + count] = True
                    x += count
                p += 4 + count

    def rgba(self, palette: np.ndarray) -> np.ndarray:
        """palette: (256,4) uint8 RGBA. Returns (h,w,4) uint8 with alpha from mask."""
        out = palette[self.indices]
        out = out.copy()
        out[..., 3] = np.where(self.mask, 255, 0)
        return out


def load_palette(col_bytes: bytes) -> np.ndarray:
    """COL = 256 entries of Windows RGBQUAD (B, G, R, reserved). Returns
    (256,4) uint8 RGBA with the channels put in R,G,B order and full alpha."""
    pal = np.frombuffer(col_bytes, np.uint8, 256 * 4).reshape(256, 4).copy()
    pal[:, [0, 2]] = pal[:, [2, 0]]     # BGR -> RGB
    pal[:, 3] = 255
    return pal
