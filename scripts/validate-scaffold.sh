#!/usr/bin/env bash
set -euo pipefail

go test ./...
python3 scripts/check_required_files.py
bash -n scripts/mvp_smoke.sh
bash -n scripts/run_performance_smoke.sh
bash -n scripts/package_release.sh
# Syntax-check helper scripts without writing __pycache__ bytecode.
python3 -c 'import ast, sys
for path in sys.argv[1:]:
    with open(path) as fh:
        ast.parse(fh.read(), path)
' scripts/check_required_files.py scripts/check_plan_loops.py scripts/check_release_zip.py

echo "Scaffold validation passed."
