#!/usr/bin/env bash
# The upgrade the installation page documents, from the last package anyone
# could have installed: the workflow-built 0.8.0 (run 34697727452) to 1.0.0.
# 1.0 opens a 0.8 library at schema 27 and moves it to 28 (J18), so this is the
# one upgrade in which the library changes as well as the program.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
RV=${RELEASE_VERSION:?}
OLD=/pkg/notrios_0.8.0-1_amd64.deb
RELEASE=/pkg/notrios_${RV}-1_amd64.deb
id tester >/dev/null 2>&1 || useradd -m -s /bin/bash tester
as_tester() { su - tester -c "$1"; }
holds_note() { as_tester "notriosctl search '$1'" | grep -qi -- "$1"; }

apt-get install -y -qq "$OLD"
before=$(notriosctl version); echo "OBSERVE version_before=$before"
[ "$before" = "0.8.0" ] || { echo "the 0.8.0 package does not report 0.8.0"; exit 1; }
as_tester "notriosctl notes create --title 'Written by 0.8.0' --body 'x'" >/dev/null

apt-get install -y -qq "$RELEASE"
after=$(notriosctl version); echo "OBSERVE version_after=$after"
[ "$after" = "$RV" ] || { echo "the upgrade did not replace the binary"; exit 1; }
holds_note "Written by 0.8.0" || { echo "the note did not survive the upgrade"; exit 1; }
echo "OBSERVE note_survived_upgrade=yes"
as_tester "notriosctl notes create --title 'Written by 1.0 after the upgrade' --body 'x'" >/dev/null
holds_note "Written by 1.0 after the upgrade" || { echo "the upgraded library does not take a write"; exit 1; }
echo "OBSERVE upgraded_library_writable=yes"
as_tester "notriosctl doctor" >/tmp/doctor.log 2>&1 || { cat /tmp/doctor.log; echo "doctor failed after the upgrade"; exit 1; }
echo "OBSERVE doctor_after_upgrade=pass"
