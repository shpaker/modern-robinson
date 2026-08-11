#!/usr/bin/env python3
"""
ngiunpack.py — list / extract NGI resource containers.

Usage:
    ngiunpack.py list   <file.dan|.dat|.mv>
    ngiunpack.py info   <file>                 # header + stats
    ngiunpack.py extract <file> <outdir> [glob] # decode entries (skips undecoded)
    ngiunpack.py scan   <root>                  # inventory every NL file under root
"""
import sys, os, struct, fnmatch, json
from ngi import Container


def cmd_info(path):
    c = Container.open(path)
    print(f"{path}")
    print(f"  version   {c.version:#06x}")
    print(f"  entries   {c.count}")
    print(f"  flag      {c.flag:#06x}   marker {c.marker:#06x}"
          f"  {'(ABBA/encrypted-dir)' if c.marker == 0xABBA else ''}")
    print(f"  usize_tot {c.usize_total:,}")
    print(f"  csize_tot {c.csize_total:,}  (= cipher key {c.key:#010x})")
    methods = {}
    for e in c:
        methods[e.method] = methods.get(e.method, 0) + 1
    print(f"  methods   " + ", ".join(f"{m:#06x}:{n}" for m, n in sorted(methods.items())))


def cmd_list(path):
    c = Container.open(path)
    print(f"# {path}  ({c.count} entries)")
    print(f"{'name':16} {'method':>7} {'usize':>10} {'csize':>10} {'offset':>10}")
    for e in c:
        print(f"{e.name:16} {e.method:#07x} {e.usize:>10} {e.csize:>10} {e.offset:>10}")


def cmd_extract(path, outdir, pattern="*"):
    c = Container.open(path)
    os.makedirs(outdir, exist_ok=True)
    ok = skip = 0
    for e in c:
        if not fnmatch.fnmatch(e.name.upper(), pattern.upper()):
            continue
        try:
            data = c.extract(e)
        except NotImplementedError:
            skip += 1
            continue
        with open(os.path.join(outdir, e.name), "wb") as f:
            f.write(data)
        ok += 1
    print(f"{path}: extracted {ok}, skipped {skip} (undecoded codec)")


def cmd_scan(root):
    rows = []
    for dp, _, fns in os.walk(root):
        for fn in fns:
            p = os.path.join(dp, fn)
            try:
                with open(p, "rb") as f:
                    head = f.read(6)
                if head[:2] != b"NL":
                    continue
                c = Container.open(p)
            except Exception:
                continue
            exts = {}
            for e in c:
                ext = e.name.rsplit(".", 1)[-1].upper() if "." in e.name else "?"
                exts[ext] = exts.get(ext, 0) + 1
            rows.append(dict(path=os.path.relpath(p, root), entries=c.count,
                             flag=c.flag, marker=c.marker, contents=exts))
    rows.sort(key=lambda r: -r["entries"])
    print(json.dumps(rows, ensure_ascii=False, indent=2))


def main():
    if len(sys.argv) < 2:
        print(__doc__); return 1
    cmd = sys.argv[1]
    a = sys.argv[2:]
    if cmd == "info": cmd_info(*a)
    elif cmd == "list": cmd_list(*a)
    elif cmd == "extract": cmd_extract(*a)
    elif cmd == "scan": cmd_scan(*a)
    else:
        print(__doc__); return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
