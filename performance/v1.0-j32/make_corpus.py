#!/usr/bin/env python3
"""Generate the synthetic corpora the v1.0 J32 harness imports.

    python3 performance/v1.0-j32/make_corpus.py <work-dir> [shape ...]

Every corpus is generated from a fixed seed and nothing else, so it is never
private and a baseline and a candidate read the same bytes. Each is written to
<work-dir>/corpora/<shape>/ with a MANIFEST.json beside it holding the file
count, byte count and a hash over every path and file hash; the runner records
that hash with every measurement.

The shapes (default: all of them):

  joplin-10k     the shape TestJoplinImporterProfile generates, at 10,000 notes
  obsidian-10k   the shape TestObsidianImporterProfile generates, at 10,000 notes
  link-dense     four Obsidian notes of about 1 MiB carrying thousands of links each,
                 half of them a single line with no newline at all
  collision      Obsidian notes that share names -- thousands of index.md in
                 different folders -- and repeated aliases, linked by those names
  near-limit-obsidian, near-limit-joplin
                 a few notes just under the importers' 64 MiB limit among
                 ordinary ones

A corpus that already exists with a matching manifest is left alone, so the
large ones are written once.
"""
from __future__ import annotations

import hashlib
import json
import os
import pathlib
import random
import shutil
import sys

SEED = 32
NEAR_LIMIT_BYTES = 60 * 1024 * 1024  # the limit is 64 MiB; stay clear of it


def write(path: pathlib.Path, body: str | bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    data = body.encode("utf-8") if isinstance(body, str) else body
    path.write_bytes(data)


def joplin_10k(root: pathlib.Path, count: int = 10_000) -> None:
    """internal/importers/joplinraw/profile_test.go, TestJoplinImporterProfile."""
    folders, tags, resources = 10, 20, 20
    for index in range(folders):
        parent = f"parent_id: folder-{index - 1:02d}\n" if index > 0 else ""
        write(root / f"folder-{index:02d}.md",
              f"id: folder-{index:02d}\n{parent}title: Folder {index:02d}\ntype_: 2\n")
    for index in range(tags):
        write(root / f"tag-{index:02d}.md", f"id: tag-{index:02d}\ntitle: tag-{index:02d}\ntype_: 5\n")
    for index in range(resources):
        write(root / f"resource-{index:02d}.md",
              f"id: resource-{index:02d}\ntitle: resource-{index:02d}.bin\n"
              f"filename: resource-{index:02d}.bin\ntype_: 4\n")
        write(root / "resources" / f"resource-{index:02d}", f"generated resource {index}")
    for index in range(count):
        note = f"profile-{index:06d}"
        write(root / f"{note}.md",
              f"Profile body {index}\n\n![resource](:/resource-{index % resources:02d})\n\n"
              f"future_{index}: exact unknown value\nid: {note}\nparent_id: folder-{index % folders:02d}\n"
              f"title: Profile {index}\ntype_: 1\n")
        write(root / "relations" / f"{note}.md",
              f"id: relation-{index:06d}\nnote_id: {note}\ntag_id: tag-{index % tags:02d}\ntype_: 6\n")


def obsidian_10k(root: pathlib.Path, count: int = 10_000) -> None:
    """internal/importers/obsidian/profile_test.go, TestObsidianImporterProfile."""
    folders, resources = 10, 20
    for index in range(resources):
        write(root / "assets" / f"resource-{index:02d}.bin", f"generated resource {index}")
    for index in range(count):
        previous = (index - 1 + count) % count
        write(root / f"Folder-{index % folders:02d}" / "Notes" / f"Profile-{index:06d}.md",
              f"---\naliases: [Alias {index:06d}]\nunknown_{index}: retained\n---\n# Profile {index}\n\n"
              f"Relative [[Profile-{previous:06d}#Heading]].\nAlias ![[Alias {previous:06d}#^block]].\n"
              f"Asset ![[../../assets/resource-{index % resources:02d}.bin]].\n\nBlock ^block\n")


WORDS = ("alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike "
         "november oscar papa quebec romeo sierra tango uniform victor whiskey xray").split()


def link_dense(root: pathlib.Path, notes: int = 4, target_bytes: int = 1 << 20) -> None:
    """Notes of about 1 MiB with a link every few hundred bytes.

    The shape F8 (link coordinates rescanned from the start of the body for
    every link) and §3.1A (replacements spliced one at a time) are about. Half
    the notes are a single line, because a column is counted from the last
    newline and a note with none is the worst case for that. Links mix wiki
    links, embeds, headings, block references and Markdown links, and include
    multi-byte text so a rune column differs from a byte offset.

    Four notes, not more: the cost this shape exposes grows within a note, so
    more notes add time and no information. The first version had twenty, and
    one unmeasured import of them took 1,300 s (J32-A, 2026-09-22), which would
    have made every candidate's comparison several hours long.
    """
    rng = random.Random(SEED)
    names = [f"Dense-{index:03d}" for index in range(notes)]
    for index, name in enumerate(names):
        single_line = index % 2 == 1
        parts, size, link = [f"# {name}" + (" " if single_line else "\n\n")], 0, 0
        while size < target_bytes:
            words = " ".join(rng.choice(WORDS) for _ in range(rng.randint(30, 60)))
            target = names[(index + link + 1) % notes]
            choice = link % 5
            if choice == 0:
                reference = f"[[{target}]]"
            elif choice == 1:
                reference = f"[[{target}#Heading {link % 7}|shown ünïcode {link}]]"
            elif choice == 2:
                reference = f"![[{target}#^b{link % 11}]]"
            elif choice == 3:
                reference = f"[text {link}]({target}.md)"
            else:
                reference = f"[[Missing-{link % 13}]]"
            sentence = f"{words} — ελληνικά {reference}."
            parts.append(sentence + (" " if single_line else "\n"))
            size += len(sentence.encode("utf-8")) + 1
            link += 1
        if not single_line:
            parts.append("\n" + "\n".join(f"## Heading {h}\nText ^b{h}\n" for h in range(11)))
        write(root / "dense" / f"{name}.md", "".join(parts))


def collision(root: pathlib.Path, folders: int = 5_000, alias_groups: int = 50) -> None:
    """Thousands of index.md in different folders, and aliases that repeat.

    The shape F9 (namespace construction with linear duplicate scans) is about:
    a name that thousands of files share, and an alias that hundreds of notes
    claim, each linked by that name so resolution has to look at all of them.
    """
    for index in range(folders):
        group = index % alias_groups
        folder = root / f"area-{index % 50:02d}" / f"topic-{index:05d}"
        write(folder / "index.md",
              f"---\naliases: [Shared {group:02d}, Topic {index:05d}]\n---\n# Topic {index}\n\n"
              f"See [[index]], [[Shared {(group + 1) % alias_groups:02d}]] and "
              f"[[topic-{(index + 1) % folders:05d}/index]].\n")
        if index % 10 == 0:
            write(folder / "notes.md", f"# Notes {index}\n\nBack to [[index]] and [[Topic {index:05d}]].\n")


def near_limit_obsidian(root: pathlib.Path, large: int = 4, small: int = 200) -> None:
    """A few notes just under 64 MiB among ordinary ones (F5: byte-bounded batches)."""
    line = ("near limit text " * 8).rstrip() + "\n"
    body = line * (NEAR_LIMIT_BYTES // len(line))
    for index in range(large):
        write(root / "large" / f"Large-{index}.md", f"# Large {index}\n\n[[Small-0000]]\n\n{body}")
    for index in range(small):
        write(root / "small" / f"Small-{index:04d}.md",
              f"# Small {index}\n\nLinks to [[Large-{index % large}]] and [[Small-{(index + 1) % small:04d}]].\n")


def near_limit_joplin(root: pathlib.Path, large: int = 4, small: int = 200) -> None:
    """The Joplin RAW form of the same shape."""
    write(root / "folder-00.md", "id: folder-00\ntitle: Folder 00\ntype_: 2\n")
    line = ("near limit text " * 8).rstrip() + "\n"
    body = line * (NEAR_LIMIT_BYTES // len(line))
    for index in range(large):
        note = f"large-{index:06d}"
        write(root / f"{note}.md",
              f"Large {index}\n\n{body}\nid: {note}\nparent_id: folder-00\ntitle: Large {index}\ntype_: 1\n")
    for index in range(small):
        note = f"small-{index:06d}"
        write(root / f"{note}.md",
              f"Small {index}\n\nSmall body {index}\n\nid: {note}\nparent_id: folder-00\n"
              f"title: Small {index}\ntype_: 1\n")


SHAPES = {
    "joplin-10k": ("joplin-raw", joplin_10k),
    "obsidian-10k": ("obsidian", obsidian_10k),
    "link-dense": ("obsidian", link_dense),
    "collision": ("obsidian", collision),
    "near-limit-obsidian": ("obsidian", near_limit_obsidian),
    "near-limit-joplin": ("joplin-raw", near_limit_joplin),
}
GENERATOR_VERSION = 2


def manifest_of(root: pathlib.Path) -> dict:
    digest, files, total = hashlib.sha256(), 0, 0
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        data = path.read_bytes()
        relative = path.relative_to(root).as_posix()
        digest.update(f"{relative}\0{hashlib.sha256(data).hexdigest()}\n".encode())
        files += 1
        total += len(data)
    return {"files": files, "bytes": total, "sha256": digest.hexdigest()}


def build(work: pathlib.Path, shape: str) -> dict:
    kind, generate = SHAPES[shape]
    root = work / "corpora" / shape
    manifest_path = work / "corpora" / f"{shape}.MANIFEST.json"
    if manifest_path.is_file() and root.is_dir():
        recorded = json.loads(manifest_path.read_text(encoding="utf-8"))
        if recorded.get("generator_version") == GENERATOR_VERSION and \
                recorded["content"] == manifest_of(root):
            print(f"{shape}: present, {recorded['content']['files']} files", file=sys.stderr)
            return recorded
    if root.exists():
        shutil.rmtree(root)
    root.mkdir(parents=True)
    generate(root)
    recorded = {"shape": shape, "kind": kind, "seed": SEED,
                "generator_version": GENERATOR_VERSION, "content": manifest_of(root)}
    manifest_path.write_text(json.dumps(recorded, indent=2) + "\n", encoding="utf-8")
    print(f"{shape}: wrote {recorded['content']['files']} files, "
          f"{recorded['content']['bytes']} bytes", file=sys.stderr)
    return recorded


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2
    work = pathlib.Path(sys.argv[1]).resolve()
    repository = pathlib.Path(__file__).resolve().parents[2]
    if work == repository or repository in work.parents:
        print("the work directory must be outside the checkout", file=sys.stderr)
        return 2
    shapes = sys.argv[2:] or list(SHAPES)
    unknown = [shape for shape in shapes if shape not in SHAPES]
    if unknown:
        print(f"unknown shape(s): {', '.join(unknown)}; known: {', '.join(SHAPES)}", file=sys.stderr)
        return 2
    for shape in shapes:
        build(work, shape)
    return 0


if __name__ == "__main__":
    os.umask(0o077)
    raise SystemExit(main())
