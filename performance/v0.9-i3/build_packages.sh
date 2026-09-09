#!/usr/bin/env bash
# Build the two packages the I3 matrix upgrades between.
#
# The release package is built from the checkout. The prerelease is built from a
# disposable `git worktree` with `internal/version/version.go` patched, so the
# binary it ships reports the same version its control file declares. Overriding
# only the packaged version would have been one line and a lie: the rehearsal is
# about upgrading between versions, and a package whose binary disagrees with
# its metadata would make the interesting assertion -- that the installed binary
# changed -- untestable.
#
# `0.8.0~rc1` rather than `0.8.0-rc1`: in Debian ordering `~` sorts *before* the
# release, which is what makes this an upgrade rather than a downgrade. That is
# the whole point of the pair, and getting it backwards would have the matrix
# rehearse the opposite of what it claims.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
OUT=${1:-$ROOT/dist/i3-packages}
PRERELEASE=${I3_PRERELEASE_VERSION:-0.8.0~rc1}
mkdir -p "$OUT"

release_version=$(sed -n 's/.*Version = "\([^"]*\)".*/\1/p' "$ROOT/internal/version/version.go")
echo "release: $release_version   prerelease: $PRERELEASE"

if [[ ! -f "$OUT/notrios_${release_version}-1_amd64.deb" ]]; then
  bash "$ROOT/scripts/build_deb.sh" "$OUT" >/dev/null
  echo "built $OUT/notrios_${release_version}-1_amd64.deb"
fi

if [[ ! -f "$OUT/notrios_${PRERELEASE}-1_amd64.deb" ]]; then
  # Under dist/ rather than /tmp: the hardlinks below need the same filesystem
  # as the checkout, and /tmp is tmpfs here. dist/ is git-ignored.
  work=$(mktemp -d "$ROOT/dist/notrios-i3-XXXXXX")
  trap 'git -C "$ROOT" worktree remove --force "$work/tree" 2>/dev/null || true; rm -rf "$work"' EXIT
  git -C "$ROOT" worktree add --detach --quiet "$work/tree" HEAD
  sed -i "s/Version = \"$release_version\"/Version = \"$PRERELEASE\"/" \
    "$work/tree/internal/version/version.go"
  # Hardlink the installed frontend dependencies rather than reinstalling them:
  # same lockfile, same tree, and a second `npm ci` would cost minutes to
  # produce an identical directory.
  cp -al "$ROOT/web/node_modules" "$work/tree/web/node_modules"
  # build_deb.sh packages web/dist and refuses without it. `make deb` builds the
  # frontend first; calling the script directly does not, so it is built here --
  # from this worktree's own sources, which is the point of using a worktree.
  (cd "$work/tree/web" && npm run build >/dev/null 2>&1)
  (cd "$work/tree" && bash scripts/build_deb.sh "$OUT" >/dev/null)
  echo "built $OUT/notrios_${PRERELEASE}-1_amd64.deb"
fi

ls -1 "$OUT"/*.deb
