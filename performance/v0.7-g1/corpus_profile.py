#!/usr/bin/env python3
"""Emit aggregate-only size and static-history profiles for G1 corpora.

The output deliberately contains labels, counts, byte distributions, and
parse-quality counters only. It never emits source paths, names, titles,
bodies, hashes of source content, or resource MIME/name metadata.
"""

from __future__ import annotations

import argparse
import json
import math
import os
import re
from collections import Counter
from pathlib import Path
from typing import Iterable


BUCKETS = (0, 256, 1024, 4096, 16384, 65536, 262144, 1048576)
TYPE_RE = re.compile(rb"(?m)^type_: ([0-9]+)\r?$")
FIELD_RE = re.compile(rb"^[A-Za-z0-9_]+: ")


def percentile(sorted_values: list[int], fraction: float) -> int:
    if not sorted_values:
        return 0
    index = max(0, math.ceil(len(sorted_values) * fraction) - 1)
    return sorted_values[index]


def distribution(values: Iterable[int]) -> dict[str, object]:
    ordered = sorted(values)
    histogram: Counter[str] = Counter()
    for value in ordered:
        lower = BUCKETS[0]
        label = f">={BUCKETS[-1]}"
        for upper in BUCKETS[1:]:
            if value < upper:
                label = f"{lower}-{upper - 1}"
                break
            lower = upper
        histogram[label] += 1
    return {
        "count": len(ordered),
        "bytes_total": sum(ordered),
        "bytes_min": ordered[0] if ordered else 0,
        "bytes_p50": percentile(ordered, 0.50),
        "bytes_p90": percentile(ordered, 0.90),
        "bytes_p95": percentile(ordered, 0.95),
        "bytes_p99": percentile(ordered, 0.99),
        "bytes_max": ordered[-1] if ordered else 0,
        "histogram": dict(sorted(histogram.items())),
    }


def body_size_from_joplin(raw: bytes) -> tuple[int | None, int | None]:
    matches = list(TYPE_RE.finditer(raw))
    if not matches:
        return None, None
    item_type = int(matches[-1].group(1))
    if item_type != 1:
        return item_type, None

    normalized = raw.replace(b"\r\n", b"\n").replace(b"\r", b"\n")
    lines = normalized.split(b"\n")
    type_index = -1
    for index in range(len(lines) - 1, -1, -1):
        if lines[index].startswith(b"type_: "):
            type_index = index
            break
    if type_index < 0:
        return item_type, None

    separator = -1
    for index in range(type_index - 1, -1, -1):
        if lines[index] == b"":
            separator = index
            break
        if not FIELD_RE.match(lines[index]):
            break

    if separator >= 0:
        content = lines[:separator]
        # Canonical Joplin RAW is title, blank, body, blank, metadata. The
        # source title is not a Notrios body byte.
        if len(content) >= 2 and content[1] == b"":
            content = content[2:]
        return item_type, len(b"\n".join(content).rstrip(b"\n"))

    # Metadata-first legacy shape: consume property lines through the first
    # blank and treat the remainder as body.
    body_start = 0
    for index, line in enumerate(lines):
        if line == b"":
            body_start = index + 1
            break
        if not FIELD_RE.match(line):
            return item_type, None
    return item_type, len(b"\n".join(lines[body_start:]).rstrip(b"\n"))


def scan_joplin(label: str, root: Path) -> dict[str, object]:
    body_sizes: list[int] = []
    resource_sizes: list[int] = []
    type_counts: Counter[str] = Counter()
    invalid_items = 0
    for directory, _, names in os.walk(root):
        for name in names:
            path = Path(directory, name)
            if path.suffix.lower() == ".md":
                try:
                    item_type, body_size = body_size_from_joplin(path.read_bytes())
                except (OSError, ValueError):
                    invalid_items += 1
                    continue
                if item_type is None:
                    invalid_items += 1
                    continue
                type_counts[str(item_type)] += 1
                if body_size is not None:
                    body_sizes.append(body_size)
            else:
                try:
                    resource_sizes.append(path.stat().st_size)
                except OSError:
                    invalid_items += 1
    revision_items = type_counts.get("13", 0)
    return {
        "label": label,
        "kind": "joplin-raw",
        "privacy": "aggregate-only",
        "body_sizes": distribution(body_sizes),
        "resource_sizes": distribution(resource_sizes),
        "item_type_counts": dict(sorted(type_counts.items())),
        "revision_observation": {
            "revision_items": revision_items,
            "static_current_bodies": len(body_sizes),
            "inference": "no revision-chain depth is inferred from a static export",
        },
        "invalid_or_unclassified_files": invalid_items,
    }


def scan_plain(label: str, root: Path, body_suffix: str) -> dict[str, object]:
    body_sizes: list[int] = []
    resource_sizes: list[int] = []
    invalid_items = 0
    suffix = body_suffix.lower()
    for directory, _, names in os.walk(root):
        for name in names:
            path = Path(directory, name)
            try:
                size = path.stat().st_size
            except OSError:
                invalid_items += 1
                continue
            if path.suffix.lower() == suffix:
                body_sizes.append(size)
            else:
                resource_sizes.append(size)
    return {
        "label": label,
        "kind": "plain-files",
        "privacy": "aggregate-only",
        "body_sizes": distribution(body_sizes),
        "resource_sizes": distribution(resource_sizes),
        "revision_observation": {
            "revision_items": 0,
            "static_current_bodies": len(body_sizes),
            "inference": "no revision-chain depth is inferred from a static corpus",
        },
        "invalid_or_unclassified_files": invalid_items,
    }


def parse_corpus(value: str) -> tuple[str, str, Path, str]:
    parts = value.split(":", 3)
    if len(parts) != 4:
        raise argparse.ArgumentTypeError("expected label:kind:path:body-suffix")
    label, kind, raw_path, suffix = parts
    if not re.fullmatch(r"[a-z0-9-]+", label):
        raise argparse.ArgumentTypeError("label must contain lowercase letters, digits, or hyphens")
    if kind not in {"joplin", "plain"}:
        raise argparse.ArgumentTypeError("kind must be joplin or plain")
    return label, kind, Path(raw_path), suffix


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--corpus", action="append", required=True, type=parse_corpus)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    profiles = []
    for label, kind, root, suffix in args.corpus:
        if not root.is_dir():
            parser.error(f"corpus root is not a directory: {label}")
        profiles.append(scan_joplin(label, root) if kind == "joplin" else scan_plain(label, root, suffix))
    payload = {
        "schema": "notrios.g1.corpus-profile.v1",
        "privacy": "aggregate-only; source paths and content are intentionally absent",
        "corpora": profiles,
    }
    rendered = json.dumps(payload, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")


if __name__ == "__main__":
    main()
