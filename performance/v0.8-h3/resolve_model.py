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


@dataclass
class Resolution:
    roots: dict[str, str] = field(default_factory=dict)
    notices: list[str] = field(default_factory=list)
    mode: str = "installed"


def _join(os_name: str, *parts: str) -> str:
    joiner = ntpath.join if os_name == WINDOWS else posixpath.join
    return joiner(*[p for p in parts if p])


def _is_abs(os_name: str, path: str) -> bool:
    return ntpath.isabs(path) if os_name == WINDOWS else posixpath.isabs(path)


def _xdg(env: dict, os_name: str, variable: str, default: str, notices: list[str]) -> str:
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
        notices.append(
            f"{variable} is {raw!r}, which is relative; the specification requires an "
            f"absolute path, so it was ignored and {default} used instead"
        )
        return default
    return raw


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
        result.notices.append(f"portable mode: selected by the marker file beside {executable_dir}")
    elif source_checkout:
        result.mode = "source"
        result.roots = {name: _join(os_name, ".", "data", name) for name in ROOT_NAMES}
        result.roots["config"] = _join(os_name, ".", "config")
        result.roots["program_assets"] = _join(os_name, ".", "web", "dist")
        result.notices.append("source mode: a checkout was detected beside the executable")
    else:
        if not home:
            raise ResolutionError(
                "no home directory is set, so no user root can be resolved; "
                "pass explicit paths rather than falling back to the working directory"
            )
        if os_name == LINUX:
            result.roots["config"] = _join(os_name, _xdg(env, os_name, "XDG_CONFIG_HOME", _join(os_name, home, ".config"), result.notices), "notrios")
            result.roots["data"] = _join(os_name, _xdg(env, os_name, "XDG_DATA_HOME", _join(os_name, home, ".local", "share"), result.notices), "notrios")
            result.roots["state"] = _join(os_name, _xdg(env, os_name, "XDG_STATE_HOME", _join(os_name, home, ".local", "state"), result.notices), "notrios")
            result.roots["cache"] = _join(os_name, _xdg(env, os_name, "XDG_CACHE_HOME", _join(os_name, home, ".cache"), result.notices), "notrios")
            result.roots["program_assets"] = "/usr/local/share/notrios"

            runtime = (env.get("XDG_RUNTIME_DIR") or "").strip()
            fallback = _join(os_name, result.roots["state"], "runtime")
            if not runtime:
                result.roots["runtime"] = fallback
                result.notices.append(
                    "XDG_RUNTIME_DIR is not set and the specification names no fallback; "
                    f"using {fallback} rather than a shared temporary directory"
                )
            elif not _is_abs(os_name, runtime):
                result.roots["runtime"] = fallback
                result.notices.append(f"XDG_RUNTIME_DIR is {runtime!r}, which is relative; using {fallback}")
            elif runtime_dir_is_private is not None and not runtime_dir_is_private(runtime):
                result.roots["runtime"] = fallback
                result.notices.append(
                    f"{runtime} is not owner-only, so staged backup material would be readable by others; using {fallback}"
                )
            else:
                result.roots["runtime"] = _join(os_name, runtime, "notrios")

        elif os_name == WINDOWS:
            roaming = (env.get("APPDATA") or "").strip() or ntpath.join(home, "AppData", "Roaming")
            local = (env.get("LOCALAPPDATA") or "").strip() or ntpath.join(home, "AppData", "Local")
            if any((env.get(v) or "").strip() for v in ("XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME")):
                result.notices.append("XDG_* variables are ignored on Windows; the native locations are authoritative")
            result.roots["config"] = ntpath.join(roaming, "Notrios", "Config")
            result.roots["data"] = ntpath.join(local, "Notrios", "Data")
            result.roots["state"] = ntpath.join(local, "Notrios", "State")
            result.roots["cache"] = ntpath.join(local, "Notrios", "Cache")
            result.roots["runtime"] = ntpath.join(local, "Notrios", "Runtime")
            result.roots["program_assets"] = executable_dir

        elif os_name == MACOS:
            support = posixpath.join(home, "Library", "Application Support")
            if any((env.get(v) or "").strip() for v in ("XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME")):
                result.notices.append("XDG_* variables are ignored on macOS; the native locations are authoritative")
            result.roots["config"] = posixpath.join(support, "Notrios", "Config")
            result.roots["data"] = posixpath.join(support, "Notrios", "Data")
            result.roots["state"] = posixpath.join(support, "Notrios", "State")
            result.roots["cache"] = posixpath.join(home, "Library", "Caches", "Notrios")
            tmp = (env.get("TMPDIR") or "").strip()
            result.roots["runtime"] = posixpath.join(tmp or posixpath.join(home, "Library", "Caches"), "Notrios", "Runtime")
            result.roots["program_assets"] = posixpath.join(executable_dir, "..", "Resources")

        else:
            raise ResolutionError(f"unsupported operating system {os_name!r}")

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
        result.notices.append(f"{name} was given explicitly as {value}")

    return result
