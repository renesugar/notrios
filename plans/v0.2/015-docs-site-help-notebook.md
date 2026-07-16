# v0.2 Task R15 — Help notebook and documentation site

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Changes

- `docs/`: user/reference documentation per `DOCS_SITE.md` — index, service (config + Recoll sidecar), CLI, query language, GUI (modes/layout/themes/protections), import & export (archive + dry-run rename flow), REST API guide, MCP guide (profiles/tools/safety).
- `scripts/build_docs_site.sh`: converts docs/ to HTML (npx marked, shared template with nav + light/dark support), rewrites internal `.md` links, and builds a PageFind static search index (npx pagefind). Verified live in a browser: the PageFind search box on the built site returned the correct pages for "recoll".
- `.github/workflows/docs.yml`: GitHub Pages deployment (build + upload-pages-artifact + deploy-pages) on pushes to main touching docs.
- `internal/helpdocs` + `notriosctl seed-help [docs-dir]`: mirrors docs/ into the builtin read-only Help notebook with deterministic IDs (`doc_help_<path>`), updating changed notes, keeping unchanged ones, and purging notes whose source file disappeared. Verified against the real docs (8 notes) and `notebook:help` search.
- Help-note read-only enforcement (closing an R3 gap): REST PUT/PATCH/DELETE/append/prepend on Help notes return `403 forbidden`; MCP write tools refuse them. Store-level writes remain available for the seeder.

## Validation

`go test ./...` (seeder lifecycle test + REST read-only guard test), `go vet`, scaffold checks, `mvp_smoke.sh`, local site build + live PageFind search check, CLI seed of the real docs. All passing.
