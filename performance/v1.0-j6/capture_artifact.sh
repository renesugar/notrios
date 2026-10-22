#!/usr/bin/env bash
# J6 — build the C ABI artifact, describe it, and run the examples against it.
#
#   bash performance/v1.0-j6/capture_artifact.sh
#
# What is recorded is what a third party would get: the tarball's contents, its
# exported symbols, its soname, its dynamic dependencies, and the result of
# compiling six C examples against the extracted tarball rather than this
# repository. The tarball's own hash is not recorded: a Go build is not
# bit-reproducible here, and a number that changes every run would be evidence
# of nothing.
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
OUT=${J6_OUT:-$ROOT/performance/v1.0-j6}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/notrios-j6-XXXXXX")
trap 'rm -rf "$WORK"' EXIT

echo "# building the artifact"
bash "$ROOT/scripts/build_abi_artifact.sh" "$WORK/dist" > "$WORK/build.log" 2>&1 || {
  echo "the artifact did not build:" >&2; tail -20 "$WORK/build.log" >&2; exit 1; }
artifact=$(ls "$WORK"/dist/notrios-c-abi-*.tar.gz)
echo "# $(basename "$artifact")"

tar -xzf "$artifact" -C "$WORK"
sdk=$(ls -d "$WORK"/notrios-c-abi-*/)
sdk=${sdk%/}
lib=$(ls "$sdk"/lib/libnotrios.so.[0-9]*)

echo "# running the examples against the extracted artifact"
if bash "$ROOT/examples/c/run_examples.sh" "$artifact" > "$WORK/examples.log" 2>&1; then
  examples=pass
else
  examples=fail
fi
tail -1 "$WORK/examples.log"

python3 - "$OUT/ARTIFACT.json" "$artifact" "$sdk" "$lib" "$examples" "$WORK/examples.log" <<'PY'
import hashlib
import json
import os
import pathlib
import subprocess
import sys

out, artifact, sdk, lib, examples, examples_log = sys.argv[1:7]
sdk_path = pathlib.Path(sdk)


def run(*command: str) -> str:
    return subprocess.run(command, capture_output=True, text=True, check=True).stdout


symbols = sorted(
    line.split()[2]
    for line in run("nm", "-D", "--defined-only", lib).splitlines()
    if len(line.split()) >= 3 and line.split()[2].startswith("notrios_")
)
needed = sorted(
    line.split()[1] for line in run("objdump", "-p", lib).splitlines()
    if line.strip().startswith("NEEDED")
)
soname = next(line.split()[1] for line in run("objdump", "-p", lib).splitlines()
              if line.strip().startswith("SONAME"))
contents = sorted(
    str(path.relative_to(sdk_path)) for path in sdk_path.rglob("*") if not path.is_dir()
)
examples_ran = [line.strip() for line in pathlib.Path(examples_log).read_text(encoding="utf-8").splitlines()
                if line.strip().startswith("--- ")]

pathlib.Path(out).write_text(json.dumps({
    "schema": "notrios.j6.abi-artifact/1",
    "what_this_is": (
        "One build of the C ABI artifact and one run of the C examples against it. The tarball's "
        "hash is deliberately absent: a Go build is not bit-reproducible here, so it would change "
        "every run and prove nothing. What is recorded is what a third party gets."),
    "artifact": os.path.basename(artifact),
    "artifact_bytes": os.path.getsize(artifact),
    "contents": contents,
    "library": {
        "name": os.path.basename(lib),
        "soname": soname,
        "bytes": os.path.getsize(lib),
        "exported_symbols": symbols,
        "exports_sqlite": any(symbol.startswith("sqlite3_") for symbol in symbols),
        "dynamic_dependencies": needed,
    },
    "header": {
        "name": "include/notrios_abi.h",
        "sha256": hashlib.sha256((sdk_path / "include/notrios_abi.h").read_bytes()).hexdigest(),
    },
    "checksums_verify": subprocess.run(
        ["sha256sum", "-c", "SHA256SUMS"], cwd=sdk, capture_output=True).returncode == 0,
    "examples": {
        "built_against": "the extracted tarball, with -I and -L inside it and no repository path",
        "result": examples,
        "ran": [name.removeprefix("--- ") for name in examples_ran],
    },
}, indent=2) + "\n", encoding="utf-8")
print(f"wrote {out}")
PY

[[ "$examples" == pass ]]
