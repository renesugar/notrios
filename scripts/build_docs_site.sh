#!/usr/bin/env bash
# Build the repository-owned Hugo/Ledger documentation site and local Pagefind index.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:-"$ROOT/_site"}
HUGO=${HUGO:-hugo}
PAGEFIND=${PAGEFIND:-"$ROOT/docs-site/node_modules/.bin/pagefind"}

export LC_ALL=C.UTF-8
export TZ=UTC
export SOURCE_DATE_EPOCH=0

hugo_version=$("$HUGO" version)
if [[ ! "$hugo_version" =~ hugo\ v0\.164\.0[^[:space:]]*\+extended ]]; then
  echo "build_docs_site: Hugo Extended 0.164.0 is required; got: $hugo_version" >&2
  exit 1
fi
node_version=$(node --version)
if [[ "$node_version" != "v26.3.0" ]]; then
  echo "build_docs_site: Node 26.3.0 is required; got: $node_version" >&2
  exit 1
fi
if [[ ! -x "$PAGEFIND" ]]; then
  echo "build_docs_site: local Pagefind missing; run npm ci --prefix docs-site" >&2
  exit 1
fi
pagefind_version=$("$PAGEFIND" --version)
if [[ "$pagefind_version" != "pagefind 1.5.2" ]]; then
  echo "build_docs_site: Pagefind 1.5.2 is required; got: $pagefind_version" >&2
  exit 1
fi

mapfile -d '' documents < <(find "$ROOT/docs" -type f -name '*.md' -print0 | sort -z)
if [[ "${#documents[@]}" -ne 15 ]]; then
  echo "build_docs_site: expected exactly 15 docs/**/*.md files; got ${#documents[@]}" >&2
  exit 1
fi

destination=$(realpath -m -- "$OUT")
case "$destination" in
  /|"$ROOT"|"$ROOT/docs"|"$ROOT/docs-site"|"$ROOT/.git"|"${HOME:-/nonexistent}")
    echo "build_docs_site: refusing unsafe output directory: $destination" >&2
    exit 1
    ;;
esac
mkdir -p "$(dirname "$destination")"
rm -rf -- "$destination"
mkdir -p "$destination"

temporary=$(mktemp -d "${TMPDIR:-/tmp}/notrios-docs-site.XXXXXX")
trap 'rm -rf "$temporary"' EXIT
source="$temporary/site"
mkdir -p "$source"
cp -a "$ROOT/docs-site/hugo.toml" "$ROOT/docs-site/layouts" \
  "$ROOT/docs-site/static" "$ROOT/docs-site/themes" "$source/"
# Publish the external archive contract byte-for-byte alongside the rendered
# guides. These are downloadable schemas/fixtures, not Hugo content pages.
mkdir -p "$source/static/contracts"
cp -a "$ROOT/contracts/archive-v2" "$source/static/contracts/archive-v2"
# The search landing page is reviewed site furniture. Canonical documentation
# is staged separately from docs/ below and remains byte-for-byte identical.
mkdir -p "$source/content"
cp -p "$ROOT/docs-site/content/search.md" "$source/content/search.md"
for document in "${documents[@]}"; do
  relative=${document#"$ROOT/docs/"}
  target_relative=$relative
  if [[ "$relative" == index.md ]]; then
    target_relative=_index.md
  fi
  target="$source/content/$target_relative"
  mkdir -p "$(dirname "$target")"
  cp -p "$document" "$target"
done

"$HUGO" --source "$source" --destination "$destination" --cleanDestinationDir --gc --minify --environment production
"$PAGEFIND" --site "$destination"
echo "docs site built at $OUT"
