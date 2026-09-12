#!/usr/bin/env python3
"""Install, uninstall and purge an end-user Notrios from a checkout.

This implements the contract H3 investigated and recorded in
`performance/v0.8-h3/`: the artifact list and GNU directory variables in
`LAYOUT.json`, and the ownership manifest in the same file.

# Why purge delegates instead of deleting

`purge` has two halves. The program half -- removing what the install manifest
recorded writing -- belongs here, because the manifest is this script's own
record and a package manager owns its files instead. The data half belongs to
`notriosctl purge`, and is run from here rather than repeated here.

It was repeated here until v1.0 J3. A packaged installation ships no Makefile
and no scripts/, so the command had to exist; once it did, "delete a library
after taking a verified backup" existed twice, in two languages, and a
divergence between the two would be a purge that deletes without the backup
somebody was promised. The oracle halves were gated against each other by
fixtures. The backup and deletion halves were not gated by anything.

So the decision procedure, the measurement, the backup, the verification and the
deletion are all the command's, and this script contributes the half the command
cannot see: what `make install` wrote. It shows both, asks once about both, and
then runs the command with `--confirm`, which skips the question the command
would ask and nothing else.

That inverts the order. This script used to uninstall and then delete the data;
it now deletes the data and then uninstalls, because the binary that deletes the
data is one of the files uninstall removes.

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
import subprocess
import sys
import time
from dataclasses import dataclass

# The tree artifacts are copied *from*. It is the checkout this script lives in,
# unless NOTRIOS_LIFECYCLE_SOURCE names another one -- which lets a packager
# build in one tree and install from another, and lets the tests install from a
# fixture instead of requiring a built checkout. It changes only what is read;
# where things are written is still decided by the directory variables.
CHECKOUT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROOT = os.environ.get("NOTRIOS_LIFECYCLE_SOURCE", "").strip() or CHECKOUT

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


def run_uninstall(dirs: Directories, dry_run: bool, manifest: dict | None = None) -> dict:
    """Remove what the manifest recorded installing.

    `manifest` is passed in by purge and read from disk by everything else. The
    install manifest lives inside the data root -- it is a record about this
    user's installation, not about the machine -- so the data half of a purge
    deletes it along with everything else there. While uninstall ran first that
    was invisible. It stopped being invisible the moment purge started deleting
    the data first, which it must, because the binary that deletes the data is
    one of the files uninstall removes: the second call found no manifest and
    refused, leaving the installed files behind after the notes were gone.
    """
    if manifest is None:
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


def delegate_binary(dirs: Directories) -> str:
    """The `notriosctl` that does the data half of this purge.

    `make purge` used to plan, back up and delete by itself, which meant the
    dangerous half existed twice: here in Python for a checkout, and in Go for
    the packaged installation that ships no Makefile. Two implementations of
    "delete a library after taking a verified backup" is one implementation and
    one liability, because a divergence between them is a purge that deletes
    without the backup somebody was promised.

    So it asks the installed binary, which was already the authority on where
    the roots are. H3's first finding was two disagreeing resolvers; this is the
    same lesson applied to the deletion rather than to the lookup.
    """
    binary = os.path.join(dirs.bindir, "notriosctl")
    if not os.path.isfile(binary):
        raise LifecycleError(
            f"{binary} is not there, so nothing can say which roots this installation\n"
            "uses, and nothing can delete them. Purge refuses rather than doing it here:\n"
            "a second opinion about where your notes live -- and a second implementation\n"
            "of removing them -- is exactly what must not decide a deletion.\n"
            "Install first, or purge with the prefix you installed to."
        )
    return binary


def delegate(binary: str, arguments: list[str], capture: bool) -> subprocess.CompletedProcess:
    """Run `notriosctl purge` from outside the checkout.

    `make purge` runs with the repository as the working directory, and Notrios
    treats a checkout it is standing in as a source instance -- so asking the
    installed binary from here answered with the *checkout's* ./data roots
    rather than the user's. The oracle refused them for being relative, which is
    how this was found, but a purge that asks the wrong instance where the notes
    are has already failed by the time anything protects it.
    """
    elsewhere = environ_home() or os.sep
    return subprocess.run([binary, "purge", *arguments], cwd=elsewhere, check=False,
                          capture_output=capture, text=capture)


def delegated_plan(binary: str, no_backup: bool) -> dict:
    """What the command says it would do, before anything is asked or deleted."""
    arguments = ["--dry-run", "--json", "--no-redact"]
    if no_backup:
        arguments.append("--no-backup")
    completed = delegate(binary, arguments, capture=True)
    if completed.returncode != 0:
        raise LifecycleError(
            f"{binary} could not plan the purge:\n{completed.stderr.strip()}"
        )
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        raise LifecycleError(
            f"{binary} did not answer with a plan ({error}). Nothing was deleted."
        )


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


def describe_plan(steps: list[dict], uninstall_result: dict | None, destination: str,
                  no_backup: bool) -> None:
    """The combined plan: the program half from here, the data half from the command.

    `make purge` is the only place both halves are visible at once, so it is the
    only place that can ask one question about both. The steps are the command's
    own plan, read back from its JSON rather than worked out again here.
    """
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
        action = step.get("action", "")
        path = step.get("path", "")
        category = step.get("category", "")
        if action == "backup_then_delete":
            detail = f"{step.get('files', 0)} files, {step.get('bytes', 0)} bytes"
            print(f"  BACK UP AND DELETE  {path}  ({category}; {detail})")
        elif action == "dispose":
            print(f"  DELETE WITHOUT BACKUP  {path}  ({category}; rebuildable)")
        elif action == "keep":
            print(f"  KEEP    {path}  ({category}; {step.get('reason', '')})")
        else:
            print(f"  REFUSED {path}  ({category}; {step.get('reason', '')})")

    print("\nBackup:")
    if no_backup:
        print("  NONE. NO_BACKUP=1 was set.")
    else:
        print(f"  {destination}")
        print("  written and verified before anything is deleted")

    # Said before the confirmation rather than after the deletion. A user who
    # wants to keep their sync identity has exactly one chance to copy it, and
    # it is now. The list comes from the command's plan: the rule for what
    # counts as key material is the command's, so that this file cannot drift
    # into warning about a different set of files than the one it excludes.
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


def find_sync_key_material(steps: list[dict]) -> list[str]:
    """Every sync key file the command found inside the roots it would remove."""
    found: list[str] = []
    for step in steps:
        if step.get("action") not in ("backup_then_delete", "dispose"):
            continue
        found.extend(step.get("sync_key_material") or [])
    return found


def irreversible_warning(steps: list[dict]) -> str:
    categories = ", ".join(step.get("category", "") for step in steps
                           if step.get("action") == "backup_then_delete") or "none"
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
    """Delete this user's data, then remove what the install manifest recorded.

    The order is the interesting part, and it is the opposite of what it was.
    While both halves lived in this file, uninstalling first and deleting the
    data second was fine. It is not fine now: the thing that deletes the data is
    a program file, and removing the program first would take the binary this is
    about to run. Data, then program.
    """
    dry_run = flag("DRYRUN", environ)
    force = flag("FORCE", environ)
    no_backup = flag("NO_BACKUP", environ)

    binary = delegate_binary(dirs)
    plan = delegated_plan(binary, no_backup)
    steps = plan.get("steps") or []
    destination = plan.get("backup") or ""

    # The backup destination is checked by the command, against the same oracle,
    # on the run that writes it. It is not re-checked here: a second opinion
    # about whether a deletion is safe is the thing this delegation removes.

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

    # One question, asked here, covering both halves -- because this is the only
    # place both are visible. The command is then run with --confirm, which
    # skips the question it would ask about the data half and nothing else.
    prompt = ("\nType PURGE to delete the roots listed above"
              + (" WITHOUT A BACKUP" if no_backup else "")
              + ": ")
    if not confirm_purge(prompt, environ, force):
        print("Nothing was deleted.")
        return 1

    arguments = ["--confirm", "--no-plan", "--no-redact"]
    if no_backup:
        arguments.append("--no-backup")
    else:
        # Passed rather than left to the command to choose again. The
        # destination carries a timestamp, so a second call a second later picks
        # a different directory -- and the path described above has to be the
        # path written to, or the sentence telling the user where their data is
        # names somewhere empty.
        arguments += ["--backup-dir", destination]

    completed = delegate(binary, arguments, capture=False)
    if completed.returncode != 0:
        print("\nThe data was not purged, so the installed files were left alone too.",
              file=sys.stderr)
        return completed.returncode

    if uninstall_result is not None:
        # The manifest read before the deletion, not read again after it: it
        # lived in the data root, which no longer exists.
        run_uninstall(dirs, dry_run=False, manifest=uninstall_result["manifest"])
        print("\nremoved the files the install manifest recorded installing.")
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
