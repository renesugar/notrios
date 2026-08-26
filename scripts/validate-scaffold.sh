#!/usr/bin/env bash
set -euo pipefail

go test ./...
python3 scripts/check_required_files.py
bash -n scripts/mvp_smoke.sh
bash -n scripts/run_performance_smoke.sh
bash -n scripts/run_sync_journal_profile.sh
bash -n scripts/run_joplin_import_profile.sh
bash -n scripts/package_release.sh
bash -n scripts/verify_evidence_pre_push.sh
# Syntax-check helper scripts without writing __pycache__ bytecode.
python3 -c 'import ast, sys
for path in sys.argv[1:]:
    with open(path) as fh:
        ast.parse(fh.read(), path)
' scripts/check_required_files.py scripts/check_plan_loops.py scripts/check_release_zip.py evidence/verify_evidence.py evidence/run_refusal_tests.py scripts/g17b_evidence.py evidence/test_verify_evidence.py performance/v0.7-g18/validate_evidence.py performance/v0.7-g18/test_validate_evidence.py
python3 -m unittest evidence.test_verify_evidence
python3 -m unittest discover -s performance/v0.7-g18 -p 'test_*.py'
python3 performance/v0.7-g18/validate_evidence.py

echo "Scaffold validation passed."
