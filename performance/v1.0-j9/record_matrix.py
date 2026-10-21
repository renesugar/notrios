#!/usr/bin/env python3
"""v1.0 J9's support matrix: derive its runtime rows, and keep the
installation page's table in step with it.

    python3 performance/v1.0-j9/record_matrix.py derive --package <deb> --workflow-package <deb> --abi-artifact <tar.gz>
    python3 performance/v1.0-j9/record_matrix.py            # check
    python3 performance/v1.0-j9/record_matrix.py --write    # rewrite the table

`derive` reads the built package, because a package's dependencies are exactly
what a stranger installs and a hand-written list is a list that disagrees with
it. It needs dpkg-deb and objdump, and the packages, which are not in the
repository; so it runs when the matrix is recorded, and the check below runs
everywhere else.

The check is offline. It refuses a supported row without executed evidence, a
row whose evidence file is missing, a container run that did not pass or did not
run 1.0.0, and an installation table that is not the one this matrix renders.
"""
import argparse
import json
import pathlib
import re
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = ROOT / "performance/v1.0-j9"
MATRIX = HERE / "MATRIX.json"
RESULTS = HERE / "I3-RESULTS.jsonl"
DOC = ROOT / "docs/installation.md"
BEGIN = "<!-- notrios:generated:matrix:rows:begin -->"
END = "<!-- notrios:generated:matrix:rows:end -->"
VERSION_SOURCE = ROOT / "internal/version/version.go"
VERIFY = HERE / "VERIFY.json"
VERIFY_SECTION = "### Verifying what you downloaded"


def fail(message):
    print(f"J9 support matrix invalid: {message}", file=sys.stderr)
    sys.exit(1)


def product_version():
    found = re.search(r'const Version = "([^"]+)"', VERSION_SOURCE.read_text())
    if not found:
        fail("internal/version states no Version")
    return found.group(1)


# ------------------------------------------------------------------ derive

def shlib_depends(binary):
    """What dpkg-shlibdeps computes for one binary: the same computation
    build_deb.sh makes for the whole package, made per surface, so a row says
    what *its* binaries need rather than what the package as a whole pulls in."""
    with tempfile.TemporaryDirectory() as work:
        pathlib.Path(work, "debian").mkdir()
        pathlib.Path(work, "debian/control").touch()
        out = subprocess.run(["dpkg-shlibdeps", "-O", "-e", str(binary)], cwd=work, check=True,
                             capture_output=True, text=True).stdout
    found = re.search(r"shlibs:Depends=(.*)", out)
    return [part.strip() for part in found.group(1).split(",")] if found else []


def glibc_key(symbol):
    """GLIBC_2.34 as (2, 34): a version compares by number, and as strings
    GLIBC_2.4 would sort above GLIBC_2.34."""
    return tuple(int(part) for part in symbol.removeprefix("GLIBC_").split("."))


def binary_facts(binary):
    dynamic = subprocess.run(["objdump", "-p", str(binary)], check=True,
                             capture_output=True, text=True).stdout
    symbols = subprocess.run(["objdump", "-T", str(binary)], check=True,
                             capture_output=True, text=True).stdout
    newest = max([(int(a), int(b)) for a, b in re.findall(r"GLIBC_(\d+)\.(\d+)", symbols)] or [(0, 0)])
    return {"needed": re.findall(r"^\s*NEEDED\s+(\S+)", dynamic, re.M),
            "depends": shlib_depends(binary),
            "newest_glibc_symbol": f"GLIBC_{newest[0]}.{newest[1]}"}


def package_facts(deb):
    depends = subprocess.run(["dpkg-deb", "-f", deb, "Depends"], check=True,
                             capture_output=True, text=True).stdout.strip()
    with tempfile.TemporaryDirectory() as work:
        subprocess.run(["dpkg-deb", "-x", deb, work], check=True)
        binaries = {binary.name: binary_facts(binary)
                    for binary in sorted(pathlib.Path(work, "usr/bin").iterdir())}
    return [part.strip() for part in depends.split(",") if part.strip()], binaries


def abi_facts(tarball):
    with tempfile.TemporaryDirectory() as work:
        subprocess.run(["tar", "-xzf", tarball, "-C", work], check=True)
        library = next(pathlib.Path(work).glob("*/lib/libnotrios.so.1"))
        return {library.name: binary_facts(library)}


def derive(package, workflow_package, abi_artifact):
    matrix = json.loads(MATRIX.read_text())
    depends, binaries = package_facts(package)
    workflow_depends, _ = package_facts(workflow_package)
    binaries.update(abi_facts(abi_artifact))
    newest = max((facts["newest_glibc_symbol"] for facts in binaries.values()), key=glibc_key)
    matrix["runtime"].update({
        "package": pathlib.Path(package).name,
        "abi_artifact": pathlib.Path(abi_artifact).name,
        "depends": depends,
        "binaries": binaries,
        "workflow_package": pathlib.Path(workflow_package).name,
        "workflow_package_depends_identical": depends == workflow_depends,
        "newest_glibc_symbol": newest,
    })
    MATRIX.write_text(json.dumps(matrix, indent=2, ensure_ascii=False) + "\n")
    print(f"runtime rows derived from {pathlib.Path(package).name} and {pathlib.Path(abi_artifact).name}: "
          f"{len(depends)} package dependencies, {len(binaries)} binaries, newest {newest}")


# ------------------------------------------------------------------- check

def runtime_cell(matrix, row):
    """A row's run-time needs, from the derived per-binary facts and nothing
    typed into the row: its binaries' dependencies, in the package's terms."""
    if row.get("runtime_note"):
        return row["runtime_note"]
    names = row.get("runtime_of")
    if not names:
        return "—"
    binaries = matrix["runtime"]["binaries"]
    depends = []
    for name in names:
        for dependency in binaries[name]["depends"]:
            if dependency not in depends:
                depends.append(dependency)
    newest = max((binaries[name]["newest_glibc_symbol"] for name in names), key=glibc_key)
    return ", ".join(depends) + f"; newest glibc symbol {newest}"


def render(matrix):
    lines = [BEGIN,
             "| Surface | Platform | Status | Needs at run time |",
             "|---|---|---|---|"]
    for row in matrix["surfaces"]:
        status = row["status"]
        if not row["supported"]:
            status = f"**{status}**" if row["surface"].startswith("desktop") else status
        lines.append(f"| {row['surface']} | {row['platform']} | {status} | {runtime_cell(matrix, row)} |")
    lines.append(END)
    return "\n".join(lines)


def check_matrix(matrix):
    if matrix.get("product_version") != product_version():
        fail(f"the matrix is for {matrix.get('product_version')}, the product is {product_version()}")
    runtime = matrix["runtime"]
    if not runtime["depends"] or not runtime["binaries"]:
        fail("the runtime rows were never derived from a package")
    if not runtime["package"].startswith(f"notrios_{product_version()}-"):
        fail(f"the runtime rows come from {runtime['package']}, not a {product_version()} package")
    if not runtime["workflow_package_depends_identical"]:
        fail("the workstation package and the workflow package declare different dependencies")
    if set(runtime["binaries"]) != {"notrios", "notriosctl", "notriosd", "libnotrios.so.1"}:
        fail(f"the runtime rows cover {sorted(runtime['binaries'])}, not the three binaries and the library")
    for row in matrix["surfaces"]:
        for name in row.get("runtime_of", []):
            if name not in runtime["binaries"] or not runtime["binaries"][name]["depends"]:
                fail(f"{row['surface']}: no derived dependencies for {name}")
    for row in matrix["surfaces"]:
        name = f"{row['surface']} on {row['platform']}"
        if not (ROOT / row["evidence"]).is_file():
            fail(f"{name}: evidence {row['evidence']} does not exist")
        if row["supported"] and not (row["executed"] and row["claim_level"] >= 3):
            fail(f"{name} is supported without being executed")
        if not row["supported"] and not row.get("blocked_on") and not row.get("why_it_matters"):
            fail(f"{name} is not supported and does not say why")
    for platform in ("Windows", "macOS"):
        rows = [r for r in matrix["surfaces"] if r["platform"] == platform]
        if not rows or not all(r.get("absent_from_release_claims") and not r["supported"] for r in rows):
            fail(f"{platform} must be listed, unsupported, and absent from release claims")


def check_results():
    rows = [json.loads(line) for line in RESULTS.read_text().splitlines() if line.strip()]
    expected = {f"0{n}-" for n in range(1, 10)}
    seen = {row["scenario"][:3] for row in rows}
    if seen != expected:
        fail(f"the container run has scenarios {sorted(seen)}, want 01 to 09")
    failed = [row["scenario"] for row in rows if row["status"] != "pass"]
    if failed:
        fail(f"container scenarios did not pass: {failed}")
    version = product_version()
    reported = {value for row in rows for key, value in row["observations"].items()
                if key in ("version", "version_after", "version_after_refusal")}
    if reported != {version}:
        fail(f"the container run reported versions {sorted(reported)}, want {version}")
    images = {row["image"] for row in rows}
    if "ubuntu:24.04" not in images:
        fail("no scenario started from a pristine ubuntu:24.04")
    return len(rows)


def check_verification():
    """The verification run must be of the blocks the page publishes now.

    run_verification.py reads the blocks out of the page, so its record holds
    what ran. A block edited after the run is a block nobody has run, and this
    refuses it until the run is repeated."""
    record = json.loads(VERIFY.read_text())
    if record["failed"] or record["passed"] != len(record["checks"]):
        fail(f"the verification run has failures: {record['failed']}")
    text = DOC.read_text()
    start = text.index(VERIFY_SECTION)
    end = text.index("\n### ", start + len(VERIFY_SECTION))
    published = [block.replace(record["set"]["published_name_replaced"], record["set"]["package"]).strip().splitlines()
                 for block in re.findall(r"```sh\n(.*?)```", text[start:end], re.S)]
    ran = []
    for check in record["checks"]:
        if check["block"] not in ran:
            ran.append(check["block"])
    if ran != published:
        fail("the verification blocks on the page are not the ones run_verification.py ran; run it again")
    refusals = [c for c in record["checks"] if c["check"].startswith("refusal:")]
    if len(refusals) < 4:
        fail(f"only {len(refusals)} refusals were checked")
    return len(record["checks"])


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("command", nargs="?", choices=["derive"])
    parser.add_argument("--package")
    parser.add_argument("--workflow-package")
    parser.add_argument("--abi-artifact")
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    if args.command == "derive":
        if not args.package or not args.workflow_package or not args.abi_artifact:
            parser.error("derive needs --package, --workflow-package and --abi-artifact")
        derive(args.package, args.workflow_package, args.abi_artifact)
        return

    matrix = json.loads(MATRIX.read_text())
    check_matrix(matrix)
    scenarios = check_results()
    verifications = check_verification()
    text = DOC.read_text()
    start, stop = text.find(BEGIN), text.find(END)
    if start < 0 or stop < start:
        fail(f"{DOC.relative_to(ROOT)} has no matrix block")
    rendered = text[:start] + render(matrix) + text[stop + len(END):]
    if args.write:
        DOC.write_text(rendered)
        print(f"installation table written from the matrix: {len(matrix['surfaces'])} rows")
        return
    if rendered != text:
        fail(f"the table in {DOC.relative_to(ROOT)} is stale; run record_matrix.py --write")
    print(f"J9 support matrix valid: {len(matrix['surfaces'])} rows, "
          f"{sum(r['supported'] for r in matrix['surfaces'])} supported, "
          f"{scenarios} container scenarios passed at {product_version()}, "
          f"{verifications} verification checks as published")


if __name__ == "__main__":
    main()
