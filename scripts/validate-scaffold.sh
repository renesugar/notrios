#!/usr/bin/env bash
set -euo pipefail

go test ./...

# Tests must not write a data directory into the source tree. This is invisible
# to `git status` because data/ is ignored, and it happened: H4 resolved profile
# data from the data root, and tests that had been relying on it landing beside
# their temp registry started creating internal/profiles/data instead. A test
# that pollutes the checkout is also a test not exercising the layout it claims.
stray_data="$(find . -mindepth 2 -type d -name data \
  -not -path './.git/*' -not -path './web/node_modules/*' \
  -not -path './node_modules/*' -not -path './docs-site/*' \
  -not -path './testdata/*' -not -path './performance/*' 2>/dev/null || true)"
if test -n "$stray_data"; then
  echo "tests left a data directory inside the source tree:"
  echo "$stray_data"
  echo "isolate the test (see isolateRoots in internal/profiles) rather than deleting this by hand"
  exit 1
fi
# A compiled binary left in the repository root.
#
# `go build ./cmd/notriosctl` without -o writes its executable to the working
# directory, and .gitignore hides the result -- so `git status` stays clean while
# an 11 MB binary sits in the tree. That is exactly how notrioslib came to be
# committed in v0.8 H1 and to ship in five release archives. The release gate now
# rejects an archive containing one, but nothing noticed the working tree, and
# the same litter has appeared three times since.
#
# ELF magic rather than a list of names: a guard that has to be told each new
# binary's name is a guard someone has to remember to update.
stray_binaries=""
for candidate in ./*; do
  test -f "$candidate" || continue
  test -x "$candidate" || continue
  if head -c 4 "$candidate" 2>/dev/null | grep -q "^.ELF"; then
    stray_binaries="$stray_binaries $candidate"
  fi
done
if test -n "$stray_binaries"; then
  echo "a compiled binary is in the repository root:$stray_binaries"
  echo "build with an explicit destination instead: go build -o bin/<name> ./cmd/<name>"
  exit 1
fi
# A literal ":memory:" file anywhere in the tree.
#
# ":memory:" is SQLite's name for a database that has no file, so a file called
# that is never intentional: it means code derived a path from it -- a lock, a
# marker, a sidecar -- instead of recognising it as transient. One appeared in
# internal/store while H4b was being built, and `make validate` passed with it
# sitting there, because the stray-directory check above only looks for
# directories named data.
stray_memory="$(find . -name ':memory:*' -not -path './.git/*' \
  -not -path './web/node_modules/*' -not -path './node_modules/*' 2>/dev/null || true)"
if test -n "$stray_memory"; then
  echo "a path was derived from the in-memory database name:"
  echo "$stray_memory"
  echo "the caller should treat \":memory:\" as transient rather than as a filename"
  exit 1
fi
# The tests for scripts/ are run here.
#
# scripts/test_check_agent_usage.py existed and nothing ran it: it was listed in
# check_required_files.py, which asserts the file is present, not that it
# passes. A test nobody runs is a comment with a confusing filename, and the
# lifecycle tests being added beside it delete directories for a living.
python3 -m unittest discover -s scripts -p 'test_*.py'
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
' scripts/check_required_files.py scripts/check_sqlite_provenance.py performance/v0.8-h2a/validate_evidence.py scripts/check_plan_loops.py scripts/check_release_zip.py evidence/verify_evidence.py evidence/run_refusal_tests.py scripts/g17b_evidence.py evidence/test_verify_evidence.py performance/v0.7-g18/validate_evidence.py performance/v0.7-g18/test_validate_evidence.py performance/v0.7-g18a/build_inventory.py performance/v0.7-g18a/validate_evidence.py performance/v0.7-g18a/test_validate_evidence.py performance/v0.7-g18b/build_prototype.py performance/v0.7-g18b/validate_evidence.py performance/v0.7-g18b/test_validate_evidence.py performance/v0.7-g18c/validate_evidence.py performance/v0.7-g18d/validate_evidence.py performance/v0.7-g18d/test_validate_evidence.py performance/v0.7-g18e/validate_evidence.py performance/v0.7-g18e/test_validate_evidence.py performance/v0.7-g18f/validate_evidence.py performance/v0.7-g18f/test_validate_evidence.py performance/v0.7-g18g/validate_evidence.py performance/v0.7-g18g/test_validate_evidence.py performance/v0.7-g20/validate_evidence.py performance/v0.7-g20/test_validate_evidence.py performance/v0.7-g20/check_dependency_licenses.py performance/v0.7-g20/test_check_dependency_licenses.py performance/v0.8-h3/validate_evidence.py performance/v0.8-h3/purge_oracle.py performance/v0.8-h3/resolve_model.py performance/v0.8-h3/test_purge_oracle.py performance/v0.8-h3/test_resolve_model.py performance/v0.8-h3/test_backup_restore.py performance/v0.8-h4a/validate_evidence.py performance/v0.8-h6a/validate_evidence.py performance/v0.8-h6/validate_evidence.py performance/v0.8-h8/validate_evidence.py performance/v0.8-h8/../../scripts/integration_matrix.py performance/v0.8-h9/validate_evidence.py performance/v0.8-h14/validate_evidence.py performance/v0.8-h14/actionability.py performance/v0.8-h15/validate_evidence.py performance/v0.8-h11/validate_evidence.py
python3 -m unittest evidence.test_verify_evidence
python3 -m unittest discover -s performance/v0.7-g18 -p 'test_*.py'
# G18c is run here, not only from package_release.sh.
#
# It was release-only, and its registry assertion went stale for two commits
# without anything noticing: the failure appeared when the release gate ran,
# long after the change that caused it. A gate nobody runs until release is a
# gate that fails at the worst moment.
python3 performance/v0.7-g18c/validate_evidence.py
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
python3 performance/v0.8-h4a/validate_evidence.py
python3 performance/v0.8-h6a/validate_evidence.py
python3 performance/v0.8-h6/validate_evidence.py
python3 performance/v0.8-h8/validate_evidence.py
python3 performance/v0.8-h9/validate_evidence.py
python3 performance/v0.8-h14/validate_evidence.py
python3 performance/v0.8-h15/validate_evidence.py
python3 performance/v0.8-h11/validate_evidence.py

echo "Scaffold validation passed."
