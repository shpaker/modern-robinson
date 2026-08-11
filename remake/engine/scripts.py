"""
scripts.py — parsers for NGI text scripts: .SCN (scene), .OB (object),
.FS (frame script), .CHR (character).

Grammar: statements terminated by ';'. The first token of a statement is a
keyword; list-type keywords (ObjectList, SoundVariables, ...) are followed by
continuation statements (no keyword) until the next keyword.
"""
from __future__ import annotations
import re
from dataclasses import dataclass, field


# ------------------------------------------------------------------ tokenizer
_KEYWORDS = {
    # scene
    "scenename", "screenname", "barname", "screensize", "scrollpar", "scrolldesc",
    "scrolldescret", "lefttopgrid", "gridsize", "gridlength", "gridshift",
    "zpergrid", "closedvert", "closeddir", "objectlist", "soundvariables",
    "textvariables", "intvariables", "charvariables", "music", "griddebug",
    "gridtoclose",
    # object
    "objectname", "fonscript", "zcoord", "activezone", "cursor", "text", "mousez",
    # frame script
    "scriptname", "moviename", "shift", "totalframes", "frame", "delay", "sound",
    "setrest", "set", "setvar", "setcharvar", "if", "endif", "goscene", "additem",
    "deleteitem", "createobject", "delobject", "aproach", "approach", "setvert",
    "shiftscreen", "setmouse", "hidechar", "showchar", "map", "mouse", "setbar",
    "setactive", "end", "delayfactor",
}


def statements(text: str):
    """Yield (keyword, args_str, is_continuation) tuples.

    Line-oriented: a line whose first token is a known keyword starts a new
    statement (terminating ';' is optional in the data). Any other non-empty
    line is a continuation of the current keyword (list items, extra rows).
    A single line may pack several ';'-separated sub-statements.
    """
    cur_kw = None
    for line in text.splitlines():
        for chunk in line.split(";"):
            s = chunk.strip()
            if not s:
                continue
            m = re.match(r"([A-Za-z_]\w*)(.*)", s, re.S)
            head = m.group(1).lower() if m else ""
            if m and head in _KEYWORDS:
                cur_kw = head
                yield (head, m.group(2).strip(), False)
            elif cur_kw:
                yield (cur_kw, s, True)


def _ints(s):
    return [int(x) for x in re.findall(r"-?\d+", s)]


# ------------------------------------------------------------------ models
@dataclass
class ObjectRef:
    name: str
    gx: int
    gy: int
    flag: bool = False


@dataclass
class Scene:
    name: str = ""
    screen: str = ""
    bar: str = ""
    size: tuple = (0, 0)
    scroll_par: tuple = (0, 0)
    left_top_grid: tuple = (0, 0)
    grid_size: tuple = (0, 0)
    grid_length: tuple = (0, 0)
    grid_shift: tuple = (0, 0)
    z_per_grid: int = 0
    closed_vert: list = field(default_factory=list)   # [(gx,gy)]
    closed_dir: list = field(default_factory=list)     # [(gx,gy,dir)]
    objects: list = field(default_factory=list)        # [ObjectRef]
    sound_vars: dict = field(default_factory=dict)     # name -> (wav, ch)
    text_vars: dict = field(default_factory=dict)
    music: str = ""


def parse_scene(text: str) -> Scene:
    sc = Scene()
    for kw, args, cont in statements(text):
        if kw == "scenename":
            sc.name = args.strip()
        elif kw == "screenname":
            sc.screen = args.strip()
        elif kw == "barname":
            sc.bar = args.strip()
        elif kw == "screensize":
            v = _ints(args); sc.size = (v[0], v[1]) if len(v) >= 2 else sc.size
        elif kw == "scrollpar":
            v = _ints(args); sc.scroll_par = tuple(v[:2])
        elif kw == "lefttopgrid":
            v = _ints(args); sc.left_top_grid = tuple(v[:2])
        elif kw == "gridsize":
            v = _ints(args); sc.grid_size = tuple(v[:2])
        elif kw == "gridlength":
            v = _ints(args); sc.grid_length = tuple(v[:2])
        elif kw == "gridshift":
            v = _ints(args); sc.grid_shift = tuple(v[:2])
        elif kw == "zpergrid":
            v = _ints(args); sc.z_per_grid = v[0] if v else 0
        elif kw == "closedvert":
            v = _ints(args)
            for i in range(0, len(v) - 1, 2):
                sc.closed_vert.append((v[i], v[i + 1]))
        elif kw == "closeddir":
            v = _ints(args)
            for i in range(0, len(v) - 2, 3):
                sc.closed_dir.append((v[i], v[i + 1], v[i + 2]))
        elif kw == "objectlist":
            parts = args.split(",")
            name = parts[0].strip()
            nums = _ints(args)
            if name and nums:
                gx, gy = (nums + [0, 0])[:2]
                sc.objects.append(ObjectRef(name, gx, gy, "*" in args))
        elif kw == "soundvariables":
            m = re.match(r'(\w+)\s*,\s*"([^"]+)"\s*,?\s*(\d*)', args)
            if m:
                sc.sound_vars[m.group(1)] = (m.group(2), int(m.group(3) or 0))
        elif kw == "textvariables":
            m = re.match(r'(\w+)\s*,\s*(.*)', args)
            if m:
                sc.text_vars[m.group(1)] = m.group(2).strip().strip('"')
        elif kw == "music":
            sc.music = args.strip().strip('"')
    return sc


@dataclass
class SceneObject:
    name: str = ""
    fon_script: str = ""
    z: int = 0
    active_zone: tuple = (0, 0, 0, 0)   # x,y,w,h
    cursor: int = 0
    text: int = 0
    closed_vert: list = field(default_factory=list)
    closed_dir: list = field(default_factory=list)


def parse_object(text: str) -> SceneObject:
    ob = SceneObject()
    for kw, args, cont in statements(text):
        if kw == "objectname":
            ob.name = args.strip()
        elif kw == "fonscript":
            ob.fon_script = args.strip()
        elif kw == "zcoord":
            v = _ints(args); ob.z = v[0] if v else 0
        elif kw == "activezone":
            v = _ints(args)
            if len(v) >= 4:
                ob.active_zone = tuple(v[:4])
        elif kw == "cursor":
            v = _ints(args); ob.cursor = v[0] if v else 0
        elif kw == "text":
            v = _ints(args); ob.text = v[0] if v else 0
    return ob


@dataclass
class Frame:
    index: int = 0
    sub: int = 0
    delay: int = 0
    sounds: list = field(default_factory=list)   # [(wav, ch, at)]
    texts: list = field(default_factory=list)    # [(id, flag)]
    commands: list = field(default_factory=list)  # [(kw, args)]


@dataclass
class FrameScript:
    script_name: str = ""
    movie_name: str = ""
    shift: tuple = (0, 0)
    total_frames: int = 0
    frames: list = field(default_factory=list)


def parse_frame_script(text: str) -> FrameScript:
    fs = FrameScript()
    cur = None
    for kw, args, cont in statements(text):
        if kw == "scriptname":
            fs.script_name = args.strip()
        elif kw == "moviename":
            fs.movie_name = args.strip().strip('"')
        elif kw == "shift":
            v = _ints(args); fs.shift = tuple(v[:2]) if len(v) >= 2 else (0, 0)
        elif kw == "totalframes":
            v = _ints(args); fs.total_frames = v[0] if v else 0
        elif kw == "frame":
            v = _ints(args)
            cur = Frame(index=v[0] if v else 0, sub=v[1] if len(v) > 1 else 0)
            fs.frames.append(cur)
        elif kw == "delay":
            if cur is not None:
                v = _ints(args); cur.delay = v[0] if v else 0
        elif kw == "sound":
            if cur is not None:
                m = re.match(r'"([^"]+)"\s*,?\s*(-?\d*)\s*,?\s*(-?\d*)', args)
                if m:
                    cur.sounds.append((m.group(1), int(m.group(2) or 0),
                                       int(m.group(3) or 0)))
        elif kw == "text":
            if cur is not None:
                v = _ints(args); cur.texts.append(tuple((v + [0, 0])[:2]))
        elif cur is not None:
            cur.commands.append((kw, args.strip()))
    return fs
