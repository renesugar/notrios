#!/usr/bin/env python3
"""Run the installed-integration matrix and record what actually happened.

Every row is executed against a *packaged* installation in a disposable HOME --
not against the checkout, and not against the developer's own data. The point of
the matrix is to say what an installed Notrios does, so running it from a source
tree would answer a different question.

# The rule this file exists to enforce

A row is executed and carries a result, or it is postponed and carries a reason.
There is no third state. A row that could not run here does not become a pass
because the code "obviously" works, and a platform is not supported because its
rows were skipped -- H8's own boundary says a skipped row blocks the claim rather
than closing it.

Windows and macOS rows are defined and postponed. H7 moved to post-v1.0 because
the hardware is not available, and H6a measured why no local substitute exists:
CGO_ENABLED=0 does not build this project, so a cross-built artifact that has
never run proves nothing about the platform.
"""

from __future__ import annotations

import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LINUX, WINDOWS, MACOS = "linux", "windows", "macos"

POSTPONED_NATIVE = (
    "Needs the platform itself. H7 is deferred to post-v1.0 because the hardware "
    "is not available, and a cross-built artifact that has never run is not "
    "evidence that the platform works."
)


class Row:
    def __init__(self, identifier, area, description, platforms=(LINUX,), runner=None,
                 postponed=None):
        self.id = identifier
        self.area = area
        self.description = description
        self.platforms = platforms
        self.runner = runner
        self.postponed = postponed


class Harness:
    """A packaged installation in a throwaway HOME."""

    def __init__(self, workspace: str):
        self.workspace = workspace
        self.home = os.path.join(workspace, "home")
        self.prefix = os.path.join(workspace, "opt")
        os.makedirs(self.home)
        self.notes = []

    @property
    def bindir(self) -> str:
        return os.path.join(self.prefix, "bin")

    def install(self) -> None:
        environment = dict(os.environ, HOME=self.home, prefix=self.prefix)
        environment.pop("DESTDIR", None)
        subprocess.run([sys.executable, os.path.join(ROOT, "scripts/lifecycle.py"), "install"],
                       check=True, capture_output=True, text=True, env=environment)

    def run(self, *args, home=None, expect=None, timeout=120, env_extra=None):
        environment = {
            "HOME": home or self.home,
            "PATH": f"{self.bindir}:/usr/bin:/bin",
        }
        if env_extra:
            environment.update(env_extra)
        completed = subprocess.run(list(args), capture_output=True, text=True,
                                   env=environment, cwd=self.workspace, timeout=timeout)
        if expect is not None and completed.returncode != expect:
            raise AssertionError(
                f"{' '.join(args)} exited {completed.returncode}, expected {expect}\n"
                f"{completed.stdout}\n{completed.stderr}")
        return completed

    def cli(self, *args, **kwargs):
        return self.run(os.path.join(self.bindir, "notriosctl"), *args, **kwargs)

    def free_port(self) -> int:
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            return probe.getsockname()[1]

    def serve(self, port: int, home=None, extra=()):
        environment = {"HOME": home or self.home, "PATH": f"{self.bindir}:/usr/bin:/bin"}
        process = subprocess.Popen(
            [os.path.join(self.bindir, "notriosd"), "-addr", f"127.0.0.1:{port}", *extra],
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            env=environment, cwd=self.workspace)
        for _ in range(80):
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.25):
                    self.assert_serving_this_library(port)
                    return process
            except OSError:
                if process.poll() is not None:
                    return process
                time.sleep(0.25)
        return process

    def assert_serving_this_library(self, port: int) -> None:
        """Confirm the service opened *this* sandbox's database.

        Every row that starts a service gets this, because the failure it
        prevents is silent and convincing. Twice while building this matrix a
        command was run with the checkout as its working directory, so an
        installed binary resolved source mode and used the checkout's library
        while the service under test used the sandbox's -- and search then
        honestly reported nothing about a library nobody had written to. It read
        as a product defect and was reported as one.

        GET /api/v1/status answers it directly: it reports the open database path
        and its logical database_id, so a test never has to infer which library
        it is talking to.
        """
        response = subprocess.run(
            ["curl", "-s", f"http://127.0.0.1:{port}/api/v1/status"],
            capture_output=True, text=True, timeout=30).stdout
        try:
            status = json.loads(response)
        except json.JSONDecodeError as error:
            raise AssertionError(f"status did not return JSON: {response[:200]}") from error
        served = status.get("storage", {}).get("database_path", "")
        expected = os.path.realpath(os.path.join(self.home, ".local", "share", "notrios"))
        assert os.path.realpath(served).startswith(expected), (
            f"the service opened {served}, which is outside this harness's HOME "
            f"({expected}). A row that writes through the CLI and reads through this "
            "service would be comparing two different libraries.")
        self.database_id = status.get("database_info", {}).get("database_id", "")

    @staticmethod
    def stop(process) -> str:
        if process.poll() is None:
            process.send_signal(signal.SIGTERM)
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                process.kill()
        try:
            return process.communicate(timeout=5)[0] or ""
        except Exception:
            return ""


# --------------------------------------------------------------------- rows



def row_no_development_toolchain(h: Harness) -> str:
    """The installed CLI must not need the things that built it."""
    stripped = {"HOME": h.home, "PATH": h.bindir}   # no /usr/bin: no go, node, npm, cc
    completed = subprocess.run([os.path.join(h.bindir, "notriosctl"), "version"],
                               capture_output=True, text=True, env=stripped, timeout=60)
    assert completed.returncode == 0, completed.stderr
    for tool in ("go", "node", "npm", "cc", "gcc", "wails"):
        assert shutil.which(tool, path=h.bindir) is None, f"{tool} is inside the install"
    return f"ran with PATH={h.bindir} only; reported {completed.stdout.strip()}"


def row_loopback_only_listener(h: Harness) -> str:
    port = h.free_port()
    process = h.serve(port)
    try:
        listening = subprocess.run(["ss", "-ltnH"], capture_output=True, text=True).stdout
        # Columns are State, Recv-Q, Send-Q, Local, Peer. The peer column of a
        # listening socket is "0.0.0.0:*", meaning any peer may connect -- it is
        # not a wildcard bind, and reading the whole line fails a correct bind.
        locals_ = [line.split()[3] for line in listening.splitlines()
                   if len(line.split()) > 3 and line.split()[3].endswith(f":{port}")]
        assert locals_, f"nothing is listening on {port}"
        for local in locals_:
            assert local.startswith("127.0.0.1:") or local.startswith("[::1]:"), \
                f"bound beyond loopback: {local}"
        return f"local address {locals_[0]}; loopback only, {len(locals_)} socket(s)"
    finally:
        h.stop(process)


def row_port_collision_refusal(h: Harness) -> str:
    port = h.free_port()
    first = h.serve(port)
    try:
        assert first.poll() is None, "the first instance did not stay up"
        second = subprocess.Popen(
            [os.path.join(h.bindir, "notriosd"), "-addr", f"127.0.0.1:{port}"],
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            env={"HOME": h.home, "PATH": f"{h.bindir}:/usr/bin:/bin"}, cwd=h.workspace)
        output, _ = second.communicate(timeout=60)
        assert second.returncode not in (0, None), "the second instance did not fail"
        for expected in ("already in use", "Another Notrios instance", "-addr"):
            assert expected in output, f"the refusal does not mention {expected!r}:\n{output}"
        assert "-addr 127.0.0.1:8099" not in output, "advice names the development default"
        return "second instance refused, named the cause and a non-default port"
    finally:
        h.stop(first)


def row_multi_profile_isolation(h: Harness) -> str:
    """Two profiles must not share a database."""
    for name in ("work", "personal"):
        h.cli("profile", "create", "--name", name, "--listen", f"127.0.0.1:{h.free_port()}",
              expect=0, timeout=180)
    listed = h.cli("profile", "list", expect=0).stdout
    assert "work" in listed and "personal" in listed, listed
    databases = set()
    for name in ("work", "personal"):
        shown = h.cli("profile", "show", "--name", name, expect=0).stdout
        for line in shown.splitlines():
            if ".sqlite" in line:
                databases.add(line.split()[-1])
    assert len(databases) >= 2, f"profiles share a database: {databases}"
    return f"two profiles, {len(databases)} distinct databases, both listed"


def row_profile_collision_refusal(h: Harness) -> str:
    """A second profile on one database is refused rather than silently allowed."""
    port = h.free_port()
    h.cli("profile", "create", "--name", "first", "--listen", f"127.0.0.1:{port}",
          expect=0, timeout=180)
    again = h.cli("profile", "create", "--name", "second", "--listen", f"127.0.0.1:{port}",
                  timeout=180)
    assert again.returncode != 0, "a duplicate listen address was accepted"
    return f"duplicate listen address refused (exit {again.returncode})"


def row_url_handler_lifecycle(h: Harness) -> str:
    """Register the notrios:// handler, read it back, and leave it removable."""
    printed = h.cli("register-url-handler", expect=0).stdout
    assert "notrios" in printed, printed
    desktop = os.path.join(h.prefix, "share", "applications", "notrios.desktop")
    assert os.path.isfile(desktop), "install did not place a desktop entry"
    body = open(desktop, encoding="utf-8").read()
    assert "x-scheme-handler/notrios" in body, body
    assert "Icon=notrios" in body, "the entry names no icon"
    return "handler printed without --apply; installed entry declares the scheme and an icon"


def row_deep_link_resolution(h: Harness) -> str:
    """Create a note, link it, and resolve the link on this machine.

    The document is created through the REST API rather than found by searching
    a seeded library. Earlier versions of this row looked for an existing
    document by search and by graph export, and both were the wrong question:
    search over a CLI-seeded library returned no hits here, and the default
    collection's graph held nothing. Creating the note makes the row deterministic
    and tests the journey a stable link is actually for -- write a note, hand
    someone the link, have it open.

    The exit codes carry the meaning: the desktop protocol handler runs this
    command and has nothing else to tell "not our link" from "our link, nothing
    here".
    """
    port = h.free_port()
    service = h.serve(port)
    try:
        assert service.poll() is None, "the service did not start"
        created = subprocess.run(
            ["curl", "-s", "-X", "POST", f"http://127.0.0.1:{port}/api/v1/documents",
             "-H", "Content-Type: application/json",
             "-d", '{"title": "H8 integration note", "body": "written by the matrix"}'],
            capture_output=True, text=True, timeout=30).stdout
    finally:
        h.stop(service)
    try:
        document_id = json.loads(created)["id"]
    except (json.JSONDecodeError, KeyError) as error:
        raise AssertionError(f"creating a note did not return an id: {created[:200]}") from error

    # A link names a database, and the profile registry is how this machine
    # knows which one -- filled in explicitly, by design. Without it `open`
    # correctly answers unregistered_database.
    database = os.path.join(h.home, ".local", "share", "notrios", "notes.sqlite")
    h.cli("profile", "register", "--name", "installed", "--db", database, expect=0, timeout=120)

    link = json.loads(h.cli("link", document_id, expect=0).stdout)["stable_uri"]
    assert link.startswith("notrios://databases/"), f"unexpected link shape: {link!r}"

    resolved = h.cli("open", link)
    assert resolved.returncode == 0, f"a real link did not resolve: {resolved.stdout[:300]}"

    unknown = h.cli("open", link.rsplit("/", 1)[0] + "/doc_does_not_exist")
    assert unknown.returncode == 1, f"unknown document exited {unknown.returncode}, expected 1"

    malformed = h.cli("open", "not-a-notrios-uri")
    assert malformed.returncode == 2, f"malformed exited {malformed.returncode}, expected 2"
    return "created a note, linked it, resolved it (0); unknown is 1, malformed is 2"


def row_service_and_cli_agree_on_the_library(h: Harness) -> str:
    """The service and the CLI must resolve the same database.

    This is the property whose absence produced two false defect reports while
    this matrix was written. It is cheap to check and the failure it catches is
    otherwise indistinguishable from the feature being broken: writes land in
    one library and reads come from another, and every symptom points at the
    wrong place.
    """
    port = h.free_port()
    service = h.serve(port)          # serve() already asserts the path is ours
    try:
        response = subprocess.run(
            ["curl", "-s", f"http://127.0.0.1:{port}/api/v1/status"],
            capture_output=True, text=True, timeout=30).stdout
        status = json.loads(response)
    finally:
        h.stop(service)

    served = status["storage"]["database_path"]
    served_id = status["database_info"]["database_id"]

    # `config show --json` reports settings as a list of {key, origin, value},
    # not a mapping. The origin is worth carrying into the message: "resolved"
    # and "file" fail differently, and knowing which one a mismatch came from
    # says whether a configuration or the resolver is at fault.
    configured = json.loads(h.cli("config", "show", "--json", "--no-redact", expect=0).stdout)
    setting = next((entry for entry in configured.get("settings", [])
                    if entry.get("key") == "data.database_path"), None)
    assert setting, f"config show reported no data.database_path: {configured.get('settings')}"
    cli_path, origin = setting.get("value", ""), setting.get("origin", "?")
    assert cli_path, f"data.database_path is empty: {setting}"
    assert os.path.realpath(cli_path) == os.path.realpath(served), (
        f"the CLI resolves {cli_path} and the service opened {served}; a test writing "
        "through one and reading through the other would compare two libraries")
    return (f"both resolve {os.path.basename(served)} ({served_id[:16]}...), "
            f"CLI origin {origin}")


def row_help_notes_are_searchable(h: Harness) -> str:
    """Help is a notebook in the user's own library, so it must be searchable.

    That is the whole reason the documentation is seeded into a read-only
    notebook rather than shipped as a separate viewer: a user looks for help the
    way they look for anything else. Nothing tested it, and a change to seeding,
    projection or the FTS index could have taken it away silently.

    This row exists because of a false alarm rather than a defect. During H8 the
    behaviour looked broken, and the cause was the harness: `seed-help` had been
    run with the checkout as the working directory, so an installed binary
    resolved source mode and wrote the notes into the checkout's library, while
    the service under test correctly searched the sandbox's. Two databases, and
    search honestly reporting nothing in the one it was asked about. The test is
    worth having anyway -- the promise is real and was unguarded.
    """
    seeded = json.loads(h.cli("seed-help", expect=0, timeout=300).stdout)
    assert seeded["files_seen"] > 0, f"nothing was seeded: {seeded}"

    port = h.free_port()
    service = h.serve(port)
    try:
        assert service.poll() is None, "the service did not start"
        found = {}
        # Terms that appear in the shipped documentation and nowhere else in an
        # empty library, so a hit means the help notes are genuinely indexed.
        for term in ("Recoll", "notriosctl", "troubleshooting"):
            response = subprocess.run(
                ["curl", "-s", "-X", "POST", f"http://127.0.0.1:{port}/api/v1/search",
                 "-H", "Content-Type: application/json",
                 "-d", json.dumps({"query": term, "limit": 5})],
                capture_output=True, text=True, timeout=30).stdout
            try:
                hits = json.loads(response).get("hits", [])
            except json.JSONDecodeError as error:
                raise AssertionError(f"search for {term!r} returned {response[:200]}") from error
            found[term] = len(hits)
        missing = [term for term, count in found.items() if count == 0]
        assert not missing, (
            f"help notes are not searchable for {missing}: {found}. They live in the "
            "user's own database precisely so help is searchable like any other note.")
        return f"seeded {seeded['files_seen']} pages; search finds them: {found}"
    finally:
        h.stop(service)


def row_service_restart_preserves_data(h: Harness) -> str:
    h.cli("seed-help", expect=0, timeout=300)
    database = os.path.join(h.home, ".local", "share", "notrios", "notes.sqlite")
    before = os.path.getsize(database)
    for _ in range(2):
        port = h.free_port()
        process = h.serve(port)
        assert process.poll() is None, "the service did not start"
        h.stop(process)
    status = h.cli("doctor", expect=0).stdout
    assert "schema version" in status, status
    assert os.path.getsize(database) >= before, "the library shrank across restarts"
    return "two start/stop cycles; the library is intact and doctor passes"


def row_lifecycle_preserves_user_data(h: Harness) -> str:
    """make uninstall parity: the program goes, the library stays."""
    h.cli("seed-help", expect=0, timeout=300)
    database = os.path.join(h.home, ".local", "share", "notrios", "notes.sqlite")
    before = open(database, "rb").read()
    environment = dict(os.environ, HOME=h.home, prefix=h.prefix)
    subprocess.run([sys.executable, os.path.join(ROOT, "scripts/lifecycle.py"), "uninstall"],
                   check=True, capture_output=True, text=True, env=environment)
    assert not os.path.exists(os.path.join(h.bindir, "notriosd")), "uninstall left the binary"
    assert open(database, "rb").read() == before, "uninstall changed the library"
    h.install()   # put it back for later rows
    return "uninstall removed the program and left the library byte-identical"


def row_gui_starts_headless(h: Harness) -> str:
    """The desktop binary starts under a virtual display, or says why it cannot."""
    binary = os.path.join(h.bindir, "notrios")
    if not os.path.isfile(binary):
        raise AssertionError("no desktop binary was installed")
    port = h.free_port()
    # -no-gui runs the service and never exits, so this waits for the socket
    # rather than for the process. Waiting for the process is how the first
    # version of this row "failed": it timed out on a program doing its job.
    process = subprocess.Popen(
        ["xvfb-run", "-a", binary, "-no-gui", "-addr", f"127.0.0.1:{port}"],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
        env={"HOME": h.home, "PATH": f"{h.bindir}:/usr/bin:/bin", "DISPLAY": ":99"},
        cwd=h.workspace)
    try:
        for _ in range(120):
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.25):
                    return f"the desktop binary served on 127.0.0.1:{port} under xvfb"
            except OSError:
                if process.poll() is not None:
                    raise AssertionError(
                        f"exited {process.returncode}: {(process.communicate()[0] or '')[:300]}")
                time.sleep(0.25)
        raise AssertionError("did not begin serving within 30s")
    finally:
        h.stop(process)


def row_cleanup_audit(h: Harness) -> str:
    """After purge, the roots are gone and the backup is not."""
    h.cli("seed-help", expect=0, timeout=300)
    environment = dict(os.environ, HOME=h.home, prefix=h.prefix, FORCE="1")
    completed = subprocess.run(
        [sys.executable, os.path.join(ROOT, "scripts/lifecycle.py"), "purge"],
        capture_output=True, text=True, env=environment, timeout=600)
    assert completed.returncode == 0, completed.stderr
    for root in (".config/notrios", ".local/share/notrios", ".cache/notrios"):
        assert not os.path.exists(os.path.join(h.home, root)), f"{root} survived purge"
    backups = os.path.join(h.home, ".local", "state", "notrios-purge-backups")
    assert os.path.isdir(backups) and os.listdir(backups), "the backup is missing"
    return "roots removed; the verified backup survives outside them"


MATRIX = [
    Row("no-development-toolchain", "install", "installed CLI runs without go, node, npm or a compiler",
        runner=row_no_development_toolchain),
    Row("loopback-only-listener", "network", "the service binds 127.0.0.1 and never a wildcard",
        runner=row_loopback_only_listener),
    Row("port-collision-refusal", "network", "a second instance on a taken port refuses and explains",
        runner=row_port_collision_refusal),
    Row("multi-profile-isolation", "profiles", "two profiles keep separate databases",
        runner=row_multi_profile_isolation),
    Row("profile-collision-refusal", "profiles", "a duplicate listen address is refused",
        runner=row_profile_collision_refusal),
    Row("url-handler-lifecycle", "desktop", "the notrios:// handler is declared and printable",
        runner=row_url_handler_lifecycle),
    Row("deep-link-resolution", "desktop", "malformed and unresolved links exit 2 and 1",
        runner=row_deep_link_resolution),
    Row("service-cli-same-library", "install", "the service and the CLI resolve the same database",
        runner=row_service_and_cli_agree_on_the_library),
    Row("help-notes-searchable", "search", "seeded help notes are found by search like any other note",
        runner=row_help_notes_are_searchable),
    Row("service-restart", "lifecycle", "restarting the service preserves the library",
        runner=row_service_restart_preserves_data),
    Row("lifecycle-uninstall-parity", "lifecycle", "uninstall removes the program and keeps data",
        runner=row_lifecycle_preserves_user_data),
    Row("gui-headless-start", "desktop", "the desktop binary starts under a virtual display",
        runner=row_gui_starts_headless),
    Row("cleanup-audit", "lifecycle", "purge removes the roots and keeps the backup outside them",
        runner=row_cleanup_audit),

    # Rows defined and not executed. Each says what it needs, so it can be run
    # when that is available rather than quietly forgotten.
    Row("firewall-observation", "network", "no firewall rule is added or widened by install or run",
        postponed="Reading or changing firewall state needs root, and this harness "
                  "deliberately runs unprivileged. The related property that can be "
                  "checked without root is covered by loopback-only-listener."),
    Row("native-file-picker", "desktop", "a cancelled picker returns no path and no capability",
        postponed="The picker is a Wails runtime dialog driven by a person. Automating "
                  "it would test the automation rather than the dialog."),
    Row("package-upgrade-removal", "lifecycle", "dpkg upgrade, removal and reinstall keep the library",
        postponed="Executed in H6 against a packaged install rather than repeated here; "
                  "see performance/v0.8-h6/PACKAGE.json."),
    Row("windows-install-launch", "platform", "install, launch, upgrade, remove, preserve on Windows",
        platforms=(WINDOWS,), postponed=POSTPONED_NATIVE),
    Row("macos-install-launch", "platform", "install, launch, upgrade, remove, preserve on macOS",
        platforms=(MACOS,), postponed=POSTPONED_NATIVE),
]


def main() -> int:
    results = []
    workspace = tempfile.mkdtemp(prefix="notrios-h8-")
    harness = Harness(workspace)
    try:
        harness.install()
        for row in MATRIX:
            if row.postponed is not None:
                results.append({"id": row.id, "area": row.area, "platforms": list(row.platforms),
                                "status": "postponed", "reason": row.postponed})
                print(f"  postponed  {row.id}")
                continue
            started = time.time()
            try:
                detail = row.runner(harness)
                status, note = "passed", detail
            except Exception as error:                      # a failure is a result
                status, note = "failed", f"{type(error).__name__}: {error}"
            results.append({"id": row.id, "area": row.area, "platforms": list(row.platforms),
                            "status": status, "detail": note,
                            "seconds": round(time.time() - started, 1)})
            print(f"  {status:9}  {row.id}: {note[:110]}")
    finally:
        shutil.rmtree(workspace, ignore_errors=True)

    report = {
        "schema": "notrios.h8.integration-matrix.v1",
        "milestone": "v0.8", "item": "H8",
        "host_platform": LINUX,
        "rows": results,
        "totals": {
            "passed": sum(1 for r in results if r["status"] == "passed"),
            "failed": sum(1 for r in results if r["status"] == "failed"),
            "postponed": sum(1 for r in results if r["status"] == "postponed"),
        },
    }
    destination = os.path.join(ROOT, "performance/v0.8-h8/RESULTS.json")
    os.makedirs(os.path.dirname(destination), exist_ok=True)
    with open(destination, "w", encoding="utf-8") as stream:
        json.dump(report, stream, indent=2)
        stream.write("\n")
    print(f"\n{report['totals']} -> {destination}")
    return 1 if report["totals"]["failed"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
