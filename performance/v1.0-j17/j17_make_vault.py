#!/usr/bin/env python3
"""Generate a deterministic Obsidian vault that exercises what an import writes:
wikilinks forward and backward, aliases, headings, display text, markdown links,
embedded attachments, a missing target, and nested folders."""
import os
import random
import struct
import sys
import zlib

root = sys.argv[1]
notes = int(sys.argv[2]) if len(sys.argv) > 2 else 300
rng = random.Random(17)

def png(seed):
    raw = b"\x00" + bytes([seed % 256, 7, 9])
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b""))

folders = ["", "alpha", "alpha/inner", "beta", "beta/deep/deeper", "gamma"]
paths = []
for i in range(notes):
    folder = folders[i % len(folders)]
    paths.append((folder, f"note-{i:04d}"))
assets = []
for j in range(20):
    folder = folders[(j * 2) % len(folders)]
    d = os.path.join(root, folder, "attachments")
    os.makedirs(d, exist_ok=True)
    name = f"img-{j:02d}.png"
    with open(os.path.join(d, name), "wb") as f:
        f.write(png(j))
    assets.append(name)

for i, (folder, name) in enumerate(paths):
    d = os.path.join(root, folder)
    os.makedirs(d, exist_ok=True)
    lines = []
    if i % 3 == 0:
        lines += ["---", f"aliases: [alias-{i}, other {i}]", "tags: [t1]", "---"]
    lines += [f"# {name}", "", "Intro paragraph.", ""]
    for k in range(rng.randint(1, 6)):
        target = rng.randrange(notes)
        tname = paths[target][1]
        style = rng.randrange(6)
        if style == 0:
            lines.append(f"- see [[{tname}]]")
        elif style == 1:
            lines.append(f"- see [[{tname}|shown {k}]]")
        elif style == 2:
            lines.append(f"- see [[{tname}#{tname}]]")
        elif style == 3:
            tf = paths[target][0]
            rel = os.path.relpath(os.path.join(tf, tname + ".md"), folder or ".")
            lines.append(f"- see [md]({rel.replace(os.sep, '/')})")
        elif style == 4:
            lines.append(f"- see [[alias-{target - target % 3}]]")
        else:
            lines.append(f"![[{rng.choice(assets)}]]")
    if i % 17 == 0:
        lines.append("A [[does-not-exist]] link.")
    lines += ["", "## Section", "", "1. one", "2. two", "", "> quote", ""]
    with open(os.path.join(d, name + ".md"), "w") as f:
        f.write("\n".join(lines))
print(f"{notes} notes, {len(assets)} attachments in {root}")
