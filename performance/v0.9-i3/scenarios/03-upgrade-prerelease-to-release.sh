#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
# The versions default to the pair I3 recorded; v1.0 J9 runs the same
# scenarios against 1.0.0 by passing RELEASE_VERSION and PRERELEASE_VERSION.
RV=${RELEASE_VERSION:-0.8.0}
PV=${PRERELEASE_VERSION:-0.8.0~rc1}
RELEASE=/pkg/notrios_${RV}-1_amd64.deb
PRERELEASE=/pkg/notrios_${PV}-1_amd64.deb
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

# A prerelease, real data written against it, then the release over the top.
apt-get install -y -qq "$PRERELEASE"
before=$(notriosctl version); echo "OBSERVE version_before=$before"
[ "$before" = "$PV" ] || { echo "the prerelease binary does not report its own version"; exit 1; }

as_tester "notriosctl notes create --title 'Survives the upgrade' --body 'written under the prerelease'" >/dev/null
echo "OBSERVE data_root=$(data_root)"

apt-get install -y -qq "$RELEASE"
after=$(notriosctl version); echo "OBSERVE version_after=$after"
[ "$after" = "$RV" ] || { echo "the upgrade did not replace the binary"; exit 1; }

holds_note "Survives the upgrade" || { echo "the note did not survive the upgrade"; exit 1; }
echo "OBSERVE note_survived_upgrade=yes"
