"""
play.py — playable prototype of "Новый Робинзон" (New Robinson) on top of the
reverse-engineered NGI resources.

  python play.py                 # interactive window (SCENA0)
  python play.py SCENA5          # a different scene
  python play.py --verify        # headless: render a demo sequence to PNGs

Controls: left-click a spot to walk there; click an object to inspect it;
ESC quits.  Movement uses the isometric grid; walkable cells come from the
scene's grid + closed_vert list.
"""
import sys, os
import numpy as np

sys.path.insert(0, os.path.dirname(__file__))

HEADLESS = "--verify" in sys.argv
if HEADLESS:
    os.environ.setdefault("SDL_VIDEODRIVER", "dummy")
    os.environ.setdefault("SDL_AUDIODRIVER", "dummy")

import pygame
try:
    import pygame.font
    _HAVE_FONT = True
except Exception:
    _HAVE_FONT = False

_HAVE_MIXER = False
try:
    pygame.mixer.init(frequency=22050)
    _HAVE_MIXER = True
except Exception:
    _HAVE_MIXER = False


def _make_font(size=16):
    if not _HAVE_FONT:
        return None
    try:
        pygame.font.init()
        return pygame.font.SysFont("menlo", size)
    except Exception:
        return None


from engine.resources import Resources
from engine.ngb import Ngb, load_palette
from engine import scripts
from engine.grid import Grid

ROOT = os.path.join(os.path.dirname(__file__), "..",
                    "extracted", "ROBINSON_ISO", "ROBINSON")


# --------------------------------------------------------------------- assets
import math


def ngb_to_surface(ngb: Ngb, pal) -> pygame.Surface:
    rgba = ngb.rgba(pal)                      # (h,w,4)
    # frombuffer with RGBA already yields a per-pixel-alpha surface; no convert()
    # (convert_alpha needs a set video mode, which we may not have yet).
    return pygame.image.frombuffer(rgba.tobytes(), (ngb.width, ngb.height), "RGBA")


_NUMPAD_ANGLES = [(0, 6), (45, 9), (90, 8), (135, 7),
                  (180, 4), (-135, 1), (-90, 2), (-45, 3)]


def screen_to_numpad(dx, dy):
    """Map a screen-space movement vector to a numpad direction (1-9, no 5)."""
    ang = math.degrees(math.atan2(-dy, dx))   # 0=E, 90=N
    return min(_NUMPAD_ANGLES,
              key=lambda d: abs((ang - d[0] + 180) % 360 - 180))[1]


class Animation:
    """Movie animation for a character. Each frame is cropped to its OWN opaque
    bbox and given a per-frame bottom-center anchor (feet), so that no matter how
    the artwork drifts inside the movie canvas (the walk movies march the figure
    across the frame), the feet stay pinned to the grid point — the figure walks
    in place while our own grid logic moves it across the scene.

    Uses the movie's own palette unless one is passed explicitly.
    Frames are stored as (surface, (anchor_x, anchor_y))."""
    def __init__(self, res, movie_name, pal=None):
        mv = res.movie(movie_name)
        self.frames = []
        if not mv:
            return
        if pal is None:
            col = Resources.entry(mv, ".COL")
            pal = load_palette(mv.extract(col)) if col else None
        if pal is None:
            return
        for e in mv:
            if not e.name.upper().endswith(".NGB"):
                continue
            n = Ngb(mv.extract(e))
            rgba = n.rgba(pal)
            ys, xs = np.where(rgba[..., 3] > 0)
            if len(xs) == 0:
                continue
            x0, x1, y0, y1 = xs.min(), xs.max(), ys.min(), ys.max()
            crop = np.ascontiguousarray(rgba[y0:y1 + 1, x0:x1 + 1])
            w, h = x1 - x0 + 1, y1 - y0 + 1
            surf = pygame.image.frombuffer(crop.tobytes(), (w, h), "RGBA")
            # anchor: horizontal centre of the lowest 12 rows (the feet), bottom
            foot_band = crop[max(0, h - 12):, :, 3] > 0
            cols = np.where(foot_band.any(0))[0]
            ax = int((cols.min() + cols.max()) // 2) if len(cols) else w // 2
            self.frames.append((surf, (ax, h)))

    def __bool__(self):
        return bool(self.frames)


# --------------------------------------------------------------------- scene
class Hotspot:
    def __init__(self, key, ob, screen_xy):
        self.key = key
        self.ob = ob
        x, y, w, h = ob.active_zone
        # ActiveZone is relative to the object's grid screen point
        sx, sy = screen_xy
        self.rect = pygame.Rect(sx + x, sy + y, max(w, 8), max(h, 8))


class Game:
    def __init__(self, scene_name="SCENA0"):
        self.res = Resources(ROOT)
        self.scene_name = scene_name
        self._load_scene(scene_name)

    def load_scene(self, name, spawn=None):
        self.scene_name = name
        self._load_scene(name, spawn)

    def _load_scene(self, name, spawn=None):
        res = self.res
        bg_ngb, pal, fad = res.scene_background(name)
        self.pal = pal
        self.bg = ngb_to_surface(bg_ngb, pal)
        cont = res.scene_container(name)
        ent = {e.name.upper(): e for e in cont}
        self.scene = scripts.parse_scene(
            cont.extract(ent[name.upper() + ".SCN"]).decode("cp1251", "ignore"))
        self.W, self.H = self.scene.size if self.scene.size != (0, 0) else (bg_ngb.width, bg_ngb.height)
        self.grid = Grid(self.scene, bounds=(self.W, self.H))
        # objects + hotspots
        self.objects = {}
        for k, e in ent.items():
            if k.endswith(".OB"):
                ob = scripts.parse_object(cont.extract(e).decode("cp1251", "ignore"))
                self.objects[ob.name.lower()] = ob
        self.fscripts = {k[:-3].lower(): e for k, e in ent.items() if k.endswith(".FS")}
        # scene connections: scan *GOL/*GOR frame scripts for GoScene targets
        self.exit_left = self.exit_right = None
        import re as _re
        for k, e in ent.items():
            if not k.endswith(".FS"):
                continue
            base = k[:-3]
            if base.endswith("GOL") or base.endswith("GOR"):
                txt = cont.extract(e).decode("cp1251", "ignore")
                m = _re.search(r"GoScene\s+(\w+)\s*,.*?,\s*(-?\d+)\s*,\s*(-?\d+)\s*;",
                               txt, _re.S | _re.I)
                if m:
                    target = (m.group(1).upper(), int(m.group(2)), int(m.group(3)))
                    if base.endswith("GOL"):
                        self.exit_left = target
                    else:
                        self.exit_right = target
        self.pending = None            # (scene_name, spawn_cell) requested transition
        self.hotspots = []
        self.object_sprites = []      # (screen_xy, surface, z)
        for ref in self.scene.objects:
            ob = self.objects.get(ref.name.lower())
            if not ob:
                continue
            sxy = self.grid.to_screen(ref.gx, ref.gy)
            self.hotspots.append(Hotspot(ref.name, ob, sxy))
        # step sound (scene sound bank: name -> (wav, channel))
        self.step_sound = None
        if _HAVE_MIXER:
            wavname = None
            for key in ("step", "frstep"):
                if key in self.scene.sound_vars:
                    wavname = self.scene.sound_vars[key][0]; break
            if wavname:
                data = res.sound(wavname)
                if data:
                    try:
                        import io
                        self.step_sound = pygame.mixer.Sound(io.BytesIO(data))
                    except Exception:
                        self.step_sound = None
        # character animations: idle + 8-direction walk (own palettes)
        self.roby_idle = Animation(res, "Roby1.mv")
        self.walk_cache = {}
        self.cur_dir = 6
        if spawn is not None:
            self.start_cell = self.grid.nearest_free(*spawn)
        else:
            self.start_cell = self.grid.nearest_free(*self.grid.to_cell(self.W * 0.35, self.H * 0.62))
        self.roby_cell = list(self.start_cell)
        self.roby_pos = list(self.grid.to_screen(*self.start_cell))
        self.path = []
        self.anim_t = 0.0
        self.frame_i = 0
        self.facing = 1            # +1 right, -1 left
        self.moving = False
        self.message = ""
        self.message_t = 0.0

    def _walk_anim(self, numpad):
        a = self.walk_cache.get(numpad)
        if a is None:
            a = Animation(self.res, f"Rg_{numpad}{numpad}.mv")
            if not a:
                a = self.roby_idle
            self.walk_cache[numpad] = a
        return a

    def _object_sprite(self, cont, fon_script):
        e = self.fscripts.get(fon_script.lower())
        if not e:
            return None
        fs = scripts.parse_frame_script(cont.extract(e).decode("cp1251", "ignore"))
        mv = self.res.movie(fs.movie_name)
        if not mv:
            return None
        ngbs = [x for x in mv if x.name.upper().endswith(".NGB")]
        if not ngbs:
            return None
        spr = Ngb(mv.extract(ngbs[0]))
        surf = ngb_to_surface(spr, self.pal)
        return {"surf": surf, "shift": fs.shift, "x0": spr.x0, "y0": spr.y0}

    # -- interaction -------------------------------------------------------
    def click(self, mx, my):
        for hs in self.hotspots:
            if hs.rect.collidepoint(mx, my):
                key = hs.key.lower()
                if key == "goleft" and self.exit_left:
                    self.pending = (self.exit_left[0], self.exit_left[1:])
                    return
                if key == "gorght" and self.exit_right:
                    self.pending = (self.exit_right[0], self.exit_right[1:])
                    return
                self.message = f"{hs.ob.name}  (text #{hs.ob.text})"
                self.message_t = 3.0
                return
        # edge transitions: clicking far left/right also travels
        if mx < 40 and self.exit_left:
            self.pending = (self.exit_left[0], self.exit_left[1:]); return
        if mx > self.W - 40 and self.exit_right:
            self.pending = (self.exit_right[0], self.exit_right[1:]); return
        cell = self.grid.nearest_free(*self.grid.to_cell(mx, my))
        if cell:
            self.path = self.grid.path(tuple(self.roby_cell), cell)[1:]

    def update(self, dt):
        # walk along path
        self.moving = bool(self.path)
        if self.path:
            tx, ty = self.grid.to_screen(*self.path[0])
            dx, dy = tx - self.roby_pos[0], ty - self.roby_pos[1]
            if abs(dx) + abs(dy) > 1:
                self.cur_dir = screen_to_numpad(dx, dy)
            dist = (dx * dx + dy * dy) ** 0.5
            speed = 220 * dt
            if dist <= speed or dist == 0:
                self.roby_pos = [tx, ty]
                self.roby_cell = list(self.path.pop(0))
                if self.step_sound is not None:
                    try:
                        self.step_sound.play()
                    except Exception:
                        pass
            else:
                self.roby_pos[0] += dx / dist * speed
                self.roby_pos[1] += dy / dist * speed
        # advance animation frame
        anim = self._walk_anim(self.cur_dir) if self.moving else self.roby_idle
        rate = 0.07 if self.moving else 0.09
        self.anim_t += dt
        if anim and anim.frames:
            if self.anim_t >= rate:
                self.anim_t = 0.0
                self.frame_i = (self.frame_i + 1) % len(anim.frames)
        else:
            self.frame_i = 0
        if self.message_t > 0:
            self.message_t -= dt
            if self.message_t <= 0:
                self.message = ""

    # -- rendering ---------------------------------------------------------
    def draw(self, surf, font=None, show_hotspots=False):
        surf.blit(self.bg, (0, 0))
        # Static decorations (stones, trees, plants) are already painted into
        # the scene background; object entries are hotspots + animated overlays,
        # so we don't re-blit their sprites here.
        if show_hotspots:
            for hs in self.hotspots:
                pygame.draw.rect(surf, (255, 255, 0), hs.rect, 1)
        # character: 8-direction walk while moving, idle otherwise (+ shadow)
        anim = self._walk_anim(self.cur_dir) if self.moving else self.roby_idle
        if anim and anim.frames:
            frame, (ax, ay) = anim.frames[self.frame_i % len(anim.frames)]
            px, py = int(self.roby_pos[0]), int(self.roby_pos[1])
            shadow = pygame.Surface((40, 12), pygame.SRCALPHA)
            pygame.draw.ellipse(shadow, (0, 0, 0, 90), shadow.get_rect())
            surf.blit(shadow, (px - 20, py - 8))
            surf.blit(frame, (px - ax, py - ay))
        # hud
        if font:
            if self.message:
                self._text(surf, font, self.message, 12, 10)
            self._text(surf, font, f"cell {tuple(self.roby_cell)}", 12, self.H - 22)

    @staticmethod
    def _text(surf, font, s, x, y):
        img = font.render(s, True, (255, 255, 200))
        bg = pygame.Surface((img.get_width() + 8, img.get_height() + 4), pygame.SRCALPHA)
        bg.fill((0, 0, 0, 150))
        surf.blit(bg, (x - 4, y - 2)); surf.blit(img, (x, y))


# --------------------------------------------------------------------- loops
def run_interactive(scene="SCENA0"):
    pygame.init()
    g = Game(scene)
    screen = pygame.display.set_mode((g.W, g.H))
    pygame.display.set_caption("Новый Робинзон — remake prototype")
    font = _make_font(16)
    clock = pygame.time.Clock()
    running = True
    while running:
        dt = clock.tick(60) / 1000.0
        for e in pygame.event.get():
            if e.type == pygame.QUIT:
                running = False
            elif e.type == pygame.KEYDOWN and e.key == pygame.K_ESCAPE:
                running = False
            elif e.type == pygame.MOUSEBUTTONDOWN and e.button == 1:
                g.click(*e.pos)
        g.update(dt)
        if g.pending:
            name, cell = g.pending; g.pending = None
            g.load_scene(name, cell)
            if (g.W, g.H) != screen.get_size():
                screen = pygame.display.set_mode((g.W, g.H))
        g.draw(screen, font)
        pygame.display.flip()
    pygame.quit()


def _save_png(surf, path):
    from PIL import Image
    raw = pygame.image.tostring(surf, "RGB")
    Image.frombytes("RGB", surf.get_size(), raw).save(path)


def _surf_to_pil(surf):
    from PIL import Image
    return Image.frombytes("RGB", surf.get_size(), pygame.image.tostring(surf, "RGB"))


def run_verify(scene="SCENA0"):
    pygame.init()
    g = Game(scene)
    surf = pygame.Surface((g.W, g.H))
    font = _make_font(16)
    out = "/tmp/robigame"
    os.makedirs(out, exist_ok=True)
    frames = []

    def snap():
        g.draw(surf, font)
        frames.append(_surf_to_pil(surf))

    def walk_to(dest, maxs=400):
        cell = g.grid.nearest_free(*dest)
        tx, ty = g.grid.to_screen(*cell)
        g.click(int(tx), int(ty))
        s = 0
        while g.path and s < maxs:
            g.update(1 / 60)
            if s % 3 == 0:
                snap()
            s += 1
        for _ in range(6):
            g.update(1 / 60); snap()

    g.draw(surf, font); _save_png(surf, f"{out}/00_start.png")
    print(f"SCENA0 exits: left={g.exit_left} right={g.exit_right}")
    # tour scene 0, then travel right into the connected scene
    for dest in [(5, 1), (2, 0), (6, 0)]:
        walk_to(dest)
    _save_png(surf, f"{out}/01_walked.png")
    # trigger a scene transition to the right
    if g.exit_right:
        g.pending = (g.exit_right[0], g.exit_right[1:])
        name, cell = g.pending; g.pending = None
        g.load_scene(name, cell)
        surf = pygame.Surface((g.W, g.H))
        print(f"transitioned to {name} spawn {cell}: {g.W}x{g.H}")
        g.draw(surf, font); _save_png(surf, f"{out}/03_scene_{name}.png")
        for dest in [(3, 0), (1, 1)]:
            walk_to(dest)
    # click a hotspot
    for hs in g.hotspots:
        if 0 < hs.rect.centerx < g.W and 0 < hs.rect.centery < g.H:
            g.click(hs.rect.centerx, hs.rect.centery)
            g.update(0.1); g.draw(surf, font); _save_png(surf, f"{out}/02_object.png")
            print("clicked hotspot:", hs.ob.name, "->", g.message)
            break
    if frames:
        frames[0].save(f"{out}/walk.gif", save_all=True, append_images=frames[1:],
                       duration=60, loop=0)
    print(f"verify OK: {len(frames)} frames -> {out}/walk.gif")
    pygame.quit()


if __name__ == "__main__":
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    scene = args[0] if args else "SCENA0"
    if HEADLESS:
        run_verify(scene)
    else:
        run_interactive(scene)
