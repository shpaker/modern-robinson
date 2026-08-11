"""
resources.py — resource index for the whole game.

The game refers to assets by short name (movie "bgstone.mv", sound "rr447.wav",
scene "scena0"). This builds one index over the extracted disc:
  * every NL container found under the game root (.DAN / .DAT / .MV);
  * a name->Container map so movies can be opened by name;
  * WAVE.DAN as the sound bank (name.wav -> bytes).
Results are cached; containers are parsed lazily.
"""
from __future__ import annotations
import os
import functools

from .ngi import Container
from .ngb import Ngb, load_palette


class Resources:
    def __init__(self, root: str):
        self.root = root
        self._by_path: dict[str, Container] = {}
        self.movies: dict[str, str] = {}     # "BGSTONE.MV" -> abspath
        self.scene_dirs: dict[str, str] = {}  # "SCENA0" -> abspath of SCENA0.DAN
        self.scene_dat: dict[str, str] = {}   # "SCENA0" -> abspath of SCENA0.DAT
        self._index()
        self._wave = None

    # -- indexing ----------------------------------------------------------
    def _index(self):
        for dp, _, fns in os.walk(self.root):
            for fn in fns:
                up = fn.upper()
                p = os.path.join(dp, fn)
                if up.endswith(".MV"):
                    self.movies[up] = p
                elif up.endswith(".DAN") and up.startswith("SCENA"):
                    self.scene_dirs[up[:-4]] = p
                elif up.endswith(".DAT") and up.startswith("SCENA"):
                    self.scene_dat[up[:-4]] = p

    # -- container access --------------------------------------------------
    def container(self, path: str) -> Container:
        c = self._by_path.get(path)
        if c is None:
            c = Container.open(path)
            self._by_path[path] = c
        return c

    def movie(self, name: str) -> Container | None:
        p = self.movies.get(name.upper())
        return self.container(p) if p else None

    # -- sound bank --------------------------------------------------------
    @property
    def wave(self) -> Container:
        if self._wave is None:
            self._wave = self.container(os.path.join(self.root, "DATA", "WAVE", "WAVE.DAN"))
        return self._wave

    @functools.cached_property
    def _wave_index(self) -> dict[str, object]:
        return {e.name.upper(): e for e in self.wave}

    def sound(self, name: str) -> bytes | None:
        e = self._wave_index.get(name.upper())
        return self.wave.extract(e) if e else None

    # -- scene assets ------------------------------------------------------
    def scene_container(self, scene: str) -> Container:
        return self.container(self.scene_dirs[scene.upper()])

    def scene_background(self, scene: str):
        """Return (Ngb background, palette (256,4), fade-or-None)."""
        p = self.scene_dat.get(scene.upper())
        if not p:
            return None, None, None
        c = self.container(p)
        by = {e.name.upper(): e for e in c}
        ngb = pal = fad = None
        for k, e in by.items():
            if k.endswith(".NGB"):
                ngb = Ngb(c.extract(e))
            elif k.endswith(".COL"):
                pal = load_palette(c.extract(e))
            elif k.endswith(".FAD"):
                fad = c.extract(e)
        return ngb, pal, fad

    # -- generic entry lookup inside a container by extension --------------
    @staticmethod
    def entry(container: Container, ext: str):
        ext = ext.upper()
        for e in container:
            if e.name.upper().endswith(ext):
                return e
        return None
