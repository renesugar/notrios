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
- documentation (`docs/`, `docs-site/` without dependencies, design documents),
  plans, skills, prompts, fixtures, and validation scripts. The packager builds
  and validates the pinned Hugo/Ledger/Pagefind site before ZIP creation.

Excluded (enforced by both the zip exclusions and `check_release_zip.py`):

- `.git/` history, `node_modules/` at any depth (including `web/` and
  `docs-site/`);
- runtime `data/` directories, SQLite databases and their WAL/SHM sidecars;
- build/test/dev artifacts: `bin/`, `dist/`, `_site/`, `.playwright-mcp/`, `__pycache__`/`*.pyc`, coverage output, editor backups, other ZIPs, `.claude/`.

Because tests and the web build run first, a ZIP is only produced from a
validated tree. File selection and exclusions are deterministic and verified;
byte-for-byte ZIP reproducibility is not claimed because archive timestamps and
hashed build assets may vary. Each handoff records the exact ZIP SHA-256.

The v0.7 G20 gate also validates the exact Go/npm dependency-license inventory,
frozen aggregate convergence/recovery evidence, the security finding
dispositions, and upgrade/rollback notes before packaging.

## Binary "packaging" today

`make build` and `make gui` produce local binaries under `bin/` (see [docs/installation.md](docs/installation.md)). Two runtime facts matter for anyone redistributing them informally:

- the browser UI is served from `web/dist/` relative to the working directory, so a bare binary without that directory serves the API only;
- the SQLite store statically links the vendored SQLite amalgamation
  (v0.8 H1 slice C), so a binary carries its own engine and depends only on a
  compatible glibc, not on any installed `libsqlite3`. The exact version,
  hashes, compile options, and update policy are in
  `internal/store/csqlite/PROVENANCE.json`, enforced by
  `python3 scripts/check_sqlite_provenance.py`.

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

One dependency is vendored rather than resolved by a package manager, so it
appears in neither the Go-module nor the npm inventory: the SQLite amalgamation
under `internal/store/csqlite/`. SQLite is public domain, which imposes no
conditions and is compatible with Apache-2.0; the upstream blessing is
reproduced in `internal/store/csqlite/NOTICE` and redistributed with the
source. A release audit should read that notice alongside the generated
inventories, because no dependency scanner will find it for you.
