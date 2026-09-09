#!/usr/bin/env bash
# v0.8e E1 — build the handoff archives twenty-seven v0.8 slices never got.
#
# Each archive is built from its own close-out commit in a disposable
# `git worktree`, so the live checkout is never touched and each reflects the
# repository as it stood when that slice finished. The commits come from
# CLOSEOUTS.json, which was resolved and checked rather than assumed.
#
# It does not run today's gates against yesterday's trees. A historical commit
# fails gates written after it -- pinned counts that have since moved, an npm
# advisory published later, a document count that grew -- and none of that says
# anything about the archive. What an archive has to prove is that it *is* that
# commit: every tracked file byte-identical to the commit's tree, and
# scripts/check_release_zip.py clean. Those are both checked below.
#
# It uses *today's* packaging exclusions, which is the one place the present
# must win over the past: v0.8 H13 added the `.zvec-grep/` exclusion, and
# without it every archive would carry a 95 MB local index.
#
#   bash performance/v0.8e/backfill.sh
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
closeouts=$root/performance/v0.8e/CLOSEOUTS.json
outdir=${NOTRIOS_BACKFILL_OUT:-$root/dist/backfill}
evidence=${NOTRIOS_EVIDENCE_DIR:-$HOME/evidence/notrios}
work=$(mktemp -d "${TMPDIR:-/tmp}/notrios-backfill.XXXXXX")
results=$outdir/RESULTS.jsonl
trap 'git -C "$root" worktree prune; rm -rf "$work"' EXIT

mkdir -p "$outdir"
: > "$results"

say() { printf '\n== %s\n' "$1"; }

# One npm install per distinct lockfile rather than one per commit. Thirty-three
# commits carry three lockfiles between them, so installing per commit would
# repeat identical work thirty times. Each install comes from *that commit's*
# lockfile rather than today's, which v0.8 H13 changed when it upgraded vitest:
# the archive has to carry the frontend that commit would have built.
install_for() { # commit -> prints the node_modules directory to use
  local commit=$1
  local hash
  hash=$(git -C "$root" show "$commit:web/package-lock.json" | sha256sum | cut -c1-12)
  local cache=$work/npm-$hash
  if [[ ! -d "$cache/web/node_modules" ]]; then
    mkdir -p "$cache/web"
    git -C "$root" show "$commit:web/package.json"      > "$cache/web/package.json"
    git -C "$root" show "$commit:web/package-lock.json" > "$cache/web/package-lock.json"
    say "installing web dependencies for lockfile $hash" >&2
    (cd "$cache/web" && npm ci --silent --no-audit --no-fund >/dev/null 2>&1)
  fi
  printf '%s' "$cache/web/node_modules"
}

count=0
python3 -c "
import json
for e in json.load(open('$closeouts'))['items']:
    print(e['item'], e['commit'])
" | while read -r item commit; do
  count=$((count + 1))
  short=${commit:0:7}
  tree=$work/tree-$item
  say "$item at $short"

  git -C "$root" worktree add --detach --quiet "$tree" "$commit"
  # The built frontend is not in any commit -- it is gitignored, and the
  # packager builds it before zipping. The archive needs it: check_release_zip
  # requires web/dist/index.html and web/dist/assets/.
  cp -a "$(install_for "$commit")" "$tree/web/node_modules"
  (cd "$tree/web" && npm run build >/dev/null 2>&1)

  zip_path=$outdir/notrios-v0.8-${item,,}-$short.zip
  rm -f "$zip_path"
  (cd "$tree" && zip -qr "$zip_path" . \
    -x 'node_modules/*' -x 'web/node_modules/*' -x '*/node_modules/*' \
    -x 'data/*' -x 'data/' -x '*/data/*' -x '*/data/' \
    -x '.git/*' -x '*.sqlite' -x '*.sqlite-*' -x 'tmp/*' -x '.DS_Store' \
    -x 'dist/*' -x 'bin/*' -x '_site/*' -x '.playwright-mcp/*' \
    -x '*/__pycache__/*' -x '*.pyc' -x '*~' -x '.claude/*' \
    -x 'notrios-*.zip' -x 'coverage.*' \
    -x 'notrios' -x 'notriosd' -x 'notriosctl' -x 'notrioslib' \
    -x '.zvec-grep/*' -x '.zvec-grep')

  python3 "$root/scripts/check_release_zip.py" "$zip_path" >/dev/null
  python3 "$root/performance/v0.8e/verify_archive.py" "$zip_path" "$commit" "$root"

  cp -p "$zip_path" "$evidence/"
  cmp -s "$zip_path" "$evidence/$(basename "$zip_path")"

  python3 - "$item" "$commit" "$zip_path" >> "$results" <<'PY'
import hashlib, json, os, sys, zipfile
item, commit, path = sys.argv[1:4]
data = open(path, 'rb').read()
with zipfile.ZipFile(path) as archive:
    entries = len(archive.namelist())
print(json.dumps({"item": item, "commit": commit, "archive": os.path.basename(path),
                  "bytes": len(data), "entries": entries,
                  "sha256": hashlib.sha256(data).hexdigest(), "retroactive": True}))
PY
  git -C "$root" worktree remove --force "$tree"
  printf '   %s: %s bytes\n' "$(basename "$zip_path")" "$(stat -c%s "$zip_path")"
done

say "done"
wc -l < "$results" | xargs printf '%s archives built, verified and copied to %s\n' >&2 || true
