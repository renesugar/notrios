# Notrios documentation site inputs

This directory is the production Hugo site materialized from the reviewed
G18b prototype. The 44-file `themes/hugo-theme-ledger` snapshot, including its
license, is copied from `performance/v0.7-g18b/prototype` at upstream
`hugo-theme-ledger` commit `f9d28ea297427890ecffa31fa74caa9ee385d9f5`.

Install the pinned local search tool with `npm ci --prefix docs-site`, then use
`scripts/build_docs_site.sh [output-directory]` to build the site. Generated
output and `node_modules` are intentionally not tracked.
