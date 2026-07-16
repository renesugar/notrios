#!/usr/bin/env bash
# Build the GitHub Pages documentation site from docs/ (task R15):
# Markdown -> HTML via marked, then a PageFind static search index.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:-"$ROOT/_site"}

rm -rf "$OUT"
mkdir -p "$OUT"

nav='<nav class="site-nav">
  <a href="/index.html"><strong>Notrios</strong></a>
  <a href="/service.html">Service</a>
  <a href="/cli.html">CLI</a>
  <a href="/query-language.html">Query language</a>
  <a href="/gui.html">GUI</a>
  <a href="/import-export.html">Import &amp; export</a>
  <a href="/api/rest.html">REST API</a>
  <a href="/api/mcp.html">MCP</a>
</nav>
<div id="search"></div>'

style='body{font-family:ui-sans-serif,system-ui,sans-serif;max-width:860px;margin:0 auto;padding:24px;line-height:1.6;color:#172033}
a{color:#2563eb}
code{background:#eef1f6;padding:1px 5px;border-radius:4px;font-size:0.92em}
pre{background:#0f172a;color:#e2e8f0;padding:14px;border-radius:10px;overflow-x:auto}
pre code{background:transparent;padding:0;color:inherit}
table{border-collapse:collapse}
td,th{border:1px solid #d9dee7;padding:6px 10px;text-align:left}
.site-nav{display:flex;gap:14px;flex-wrap:wrap;border-bottom:1px solid #d9dee7;padding-bottom:12px;margin-bottom:8px}
#search{margin:12px 0 24px}
@media (prefers-color-scheme: dark){body{background:#0f172a;color:#e2e8f0}code{background:#1e293b}td,th{border-color:#334155}.site-nav{border-color:#334155}a{color:#60a5fa}}'

find "$ROOT/docs" -name '*.md' | while read -r src; do
  rel=${src#"$ROOT/docs/"}
  dest="$OUT/${rel%.md}.html"
  mkdir -p "$(dirname "$dest")"
  title=$(grep -m1 '^# ' "$src" | sed 's/^# //' || true)
  body=$(npx --yes marked --gfm < "$src")
  cat > "$dest" <<HTML
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title:-Notrios} — Notrios</title>
<link href="/pagefind/pagefind-ui.css" rel="stylesheet">
<script src="/pagefind/pagefind-ui.js"></script>
<style>${style}</style>
</head>
<body>
${nav}
<main data-pagefind-body>
${body}
</main>
<script>
window.addEventListener('DOMContentLoaded', () => {
  if (window.PagefindUI) {
    new PagefindUI({ element: '#search', showSubResults: true });
  }
});
</script>
</body>
</html>
HTML
  echo "built ${rel%.md}.html"
done

# Internal .md links -> .html
find "$OUT" -name '*.html' -exec sed -i 's/href="\([^"#]*\)\.md\(#[^"]*\)\?"/href="\1.html\2"/g' {} +

npx --yes pagefind --site "$OUT"
echo "docs site built at $OUT"
