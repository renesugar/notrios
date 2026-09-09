#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
RELEASE=/pkg/notrios_0.8.0-1_amd64.deb
PRERELEASE=/pkg/notrios_0.8.0~rc1-1_amd64.deb
id tester >/dev/null 2>&1 || useradd -m -s /bin/bash tester
as_tester() { su - tester -c "$1"; }
# Nothing here parses JSON with a scripting language: a clean ubuntu:24.04 has
# no python3, and installing one to read the output would put an interpreter
# into the environment whose emptiness is the thing under test.
# --no-redact, because the default output abbreviates the home directory to a
# literal "~" for display and a test that shell-quotes that string looks for a
# directory named "~".
data_root() { as_tester "notriosctl paths --no-redact" | awk '$1=="data"{print $2}'; }
holds_note() { as_tester "notriosctl search '$1'" | grep -qi -- "$1"; }

# A pre-0.8 library lived in ./data beside the checkout. Someone who installs
# the package still has that directory, and the notes in it are the only copy.
apt-get install -y -qq "$RELEASE"
as_tester "mkdir -p /home/tester/old-checkout/data"
as_tester "notriosctl notes create --db /home/tester/old-checkout/data/notes.sqlite --asset-store /home/tester/old-checkout/data/assets --title 'Written before the installer existed' --body 'x'" >/dev/null
as_tester "test -f /home/tester/old-checkout/data/notes.sqlite"
echo "OBSERVE legacy_library=created"

as_tester "cd /home/tester/old-checkout && notriosctl migrate --dry-run" > /tmp/dry.txt 2>&1
grep -qi "dry" /tmp/dry.txt || { echo "the dry run did not say it was one"; cat /tmp/dry.txt; exit 1; }
as_tester "test -f /home/tester/old-checkout/data/notes.sqlite" || { echo "the dry run moved the library"; exit 1; }
echo "OBSERVE dry_run_moved_nothing=confirmed"

as_tester "cd /home/tester/old-checkout && notriosctl migrate" > /tmp/migrate.txt 2>&1 || { cat /tmp/migrate.txt; exit 1; }
echo "OBSERVE migrate_exit=0"
holds_note "Written before the installer existed" || { echo "the migrated library is not the one in use"; tail -5 /tmp/migrate.txt; exit 1; }
echo "OBSERVE note_available_after_migration=yes"
echo "OBSERVE data_root=$(data_root)"
