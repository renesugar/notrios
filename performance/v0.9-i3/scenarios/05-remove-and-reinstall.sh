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

# Removing the program must not remove the user's notes. That is the claim the
# lifecycle documentation makes most loudly, and nothing had tested it against
# the packaged form.
apt-get install -y -qq "$RELEASE"
as_tester "notriosctl notes create --title 'Owned by the user not the package' --body 'x'" >/dev/null
root=$(data_root); echo "OBSERVE data_root=$root"

apt-get remove -y -qq notrios
command -v notriosctl >/dev/null && { echo "the binary survived removal"; exit 1; }
echo "OBSERVE binary_after_remove=absent"
su - tester -c "test -d '$root'" || { echo "user data was deleted by package removal"; exit 1; }
echo "OBSERVE user_data_after_remove=present"

apt-get install -y -qq "$RELEASE"
holds_note "Owned by the user not the package" || { echo "the note did not survive remove and reinstall"; exit 1; }
echo "OBSERVE note_survived_remove_reinstall=yes"
