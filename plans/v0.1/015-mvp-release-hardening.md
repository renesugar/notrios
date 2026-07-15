# 015 — MVP Release Hardening

Status: completed in MVP Task 10.

## Scope

Harden the v0.1 MVP release candidate so it can be checked into Gitea/GitHub and resumed by Codex from repository files alone.

## Completed work

- Added generated-dataset store smoke test and benchmark.
- Added REST/MCP/resource smoke script.
- Added performance smoke script.
- Added repeatable release ZIP packaging script.
- Added ZIP-content verifier to prevent missing `web/dist` and accidental `node_modules`/runtime-data inclusion.
- Added release, packaging, and security review documents.
- Hardened resource download headers.
- Updated validation and required-file checks.
- Generated the next draft plan from `ROADMAP.md` v0.2.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp-check.zip
python3 scripts/check_release_zip.py /tmp/notes-companion-v0.1.0-mvp-check.zip
```

## Working state

The project can be tagged as a local-only v0.1 MVP candidate after user review and license selection.
