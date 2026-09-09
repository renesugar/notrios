#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
RELEASE=/pkg/notrios_0.8.0-1_amd64.deb
PRERELEASE=/pkg/notrios_0.8.0~rc1-1_amd64.deb
id tester >/dev/null 2>&1 || useradd -m -s /bin/bash tester
as_tester() { su - tester -c "$1"; }

# A pristine ubuntu:24.04. Nothing here has ever seen this project, and the
# declared dependencies are part of what is under test: apt must be able to
# satisfy them on its own, from the package alone.
apt-get update -qq
apt-get install -y -qq "$RELEASE"

for binary in notrios notriosd notriosctl; do
  command -v "$binary" >/dev/null || { echo "missing $binary"; exit 1; }
done
OBSERVE_VERSION=$(notriosctl version)
echo "OBSERVE version=$OBSERVE_VERSION"
[ "$OBSERVE_VERSION" = "0.8.0" ] || { echo "version is not 0.8.0"; exit 1; }

# The toolchain that built it must be absent, or this proves nothing.
absent=""
for tool in go node npm git gcc make; do
  command -v "$tool" >/dev/null && absent="$absent $tool"
done
[ -z "$absent" ] || { echo "build tooling present:$absent"; exit 1; }
echo "OBSERVE toolchain_absent=go,node,npm,git,gcc,make"

test -f /usr/share/applications/notrios.desktop
echo "OBSERVE desktop_entry=present"

# doctor as an unprivileged user, on a machine with no keyring at all. Until
# v0.9 I1 this exited 1 here, because it required a credential store for a
# profile that stores nothing.
as_tester "notriosctl doctor" > /tmp/doctor.txt 2>&1 || { cat /tmp/doctor.txt; exit 1; }
echo "OBSERVE doctor_exit=0"
grep -q "credential store" /tmp/doctor.txt || { echo "doctor said nothing about the credential store"; exit 1; }
echo "OBSERVE doctor_credential_line=$(grep 'credential store' /tmp/doctor.txt | head -1 | cut -c1-60)"
