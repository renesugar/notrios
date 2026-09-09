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

# Going backwards must not happen by accident. apt refuses a downgrade unless
# asked explicitly, and what matters is what is left behind when it refuses.
apt-get install -y -qq "$RELEASE"
as_tester "notriosctl notes create --title 'Present before the downgrade attempt' --body 'x'" >/dev/null

set +e
apt-get install -y -qq "$PRERELEASE" > /tmp/downgrade.log 2>&1
code=$?
set -e
echo "OBSERVE downgrade_exit=$code"
[ "$code" -ne 0 ] || { echo "apt performed a downgrade without being asked"; tail -5 /tmp/downgrade.log; exit 1; }

still=$(notriosctl version); echo "OBSERVE version_after_refusal=$still"
[ "$still" = "0.8.0" ] || { echo "the refusal left a different version installed"; exit 1; }
holds_note "Present before the downgrade attempt" || { echo "data lost during a refused downgrade"; exit 1; }
echo "OBSERVE data_intact_after_refusal=yes"

# And when it is asked for explicitly it is a deliberate act that works, with
# the data still there afterwards -- a rollback has to be usable, not just legal.
apt-get install -y -qq --allow-downgrades "$PRERELEASE"
rolled=$(notriosctl version); echo "OBSERVE version_after_explicit_downgrade=$rolled"
[ "$rolled" = "0.8.0~rc1" ] || { echo "an explicit downgrade did not take"; exit 1; }
holds_note "Present before the downgrade attempt" || { echo "the rollback lost the data"; exit 1; }
echo "OBSERVE data_intact_after_rollback=yes"
