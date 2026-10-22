#!/usr/bin/env bash
# Create the 1.0 release as a draft, from the set the tag's own workflow run built.
#
#   bash performance/v1.0-j10/draft_release.sh <set>              # dry: checks and prints
#   bash performance/v1.0-j10/draft_release.sh <set> --execute    # creates the draft
#
# v0.9 I6's script drafts release candidates: it tags `-rc` and its notes say the
# draft is not an end-user release. This one drafts the release itself, and it
# refuses a set that is not the tag's. The tag must already exist on GitHub, and
# the commit the set's provenance names must be the commit the tag points at. A
# set from a dispatch on `main`, such as J10-A's rehearsal, names a commit too,
# but not through a tag, so the tag check is what tells the two apart.
#
# Dry is the default. A draft uploads every file in the set, which is an
# external write the owner authorises; publishing the draft is a further step,
# and this script does not take it.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
SET=${1:?usage: draft_release.sh <release-set-dir> [--execute]}
EXECUTE=no
[[ ${2:-} == --execute ]] && EXECUTE=yes
REPO=${RELEASE_REPO:-renesugar/notrios}
NOTES=$ROOT/performance/v1.0-j10/RELEASE_NOTES.md

read -r package_version built_commit < <(python3 -I - "$SET/PROVENANCE.json" <<'EOF'
import json, sys
predicate = json.load(open(sys.argv[1]))["predicate"]["buildDefinition"]
commits = [d["digest"]["gitCommit"] for d in predicate["resolvedDependencies"] if "gitCommit" in d.get("digest", {})]
if len(commits) != 1:
    sys.exit(f"want one git commit in the provenance, found {commits}")
print(predicate["externalParameters"]["version"], commits[0])
EOF
)
version=${package_version%-*}
tag=v$version
product=$(sed -n 's/.*Version = "\([^"]*\)".*/\1/p' "$ROOT/internal/version/version.go")

echo "== verifying the set before anything leaves this machine"
python3 -I "$ROOT/performance/v0.9-i6/verify_release_set.py" --set "$SET"

echo "== preconditions"
[[ $version == "$product" ]] || { echo "  the set is $version, the product is $product" >&2; exit 1; }
[[ $version != *-* && $version != *rc* ]] || { echo "  $version is not a release version" >&2; exit 1; }
echo "  version: $version"
gh auth status >/dev/null 2>&1 && echo "  gh: authenticated" || { echo "  gh: NOT authenticated" >&2; exit 1; }
# An annotated tag points at a tag object; peel it to the commit.
tagged=$(gh api "repos/$REPO/commits/$tag" --jq .sha 2>/dev/null) \
  || { echo "  tag $tag: not on GitHub; push the tag, and download its run's set" >&2; exit 1; }
[[ $tagged == "$built_commit" ]] \
  || { echo "  the set was built from $built_commit, and $tag is $tagged" >&2; exit 1; }
echo "  tag $tag: $tagged, the commit the set was built from"
if gh release view "$tag" --repo "$REPO" >/dev/null 2>&1; then
  echo "  $tag already has a release; refusing to overwrite it" >&2; exit 1
fi
echo "  release $tag: none yet"
[[ -s $NOTES ]] || { echo "  no release notes at $NOTES" >&2; exit 1; }

assets=()
while IFS= read -r -d '' file; do assets+=("$file"); done \
  < <(find "$SET" -maxdepth 1 -type f -print0 | LC_ALL=C sort -z)
echo "  assets:"
printf '    %s\n' "${assets[@]##*/}"

echo "== the command"
printf '  gh release create %q --repo %q --verify-tag --draft --title %q --notes-file %q \\\n' \
  "$tag" "$REPO" "Notrios $version" "${NOTES#"$ROOT"/}"
printf '    %q \\\n' "${assets[@]##*/}"
echo

if [[ $EXECUTE != yes ]]; then
  echo "dry run: nothing was uploaded. Pass --execute to create the draft."
  exit 0
fi
gh release create "$tag" --repo "$REPO" --verify-tag --draft \
  --title "Notrios $version" --notes-file "$NOTES" "${assets[@]}"
echo "draft created; it is not published"
