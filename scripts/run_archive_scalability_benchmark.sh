#!/usr/bin/env bash
# Run one G14a benchmark phase. The workspace must be outside the repository;
# completed phase rows are immutable and are reused on the next invocation.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
G14A_GO_CACHE=${NOTRIOS_G14A_GO_CACHE:-/tmp/notrios-g14a-go-cache}
cd "$ROOT"
env GOCACHE="$G14A_GO_CACHE" go run ./performance/v0.7-g14a/cmd/harness "$@"
