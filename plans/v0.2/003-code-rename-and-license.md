# v0.2 Task R2 — Code rename and Apache-2.0 license

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5). User decisions: proceed with R2; license = Apache-2.0.

## Changes

- Go module `example.com/notes-companion` → `github.com/renesugar/notrios`; all imports updated.
- `cmd/notesd` → `cmd/notriosd`; `cmd/notesctl` → `cmd/notriosctl`; CLI usage strings, log lines, MCP service name (`notrios`), and temp asset dir renamed.
- Config: section `sist2` → `search_sidecar` (`enabled`, `binary` default `recollindex`, `index_dir` default `./data/search-index`); `Sist2Config` → `SearchSidecarConfig`; status JSON `sist2_index_dir` → `search_sidecar_index_dir`; capability flag `sist2` → `search_sidecar`; OpenAPI collection kind `sist2` → `sidecar_indexed`; `web/src/api.ts` type updated; example config and smoke-test config updated.
- `LICENSE` (Apache-2.0, Copyright 2026 Rene Sugar) replaces `LICENSE_PENDING.md`; `LICENSE` added to `scripts/check_required_files.py`.
- Makefile, Taskfile.yml, `scripts/mvp_smoke.sh`, `scripts/package_release.sh`, `scripts/check_release_zip.py`, `web/package.json`/`package-lock.json` (`notrios-web`), editor element id, and living docs updated to the new names.

## Validation

See `agent/ATTEMPT_LOG.jsonl` entry `redesign-r2` for the executed command list (full validation set including web build and smoke scripts).

Commit: "R2: rename to Notrios in code and add Apache-2.0 license".
