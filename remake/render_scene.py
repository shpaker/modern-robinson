"""
render_scene.py — headless scene compositor (no window). Loads a scene, draws
the background and every object's current movie frame, writes a PNG.
Used to verify rendering without a display.

Usage: python render_scene.py SCENA0 out.png [posmode]
  posmode: shift | grid | bbox  (how object sprites are placed; default shift)
"""
import sys, os
import numpy as np

sys.path.insert(0, os.path.dirname(__file__))
from engine.resources import Resources
from engine.ngb import Ngb, load_palette
from engine import scripts

ROOT = os.path.join(os.path.dirname(__file__), "..",
                    "extracted", "ROBINSON_ISO", "ROBINSON")


def blit(dst, src, ox, oy):
    """Alpha-blit src (h,w,4) onto dst (H,W,4) at (ox,oy) with clipping."""
    H, W = dst.shape[:2]
    h, w = src.shape[:2]
    x0, y0 = max(0, ox), max(0, oy)
    x1, y1 = min(W, ox + w), min(H, oy + h)
    if x0 >= x1 or y0 >= y1:
        return
    sx0, sy0 = x0 - ox, y0 - oy
    sub = src[sy0:sy0 + (y1 - y0), sx0:sx0 + (x1 - x0)]
    a = (sub[..., 3:4].astype(np.uint16))
    reg = dst[y0:y1, x0:x1]
    reg[..., :3] = ((sub[..., :3].astype(np.uint16) * a +
                     reg[..., :3].astype(np.uint16) * (255 - a)) // 255).astype(np.uint8)
    reg[..., 3] = np.maximum(reg[..., 3], sub[..., 3])


def grid_to_screen(sc, gx, gy):
    lx, ly = sc.left_top_grid
    gsx, gsy = sc.grid_size
    shx, shy = sc.grid_shift
    # isometric basis: gx moves along grid_size, gy along grid_shift
    x = lx + gx * gsx + gy * shx
    y = ly + gx * gsy + gy * shy
    return x, y


def load_scene(res, name):
    cont = res.scene_container(name)
    entries = {e.name.upper(): e for e in cont}
    scn = scripts.parse_scene(cont.extract(entries[name.upper() + ".SCN"]).decode("cp1251", "ignore"))
    objects = {}
    for k, e in entries.items():
        if k.endswith(".OB"):
            ob = scripts.parse_object(cont.extract(e).decode("cp1251", "ignore"))
            objects[ob.name.lower()] = ob
    fscripts = {}
    for k, e in entries.items():
        if k.endswith(".FS"):
            fscripts[k[:-3].lower()] = e
    return cont, scn, objects, fscripts


def object_sprite(res, cont, fscripts, fon_script, scene_pal):
    e = fscripts.get(fon_script.lower())
    if not e:
        return None
    fs = scripts.parse_frame_script(cont.extract(e).decode("cp1251", "ignore"))
    mv = res.movie(fs.movie_name)
    if not mv:
        return None
    ngbs = [x for x in mv if x.name.upper().endswith(".NGB")]
    if not ngbs:
        return None
    spr = Ngb(mv.extract(ngbs[0]))
    return fs, spr


def main():
    scene = sys.argv[1] if len(sys.argv) > 1 else "SCENA0"
    out = sys.argv[2] if len(sys.argv) > 2 else "/tmp/scene.png"
    posmode = sys.argv[3] if len(sys.argv) > 3 else "shift"
    res = Resources(ROOT)
    bg_ngb, pal, fad = res.scene_background(scene)
    cont, scn, objects, fscripts = load_scene(res, scene)
    W, H = scn.size if scn.size != (0, 0) else (bg_ngb.width, bg_ngb.height)
    canvas = np.zeros((H, W, 4), np.uint8)
    canvas[..., 3] = 255
    canvas[:bg_ngb.height, :bg_ngb.width, :3] = bg_ngb.rgba(pal)[..., :3]

    placed = 0
    for ref in scn.objects:
        ob = objects.get(ref.name.lower())
        if not ob:
            continue
        r = object_sprite(res, cont, fscripts, ob.fon_script or ref.name, pal)
        if not r:
            continue
        fs, spr = r
        rgba = spr.rgba(pal)
        if posmode == "shift":
            ox, oy = fs.shift[0] + spr.x0, fs.shift[1] + spr.y0
        elif posmode == "grid":
            gx, gy = grid_to_screen(scn, ref.gx, ref.gy)
            ox, oy = gx + fs.shift[0], gy + fs.shift[1]
        else:  # bbox
            ox, oy = spr.x0, spr.y0
        blit(canvas, rgba, ox, oy)
        placed += 1

    from PIL import Image
    Image.fromarray(canvas[..., :3], "RGB").save(out)
    print(f"{scene}: {W}x{H}, {len(scn.objects)} objects, {placed} placed -> {out}")
    print("grid:", scn.left_top_grid, scn.grid_size, scn.grid_shift, scn.grid_length)
    for ref in scn.objects[:12]:
        print(f"  obj {ref.name} grid=({ref.gx},{ref.gy})")


if __name__ == "__main__":
    main()
