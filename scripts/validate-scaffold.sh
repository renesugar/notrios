#!/usr/bin/env bash
set -euo pipefail

go test ./...
python3 scripts/check_required_files.py
bash -n scripts/mvp_smoke.sh
bash -n scripts/run_performance_smoke.sh
bash -n scripts/package_release.sh
python3 -m py_compile scripts/check_required_files.py scripts/check_plan_loops.py scripts/check_release_zip.py

echo "Scaffold validation passed."
