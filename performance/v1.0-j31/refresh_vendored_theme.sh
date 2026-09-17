#!/usr/bin/env bash
# J31: refresh docs-site/themes/hugo-theme-ledger from an upstream commit.
#
#   bash performance/v1.0-j31/refresh_vendored_theme.sh [ref]
#
# It clones the upstream repository fresh (never a local working copy, which may
# be mid-change), checks out `ref` (default: origin/main), and copies only the
# files the vendored runtime snapshot already has. A file upstream added to a
# vendored directory is reported and not copied: what ships is a decision
# somebody makes, not whatever upstream grew. It then rewrites
# docs-site/THEME_PROVENANCE.json from the bytes on disk.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
UPSTREAM=${J31_UPSTREAM:-https://github.com/renesugar/hugo-theme-ledger}
ref=${1:-origin/main}
vendored="$ROOT/docs-site/themes/hugo-theme-ledger"
clone=$(mktemp -d "${TMPDIR:-/tmp}/notrios-j31.XXXXXX")
trap 'rm -rf "$clone"' EXIT

git clone -q "$UPSTREAM" "$clone/theme"
commit=$(git -C "$clone/theme" rev-parse "$ref")
git -C "$clone/theme" checkout -q "$commit"
echo "upstream $UPSTREAM at $ref = $commit"

previous=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["upstream_commit"])' "$ROOT/docs-site/THEME_PROVENANCE.json")
if [[ "$previous" != "$commit" ]]; then
  echo "commits between the vendored pin and this one:"
  git -C "$clone/theme" log --format='  %h %s' "$previous..$commit" || true
fi

copied=0 missing=0
while IFS= read -r relative; do
  if [[ ! -f "$clone/theme/$relative" ]]; then
    echo "MISSING upstream: $relative" >&2
    missing=$((missing + 1))
    continue
  fi
  install -Dm644 "$clone/theme/$relative" "$vendored/$relative"
  copied=$((copied + 1))
done < <(cd "$vendored" && find . -type f | sed 's|^\./||' | sort)

# Report what upstream has inside the vendored directories that we do not take.
while IFS= read -r candidate; do
  [[ -f "$vendored/$candidate" ]] || echo "NOT vendored (upstream has it): $candidate"
done < <(cd "$clone/theme" && find $(cd "$vendored" && find . -mindepth 1 -maxdepth 1 -type d | sed 's|^\./||') -type f 2>/dev/null | sort)

python3 "$ROOT/performance/v1.0-j31/write_provenance.py" "$commit"
echo "copied $copied file(s), $missing missing upstream"
[[ "$missing" -eq 0 ]]
