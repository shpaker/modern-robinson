#!/usr/bin/env python3
"""
script_vocab.py — enumerate the scripting-command vocabulary of *Новый Робинзон*.

Scans every NL container under the game tree, extracts all .FS/.OB/.SCN text
entries (CP1251), tokenizes line-by-line taking the first identifier token as the
command keyword (case-insensitive), and aggregates frequency / container-spread /
argument-count stats / verbatim examples.

Pure stdlib + the local ngi toolkit. Run from tools/:  python3 script_vocab.py
"""
from __future__ import annotations
import os, re, json, sys
from collections import defaultdict, Counter
from ngi import Container

ROOT = os.environ.get(
    "ROBINSON_ROOT",
    os.path.join(
        os.path.dirname(os.path.abspath(__file__)),
        "..", "extracted", "ROBINSON_ISO", "ROBINSON",
    ),
)
SCRIPT_EXTS = ("FS", "OB", "SCN")

# leading identifier = command keyword. Keywords/fields ALWAYS start at column 0;
# indented lines are list-record continuations (ObjectList/SoundVariables/ClosedVert).
KW_RE = re.compile(r"^([A-Za-z][A-Za-z0-9_]*)")


def iter_containers(root):
    for dp, _, fns in os.walk(root):
        for fn in sorted(fns):
            p = os.path.join(dp, fn)
            try:
                with open(p, "rb") as f:
                    if f.read(2) != b"NL":
                        continue
                c = Container.open(p)
            except Exception:
                continue
            yield os.path.relpath(p, root), c


def split_args(remainder: str):
    """Args of the primary statement: text after keyword up to first ';', split on ','."""
    stmt = remainder.split(";", 1)[0]
    stmt = stmt.strip()
    if not stmt:
        return []
    # split on commas but keep quoted strings intact
    parts, cur, q = [], "", False
    for ch in stmt:
        if ch == '"':
            q = not q
            cur += ch
        elif ch == "," and not q:
            parts.append(cur.strip())
            cur = ""
        else:
            cur += ch
    if cur.strip():
        parts.append(cur.strip())
    return [p for p in parts if p != ""]


def main():
    # keyword -> stats
    freq = Counter()
    containers_of = defaultdict(set)     # kw -> set(container)
    entryexts_of = defaultdict(Counter)  # kw -> Counter(entry-ext)
    argcounts = defaultdict(Counter)     # kw -> Counter(argcount)
    examples = defaultdict(list)         # kw -> list of (container, entry, line)
    casings = defaultdict(Counter)       # kw_lower -> Counter(original casing)
    # for quest deep-dive: keep MANY raw arg tuples
    quest_raw = defaultdict(list)        # kw_lower -> list of (container, entry, args)

    QUEST = {  # keywords to hoard extra detail for
        "set", "setvar", "setcharvar", "addvar", "if", "endif", "else",
        "createobject", "delobject", "deleteobject", "addobject",
        "additem", "deleteitem", "item", "bar", "setbar", "lockbar",
        "goscene", "exit", "startgame", "endgame", "setvert",
        "hidechar", "showchar", "setactive", "setrest", "aproach",
    }

    n_containers = 0
    n_entries = 0
    per_ext_lines = Counter()
    # schema field order/counts (col-0 keywords) per script type
    scn_fields = Counter()
    ob_fields = Counter()
    scn_field_order = []   # first-seen order from a representative complete scene
    ob_field_order = []
    scn_field_args = defaultdict(Counter)  # field -> Counter(argcount)
    ob_field_args = defaultdict(Counter)

    for relpath, c in iter_containers(ROOT):
        touched = False
        for e in c:
            ext = e.name.rsplit(".", 1)[-1].upper() if "." in e.name else "?"
            if ext not in SCRIPT_EXTS:
                continue
            try:
                txt = c.extract(e).decode("cp1251", errors="replace")
            except Exception:
                continue
            touched = True
            n_entries += 1
            for raw in txt.splitlines():
                line = raw.rstrip("\n\r")
                if not line.strip():
                    continue
                # keyword only if line begins at column 0 (unindented)
                m = KW_RE.match(line)
                if not m:
                    # indented continuation, or data starting with digit/quote/*/-
                    per_ext_lines[("(continuation/data)", ext)] += 1
                    continue
                s = line.strip()
                kw = m.group(1)
                kwl = kw.lower()
                remainder = s[m.end():]
                args = split_args(remainder)
                freq[kwl] += 1
                containers_of[kwl].add(relpath)
                entryexts_of[kwl][ext] += 1
                argcounts[kwl][len(args)] += 1
                casings[kwl][kw] += 1
                per_ext_lines[("cmd", ext)] += 1
                if ext == "SCN":
                    scn_fields[kw] += 1
                    scn_field_args[kw][len(args)] += 1
                    if kw not in scn_field_order:
                        scn_field_order.append(kw)
                elif ext == "OB":
                    ob_fields[kw] += 1
                    ob_field_args[kw][len(args)] += 1
                    if kw not in ob_field_order:
                        ob_field_order.append(kw)
                if len(examples[kwl]) < 6:
                    examples[kwl].append((relpath, e.name, s))
                if kwl in QUEST and len(quest_raw[kwl]) < 400:
                    quest_raw[kwl].append((relpath, e.name, args, s))
        if touched:
            n_containers += 1

    out = {
        "root": ROOT,
        "n_containers": n_containers,
        "n_script_entries": n_entries,
        "per_ext_lines": {f"{k[0]}|{k[1]}": v for k, v in per_ext_lines.items()},
        "keywords": {},
        "quest_raw": {k: v for k, v in quest_raw.items()},
        "scn_schema": [
            {"field": f, "count": scn_fields[f],
             "argcount_dist": dict(sorted(scn_field_args[f].items()))}
            for f in scn_field_order
        ],
        "ob_schema": [
            {"field": f, "count": ob_fields[f],
             "argcount_dist": dict(sorted(ob_field_args[f].items()))}
            for f in ob_field_order
        ],
    }
    for kwl in freq:
        acs = argcounts[kwl]
        out["keywords"][kwl] = {
            "canonical": casings[kwl].most_common(1)[0][0],
            "casings": dict(casings[kwl]),
            "freq": freq[kwl],
            "n_containers": len(containers_of[kwl]),
            "containers": sorted(containers_of[kwl]),
            "entry_exts": dict(entryexts_of[kwl]),
            "argcount_min": min(acs),
            "argcount_max": max(acs),
            "argcount_dist": dict(sorted(acs.items())),
            "argcount_mode": acs.most_common(1)[0][0],
            "examples": examples[kwl],
        }

    with open("/private/tmp/claude-501/-Users-als-github-nrobinson/ace9e422-299e-44f9-8869-1f39241ced7a/scratchpad/vocab.json", "w") as f:
        json.dump(out, f, ensure_ascii=False, indent=1)

    # ---- console: catalog table sorted by freq ----
    print(f"containers with scripts: {n_containers}   script entries: {n_entries}")
    print(f"{'keyword':18} {'freq':>7} {'#cont':>6} {'args':>9} {'exts':>16}  example")
    for kwl, d in sorted(out["keywords"].items(), key=lambda kv: -kv[1]["freq"]):
        arng = f"{d['argcount_min']}-{d['argcount_max']}"
        ex = d["examples"][0][2] if d["examples"] else ""
        exts = ",".join(f"{k}:{v}" for k, v in d["entry_exts"].items())
        print(f"{d['canonical']:18} {d['freq']:>7} {d['n_containers']:>6} {arng:>9} {exts:>16}  {ex[:52]}")
    print()
    print("distinct keywords:", len(out["keywords"]))
    print()
    print("=== .SCN schema fields (col-0, first-seen order) ===")
    for r in out["scn_schema"]:
        print(f"  {r['field']:16} count={r['count']:<4} args={r['argcount_dist']}")
    print()
    print("=== .OB schema fields (col-0, first-seen order) ===")
    for r in out["ob_schema"]:
        print(f"  {r['field']:16} count={r['count']:<4} args={r['argcount_dist']}")


if __name__ == "__main__":
    main()
