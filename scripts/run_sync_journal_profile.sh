#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:-"$ROOT/performance/v0.7-g4/evidence-100k.json"}
bash "$ROOT/scripts/agent_usage_preflight.sh" sync-journal-profile
mkdir -p "$(dirname "$OUT")"
cd "$ROOT"
go run ./performance/v0.7-g4/cmd/evidence -count 100000 -out "$OUT"
