#!/usr/bin/env bash
set -euo pipefail

go test ./...
python3 scripts/check_required_files.py
python3 scripts/check_sqlite_provenance.py
python3 performance/v0.8-h2a/validate_evidence.py
bash -n scripts/mvp_smoke.sh
bash -n scripts/run_performance_smoke.sh
bash -n scripts/run_sync_journal_profile.sh
bash -n scripts/run_joplin_import_profile.sh
bash -n scripts/package_release.sh
bash -n scripts/build_docs_site.sh
bash -n scripts/verify_evidence_pre_push.sh
# Syntax-check helper scripts without writing __pycache__ bytecode.
python3 -c 'import ast, sys
for path in sys.argv[1:]:
    with open(path) as fh:
        ast.parse(fh.read(), path)
' scripts/check_required_files.py scripts/check_sqlite_provenance.py performance/v0.8-h2a/validate_evidence.py scripts/check_plan_loops.py scripts/check_release_zip.py evidence/verify_evidence.py evidence/run_refusal_tests.py scripts/g17b_evidence.py evidence/test_verify_evidence.py performance/v0.7-g18/validate_evidence.py performance/v0.7-g18/test_validate_evidence.py performance/v0.7-g18a/build_inventory.py performance/v0.7-g18a/validate_evidence.py performance/v0.7-g18a/test_validate_evidence.py performance/v0.7-g18b/build_prototype.py performance/v0.7-g18b/validate_evidence.py performance/v0.7-g18b/test_validate_evidence.py performance/v0.7-g18c/validate_evidence.py performance/v0.7-g18d/validate_evidence.py performance/v0.7-g18d/test_validate_evidence.py performance/v0.7-g18e/validate_evidence.py performance/v0.7-g18e/test_validate_evidence.py performance/v0.7-g18f/validate_evidence.py performance/v0.7-g18f/test_validate_evidence.py performance/v0.7-g18g/validate_evidence.py performance/v0.7-g18g/test_validate_evidence.py performance/v0.7-g20/validate_evidence.py performance/v0.7-g20/test_validate_evidence.py performance/v0.7-g20/check_dependency_licenses.py performance/v0.7-g20/test_check_dependency_licenses.py performance/v0.8-h3/validate_evidence.py performance/v0.8-h3/purge_oracle.py performance/v0.8-h3/resolve_model.py performance/v0.8-h3/test_purge_oracle.py performance/v0.8-h3/test_resolve_model.py performance/v0.8-h3/test_backup_restore.py
python3 -m unittest evidence.test_verify_evidence
python3 -m unittest discover -s performance/v0.7-g18 -p 'test_*.py'
python3 performance/v0.7-g18/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18a -p 'test_*.py'
python3 performance/v0.7-g18a/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18b -p 'test_*.py'
python3 performance/v0.7-g18b/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18d -p 'test_*.py'
python3 performance/v0.7-g18d/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18e -p 'test_*.py'
python3 performance/v0.7-g18e/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18f -p 'test_*.py'
python3 performance/v0.7-g18f/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18g -p 'test_*.py'
python3 performance/v0.7-g18g/validate_evidence.py
node --check performance/v0.7-g18g/browser_smoke.mjs
python3 -m unittest discover -s performance/v0.7-g20 -p 'test_*.py'
python3 performance/v0.7-g20/check_dependency_licenses.py
python3 performance/v0.7-g20/validate_evidence.py
python3 -m unittest discover -s performance/v0.8-h3 -p 'test_*.py'
python3 performance/v0.8-h3/validate_evidence.py

echo "Scaffold validation passed."
