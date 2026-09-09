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

# A named profile must be found again by a later process, which is what makes
# more than one library usable at all.
apt-get install -y -qq "$RELEASE"
as_tester "notriosctl profile create --name second --listen 127.0.0.1:8099" >/dev/null
as_tester "notriosctl profile list" > /tmp/profiles.txt
grep -q second /tmp/profiles.txt || { echo "the profile was not discovered"; cat /tmp/profiles.txt; exit 1; }
echo "OBSERVE profile_discovered=second"

as_tester "notriosctl paths --no-redact" > /tmp/paths.txt
echo "OBSERVE mode=$(awk -F': ' '/^mode:/{print $2}' /tmp/paths.txt)"

# Two opposite requirements, and the first draft of this check got them
# confused by forbidding /usr outright.
#
# `program_assets` is the frontend the package ships. It belongs under /usr:
# root-owned, read-only, replaced by the package manager on upgrade. Every other
# root holds things the user creates, and none of those may land in /usr, where
# the person who owns the notes could not write them and no backup would look.
assets=$(awk '$1=="program_assets"{print $2}' /tmp/paths.txt)
echo "OBSERVE program_assets=$assets"
case "$assets" in /usr/*) ;; *) echo "packaged assets are not in /usr: $assets"; exit 1;; esac

writable=$(awk 'NF==2 && $2 ~ /^\// && $1 != "program_assets" {print $1"="$2}' /tmp/paths.txt)
echo "OBSERVE writable_roots=$(echo "$writable" | tr '\n' ' ')"
if echo "$writable" | grep -q '=/usr/'; then
  echo "a root the user must write to is inside /usr"; cat /tmp/paths.txt; exit 1
fi
echo "OBSERVE writable_roots_outside_usr=confirmed"
echo "OBSERVE data_root=$(data_root)"
