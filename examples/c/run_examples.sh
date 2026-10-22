#!/usr/bin/env bash
# Compile and run every C example against the *artifact* (v1.0 J6).
#
#   bash examples/c/run_examples.sh [artifact.tar.gz]
#
# With no argument it builds the artifact first. Either way the examples are
# compiled against the extracted tarball and nothing else: no repository header,
# no repository library, no Go toolchain on the include path. Compiling against
# the source tree would prove the one thing a third party cannot do.
#
# Each example gets its own library directory, because one owner per library is
# part of what the ABI promises and two examples sharing one would fail on it.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/examples/c
work=$(mktemp -d "${TMPDIR:-/tmp}/notrios-abi-examples-XXXXXX")
trap 'rm -rf "$work"' EXIT

artifact=${1:-}
if [[ -z "$artifact" ]]; then
  bash "$ROOT/scripts/build_abi_artifact.sh" "$work/dist" >/dev/null
  artifact=$(ls "$work"/dist/notrios-c-abi-*.tar.gz)
fi
[[ -f "$artifact" ]] || { echo "no artifact at $artifact" >&2; exit 1; }
echo "examples against $(basename "$artifact")"

tar -xzf "$artifact" -C "$work"
sdk=$(ls -d "$work"/notrios-c-abi-*/)
sdk=${sdk%/}

# The downloader's first move, and it must pass before anything is compiled.
( cd "$sdk" && sha256sum -c SHA256SUMS >/dev/null ) || {
  echo "the artifact's own checksums do not verify" >&2; exit 1; }

failures=0
for source in "$HERE"/[0-9][0-9]_*.c; do
  name=$(basename "$source" .c)
  binary=$work/$name
  # -I and -L point only inside the extracted artifact.
  if ! cc -std=c11 -Wall -Wextra -Werror \
      -I"$sdk/include" "$source" \
      -L"$sdk/lib" -lnotrios -Wl,-rpath,"$sdk/lib" -pthread -o "$binary" 2>"$work/$name.cc"; then
    echo "  FAIL  $name did not compile against the artifact"
    sed 's/^/        /' "$work/$name.cc"
    failures=$((failures + 1))
    continue
  fi
  library=$work/libraries/$name
  mkdir -p "$library"
  echo "  --- $name"
  if "$binary" "$library/library.db" > "$work/$name.out" 2>&1; then
    sed 's/^/      /' "$work/$name.out"
  else
    echo "  FAIL  $name exited non-zero"
    sed 's/^/        /' "$work/$name.out"
    failures=$((failures + 1))
  fi
done

if (( failures > 0 )); then
  echo "$failures example(s) failed"
  exit 1
fi
echo "every example compiled and ran against the artifact"
