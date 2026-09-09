#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
bash scripts/agent_usage_preflight.sh performance-smoke
go test ./internal/store -run TestGeneratedDatasetSearchGraphAndResourceSmoke -bench BenchmarkGeneratedDatasetSearch -benchmem
NOTRIOS_SCALE_PROFILE=10000 go test -timeout 5m -run '^TestLargeLibraryProfile$' -count=1 ./internal/store
NOTRIOS_JOPLIN_PROFILE=100 go test -timeout 5m -run '^TestJoplinImporterProfile$' -count=1 ./internal/importers/joplinraw
