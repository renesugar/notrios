# Packaging

Notrios currently ships as **source only**. The packaging workflow produces a source-first ZIP for archiving or importing into another repository host — it is not a binary distribution and not an OS installer, and no prebuilt binaries are published.

## The source release ZIP

```bash
bash scripts/package_release.sh                 # writes dist/notrios-src.zip
bash scripts/package_release.sh /path/out.zip   # explicit output path
```

The script runs, in order: `go test ./...`, the required-files check, scaffold validation, `npm ci` + the production web build, ZIP creation, and ZIP-content verification (`scripts/check_release_zip.py`). The default output lands in the git-ignored `dist/` directory so archives cannot be committed by accident.

Included:

- Go source (`cmd/`, `internal/`, embedded migrations under `internal/store/migrations/`);
- React source (`web/src/`) **and** the freshly built `web/dist/` assets;
- `web/package-lock.json` for reproducible dependency installation;
- documentation (`docs/`, design documents), plans, skills, prompts, fixtures, and validation scripts.

Excluded (enforced by both the zip exclusions and `check_release_zip.py`):

- `.git/` history, `web/node_modules/`;
- runtime `data/` directories, SQLite databases and their WAL/SHM sidecars;
- build/test/dev artifacts: `bin/`, `dist/`, `_site/`, `.playwright-mcp/`, `__pycache__`/`*.pyc`, coverage output, editor backups, other ZIPs, `.claude/`.

Because tests and the web build run first, a ZIP is only produced from a validated tree. The archive contents are deterministic apart from build-time asset hashes in `web/dist/`.

## Binary "packaging" today

`make build` and `make gui` produce local binaries under `bin/` (see [docs/installation.md](docs/installation.md)). Two runtime facts matter for anyone redistributing them informally:

- the browser UI is served from `web/dist/` relative to the working directory, so a bare binary without that directory serves the API only;
- the SQLite store links against the system `libsqlite3` (cgo), so binaries are tied to a compatible glibc/libsqlite3.

A real installer/package story (self-contained assets, installed data/config
locations, permissions, native credential stores, per-OS packages, upgrades,
and mobile portability) is the distinct v0.8 roadmap milestone. That milestone
also adds a versioned no-GUI C ABI/shared-library build with explicit runtime,
SQLite, header, ownership, and platform packaging rules; it is not part of the
source ZIP today. Pre-1.0 mobile packaging evidence is Android-emulator-only.
v0.9 hardens release candidates; v1.0 creates user-authorized installable
GitHub releases. The Flutter client and physical mobile artifacts follow after
1.0.

## Tagging a release

Validate first (`RELEASE_CHECKLIST.md` has the current checklist and push/tag sequence):

```bash
go test ./...
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh
```

The project license is Apache-2.0 (`LICENSE`); dependency license audits are recorded in `RELEASE_CHECKLIST.md`.
