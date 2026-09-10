#!/usr/bin/env python3
"""Derive each 1.0 compatibility surface from the source that defines it.

One module, used by the generator and the validator, so a freeze detects a
change to the *surface* rather than a disagreement between two transcriptions of
it. That is the opposite of I6's SBOM cross-check, and deliberately: there the
question was whether two independent readings agree, here it is whether today's
surface is the one that was frozen.
"""
import hashlib
import json
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[2]


def _sorted_unique(values) -> list[str]:
    return sorted(set(values))


def cli_commands() -> list[str]:
    spec = json.loads((ROOT / "internal/clispec/commands.json").read_text(encoding="utf-8"))
    return _sorted_unique(" ".join(c["path"]) for c in spec["commands"])


def rest_routes() -> list[str]:
    found = []
    for path in sorted((ROOT / "internal/httpapi").glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        found += re.findall(r'mux\.HandleFunc\("([A-Z]+ [^"]+)"', path.read_text(encoding="utf-8"))
    return _sorted_unique(found)


def mcp_tools() -> list[str]:
    found = []
    for path in sorted((ROOT / "internal/httpapi").glob("mcp*.go")):
        if path.name.endswith("_test.go"):
            continue
        found += re.findall(r'case "([a-z_.]+)"', path.read_text(encoding="utf-8"))
    return _sorted_unique(found)


def make_targets() -> list[str]:
    text = (ROOT / "Makefile").read_text(encoding="utf-8")
    # `[^=]*` before the ## excluded every target whose prerequisites contain
    # one, which was most of them: it found 5 of 34.
    return _sorted_unique(re.findall(r"^([a-z][a-z0-9-]*):.*##", text, re.MULTILINE))


def abi_symbols() -> list[str]:
    header = (ROOT / "cmd/notrioslib/notrios_abi.h").read_text(encoding="utf-8")
    return _sorted_unique(re.findall(r"\bnotrios_[a-z_]+", header))


def archive_contract() -> list[str]:
    root = ROOT / "contracts/archive-v2"
    return sorted(f"{p.relative_to(root).as_posix()}:"
                  f"{hashlib.sha256(p.read_bytes()).hexdigest()[:16]}"
                  for p in root.rglob("*") if p.is_file())


def configuration_keys() -> list[str]:
    """The configuration surface, from the struct tags that define it.

    The tags are `json:`, not `yaml:` -- looking for the wrong one found zero
    keys and would have frozen an empty surface, which passes for ever.
    """
    found = []
    for path in sorted((ROOT / "internal/config").glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        found += re.findall(r'json:"([a-z0-9_]+)', path.read_text(encoding="utf-8"))
    return _sorted_unique(found)


SURFACES = {
    "cli": cli_commands,
    "rest": rest_routes,
    "mcp": mcp_tools,
    "make_lifecycle": make_targets,
    "c_abi": abi_symbols,
    "archive_v2": archive_contract,
    "configuration": configuration_keys,
}


def inventory() -> dict:
    out = {}
    for name, derive in SURFACES.items():
        members = derive()
        out[name] = {
            "count": len(members),
            "sha256": hashlib.sha256("\n".join(members).encode("utf-8")).hexdigest(),
            "members": members,
        }
    return out
