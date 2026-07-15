# Packaging

This document describes the MVP packaging workflow. The current package is a source-first ZIP suitable for creating or updating a Gitea/GitHub repository. It is not yet an OS installer.

## Release package contents

The MVP ZIP should include:

- Go service and CLI source under `cmd/` and `internal/`;
- migrations embedded under `internal/store/migrations/`;
- React source under `web/src/`;
- built production UI assets under `web/dist/`;
- docs, plans, skills, prompts, test fixtures, and validation scripts;
- `web/package-lock.json` for reproducible UI dependency installation.

The ZIP should exclude:

- `.git/` history;
- `web/node_modules/`;
- runtime `data/` directories;
- local SQLite files and WAL/SHM sidecars;
- temporary test output.

## Build and package command

Run from the repository root:

```bash
bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp.zip
```

That script runs the Go tests, scaffold validation, UI dependency install, UI production build, ZIP creation, and ZIP-content verification.

## Manual validation commands

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

## Development branch workflow

Recommended first repository setup:

```bash
git init
git add .
git commit -m "Initial notes companion MVP scaffold"
git branch -M main
git checkout -b develop
```

Do active work on `develop` or task branches. Merge to `main` only after the MVP smoke tests, generated-dataset smoke tests, and release ZIP checks pass.

## Version tag candidate

After review, the MVP can be tagged as:

```bash
git tag -a v0.1.0-mvp -m "Notrios (Notes Companion) MVP"
```

Do not tag until the user has reviewed the ZIP and selected a license in `LICENSE_PENDING.md`.

## Current package limitations

- No platform-specific installer is produced.
- The service currently uses a local cgo SQLite wrapper; consider replacing it with a maintained SQLite driver in a less constrained environment.
- The MCP endpoint is a dependency-free MVP JSON-RPC adapter, not the official Go SDK transport.
- The search sidecar (now Recoll, formerly sist2), Quartz, and remote-media localization are documented but not implemented in v0.1.
