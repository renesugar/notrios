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
# Syntax-check every Python file in the repository, without writing
# __pycache__ bytecode.
#
# This was a hand-maintained list of 53 paths, and a hand-maintained list of
# files is a claim that goes stale the moment someone adds a file: 39 tracked
# Python files were never syntax-checked by it, including the ones added by the
# most recent milestones. Deriving the set from the repository means a new
# script is covered because it exists, not because somebody remembered.
# `git ls-files` is preferred so scratch files in a working tree cannot fail the
# gate; `find` is the fallback for a checkout without git.
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  python_files=$(git ls-files '*.py')
else
  python_files=$(find . -name '*.py' -not -path './.git/*' -not -path '*/node_modules/*' | sort)
fi
if [ -z "$python_files" ]; then
  echo "no Python files found to syntax-check" >&2
  exit 1
fi
# shellcheck disable=SC2086
python3 -c 'import ast, sys
for path in sys.argv[1:]:
    with open(path) as fh:
        ast.parse(fh.read(), path)
' $python_files
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
python3 performance/v0.8-h10/validate_evidence.py
python3 performance/v0.8e/validate_evidence.py
# E4: a finished plan item with no archive in the record fails here, not at
# release time. Packaging stopped after H4 and the omission survived thirty-one
# items because nothing compared the plan to the record.
python3 performance/v0.8e/check_archive_coverage.py
# I3: the installer matrix record. Checks the record, not the containers,
# so it runs where there is no Docker.
python3 performance/v0.9-i3/validate_evidence.py
# I4: the destructive-lifecycle record. The drills install and delete; this
# reads what they recorded, so `make validate` never does either.
python3 performance/v0.9-i4/validate_evidence.py
# I5: the signing policy, and the workflow check that keeps signing material
# out of pull-request jobs. Both run offline.
python3 performance/v0.9-i5/validate_evidence.py
# I6: the release evidence record, plus the workflow hardening gate that keeps
# actions pinned by digest and permissions least-privilege.
python3 performance/v0.9-i6/validate_evidence.py
# I7: the soak, recovery drills and frozen support matrix. Reads the record;
# the soak and the drills are not run here.
python3 performance/v0.9-i7/validate_evidence.py

# The anchor and enumeration gates, which used to run only inside
# scripts/package_release.sh. That is how a dangling Go anchor survived from
# H15's rename of ChooseSyncDirectory until v0.8 H13 tried to package: a gate
# that only runs when you release is a gate that tells you at the worst moment.
# They need no network and no browser, which is the whole test for belonging
# here.
make g18e-validate g18f-validate g19-validate g20-validate

echo "Scaffold validation passed."
