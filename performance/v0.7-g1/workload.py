#!/usr/bin/env python3
"""Deterministic G1 delta and three-way-merge investigation workload."""

from __future__ import annotations

import argparse
import base64
import copy
import hashlib
import json
import os
import re
import resource
import statistics
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Sequence


INTERVAL_EDITS = {"1-hour": 1, "1-day": 4, "1-week": 12, "30-days": 32}
DELTA_CASES = ("localized", "scattered", "append", "unicode", "markdown", "binary-looking", "very-long-line")
GRANULARITIES = ("line", "word", "byte")
WORD_RE = re.compile(r"\s+|[^\w\s]+|\w+", re.UNICODE)
MAX_OPS = 10_000
MAX_INSERTED_BYTES = 2 * 1024 * 1024
MAX_BYTE_MERGE_TOKENS = 32_768
MAX_WORD_DELTA_TOKENS = 100_000


def digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def tokenize(value: bytes, unit: str) -> list[Any]:
    if unit == "byte":
        return list(value)
    text = value.decode("utf-8", errors="strict")
    if unit == "line":
        return text.splitlines(keepends=True)
    if unit == "word":
        return WORD_RE.findall(text)
    raise ValueError(f"unknown unit {unit}")


def detokenize(tokens: Sequence[Any], unit: str) -> bytes:
    if unit == "byte":
        return bytes(tokens)
    return "".join(tokens).encode("utf-8")


def inserted_value(tokens: Sequence[Any], unit: str) -> str:
    raw = detokenize(tokens, unit)
    return base64.b64encode(raw).decode("ascii")


def inserted_tokens(value: str, unit: str) -> list[Any]:
    raw = base64.b64decode(value, validate=True)
    return tokenize(raw, unit)


def sequence_delta(base: bytes, result: bytes, unit: str) -> dict[str, Any]:
    import difflib

    before = tokenize(base, unit)
    after = tokenize(result, unit)
    if unit == "word" and max(len(before), len(after)) > MAX_WORD_DELTA_TOKENS:
        raise ValueError("word delta token-count limit")
    ops: list[dict[str, Any]] = []
    matcher = difflib.SequenceMatcher(a=before, b=after, autojunk=unit != "line")
    for tag, i1, i2, j1, j2 in matcher.get_opcodes():
        if tag == "equal":
            ops.append({"k": "copy", "n": i2 - i1})
        elif tag == "delete":
            ops.append({"k": "skip", "n": i2 - i1})
        elif tag == "insert":
            ops.append({"k": "insert", "v": inserted_value(after[j1:j2], unit)})
        elif tag == "replace":
            ops.append({"k": "skip", "n": i2 - i1})
            ops.append({"k": "insert", "v": inserted_value(after[j1:j2], unit)})
    return {
        "schema": "notrios.g1.delta.v1",
        "unit": unit,
        "base_sha256": digest(base),
        "result_sha256": digest(result),
        "ops": ops,
    }


def byte_span_delta(base: bytes, result: bytes) -> dict[str, Any]:
    prefix = 0
    common = min(len(base), len(result))
    while prefix < common and base[prefix] == result[prefix]:
        prefix += 1
    suffix = 0
    while suffix < common - prefix and base[-1 - suffix] == result[-1 - suffix]:
        suffix += 1
    ops: list[dict[str, Any]] = []
    if prefix:
        ops.append({"k": "copy", "n": prefix})
    skipped = len(base) - prefix - suffix
    if skipped:
        ops.append({"k": "skip", "n": skipped})
    middle_end = len(result) - suffix if suffix else len(result)
    middle = result[prefix:middle_end]
    if middle:
        ops.append({"k": "insert", "v": base64.b64encode(middle).decode("ascii")})
    if suffix:
        ops.append({"k": "copy", "n": suffix})
    return {
        "schema": "notrios.g1.delta.v1",
        "unit": "byte",
        "base_sha256": digest(base),
        "result_sha256": digest(result),
        "ops": ops,
    }


def make_delta(base: bytes, result: bytes, method: str) -> dict[str, Any] | None:
    if method == "complete":
        return None
    if method == "byte-span":
        return byte_span_delta(base, result)
    return sequence_delta(base, result, method)


def apply_delta(
    base: bytes | None,
    delta: dict[str, Any],
    *,
    max_ops: int = MAX_OPS,
    max_inserted_bytes: int = MAX_INSERTED_BYTES,
) -> bytes:
    if base is None:
        raise ValueError("missing base")
    if digest(base) != delta.get("base_sha256"):
        raise ValueError("wrong base hash")
    ops = delta.get("ops")
    if not isinstance(ops, list) or len(ops) > max_ops:
        raise ValueError("operation count limit")
    unit = delta.get("unit")
    source = tokenize(base, unit)
    cursor = 0
    output: list[Any] = []
    inserted_bytes = 0
    for op in ops:
        kind = op.get("k")
        if kind in {"copy", "skip"}:
            count = op.get("n")
            if not isinstance(count, int) or count < 0 or cursor + count > len(source):
                raise ValueError("cursor overrun")
            if kind == "copy":
                output.extend(source[cursor : cursor + count])
            cursor += count
        elif kind == "insert":
            raw = base64.b64decode(op.get("v", ""), validate=True)
            inserted_bytes += len(raw)
            if inserted_bytes > max_inserted_bytes:
                raise ValueError("inserted byte limit")
            output.extend(tokenize(raw, unit))
        else:
            raise ValueError("unknown operation")
    if cursor != len(source):
        raise ValueError("base not fully consumed")
    rendered = detokenize(output, unit)
    rendered.decode("utf-8", errors="strict")
    if digest(rendered) != delta.get("result_sha256"):
        raise ValueError("wrong result hash")
    return rendered


def paragraph(index: int) -> str:
    return (
        f"Paragraph {index:04d} keeps deterministic Markdown with café, 東京, and 😀. "
        f"It links to [note {index % 17}](notrios://document/{index % 17:032x}) and "
        "contains enough ordinary prose to resemble a note body.\n"
    )


def delta_fixture(case: str, interval: str) -> tuple[bytes, bytes]:
    edits = INTERVAL_EDITS[interval]
    if case == "very-long-line":
        words = [f"token{index:05d}" for index in range(6_000)]
        base = (" ".join(words) + "\n").encode()
        for index in range(edits):
            words[(index * 173) % len(words)] += "-revised"
        return base, (" ".join(words) + "\n").encode()
    lines = ["# Deterministic synchronization workload\n", "\n"]
    for index in range(96 + edits * 8):
        lines.extend((paragraph(index), "\n"))
    base = "".join(lines)
    result = base
    if case == "localized":
        result = result.replace("ordinary prose", "locally revised prose", 1)
    elif case == "scattered":
        for index in range(edits):
            target = f"Paragraph {index * 7:04d}"
            result = result.replace(target, target + " revised", 1)
    elif case == "append":
        result += "".join(f"\nOffline addition {index}: exact append payload.\n" for index in range(edits))
    elif case == "unicode":
        result = result.replace("café, 東京, and 😀", "cafè, 京都, and 🧭", edits)
    elif case == "markdown":
        result = result.replace("](notrios://document/", "](notrios://stable/", edits)
    elif case == "binary-looking":
        marker = "\x00\x01\x02 valid UTF-8 control-bearing text \x7f"
        result = result.replace("ordinary prose", marker, edits)
    else:
        raise ValueError(case)
    return base.encode("utf-8"), result.encode("utf-8")


def large_delta_fixture() -> tuple[bytes, bytes]:
    lines = []
    total = 0
    while total < 1024 * 1024:
        index = len(lines)
        line = f"Large generated paragraph {index:06d}: café 東京 deterministic payload {index:06x}.\n"
        lines.append(line)
        total += len(line.encode("utf-8"))
    base = "".join(lines)
    result = base.replace("deterministic payload", "bounded revised payload", 1)
    return base.encode(), result.encode()


def percentile(values: list[int], fraction: float) -> int:
    ordered = sorted(values)
    if not ordered:
        return 0
    index = max(0, int((len(ordered) - 1) * fraction))
    return ordered[index]


def peak_rss_kib() -> int:
    return int(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss)


def delta_worker(case: str, interval: str, method: str) -> dict[str, Any]:
    base, result = delta_fixture(case, interval)
    durations: list[int] = []
    delta = None
    reconstructed = result
    for _ in range(5):
        started = time.process_time_ns()
        delta = make_delta(base, result, method)
        reconstructed = result if delta is None else apply_delta(base, delta)
        durations.append(time.process_time_ns() - started)
    if reconstructed != result:
        raise AssertionError("reconstruction mismatch")
    encoded_size = len(result) if delta is None else len(
        json.dumps(delta, sort_keys=True, separators=(",", ":")).encode("utf-8")
    )
    return {
        "case": case,
        "interval": interval,
        "method": method,
        "base_bytes": len(base),
        "result_bytes": len(result),
        "encoded_bytes": encoded_size,
        "ratio_to_complete": round(encoded_size / max(1, len(result)), 6),
        "cpu_ns_p50": int(statistics.median(durations)),
        "cpu_ns_p95": percentile(durations, 0.95),
        "peak_rss_kib": peak_rss_kib(),
        "base_sha256": digest(base),
        "result_sha256": digest(result),
        "reconstructed_sha256": digest(reconstructed),
    }


def scale_delta_worker(method: str) -> dict[str, Any]:
    base, result = large_delta_fixture()
    started = time.process_time_ns()
    try:
        delta = make_delta(base, result, method)
        reconstructed = result if delta is None else apply_delta(base, delta)
    except ValueError as error:
        return {
            "case": "large-generated-1mib",
            "method": method,
            "classification": "limit-rejected",
            "base_bytes": len(base),
            "result_bytes": len(result),
            "encoded_bytes": 0,
            "ratio_to_complete": 0,
            "cpu_ns": time.process_time_ns() - started,
            "peak_rss_kib": peak_rss_kib(),
            "rejection": str(error),
        }
    encoded_size = len(result) if delta is None else len(
        json.dumps(delta, sort_keys=True, separators=(",", ":")).encode()
    )
    return {
        "case": "large-generated-1mib",
        "method": method,
        "classification": "complete" if delta is None else "reconstructed",
        "base_bytes": len(base),
        "result_bytes": len(result),
        "encoded_bytes": encoded_size,
        "ratio_to_complete": round(encoded_size / len(result), 6),
        "cpu_ns": time.process_time_ns() - started,
        "peak_rss_kib": peak_rss_kib(),
        "result_sha256": digest(result),
        "reconstructed_sha256": digest(reconstructed),
    }


@dataclass(frozen=True)
class Change:
    side: str
    start: int
    end: int
    replacement: tuple[Any, ...]


def changes(base: list[Any], variant: list[Any], side: str, unit: str) -> list[Change]:
    import difflib

    matcher = difflib.SequenceMatcher(a=base, b=variant, autojunk=unit != "line")
    return [
        Change(side, i1, i2, tuple(variant[j1:j2]))
        for tag, i1, i2, j1, j2 in matcher.get_opcodes()
        if tag != "equal"
    ]


def overlaps(first: Change, second: Change) -> bool:
    if first.start == first.end and second.start == second.end:
        return first.start == second.start
    if first.start == first.end:
        return second.start < first.start < second.end
    if second.start == second.end:
        return first.start < second.start < first.end
    return max(first.start, second.start) < min(first.end, second.end)


def conflict_marker(base: list[Any], left: list[Any], right: list[Any], unit: str) -> list[Any]:
    raw = (
        b"<<<<<<< local\n"
        + detokenize(left, unit)
        + b"\n||||||| base\n"
        + detokenize(base, unit)
        + b"\n=======\n"
        + detokenize(right, unit)
        + b"\n>>>>>>> remote\n"
    )
    return tokenize(raw, unit)


def apply_span(base: list[Any], start: int, end: int, side_changes: list[Change]) -> list[Any]:
    output: list[Any] = []
    cursor = start
    for change in sorted(side_changes, key=lambda item: (item.start, item.end)):
        output.extend(base[cursor : change.start])
        output.extend(change.replacement)
        cursor = change.end
    output.extend(base[cursor:end])
    return output


def merge3(base_raw: bytes, left_raw: bytes, right_raw: bytes, unit: str) -> tuple[bytes, int]:
    base = tokenize(base_raw, unit)
    left = tokenize(left_raw, unit)
    right = tokenize(right_raw, unit)
    if unit == "byte" and max(len(base), len(left), len(right)) > MAX_BYTE_MERGE_TOKENS:
        raise ValueError("byte merge token-count limit")
    all_changes = changes(base, left, "left", unit) + changes(base, right, "right", unit)
    remaining = sorted(all_changes, key=lambda item: (item.start, item.end, item.side))
    groups: list[list[Change]] = []
    while remaining:
        group = [remaining.pop(0)]
        expanded = True
        while expanded:
            expanded = False
            for candidate in list(remaining):
                if any(overlaps(candidate, member) for member in group):
                    group.append(candidate)
                    remaining.remove(candidate)
                    expanded = True
        groups.append(group)

    patches: list[tuple[int, int, list[Any], bool]] = []
    for group in groups:
        start = min(item.start for item in group)
        end = max(item.end for item in group)
        left_changes = [item for item in group if item.side == "left"]
        right_changes = [item for item in group if item.side == "right"]
        left_value = apply_span(base, start, end, left_changes) if left_changes else base[start:end]
        right_value = apply_span(base, start, end, right_changes) if right_changes else base[start:end]
        if not left_changes:
            patches.append((start, end, right_value, False))
        elif not right_changes:
            patches.append((start, end, left_value, False))
        elif left_value == right_value:
            patches.append((start, end, left_value, False))
        else:
            patches.append((start, end, conflict_marker(base[start:end], left_value, right_value, unit), True))

    output: list[Any] = []
    cursor = 0
    conflicts = 0
    for start, end, replacement, is_conflict in sorted(patches, key=lambda item: (item[0], item[1])):
        if start < cursor:
            raise AssertionError("overlapping merge patches")
        output.extend(base[cursor:start])
        output.extend(replacement)
        cursor = end
        conflicts += int(is_conflict)
    output.extend(base[cursor:])
    rendered = detokenize(output, unit)
    rendered.decode("utf-8", errors="strict")
    return rendered, conflicts


def merge_fixture(case: str) -> tuple[bytes, bytes, bytes]:
    if case == "independent-paragraphs":
        base = "# Plan\n\nAlpha paragraph stays readable.\n\nOmega paragraph stays readable.\n"
        return base.encode(), base.replace("Alpha", "Local Alpha").encode(), base.replace("Omega", "Remote Omega").encode()
    if case == "same-line-disjoint-words":
        base = "red green blue\n"
        return base.encode(), base.replace("red", "crimson").encode(), base.replace("blue", "azure").encode()
    if case == "same-token":
        base = "red green blue\n"
        return base.encode(), base.replace("green", "emerald").encode(), base.replace("green", "olive").encode()
    if case == "unicode-neighbours":
        base = "café 東京 😀 finish\n"
        return base.encode(), base.replace("café", "cafè").encode(), base.replace("東京", "京都").encode()
    if case == "markdown-overlap":
        base = "[target](https://example.test/base)\n"
        return base.encode(), base.replace("/base", "/local").encode(), base.replace("/base", "/remote").encode()
    if case == "very-long-line":
        base = "start " + "word " * 10_000 + "left middle right\n"
        return base.encode(), base.replace("left", "LOCAL").encode(), base.replace("right", "REMOTE").encode()
    raise ValueError(case)


def merge_worker(case: str, unit: str) -> dict[str, Any]:
    base, left, right = merge_fixture(case)
    durations: list[int] = []
    rendered = b""
    conflicts = 0
    try:
        for _ in range(5):
            started = time.process_time_ns()
            rendered, conflicts = merge3(base, left, right, unit)
            durations.append(time.process_time_ns() - started)
    except ValueError as error:
        return {
            "case": case,
            "granularity": unit,
            "classification": "limit-rejected",
            "conflicts": 0,
            "base_bytes": len(base),
            "merged_bytes": 0,
            "cpu_ns_p50": 0,
            "cpu_ns_p95": 0,
            "peak_rss_kib": peak_rss_kib(),
            "base_sha256": digest(base),
            "left_sha256": digest(left),
            "right_sha256": digest(right),
            "merged_sha256": "",
            "valid_utf8": False,
            "rejection": str(error),
        }
    return {
        "case": case,
        "granularity": unit,
        "classification": "conflict" if conflicts else "clean",
        "conflicts": conflicts,
        "base_bytes": len(base),
        "merged_bytes": len(rendered),
        "cpu_ns_p50": int(statistics.median(durations)),
        "cpu_ns_p95": percentile(durations, 0.95),
        "peak_rss_kib": peak_rss_kib(),
        "base_sha256": digest(base),
        "left_sha256": digest(left),
        "right_sha256": digest(right),
        "merged_sha256": digest(rendered),
        "valid_utf8": True,
    }


def rejection_results() -> list[dict[str, Any]]:
    base = b"alpha beta gamma\n"
    result = b"alpha revised gamma\n"
    valid = sequence_delta(base, result, "word")
    cases: list[tuple[str, bytes | None, dict[str, Any], dict[str, int]]] = []
    cases.append(("missing-base", None, valid, {}))
    wrong_base = copy.deepcopy(valid)
    wrong_base["base_sha256"] = "0" * 64
    cases.append(("wrong-base-hash", base, wrong_base, {}))
    wrong_result = copy.deepcopy(valid)
    wrong_result["result_sha256"] = "0" * 64
    cases.append(("wrong-result-hash", base, wrong_result, {}))
    overrun = copy.deepcopy(valid)
    overrun["ops"] = [{"k": "copy", "n": 999999}]
    cases.append(("cursor-overrun", base, overrun, {}))
    cases.append(("operation-count-limit", base, valid, {"max_ops": 1}))
    cases.append(("inserted-byte-limit", base, valid, {"max_inserted_bytes": 1}))
    invalid_utf8 = {
        "schema": "notrios.g1.delta.v1",
        "unit": "byte",
        "base_sha256": digest(b""),
        "result_sha256": digest(b"\xff"),
        "ops": [{"k": "insert", "v": base64.b64encode(b"\xff").decode("ascii")}],
    }
    cases.append(("invalid-utf8-result", b"", invalid_utf8, {}))
    output = []
    for name, candidate_base, delta, limits in cases:
        rejected = False
        reason = ""
        try:
            apply_delta(candidate_base, delta, **limits)
        except (ValueError, UnicodeDecodeError) as error:
            rejected = True
            reason = str(error)
        output.append({"case": name, "rejected": rejected, "reason": reason})
    return output


def run_worker(arguments: list[str]) -> None:
    kind = arguments[0]
    if kind == "delta":
        payload = delta_worker(arguments[1], arguments[2], arguments[3])
    elif kind == "scale-delta":
        payload = scale_delta_worker(arguments[1])
    elif kind == "merge":
        payload = merge_worker(arguments[1], arguments[2])
    else:
        raise ValueError(kind)
    print(json.dumps(payload, sort_keys=True))


def invoke_worker(*arguments: str) -> dict[str, Any]:
    completed = subprocess.run(
        [sys.executable, __file__, "worker", *arguments],
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(completed.stdout)


def build_benchmark() -> dict[str, Any]:
    delta_records = []
    for interval in INTERVAL_EDITS:
        for case in DELTA_CASES:
            for method in ("complete", "line", "word", "byte-span"):
                delta_records.append(invoke_worker("delta", case, interval, method))
    merge_records = []
    for case in (
        "independent-paragraphs",
        "same-line-disjoint-words",
        "same-token",
        "unicode-neighbours",
        "markdown-overlap",
        "very-long-line",
    ):
        for unit in GRANULARITIES:
            merge_records.append(invoke_worker("merge", case, unit))
    scale_records = [invoke_worker("scale-delta", method) for method in ("complete", "line", "word", "byte-span")]
    return {
        "schema": "notrios.g1.workload.v1",
        "generator": "performance/v0.7-g1/workload.py",
        "interpretation": "synthetic divergence workload; not a human concurrency-frequency estimate",
        "offline_interval_edit_counts": INTERVAL_EDITS,
        "delta_records": delta_records,
        "merge_records": merge_records,
        "scale_records": scale_records,
        "patch_rejections": rejection_results(),
    }


def main() -> None:
    if len(sys.argv) > 1 and sys.argv[1] == "worker":
        run_worker(sys.argv[2:])
        return
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    payload = build_benchmark()
    args.output.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
