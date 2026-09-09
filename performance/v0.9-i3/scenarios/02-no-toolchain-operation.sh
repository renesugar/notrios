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

# The package is only useful if the application works where nothing was built.
apt-get install -y -qq "$RELEASE"
for tool in go node npm git gcc make; do
  command -v "$tool" >/dev/null && { echo "build tooling present: $tool"; exit 1; }
done
echo "OBSERVE toolchain_absent=go,node,npm,git,gcc,make"
test -d /usr/src/notrios && { echo "a source tree is installed"; exit 1; }
echo "OBSERVE source_tree=absent"

as_tester "notriosctl notes create --title 'Written where nothing can be compiled' --body 'body text'" >/dev/null
holds_note "Written where nothing can be compiled" || { echo "the note was not stored or not findable"; exit 1; }
echo "OBSERVE note_created_and_found=yes"

echo "OBSERVE resolved_mode=$(as_tester 'notriosctl paths' | awk -F': ' '/^mode:/{print $2}')"
echo "OBSERVE data_root=$(data_root)"
# Getting the notes back out is the part that matters most on a machine with no
# toolchain: it is the user's escape hatch. The first draft of this ran export
# with invented flags and swallowed the failure with `|| true`, then recorded
# "export_written=no" as though that were a result -- which is precisely how a
# real fault would hide here. It is fatal now.
as_tester "notriosctl export archive /home/tester/export-dir"
as_tester "test -s /home/tester/export-dir/manifest.json" || {
  echo "the export produced no manifest"; as_tester "ls -la /home/tester/export-dir" || true; exit 1; }
echo "OBSERVE export_manifest=written"
echo "OBSERVE export_entries=$(as_tester 'ls /home/tester/export-dir' | tr '\n' ' ')"
