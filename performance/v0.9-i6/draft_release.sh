#!/usr/bin/env bash
# Exercise the release-candidate flow, up to but not through the external write.
#
#   bash performance/v0.9-i6/draft_release.sh              # dry: checks and prints
#   bash performance/v0.9-i6/draft_release.sh --execute    # creates the draft
#
# Dry is the default deliberately. Creating a draft uploads artifacts to GitHub,
# and an upload is an external write the owner authorises separately -- v0.9's
# boundary is that this milestone is not an end-user release.
#
# What the dry run establishes is everything except the API call: the set exists
# and verifies, the tag it would carry does not already exist, gh is
# authenticated, and the exact command is printed so a person can read what
# would happen before it does.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
SET=${RELEASE_SET:-$ROOT/dist/release-set}
REPO=${RELEASE_REPO:-renesugar/notrios}
EXECUTE=no
[[ ${1:-} == --execute ]] && EXECUTE=yes

version=$(python3 -c "
import json,sys
print(json.load(open('$SET/PROVENANCE.json'))['predicate']['buildDefinition']['externalParameters']['version'])")
tag="v${version%%-*}-rc"

echo "== verifying the set before anything leaves this machine"
python3 "$ROOT/performance/v0.9-i6/verify_release_set.py" --set "$SET"

echo "== preconditions"
gh auth status >/dev/null 2>&1 && echo "  gh: authenticated" || { echo "  gh: NOT authenticated" >&2; exit 1; }
if gh release view "$tag" --repo "$REPO" >/dev/null 2>&1; then
  echo "  tag $tag: already has a release -- refusing to overwrite" >&2; exit 1
fi
echo "  tag $tag: free"
# A draft must not create a tag in the repository. gh only creates the tag when
# the release is published, which is precisely why the draft flow is the one
# v0.9 exercises.
echo "  artifacts:"; echo "    SHA256SUMS"
while read -r _ name; do echo "    $name"; done < "$SET/SHA256SUMS"

# SHA256SUMS is read to find the assets and must also *be* one. The first
# version of this built the list from that file and so uploaded everything
# except the file a downloader checks the rest against.
assets=("$SET/SHA256SUMS")
while read -r _ name; do assets+=("$SET/$name"); done < "$SET/SHA256SUMS"

echo "== the command"
printf '  gh release create %q --repo %q --draft --title %q --notes-file <notes> \\\n' \
  "$tag" "$REPO" "Notrios $version (release candidate)"
printf '    %q \\\n' "${assets[@]}"
echo

if [[ $EXECUTE != yes ]]; then
  echo "dry run: nothing was uploaded. Pass --execute to create the draft."
  exit 0
fi

notes=$(mktemp); trap 'rm -f "$notes"' EXIT
cat > "$notes" <<NOTES
Release candidate built from $(git -C "$ROOT" rev-parse --short HEAD).

Not an end-user release. This draft exercises the release-candidate flow for
v0.9 I6; it is unsigned, and performance/v0.9-i5/POLICY.json records what
signing will mean and why no key exists yet.

Verify with:

    sha256sum -c SHA256SUMS
    python3 performance/v0.9-i6/verify_release_set.py --set .
NOTES
gh release create "$tag" --repo "$REPO" --draft \
  --title "Notrios $version (release candidate)" --notes-file "$notes" "${assets[@]}"
echo "draft created; it is not published and creates no tag until it is"
