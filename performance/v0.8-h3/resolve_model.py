#!/usr/bin/env python3
"""Executable model of the H3 path resolver.

H3 changes no runtime default, so this is where the proposed rules are actually
run. A table in a document can be internally inconsistent and nobody notices;
a model that computes the table cannot. H4 implements these rules in Go and
should reproduce this table exactly.

The model is pure: it takes an environment dictionary and returns resolved
roots plus the notices a user would see. It touches the filesystem only for the
XDG_RUNTIME_DIR ownership check, which is injectable for the same reason.
"""

from __future__ import annotations

import ntpath
import posixpath
from dataclasses import dataclass, field

LINUX, WINDOWS, MACOS = "linux", "windows", "macos"

ROOT_NAMES = ("config", "data", "state", "cache", "runtime", "program_assets")


class ResolutionError(ValueError):
    """Raised when no root can be determined. Failing closed is the point."""


XDG_RELATIVE_IGNORED = "xdg_relative_ignored"
XDG_IGNORED_ON_PLATFORM = "xdg_ignored_on_platform"
RUNTIME_DIR_UNSET = "runtime_dir_unset"
RUNTIME_DIR_RELATIVE = "runtime_dir_relative"
RUNTIME_DIR_NOT_PRIVATE = "runtime_dir_not_private"
PORTABLE_SELECTED = "portable_selected"
SOURCE_SELECTED = "source_selected"
EXPLICIT_OVERRIDE = "explicit_override"


@dataclass
class Resolution:
    roots: dict[str, str] = field(default_factory=dict)
    notices: list[dict] = field(default_factory=list)
    mode: str = "installed"

    def note(self, code: str, message: str) -> None:
        self.notices.append({"code": code, "message": message})

    def codes(self) -> list[str]:
        return [n["code"] for n in self.notices]


def _join(os_name: str, *parts: str) -> str:
    joiner = ntpath.join if os_name == WINDOWS else posixpath.join
    return _clean(os_name, joiner(*[p for p in parts if p]))


def _clean(os_name: str, path: str) -> str:
    """Resolve "." and ".." lexically.

    H4 found that omitting this was a real defect rather than cosmetic: joining
    without cleaning produced roots like
    /media/stick/notrios/bin/../notrios-data/cache, and purge_oracle.decide
    refuses exactly that shape under its normal-form rule. The resolver and the
    oracle -- two artifacts of the same investigation -- disagreed, so a
    portable installation could not have been purged. test_resolve_model.py now
    asserts every resolved root is accepted by the oracle's normal-form rule.
    """
    module = ntpath if os_name == WINDOWS else posixpath
    return module.normpath(path)


def _is_abs(os_name: str, path: str) -> bool:
    return ntpath.isabs(path) if os_name == WINDOWS else posixpath.isabs(path)


def _xdg(env: dict, os_name: str, variable: str, default: str, result: "Resolution") -> str:
    """Apply the XDG rules to one variable.

    Empty is unset. Relative is invalid: the specification says to ignore it,
    and ignoring it is also the only safe reading, because the alternative is
    resolving a user's database location against whatever directory the process
    was started in.
    """
    raw = (env.get(variable) or "").strip()
    if not raw:
        return default
    if not _is_abs(os_name, raw):
        result.note(
            XDG_RELATIVE_IGNORED,
            f"{variable} is {raw!r}, which is relative; the specification requires an "
            f"absolute path, so it was ignored and {default} used instead",
        )
        return default
    return raw


def _resolve_installed(env: dict, os_name: str, executable_dir: str, result: "Resolution",
                       runtime_dir_is_private=None) -> None:
    """The native per-OS roots. Source mode calls this too, for config."""
    home = (env.get("HOME") or env.get("USERPROFILE") or "").strip()
    if not home:
        raise ResolutionError(
            "no home directory is set, so no user root can be resolved; "
            "pass explicit paths rather than falling back to the working directory"
        )
    if os_name == LINUX:
        result.roots["config"] = _join(os_name, _xdg(env, os_name, "XDG_CONFIG_HOME", _join(os_name, home, ".config"), result), "notrios")
        result.roots["data"] = _join(os_name, _xdg(env, os_name, "XDG_DATA_HOME", _join(os_name, home, ".local", "share"), result), "notrios")
        result.roots["state"] = _join(os_name, _xdg(env, os_name, "XDG_STATE_HOME", _join(os_name, home, ".local", "state"), result), "notrios")
        result.roots["cache"] = _join(os_name, _xdg(env, os_name, "XDG_CACHE_HOME", _join(os_name, home, ".cache"), result), "notrios")
        result.roots["program_assets"] = "/usr/local/share/notrios"

        runtime = (env.get("XDG_RUNTIME_DIR") or "").strip()
        fallback = _join(os_name, result.roots["state"], "runtime")
        if not runtime:
            result.roots["runtime"] = fallback
            result.note(
                RUNTIME_DIR_UNSET,
                "XDG_RUNTIME_DIR is not set and the specification names no fallback; "
                f"using {fallback} rather than a shared temporary directory",
            )
        elif not _is_abs(os_name, runtime):
            result.roots["runtime"] = fallback
            result.note(RUNTIME_DIR_RELATIVE, f"XDG_RUNTIME_DIR is {runtime!r}, which is relative; using {fallback}")
        elif runtime_dir_is_private is not None and not runtime_dir_is_private(runtime):
            result.roots["runtime"] = fallback
            result.note(
                RUNTIME_DIR_NOT_PRIVATE,
                f"{runtime} is not owner-only, so staged backup material would be readable by others; using {fallback}",
            )
        else:
            result.roots["runtime"] = _join(os_name, runtime, "notrios")

    elif os_name == WINDOWS:
        roaming = (env.get("APPDATA") or "").strip() or _join(os_name, home, "AppData", "Roaming")
        local = (env.get("LOCALAPPDATA") or "").strip() or _join(os_name, home, "AppData", "Local")
        if any((env.get(v) or "").strip() for v in ("XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME")):
            result.note(XDG_IGNORED_ON_PLATFORM, "XDG_* variables are ignored on Windows; the native locations are authoritative")
        result.roots["config"] = _join(os_name, roaming, "Notrios", "Config")
        result.roots["data"] = _join(os_name, local, "Notrios", "Data")
        result.roots["state"] = _join(os_name, local, "Notrios", "State")
        result.roots["cache"] = _join(os_name, local, "Notrios", "Cache")
        result.roots["runtime"] = _join(os_name, local, "Notrios", "Runtime")
        result.roots["program_assets"] = executable_dir

    elif os_name == MACOS:
        support = _join(os_name, home, "Library", "Application Support")
        if any((env.get(v) or "").strip() for v in ("XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME")):
            result.note(XDG_IGNORED_ON_PLATFORM, "XDG_* variables are ignored on macOS; the native locations are authoritative")
        result.roots["config"] = _join(os_name, support, "Notrios", "Config")
        result.roots["data"] = _join(os_name, support, "Notrios", "Data")
        result.roots["state"] = _join(os_name, support, "Notrios", "State")
        result.roots["cache"] = _join(os_name, home, "Library", "Caches", "Notrios")
        tmp = (env.get("TMPDIR") or "").strip()
        result.roots["runtime"] = _join(os_name, tmp or _join(os_name, home, "Library", "Caches"), "Notrios", "Runtime")
        result.roots["program_assets"] = _join(os_name, executable_dir, "..", "Resources")

    else:
        raise ResolutionError(f"unsupported operating system {os_name!r}")


def resolve(env: dict, os_name: str = LINUX, *,
            executable_dir: str = "/opt/notrios/bin",
            portable_marker: bool = False,
            explicit: dict | None = None,
            runtime_dir_is_private=None,
            source_checkout: bool = False) -> Resolution:
    result = Resolution()
    explicit = explicit or {}

    home = (env.get("HOME") or env.get("USERPROFILE") or "").strip()

    # 1. Portable mode, selected only by an explicit marker beside the binary.
    if portable_marker:
        base = _join(os_name, executable_dir, "..", "notrios-data")
        result.mode = "portable"
        for name in ROOT_NAMES:
            result.roots[name] = _join(os_name, base, name)
        result.roots["program_assets"] = _join(os_name, executable_dir, "..", "share", "notrios")
        result.note(PORTABLE_SELECTED, f"portable mode: selected by the notrios-portable.txt marker beside {executable_dir}")
    elif source_checkout:
        # Source mode moves the program's own files and a developer's scratch
        # data into the checkout, and deliberately leaves the config root alone.
        # H4 corrected this: overriding config relocated the profile registry
        # into the checkout's config/ directory, and a checkout is not a
        # different user. The registry and sync keys are the developer's
        # identity across every build, and writing them into the source tree
        # puts a file naming every local database path one `git add -A` away
        # from being committed.
        _resolve_installed(env, os_name, executable_dir, result, runtime_dir_is_private)
        result.mode = "source"
        # All four mutable roots collapse onto the checkout's ./data. The
        # four-way split exists for user installations -- XDG separates them,
        # purge treats them differently, only some are backed up -- and a
        # checkout has one scratch directory. Splitting it would move every
        # existing developer's database from ./data/notes.sqlite to
        # ./data/data/notes.sqlite to satisfy a distinction that does not
        # apply here.
        for name in ("data", "state", "cache", "runtime"):
            result.roots[name] = _join(os_name, ".", "data")
        result.roots["program_assets"] = _join(os_name, ".", "web", "dist")
        result.note(
            SOURCE_SELECTED,
            "source mode: a checkout was detected, so data and assets are checkout-relative; "
            "config stays in the user config root",
        )
    else:
        _resolve_installed(env, os_name, executable_dir, result, runtime_dir_is_private)

    # 2. Explicit paths win over everything, including portable and source mode.
    for name, value in explicit.items():
        if name not in ROOT_NAMES:
            raise ResolutionError(f"unknown root {name!r}")
        value = (value or "").strip()
        if not value:
            continue
        if not _is_abs(os_name, value):
            raise ResolutionError(
                f"the explicit {name} path {value!r} is relative; an explicitly given path "
                "is never resolved against the working directory"
            )
        result.roots[name] = value
        result.note(EXPLICIT_OVERRIDE, f"{name} was given explicitly as {value}")

    return result
