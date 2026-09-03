#!/usr/bin/env python3
"""Install, uninstall and purge an end-user Notrios from a checkout.

This implements the contract H3 investigated and recorded in
`performance/v0.8-h3/`: the artifact list and GNU directory variables in
`LAYOUT.json`, the ownership manifest in the same file, and -- for purge -- the
closed-by-default decision procedure in `purge_oracle.py`, which 30 fixtures
exercise against a real temporary filesystem.

# Why the oracle is imported rather than reimplemented

`purge` decides whether to delete a directory holding a user's notes. That
decision is already written down, argued for, and proven by fixtures that catch
the mistakes a second implementation would make -- textual containment instead of
`realpath`, a symlink that looks contained while pointing out, a target across a
mount boundary. Writing it again here to avoid importing from an evidence
directory would trade a slightly odd import for the chance of a divergent copy,
on the one code path where being wrong destroys a library.

# What separates these targets from `clean`

`clean` and `clobber` are source-tree only and never touch a user's files. These
targets never touch the checkout. Nothing here reads or writes `./data`.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import stat
import sys
import time
from dataclasses import dataclass, field

# The tree artifacts are copied *from*. It is the checkout this script lives in,
# unless NOTRIOS_LIFECYCLE_SOURCE names another one -- which lets a packager
# build in one tree and install from another, and lets the tests install from a
# fixture instead of requiring a built checkout. It changes only what is read;
# where things are written is still decided by the directory variables.
CHECKOUT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROOT = os.environ.get("NOTRIOS_LIFECYCLE_SOURCE", "").strip() or CHECKOUT

# The oracle is part of this tool, so it is always found relative to this file
# rather than to ROOT. Resolving it through the override let a caller point the
# tool that deletes directories at a different copy of the rules deciding what
# may be deleted -- or, as the tests found first, at no copy at all.
sys.path.insert(0, os.path.join(CHECKOUT, "performance", "v0.8-h3"))
import purge_oracle  # noqa: E402

MANIFEST_SCHEMA = "notrios.install-manifest/1"
MANIFEST_NAME = "MANIFEST.json"


class LifecycleError(Exception):
    """A refusal or failure with a message the user is meant to act on."""


# ---------------------------------------------------------------- directories


@dataclass
class Directories:
    """The GNU directory variables, plus DESTDIR staging.

    DESTDIR is prepended to installed artifact paths and to nothing else. It is
    never prepended to a user's config, data, state, cache or runtime roots, and
    staging creates none of them: a package that created ~/.local/share/notrios
    at build time would ship one build machine's idea of a user directory to
    every user who installed it.
    """

    prefix: str
    exec_prefix: str
    bindir: str
    datarootdir: str
    datadir: str
    destdir: str = ""

    @classmethod
    def resolve(cls, environ: dict[str, str]) -> "Directories":
        home = environ.get("HOME", "").strip()
        default_prefix = os.path.join(home, ".local") if home else ""
        prefix = environ.get("prefix", "").strip() or default_prefix
        if not prefix:
            raise LifecycleError(
                "no prefix and no HOME, so there is nowhere to install.\n"
                "Pass one explicitly: make install prefix=/some/where"
            )
        exec_prefix = environ.get("exec_prefix", "").strip() or prefix
        datarootdir = environ.get("datarootdir", "").strip() or os.path.join(prefix, "share")
        return cls(
            prefix=prefix,
            exec_prefix=exec_prefix,
            bindir=environ.get("bindir", "").strip() or os.path.join(exec_prefix, "bin"),
            datarootdir=datarootdir,
            datadir=environ.get("datadir", "").strip() or datarootdir,
            destdir=environ.get("DESTDIR", "").strip(),
        )

    def staged(self, path: str) -> str:
        """Apply DESTDIR to an installed artifact path."""
        if not self.destdir:
            return path
        return os.path.join(self.destdir, path.lstrip(os.sep))


# ------------------------------------------------------------------ artifacts


@dataclass
class Artifact:
    path: str          # the final installed path, before DESTDIR
    kind: str          # executable | file | tree
    source: str        # a path in the checkout, or "generated"
    required: bool = True
    generator: str = ""


def planned_artifacts(dirs: Directories) -> list[Artifact]:
    """The artifact list from LAYOUT.json, resolved against these directories.

    `bin/notrios` is optional because it is the only artifact that needs GTK and
    WebKit to build. A headless machine can install and run the service and CLI,
    and refusing the whole install because a desktop binary is missing would
    make dogfooding impossible exactly where it is most convenient.
    """
    share = os.path.join(dirs.datadir, "notrios")
    return [
        Artifact(os.path.join(dirs.bindir, "notrios"), "executable", "bin/notrios", required=False),
        Artifact(os.path.join(dirs.bindir, "notriosd"), "executable", "bin/notriosd"),
        Artifact(os.path.join(dirs.bindir, "notriosctl"), "executable", "bin/notriosctl"),
        Artifact(os.path.join(share, "web"), "tree", "web/dist"),
        Artifact(os.path.join(share, "docs"), "tree", "docs"),
        Artifact(os.path.join(dirs.datarootdir, "applications", "notrios.desktop"),
                 "file", "generated", generator="desktop"),
        # The icon theme, one tree rather than one file per size: hicolor wants
        # <size>x<size>/apps/notrios.png and a desktop picks the size it needs.
        Artifact(os.path.join(dirs.datarootdir, "icons", "hicolor"),
                 "icon-theme", "assets/icons", required=False),
    ]


def desktop_entry(bindir: str) -> str:
    """The XDG protocol handler, pointed at the installed binary.

    It matches cmd/notriosctl's generated entry; the difference is only that
    this one names the installed path rather than wherever the caller ran from.
    """
    command = os.path.join(bindir, "notriosctl") + " open --launch"
    return (
        "[Desktop Entry]\n"
        "Type=Application\n"
        "Name=Notrios stable link handler\n"
        "Comment=Open notrios:// links in Notrios\n"
        # A bare name, not a path: the icon theme resolves it across sizes from
        # hicolor, so the entry does not have to guess which size a desktop
        # wants or where the theme lives.
        "Icon=notrios\n"
        f"Exec={command} %u\n"
        "Terminal=false\n"
        "NoDisplay=true\n"
        "MimeType=x-scheme-handler/notrios;\n"
    )


# -------------------------------------------------------------------- helpers


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for chunk in iter(lambda: stream.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def copy_file(source: str, destination: str, mode: int) -> None:
    os.makedirs(os.path.dirname(destination), mode=0o755, exist_ok=True)
    shutil.copyfile(source, destination)
    os.chmod(destination, mode)


def write_file(destination: str, content: str, mode: int) -> None:
    os.makedirs(os.path.dirname(destination), mode=0o755, exist_ok=True)
    with open(destination, "w", encoding="utf-8") as stream:
        stream.write(content)
    os.chmod(destination, mode)


def entry_for(path: str, kind: str) -> dict:
    info = os.lstat(path)
    return {
        "path": path,
        "kind": kind,
        "sha256": sha256_file(path),
        "mode": stat.S_IMODE(info.st_mode) & 0o7777,
        "size": info.st_size,
    }


def product_version() -> str:
    """Read the version from source rather than running a binary.

    install may run before anything is built, and shelling out to a binary to
    learn what version is being installed would make the manifest depend on the
    thing it is describing.
    """
    source = os.path.join(ROOT, "internal", "version", "version.go")
    pattern = re.compile(r'^\s*(?:const\s+)?Version\s*=\s*"([^"]+)"')
    with open(source, encoding="utf-8") as stream:
        for line in stream:
            found = pattern.match(line)
            if found:
                return found.group(1)
    raise LifecycleError(f"cannot read the product version from {source}")


# -------------------------------------------------------------------- install


def run_install(dirs: Directories, dry_run: bool) -> dict:
    artifacts = planned_artifacts(dirs)
    entries: list[dict] = []
    actions: list[str] = []

    for artifact in artifacts:
        target = dirs.staged(artifact.path)
        if artifact.source == "generated":
            actions.append(f"generate {target}")
            if not dry_run:
                write_file(target, desktop_entry(dirs.bindir), 0o644)
                entries.append(entry_for(target, artifact.kind))
            continue

        source = os.path.join(ROOT, artifact.source)
        if not os.path.exists(source):
            if artifact.required:
                raise LifecycleError(
                    f"{artifact.source} is missing, so there is nothing to install.\n"
                    "Build first: make build web"
                )
            actions.append(f"skip {target} ({artifact.source} not built)")
            continue

        if artifact.kind == "icon-theme":
            # assets/icons/<size>x<size>/notrios.png becomes
            # hicolor/<size>x<size>/apps/notrios.png.
            actions.append(f"install icon theme {source}/ -> {target}/")
            if not dry_run:
                for entry in sorted(os.listdir(source)):
                    icon = os.path.join(source, entry, "notrios.png")
                    if not os.path.isfile(icon):
                        continue
                    placed = os.path.join(target, entry, "apps", "notrios.png")
                    copy_file(icon, placed, 0o644)
                    entries.append(entry_for(placed, "file"))
            continue

        if artifact.kind == "tree":
            actions.append(f"install tree {source}/ -> {target}/")
            if not dry_run:
                if os.path.isdir(target):
                    shutil.rmtree(target)
                shutil.copytree(source, target)
                for walk_root, _, files in os.walk(target):
                    for name in sorted(files):
                        full = os.path.join(walk_root, name)
                        os.chmod(full, 0o644)
                        entries.append(entry_for(full, "file"))
            continue

        mode = 0o755 if artifact.kind == "executable" else 0o644
        actions.append(f"install {source} -> {target}")
        if not dry_run:
            copy_file(source, target, mode)
            entries.append(entry_for(target, artifact.kind))

    manifest = {
        "schema": MANIFEST_SCHEMA,
        "version": product_version(),
        "installed_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "prefix": dirs.prefix,
        "exec_prefix": dirs.exec_prefix,
        "bindir": dirs.bindir,
        "datarootdir": dirs.datarootdir,
        "datadir": dirs.datadir,
        "entries": sorted(entries, key=lambda item: item["path"]),
    }
    manifest_path = dirs.staged(manifest_location(dirs))
    actions.append(f"write {manifest_path}")
    if not dry_run:
        write_file(manifest_path, json.dumps(manifest, indent=2) + "\n", 0o644)
    return {"manifest": manifest, "actions": actions, "manifest_path": manifest_path}


def manifest_location(dirs: Directories) -> str:
    return os.path.join(dirs.datadir, "notrios", MANIFEST_NAME)


# ------------------------------------------------------------------ uninstall


@dataclass
class Disposition:
    """What uninstall decided about one manifest entry, and why."""

    path: str
    action: str   # remove | absent | preserve | refuse
    reason: str = ""


def load_manifest(dirs: Directories) -> dict:
    path = dirs.staged(manifest_location(dirs))
    if not os.path.isfile(path):
        raise LifecycleError(
            f"no install manifest at {path}.\n"
            "Nothing is recorded as installed here, and uninstall removes only what a\n"
            "manifest says it installed. If you installed with a different prefix, pass\n"
            "the same one: make uninstall prefix=/some/where"
        )
    with open(path, encoding="utf-8") as stream:
        manifest = json.load(stream)
    if manifest.get("schema") != MANIFEST_SCHEMA:
        raise LifecycleError(
            f"{path} records format {manifest.get('schema')!r} and this build understands "
            f"{MANIFEST_SCHEMA!r}.\nRefusing to delete anything described by a manifest it "
            "cannot read."
        )
    return manifest


def install_roots(manifest: dict, dirs: Directories) -> list[str]:
    """The only directories uninstall will delete inside.

    Taken from the manifest rather than from the current environment, so a
    manifest cannot be pointed at a different tree by changing a variable
    between install and uninstall.
    """
    roots = []
    for key in ("bindir", "datadir", "datarootdir"):
        value = str(manifest.get(key, "")).strip()
        if value:
            roots.append(os.path.realpath(dirs.staged(value)))
    return roots


def within_any(path: str, roots: list[str]) -> bool:
    """Containment decided on the resolved path.

    The same rule the purge oracle uses, and for the same reason: a symlink can
    look contained while pointing somewhere else entirely.
    """
    resolved = os.path.realpath(path)
    for root in roots:
        if resolved == root:
            return True
        if resolved.startswith(root.rstrip(os.sep) + os.sep):
            return True
    return False


def classify(entry: dict, roots: list[str], dirs: Directories) -> Disposition:
    path = dirs.staged(entry["path"])

    if not roots:
        return Disposition(path, "refuse", "the manifest declares no install roots")

    # The symlink check comes before containment, and the order is about the
    # message rather than safety. Containment resolves the path, so a symlink
    # pointing anywhere outside is refused by that rule too -- two rules catching
    # one case, which is what defence in depth looks like. But it reports
    # "resolves outside every install root", which sends the reader looking for a
    # problem with their prefix instead of at the link sitting in front of them.
    if os.path.islink(path):
        # Never follow one. Something replaced an installed file with a link,
        # and deleting through it would remove whatever it points at.
        return Disposition(path, "preserve", "is now a symbolic link, not the file that was installed")

    if not within_any(path, roots):
        return Disposition(path, "refuse", "resolves outside every install root in the manifest")
    if not os.path.exists(path):
        return Disposition(path, "absent", "already gone")
    if not os.path.isfile(path):
        return Disposition(path, "preserve", "is no longer a regular file")

    try:
        current = sha256_file(path)
    except OSError as error:
        return Disposition(path, "preserve", f"cannot be read to confirm it is ours ({error})")
    if current != entry.get("sha256"):
        return Disposition(path, "preserve", "has been modified since it was installed")
    return Disposition(path, "remove", "")


def prune_empty_directories(dirs: Directories) -> list[str]:
    """Remove directories this install owns once they are empty.

    Only the private `<datadir>/notrios` subtree. `bin` and `applications` are
    shared with everything else the user has installed, so an empty one is not
    ours to remove.
    """
    removed = []
    private = dirs.staged(os.path.join(dirs.datadir, "notrios"))
    # os.walk's directory list is captured before the children are visited, so
    # after removing a child the parent still looks occupied by it. Read the
    # directory again at the moment of the decision instead: bottom-up, the
    # parent really is empty by the time its turn comes.
    for walk_root, _, _ in os.walk(private, topdown=False):
        try:
            if os.listdir(walk_root):
                continue
            os.rmdir(walk_root)
            removed.append(walk_root)
        except OSError:
            continue
    return removed


def run_uninstall(dirs: Directories, dry_run: bool) -> dict:
    manifest = load_manifest(dirs)
    roots = install_roots(manifest, dirs)
    dispositions = [classify(entry, roots, dirs) for entry in manifest.get("entries", [])]

    if not dry_run:
        for disposition in dispositions:
            if disposition.action == "remove":
                try:
                    os.remove(disposition.path)
                except FileNotFoundError:
                    disposition.action = "absent"
                    disposition.reason = "already gone"

    manifest_path = dirs.staged(manifest_location(dirs))
    pruned: list[str] = []
    if not dry_run:
        # The manifest goes last: it is the record of what is still owed, and a
        # run interrupted before this point can simply be repeated.
        if not any(d.action == "preserve" for d in dispositions):
            try:
                os.remove(manifest_path)
            except FileNotFoundError:
                pass
            pruned = prune_empty_directories(dirs)
    return {"dispositions": dispositions, "manifest_path": manifest_path,
            "pruned": pruned, "manifest": manifest}


# ---------------------------------------------------------------------- purge


PURGE_BACKUP_DIRNAME = "notrios-purge-backups"


def flag(name: str, environ: dict[str, str]) -> bool:
    """Read a lifecycle flag, accepting only unset or exactly "1".

    Anything else is refused rather than guessed at. `NO_BACKUP=0`, `FORCE=no`
    and `DRYRUN=true` all read to a human as "off", "off" and "on", and a
    deletion that turns on because a value was interpreted generously is the one
    mistake this whole target is built to avoid.
    """
    raw = environ.get(name)
    if raw is None or raw == "":
        return False
    if raw == "1":
        return True
    raise LifecycleError(
        f"{name}={raw!r} is not a value this understands. Use {name}=1 to turn it on,\n"
        f"or leave it unset. It is refused rather than guessed at because guessing wrong\n"
        "here deletes a library."
    )


def environ_home() -> str:
    home = os.environ.get("HOME", "").strip()
    return home if home and os.path.isdir(home) else ""


def resolved_roots(dirs: Directories) -> dict[str, str]:
    """Ask the installed Notrios where its roots are.

    Not computed here. H3's first finding was two disagreeing resolvers, and
    adding a third -- in Python, in the tool that deletes things -- is how the
    purge target comes to remove a directory the application never used. The
    installed binary is the authority on its own layout.
    """
    binary = os.path.join(dirs.bindir, "notriosctl")
    if not os.path.isfile(binary):
        raise LifecycleError(
            f"{binary} is not there, so nothing can say which roots this installation uses.\n"
            "Purge refuses rather than resolving them itself: a second opinion about where\n"
            "your notes live is exactly what must not decide a deletion.\n"
            "Install first, or purge with the prefix you installed to."
        )
    import subprocess  # local: nothing else here shells out

    # Run it from outside the checkout. `make purge` runs with the repository as
    # the working directory, and Notrios treats a checkout it is standing in as a
    # source instance -- so asking the installed binary from here answered with
    # the *checkout's* ./data roots rather than the user's. The oracle refused
    # them for being relative, which is how this was found, but a purge that asks
    # the wrong instance where the notes are has already failed by the time
    # anything protects it.
    elsewhere = environ_home() or os.sep
    completed = subprocess.run(
        [binary, "paths", "--json", "--no-redact"],
        capture_output=True, text=True, check=False, cwd=elsewhere,
    )
    if completed.returncode != 0:
        raise LifecycleError(
            f"{binary} could not report its paths:\n{completed.stderr.strip()}"
        )
    payload = json.loads(completed.stdout)
    return {name: value for name, value in payload.get("roots", {}).items() if value}


@dataclass
class PurgeStep:
    """One root and what purge decided to do with it."""

    category: str
    path: str
    policy: str
    action: str      # backup_then_delete | dispose | keep | refuse
    reason: str = ""
    bytes: int = 0
    files: int = 0


def measure_tree(path: str) -> tuple[int, int]:
    files = 0
    total = 0
    for walk_root, _, names in os.walk(path):
        for name in names:
            full = os.path.join(walk_root, name)
            try:
                total += os.lstat(full).st_size
                files += 1
            except OSError:
                continue
    return files, total


def plan_purge(dirs: Directories, environ: dict[str, str]) -> list[PurgeStep]:
    """Decide, for every resolved root, what purge would do.

    Every deletion candidate goes through H3's oracle. Nothing is deleted
    because this file thinks it looks like a Notrios directory.
    """
    roots = resolved_roots(dirs)
    owned = [os.path.realpath(path) for path in roots.values()]
    env = purge_oracle.Environment(
        owned_roots=owned,
        home=environ.get("HOME", ""),
        external_profile_paths=[],
    )

    steps: list[PurgeStep] = []
    for category in sorted(roots):
        path = roots[category]
        policy = purge_oracle.backup_policy(category)

        if category == "program_assets":
            # With H3's recommended prefix the installed artifacts land in
            # $(datadir)/notrios, which on Linux is the same directory as the
            # XDG data root. That overlap is worth naming rather than leaving
            # for a reader to notice two lines about one path: the manifest
            # removes the installed files, and the data step removes what is
            # left, so nothing is missed and nothing is deleted twice.
            overlapping = [name for name, other in roots.items()
                           if name != category and os.path.realpath(other) == os.path.realpath(path)]
            reason = "not a data root; removed by whatever installed it"
            if overlapping:
                reason += f"; shares a directory with the {', '.join(sorted(overlapping))} root"
            steps.append(PurgeStep(category, path, policy, "keep", reason))
            continue

        decision = purge_oracle.decide(path, env)
        if decision.verdict == purge_oracle.REFUSE:
            steps.append(PurgeStep(category, path, policy, "refuse",
                                   f"{decision.reason} [{decision.rule}]"))
            continue

        files, total = (0, 0)
        if decision.verdict == purge_oracle.ALLOW:
            files, total = measure_tree(path)

        action = "dispose" if policy == "dispose" else "backup_then_delete"
        reason = "" if decision.verdict == purge_oracle.ALLOW else "already absent"
        steps.append(PurgeStep(category, path, policy, action, reason, total, files))
    return steps


def backup_destination(roots: dict[str, str], now: float) -> str:
    """Where the purge backup goes.

    Beside the state root rather than inside any root purge removes. H3 proved
    this container and asserted the destination is itself refused by the oracle,
    so the backup cannot land somewhere the same run would delete.
    """
    state = roots.get("state", "")
    if not state:
        raise LifecycleError("no state root resolved, so there is nowhere safe to put a backup")
    parent = os.path.dirname(os.path.realpath(state))
    stamp = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime(now))
    return os.path.join(parent, PURGE_BACKUP_DIRNAME, stamp)


# Sync key material is the one thing a purge backup must not contain. The
# backup is an ordinary tar in a place chosen for convenience, and this file is
# the password to a library's synchronized traffic -- copying it there would put
# the key beside the lock. The sealed form is excluded too: on its own it is
# ciphertext, but the data key that opens it lives in the operating system's
# store and survives a purge, so the pair would be recoverable.
SYNC_KEY_FILENAMES = ("sync-keys.json",)
SYNC_KEY_PREFIX = "sync-keys-"


def is_sync_key_material(name: str) -> bool:
    """Whether a file name is a library's sync key material.

    Matched by name rather than by reading the file: an unreadable or
    unrecognised file that is named like key material is still excluded, which
    is the safe direction to be wrong in.
    """
    base = os.path.basename(name)
    return base in SYNC_KEY_FILENAMES or (base.startswith(SYNC_KEY_PREFIX) and base.endswith(".json"))


def create_purge_backup(steps: list[PurgeStep], destination: str) -> dict:
    """One owner-only tar plus a manifest, written before anything is deleted.

    The format is H3's, proven by its restore test: a per-file SHA-256 inventory
    and a hash of the archive itself, both created 0600 from the start rather
    than chmod-ed afterwards, because a file that is briefly world-readable
    while it holds someone's notes was briefly wrong.
    """
    import tarfile  # local: only purge needs it

    # makedirs applies its mode to the last component only, so the parent would
    # be created with whatever the umask allows -- 0775 here. The archive inside
    # is 0600 and so its contents were never exposed, but a readable parent still
    # publishes that this user has backups and when they were taken. Both levels
    # are created owner-only, and an existing parent is tightened.
    container = os.path.dirname(destination)
    os.makedirs(container, mode=0o700, exist_ok=True)
    os.chmod(container, 0o700)
    os.makedirs(destination, mode=0o700, exist_ok=True)
    os.chmod(destination, 0o700)
    archive = os.path.join(destination, "backup.tar")
    inventory: list[dict] = []

    excluded: list[str] = []

    def without_key_material(info):
        if is_sync_key_material(info.name):
            excluded.append(info.name)
            return None
        return info

    handle = os.open(archive, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(handle, "wb") as raw, tarfile.open(fileobj=raw, mode="w") as tar:
        for step in steps:
            if step.action != "backup_then_delete" or not os.path.isdir(step.path):
                continue
            tar.add(step.path, arcname=step.category, filter=without_key_material)
            for walk_root, _, names in os.walk(step.path):
                for name in sorted(names):
                    full = os.path.join(walk_root, name)
                    if not os.path.isfile(full) or os.path.islink(full):
                        continue
                    if is_sync_key_material(full):
                        continue
                    relative = os.path.relpath(full, step.path)
                    inventory.append({
                        "category": step.category,
                        "member": os.path.join(step.category, relative),
                        "sha256": sha256_file(full),
                        "size": os.lstat(full).st_size,
                    })

    manifest = {
        "schema": "notrios.purge-backup/1",
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "categories": sorted({item["category"] for item in inventory}),
        "entries": inventory,
        # Named rather than merely counted: a user who wanted to keep their sync
        # identity needs to know exactly what was left out, and a file name is
        # not secret material. What it held is.
        "excluded_key_material": sorted(excluded),
        "archive_sha256": sha256_file(archive),
    }
    manifest_path = os.path.join(destination, "MANIFEST.json")
    handle = os.open(manifest_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(handle, "w", encoding="utf-8") as stream:
        json.dump(manifest, stream, indent=2)
        stream.write("\n")
    return manifest


def verify_purge_backup(destination: str) -> tuple[bool, str]:
    """Confirm the backup before anything is deleted.

    A failed verification is the only thing standing between the user and the
    deletion, so it checks the archive's own hash, that every recorded file is
    actually a member, and that the archive opens.
    """
    import tarfile

    archive = os.path.join(destination, "backup.tar")
    manifest_path = os.path.join(destination, "MANIFEST.json")
    if not os.path.isfile(archive) or not os.path.isfile(manifest_path):
        return False, "the backup is incomplete"
    with open(manifest_path, encoding="utf-8") as stream:
        manifest = json.load(stream)
    if sha256_file(archive) != manifest.get("archive_sha256"):
        return False, "the archive does not match the hash recorded when it was written"
    try:
        with tarfile.open(archive) as tar:
            members = {member.name for member in tar.getmembers()}
    except tarfile.TarError as error:
        return False, f"the archive cannot be read: {error}"
    for entry in manifest["entries"]:
        if entry["member"] not in members:
            return False, f"{entry['member']} is recorded but not in the archive"
    # Checked against the archive itself rather than trusting that the filter
    # ran. An exclusion nothing verifies is an intention, and this one is a
    # boundary: sync key material must never reach a purge backup.
    leaked = sorted(name for name in members if is_sync_key_material(name))
    if leaked:
        return False, ("the archive contains sync key material, which must never be backed up: "
                       + ", ".join(leaked))
    detail = f"{len(manifest['entries'])} files verified"
    excluded = manifest.get("excluded_key_material") or []
    if excluded:
        detail += f"; {len(excluded)} sync key file(s) deliberately excluded"
    return True, detail


def confirm_purge(prompt: str, environ: dict[str, str], force: bool) -> bool:
    """Ask before deleting, and fail closed when nobody can answer.

    A non-interactive purge without FORCE=1 refuses. It does not hang waiting
    for a terminal that is not there, and it does not proceed on the grounds
    that silence is agreement -- either would turn an unattended script into an
    accidental deletion.
    """
    if force:
        return True
    if not sys.stdin.isatty():
        raise LifecycleError(
            "purge needs an affirmative answer and there is no terminal to ask.\n"
            "Nothing was deleted. Run it interactively, or pass FORCE=1 if you are\n"
            "automating it deliberately -- FORCE=1 skips the question, never the backup."
        )
    print(prompt, end="", flush=True)
    try:
        answer = sys.stdin.readline()
    except KeyboardInterrupt:
        return False
    if answer == "":
        # EOF rather than an answer.
        raise LifecycleError("purge saw end-of-input instead of an answer. Nothing was deleted.")
    return answer.strip() == "PURGE"


def describe_plan(steps: list[PurgeStep], uninstall_result: dict | None, destination: str,
                  no_backup: bool) -> None:
    print("Installed files:")
    if uninstall_result is None:
        print("  none removed here; the program is owned by your package manager")
    else:
        dispositions = uninstall_result["dispositions"]
        removable = sum(1 for d in dispositions if d.action == "remove")
        print(f"  {removable} files listed in {uninstall_result['manifest_path']}")
        for disposition in dispositions:
            if disposition.action in ("preserve", "refuse"):
                print(f"  {disposition.action.upper():8} {disposition.path}: {disposition.reason}")

    print("\nMutable roots:")
    for step in steps:
        if step.action == "backup_then_delete":
            detail = f"{step.files} files, {step.bytes} bytes"
            print(f"  BACK UP AND DELETE  {step.path}  ({step.category}; {detail})")
        elif step.action == "dispose":
            print(f"  DELETE WITHOUT BACKUP  {step.path}  ({step.category}; rebuildable)")
        elif step.action == "keep":
            print(f"  KEEP    {step.path}  ({step.category}; {step.reason})")
        else:
            print(f"  REFUSED {step.path}  ({step.category}; {step.reason})")

    print("\nBackup:")
    if no_backup:
        print("  NONE. NO_BACKUP=1 was set.")
    else:
        print(f"  {destination}")
        print("  written and verified before anything is deleted")

    # Said before the confirmation rather than after the deletion. A user who
    # wants to keep their sync identity has exactly one chance to copy it, and
    # it is now.
    key_files = sorted(find_sync_key_material(steps))
    print("\nSync keys:")
    if not key_files:
        print("  none found; this library has no sync key material")
    else:
        for path in key_files:
            print(f"  NOT BACKED UP, THEN DELETED  {path}")
        print("  Sync key material is never copied into a backup: the backup is an ordinary")
        print("  archive, and this is the password to your synchronized traffic. Copy it")
        print("  somewhere you trust first if you want this library's sync identity back.")
        print("  A data key held by your operating system's credential store is not removed")
        print("  by purge; remove it there if you want nothing left behind.")


def find_sync_key_material(steps: list[PurgeStep]) -> list[str]:
    """Every sync key file inside the roots this purge would remove."""
    found: list[str] = []
    for step in steps:
        if step.action not in ("backup_then_delete", "dispose") or not os.path.isdir(step.path):
            continue
        for walk_root, _, names in os.walk(step.path):
            for name in names:
                full = os.path.join(walk_root, name)
                if os.path.isfile(full) and not os.path.islink(full) and is_sync_key_material(full):
                    found.append(full)
    return found


def irreversible_warning(steps: list[PurgeStep]) -> str:
    categories = ", ".join(step.category for step in steps
                           if step.action == "backup_then_delete") or "none"
    return (
        "\n"
        "!! NO_BACKUP=1: nothing will be copied anywhere before it is deleted.\n"
        "!!\n"
        f"!! This permanently destroys the {categories} roots, which hold your notes\n"
        "!! database, your attachments, your configuration, your profile registry and\n"
        "!! generated profile configs, your sync key material, your sync spools and\n"
        "!! backups, the catch-up inbox, and the remote-media quarantine.\n"
        "!!\n"
        "!! There is no undo and no copy to restore from. If you want one, run this\n"
        "!! again without NO_BACKUP=1.\n"
    )


def run_purge(dirs: Directories, environ: dict[str, str]) -> int:
    dry_run = flag("DRYRUN", environ)
    force = flag("FORCE", environ)
    no_backup = flag("NO_BACKUP", environ)

    roots = resolved_roots(dirs)
    steps = plan_purge(dirs, environ)
    destination = backup_destination(roots, time.time())

    # The backup must not land anywhere this same run would delete. H3 proved
    # the property; this asserts it every time rather than trusting the layout.
    guard = purge_oracle.Environment(
        owned_roots=[os.path.realpath(path) for path in roots.values()],
        home=environ.get("HOME", ""),
    )
    verdict = purge_oracle.decide(destination, guard)
    if verdict.verdict != purge_oracle.REFUSE:
        raise LifecycleError(
            f"the backup destination {destination} is not refused by the purge oracle\n"
            f"({verdict}), which means this run could delete its own backup. Refusing."
        )

    # A packaged install has no manifest, and that is correct rather than
    # broken: dpkg owns the file list for a package and records its own
    # md5sums, so shipping a second ownership record would be the drift H6a
    # warned about. Purge still has a job here -- the data half -- so it does
    # that half and says who owns the other one.
    uninstall_result = None
    try:
        uninstall_result = run_uninstall(dirs, dry_run=True)
    except LifecycleError as error:
        if "no install manifest" not in str(error):
            raise
        print("No install manifest here, so this is a packaged install or was never\n"
              "installed by `make install`. Purge will remove your data and leave the\n"
              "program alone; remove the program with your package manager.\n")

    describe_plan(steps, uninstall_result, destination, no_backup)

    if dry_run:
        print("\nDRYRUN=1: nothing was written, nothing was deleted, nothing was asked.")
        return 0

    if no_backup:
        print(irreversible_warning(steps))

    prompt = ("\nType PURGE to delete the roots listed above"
              + (" WITHOUT A BACKUP" if no_backup else "")
              + ": ")
    if not confirm_purge(prompt, environ, force):
        print("Nothing was deleted.")
        return 1

    if not no_backup:
        print(f"\nwriting backup to {destination}")
        create_purge_backup(steps, destination)
        ok, detail = verify_purge_backup(destination)
        if not ok:
            raise LifecycleError(
                f"the backup could not be verified: {detail}.\n"
                "Nothing was deleted. The partial backup is left at\n"
                f"  {destination}\n"
                "so you can see what it did contain."
            )
        print(f"backup verified: {detail}")

    if uninstall_result is not None:
        run_uninstall(dirs, dry_run=False)

    for step in steps:
        if step.action not in ("backup_then_delete", "dispose"):
            continue
        if not os.path.isdir(step.path):
            continue
        shutil.rmtree(step.path)
        print(f"deleted {step.path}")

    if not no_backup:
        print(f"\nYour data is in {destination} until you remove it. Nothing deletes it for you.")
    return 0


# ----------------------------------------------------------------------- main


def report_uninstall(result: dict, dry_run: bool) -> int:
    dispositions = result["dispositions"]
    counts: dict[str, int] = {}
    for disposition in dispositions:
        counts[disposition.action] = counts.get(disposition.action, 0) + 1

    verb = "would remove" if dry_run else "removed"
    print(f"{verb} {counts.get('remove', 0)} files; "
          f"{counts.get('absent', 0)} already gone")

    refused = [d for d in dispositions if d.action == "refuse"]
    preserved = [d for d in dispositions if d.action == "preserve"]

    for disposition in refused:
        print(f"  REFUSED {disposition.path}: {disposition.reason}")
    for disposition in preserved:
        print(f"  kept    {disposition.path}: {disposition.reason}")

    if preserved:
        print("\nThe manifest was left in place because something it lists was kept.")
        print("Nothing installed is deleted twice, so uninstall can be repeated safely.")
    elif not dry_run:
        for directory in result["pruned"]:
            print(f"  removed empty {directory}")

    print("\nYour config, notes, profiles, state, backups and cache are untouched;")
    print("uninstall removes only what the manifest recorded installing.")
    if dry_run:
        print("Nothing was written.")
    return 1 if refused else 0


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description="Notrios install lifecycle")
    parser.add_argument("action", choices=["install", "uninstall", "purge", "manifest-path"])
    parser.add_argument("--dry-run", action="store_true",
                        help="print the exact actions and write nothing")
    arguments = parser.parse_args(argv)

    try:
        dirs = Directories.resolve(dict(os.environ))
        if arguments.action == "manifest-path":
            print(manifest_location(dirs))
            return 0

        if arguments.action == "install":
            result = run_install(dirs, arguments.dry_run)
            for action in result["actions"]:
                print(("would " if arguments.dry_run else "") + action)
            if arguments.dry_run:
                print("\nNothing was written.")
            else:
                print(f"\ninstalled {len(result['manifest']['entries'])} files; "
                      f"manifest {result['manifest_path']}")
                print(f"add {dirs.bindir} to PATH if it is not already there")
            return 0

        if arguments.action == "purge":
            return run_purge(dirs, dict(os.environ))

        return report_uninstall(run_uninstall(dirs, arguments.dry_run), arguments.dry_run)
    except LifecycleError as error:
        print(f"notrios lifecycle: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
