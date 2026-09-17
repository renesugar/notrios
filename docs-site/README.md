# Notrios documentation site inputs

This directory is the production Hugo site materialized from the reviewed
G18b prototype. The 44-file `themes/hugo-theme-ledger` snapshot, including its
license, is at upstream `hugo-theme-ledger` commit
`cf68886beb31ec0bbe7e2f05e538a20fec722e5a`; G18b selected the same file list at
`f9d28ea297427890ecffa31fa74caa9ee385d9f5`, and
`performance/v0.7-g18b/prototype` still holds that reviewed snapshot unchanged.
`THEME_PROVENANCE.json` describes what is vendored here — commit, file count,
byte count and manifest hash — and G18g's validator checks the copy against it.
Refresh it with `performance/v1.0-j31/refresh_vendored_theme.sh`; do not edit
the vendored files by hand.

Install the pinned local search tool with `npm ci --prefix docs-site`, then use
`scripts/build_docs_site.sh [output-directory]` to build the site. Generated
output and `node_modules` are intentionally not tracked.
