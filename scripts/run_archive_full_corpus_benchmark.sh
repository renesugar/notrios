#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
gocache=${GOCACHE:-/tmp/notrios-g14b-go-cache}

GOCACHE="$gocache" go build -o /tmp/notrios-g14b-notriosctl "$repo_root/cmd/notriosctl"
GOCACHE="$gocache" go build -o /tmp/notrios-g14b-tool "$repo_root/performance/v0.7-g14b/tool"
exec python3 "$repo_root/performance/v0.7-g14b/harness.py" "$@"
