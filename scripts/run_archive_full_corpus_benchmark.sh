#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# J22: the binaries used to be built to fixed names in /tmp with a Go cache
# beside them, so two runs on one machine overwrote each other's binaries and
# the cache outlived every run. Each run now builds into a private directory it
# removes, and Go uses the caller's GOCACHE or its own default.
work=$(mktemp -d "${TMPDIR:-/tmp}/notrios-g14b.XXXXXX")
trap 'rm -rf "$work"' EXIT

go build -o "$work/notriosctl" "$repo_root/cmd/notriosctl"
go build -o "$work/tool" "$repo_root/performance/v0.7-g14b/tool"
# Arguments given to this script come after the defaults, so they still win.
python3 "$repo_root/performance/v0.7-g14b/harness.py" --notriosctl "$work/notriosctl" --tool "$work/tool" "$@"
