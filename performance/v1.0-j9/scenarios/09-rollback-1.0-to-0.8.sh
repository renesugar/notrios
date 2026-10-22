#!/usr/bin/env bash
# Going back from 1.0.0 to 0.8.0, which is not what I3's downgrade rehearsed.
# I3 rolled back between two builds of one schema. 1.0 is schema 28, and J7-A
# measured that 0.8.0 refuses a 1.0 library -- so moving the program back is not
# enough, and this runs the steps the installation page gives instead: export
# with 1.0, downgrade, set the 1.0 library aside, restore with 0.8.0.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
RV=${RELEASE_VERSION:?}
OLD=/pkg/notrios_0.8.0-1_amd64.deb
RELEASE=/pkg/notrios_${RV}-1_amd64.deb
id tester >/dev/null 2>&1 || useradd -m -s /bin/bash tester
as_tester() { su - tester -c "$1"; }
data_root() { as_tester "notriosctl paths --no-redact" | awk '$1=="data"{print $2}'; }
holds_note() { as_tester "notriosctl search '$1'" | grep -qi -- "$1"; }

apt-get install -y -qq "$RELEASE"
as_tester "notriosctl notes create --title 'Kept across a rollback' --body 'x'" >/dev/null
# archive-v2, the lossless backup, rather than v1's interchange archive: J7-A
# measured 0.8.0 restoring a 1.0 archive-v2 completely.
as_tester "notriosctl export archive-v2 ~/notrios-backup" >/dev/null
as_tester "notriosctl verify archive-v2 ~/notrios-backup" >/dev/null
echo "OBSERVE export_with_1_0=verified"

apt-get install -y -qq --allow-downgrades "$OLD"
rolled=$(notriosctl version); echo "OBSERVE version_after_downgrade=$rolled"
[ "$rolled" = "0.8.0" ] || { echo "the downgrade did not take"; exit 1; }

# What a user sees first: the older program refuses the newer library.
set +e
as_tester "notriosctl search 'Kept across a rollback'" >/tmp/refused.log 2>&1
code=$?
set -e
echo "OBSERVE old_program_on_new_library_exit=$code"
echo "OBSERVE old_program_on_new_library_says=$(grep -v '^$' /tmp/refused.log | head -1)"
[ "$code" -ne 0 ] || { echo "0.8.0 opened a 1.0 library instead of refusing it"; exit 1; }

root=$(data_root); echo "OBSERVE data_root=$root"
as_tester "mv '$root' '$root.from-1.0'"
as_tester "notriosctl restore archive-v2 --intent adopt ~/notrios-backup" >/dev/null
holds_note "Kept across a rollback" || { echo "0.8.0 did not restore the 1.0 export"; exit 1; }
echo "OBSERVE restored_by_0_8=yes"
su - tester -c "test -d '$root.from-1.0'" || { echo "the 1.0 library set aside is gone"; exit 1; }
echo "OBSERVE library_set_aside=present"
