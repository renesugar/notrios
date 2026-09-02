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

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, "performance", "v0.8-h3"))
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
    parser.add_argument("action", choices=["install", "uninstall", "manifest-path"])
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

        return report_uninstall(run_uninstall(dirs, arguments.dry_run), arguments.dry_run)
    except LifecycleError as error:
        print(f"notrios lifecycle: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
