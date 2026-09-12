#!/usr/bin/env bash
# I6-D — check the SBOM against tools that were written by other people, and
# record a dated vulnerability scan.
#
#   bash performance/v0.9-i6/cross_check_sbom.sh
#
# Needs syft, cdxgen, grype and govulncheck. It writes SBOM_CROSSCHECK.json and
# SCAN.json beside this script and changes nothing else.
#
# The generators run with a scrubbed environment. cdxgen warns, correctly, that
# SBOM generation invokes build tooling which inherits whatever is exported --
# and this workstation exports API keys. Generating an SBOM is itself a
# supply-chain surface, and the warning is worth acting on rather than reading.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/performance/v0.9-i6
WORK=$(mktemp -d "${TMPDIR:-/tmp}/notrios-i6d-XXXXXX")
trap 'rm -rf "$WORK"' EXIT
SET=${RELEASE_SET:-$ROOT/dist/release-set}

scrubbed() { env -i PATH="$PATH" HOME="$HOME" TMPDIR="$WORK" "$@"; }

echo "== syft: what is discoverable on disk"
scrubbed syft "dir:$ROOT" -o cyclonedx-json \
  --exclude './dist/**' --exclude './web/node_modules/**' --exclude './docs-site/node_modules/**' \
  > "$WORK/syft.json" 2>"$WORK/syft.err"

echo "== cdxgen: the direct Go dependencies"
scrubbed cdxgen -t go -o "$WORK/cdxgen.json" --no-recurse "$ROOT" > "$WORK/cdxgen.log" 2>&1 || true

echo "== grype: advisories against the SBOM's components"
scrubbed grype "sbom:$SET/SBOM.cdx.json" -o json > "$WORK/grype.json" 2>"$WORK/grype.err"

echo "== govulncheck: which of them this code can actually reach"
( cd "$ROOT" && govulncheck ./... ) > "$WORK/govulncheck.txt" 2>&1 || true

python3 "$HERE/record_cross_check.py" --work "$WORK" --set "$SET"
