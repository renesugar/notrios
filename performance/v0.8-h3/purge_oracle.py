#!/usr/bin/env python3
"""Reference decision procedure for "may Notrios delete this path?".

H3 does not delete anything. This is the oracle H5's `purge` target must
implement, written once here so the rules can be argued about and tested before
any code can act on them.

The rules are deliberately closed by default. A purge target is refused unless
it is positively identified as a path Notrios created under a root Notrios
owns. Every other outcome -- unknown, ambiguous, outside, unreadable, crossing a
mount, reached through a symlink, or named by a profile that points elsewhere --
refuses. That asymmetry is the whole design: a wrongly refused deletion costs
the user a manual `rm`, and a wrongly allowed one costs them their notes.

Verdicts
--------
ALLOW           delete it
ALLOW_ABSENT    nothing there; a repeated purge is a no-op, not an error
REFUSE          do not delete; the reason names what to do instead

Injectable `device_of` exists so the mount-boundary rule can be exercised
without requiring a real mount in the test environment. The default reads the
filesystem. Where a rule is modelled rather than executed, REPORT.md says so.
"""

from __future__ import annotations

import os
import posixpath
import sys
from dataclasses import dataclass, field

ALLOW = "ALLOW"
ALLOW_ABSENT = "ALLOW_ABSENT"
REFUSE = "REFUSE"

# Roots no purge may ever name or contain, whatever the configuration says.
FORBIDDEN_ROOTS = (
    "/", "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib32", "/lib64",
    "/media", "/mnt", "/opt", "/proc", "/root", "/run", "/sbin", "/srv",
    "/sys", "/tmp", "/usr", "/var",
)


@dataclass
class Environment:
    """What the oracle is allowed to consider Notrios's own territory."""

    owned_roots: list[str] = field(default_factory=list)
    home: str = ""
    # Paths named inside profiles that live outside every owned root. These are
    # enumerated and backed up, never deleted automatically: H3's recommended
    # default, restated in the H3 open decisions.
    external_profile_paths: list[str] = field(default_factory=list)
    device_of = None


@dataclass
class Decision:
    verdict: str
    reason: str
    rule: str

    def __str__(self) -> str:
        return f"{self.verdict}: {self.reason} [{self.rule}]"


def _device(env: Environment, path: str) -> int:
    if env.device_of is not None:
        return env.device_of(path)
    return os.lstat(path).st_dev


def _normalize(path: str) -> str:
    # posixpath.normpath collapses "a/../b" textually. That is exactly what we
    # must NOT rely on for containment, because it disagrees with the kernel
    # when a component is a symlink -- so it is used only to reject obvious
    # nonsense, and containment is decided on the realpath below.
    return posixpath.normpath(path)


def _is_within(child: str, parent: str) -> bool:
    parent = parent.rstrip("/")
    return child == parent or child.startswith(parent + "/")


def decide(path: str, env: Environment) -> Decision:
    raw = path

    if path is None or not path.strip():
        return Decision(REFUSE, "the purge target is empty", "non-empty")
    path = path.strip()

    if not posixpath.isabs(path):
        return Decision(REFUSE, f"{raw!r} is relative; a purge target must be absolute", "absolute")

    normalized = _normalize(path)
    if normalized != path.rstrip("/") and normalized != path:
        return Decision(
            REFUSE,
            f"{raw!r} is not in normal form; state the path plainly rather than through '..' or '.'",
            "normal-form",
        )

    if normalized in FORBIDDEN_ROOTS:
        return Decision(REFUSE, f"{normalized} is a system root and is never a purge target", "system-root")
    for forbidden in FORBIDDEN_ROOTS:
        if forbidden != "/" and normalized == forbidden:
            return Decision(REFUSE, f"{normalized} is a system root", "system-root")

    if env.home:
        home = _normalize(env.home)
        if normalized == home:
            return Decision(REFUSE, "the purge target is the home directory itself", "home")
        if _is_within(home, normalized):
            return Decision(
                REFUSE,
                f"{normalized} contains the home directory {home}",
                "home-ancestor",
            )

    for external in env.external_profile_paths:
        external_n = _normalize(external)
        if normalized == external_n or _is_within(normalized, external_n) or _is_within(external_n, normalized):
            return Decision(
                REFUSE,
                f"{normalized} is or contains the external profile path {external_n}; "
                "it is enumerated and backed up, never deleted automatically",
                "external-profile-path",
            )

    if not env.owned_roots:
        return Decision(REFUSE, "no owned roots were declared, so nothing can be identified as ours", "owned-root")

    # Containment is decided on the resolved path, so a symlink cannot smuggle a
    # target out of an owned root while still looking contained.
    resolved = os.path.realpath(normalized)
    owner = None
    for root in env.owned_roots:
        resolved_root = os.path.realpath(_normalize(root))
        if _is_within(resolved, resolved_root):
            owner = resolved_root
            break
    if owner is None:
        return Decision(
            REFUSE,
            f"{normalized} resolves to {resolved}, which is outside every root Notrios owns",
            "owned-root",
        )

    if resolved == owner and normalized != owner:
        return Decision(
            REFUSE,
            f"{normalized} resolves onto the owned root {owner} itself rather than a path within it",
            "root-aliasing",
        )

    if not os.path.lexists(normalized):
        return Decision(ALLOW_ABSENT, f"{normalized} does not exist; nothing to remove", "absent")

    if os.path.islink(normalized):
        return Decision(
            REFUSE,
            f"{normalized} is a symlink; remove the link deliberately rather than following it",
            "symlink",
        )

    try:
        target_device = _device(env, normalized)
        owner_device = _device(env, owner)
    except OSError as err:
        return Decision(REFUSE, f"{normalized} could not be inspected: {err}", "unreadable")

    if target_device != owner_device:
        return Decision(
            REFUSE,
            f"{normalized} is on a different filesystem from its root {owner}; "
            "a purge does not cross a mount boundary",
            "mount-boundary",
        )

    return Decision(ALLOW, f"{normalized} is inside {owner} and safe to remove", "owned")


def main(argv: list[str]) -> int:
    if len(argv) < 3:
        print("usage: purge_oracle.py <path> <owned-root> [owned-root...]", file=sys.stderr)
        return 2
    env = Environment(owned_roots=argv[2:], home=os.path.expanduser("~"))
    decision = decide(argv[1], env)
    print(decision)
    return 0 if decision.verdict != REFUSE else 1




# --- Backup policy -----------------------------------------------------------
#
# Deciding *whether* a path may be deleted is only half the contract. The other
# half is what must happen first. Categories that hold something the user cannot
# reproduce are backed up and the backup verified before anything is removed;
# categories that are derived from them are disposed of without ceremony,
# because backing up a rebuildable index would make every purge slower and the
# backup larger while protecting nothing.
#
# quarantine is the interesting case. It holds untrusted downloaded bytes, which
# sounds disposable, but it is also the evidence of what a note tried to fetch
# and the only copy of media a user may have approved but not yet localized. It
# is therefore state and is backed up.
BACKUP_POLICY = {
    "config": "backup_and_verify",
    "data": "backup_and_verify",
    "state": "backup_and_verify",
    "cache": "dispose",
    "runtime": "dispose",
    "program_assets": "uninstall_manifest_only",
    "external": "backup_never_delete",
}


def backup_policy(category: str) -> str:
    """What purge must do with a category before removing it."""
    if category not in BACKUP_POLICY:
        # An unclassified category is treated as irreplaceable. A new root added
        # without a policy must not be silently disposed of.
        return "backup_and_verify"
    return BACKUP_POLICY[category]


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
