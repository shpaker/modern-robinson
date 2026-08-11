"""
grid.py — isometric walk grid: screen<->cell mapping and path-finding.

Screen position of cell (gx,gy) (verified against SCENA0 background):
    x = left_top.x + gx*grid_size.x + gy*grid_shift.x
    y = left_top.y + gx*grid_size.y + gy*grid_shift.y
Movement is 8-directional (character MoveType = NumPadGoing). Cells in the
scene's closed_vert list are blocked.
"""
from __future__ import annotations
from collections import deque


class Grid:
    def __init__(self, scene, bounds=None):
        self.lx, self.ly = scene.left_top_grid
        self.sx, self.sy = scene.grid_size
        self.hx, self.hy = scene.grid_shift
        self.nx, self.ny = scene.grid_length
        self.blocked = set(scene.closed_vert)
        self.bounds = bounds or scene.size or (10 ** 9, 10 ** 9)
        # determinant for the inverse mapping
        self._det = self.sx * self.hy - self.hx * self.sy or 1

    # -- mapping -----------------------------------------------------------
    def to_screen(self, gx, gy):
        return (self.lx + gx * self.sx + gy * self.hx,
                self.ly + gx * self.sy + gy * self.hy)

    def to_cell(self, px, py):
        dx, dy = px - self.lx, py - self.ly
        gx = (dx * self.hy - self.hx * dy) / self._det
        gy = (self.sx * dy - dx * self.sy) / self._det
        return round(gx), round(gy)

    # -- walkability -------------------------------------------------------
    def valid(self, gx, gy):
        if not (0 <= gx < self.nx and 0 <= gy < self.ny):
            return False
        if (gx, gy) in self.blocked:
            return False
        # must map inside the scene bounds (cells above/below the painted
        # area are not real walk cells)
        x, y = self.to_screen(gx, gy)
        W, H = self.bounds
        return 0 <= x < W and 0 <= y < H

    def nearest_free(self, gx, gy):
        gx = max(0, min(self.nx - 1, gx))
        gy = max(0, min(self.ny - 1, gy))
        if self.valid(gx, gy):
            return gx, gy
        best, bd = None, 1e9
        for y in range(self.ny):
            for x in range(self.nx):
                if self.valid(x, y):
                    d = (x - gx) ** 2 + (y - gy) ** 2
                    if d < bd:
                        bd, best = d, (x, y)
        return best

    # -- path-finding (BFS, 8-connected) -----------------------------------
    _DIRS = [(-1, -1), (0, -1), (1, -1), (-1, 0), (1, 0), (-1, 1), (0, 1), (1, 1)]

    def path(self, start, goal):
        if start == goal or not self.valid(*goal):
            return [goal] if self.valid(*goal) else []
        prev = {start: None}
        q = deque([start])
        while q:
            cur = q.popleft()
            if cur == goal:
                break
            for dx, dy in self._DIRS:
                nb = (cur[0] + dx, cur[1] + dy)
                if nb not in prev and self.valid(*nb):
                    prev[nb] = cur
                    q.append(nb)
        if goal not in prev:
            return []
        out = []
        c = goal
        while c is not None:
            out.append(c)
            c = prev[c]
        return out[::-1]
