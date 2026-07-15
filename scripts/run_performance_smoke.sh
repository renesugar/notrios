#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
go test ./internal/store -run TestGeneratedDatasetSearchGraphAndResourceSmoke -bench BenchmarkGeneratedDatasetSearch -benchmem
