# Environment Setup (contributors)

This guide sets up a development environment for working on the existing Notrios repository. End users should start with the friendlier [docs/installation.md](docs/installation.md); this file adds contributor-specific workflow, validation, and cleanup detail.

## Platform

Development and CI target **Ubuntu Linux**. Other platforms are untested.

## Required tools

Versions match `go.mod` and `.github/workflows/ci.yml`:

- Git
- **Go 1.25 or newer** (`go.mod` declares `go 1.25.0`)
- A C toolchain and `pkg-config` (the SQLite store is a cgo wrapper over system `libsqlite3`; keep `CGO_ENABLED=1`)
- `libsqlite3-dev`
- **Node.js 22** and npm (web UI and docs-site builds; Node ≥ 20.19 may work but 22 is what CI tests)
- **Python 3** — used only by the repository validation scripts (`scripts/check_required_files.py`, `scripts/check_release_zip.py`, `scripts/check_plan_loops.py`); not needed at runtime
- Bash, `make`

For GUI work additionally: `libgtk-3-dev`, `libwebkit2gtk-4.1-dev` (the exact packages CI installs for the GUI compile check).

Optional: `go-task` (the `Taskfile.yml` mirrors the main Make targets), `sqlite3` CLI, `jq`, `zip` (release archives), `xvfb` (running the GUI headless), `recoll` (the optional search sidecar — GPL, always an external process). GnuPG 2.4.4, OpenSSL 3.0.13, and xorriso 1.5.6 are the recorded external G17b sealing/verifier boundaries; they are not application runtime dependencies. G17b completed under its 2026-08-25 exact-key, credential, TSA, and reserve authorization. GitHub push and physical optical burning remain unauthorized. On the owner workstation, run `bash scripts/verify_evidence_pre_push.sh` before every future push.

```bash
sudo apt update
sudo apt install -y git build-essential pkg-config libsqlite3-dev python3 make zip jq sqlite3
sudo apt install -y nodejs npm            # or Node 22 from nodesource/a version manager
sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev   # GUI work only
```

## Getting the repository

```bash
git clone https://github.com/renesugar/notrios.git
cd notrios
```

Active development happens on `develop`; `main` takes reviewed merges. Never commit private note exports, local databases, or asset stores — `data/`, `private-testdata/`, and `exports/` are git-ignored for that reason.

## Everyday commands

`make help` lists everything, and so does the table in
[`docs/installation.md`](docs/installation.md#build). The common loop:

```sh
make test          # go test ./...
make validate      # tests + required-files + script syntax checks
make build         # bin/notriosd + bin/notriosctl
make web           # web/dist/ (npm ci runs automatically on first build)
make gui           # bin/notrios desktop binary
make serve         # run the service from source on 127.0.0.1:8080
make docs          # _site/ documentation site (uses npx marked + pagefind)
make docgen        # check committed generated documentation (no model)
make g18f-validate # validate committed G18f evidence (no model)
make smoke         # end-to-end REST/MCP smoke test on a loopback port
make doctor        # notriosctl doctor: configuration and environment check
make seed-help     # mirror docs/ into the Help notebook of the default database
```

G18f semantic review is optional and maintainer-local. When the approved local
Qwen GGUF is available through llama.cpp on `127.0.0.1:8081`, run
`make doccheck`. The client rejects non-loopback and HTTPS endpoints, uses no
note/database data, and records an advisory report without changing prose.

To see the interface while working on it, `make serve` and open
<http://127.0.0.1:8080> — no binary build needed. For the frontend dev server
with hot reload, run `make serve` in one terminal and `npm run dev` in `web/` in
another; Vite proxies the API to port 8080.

## Runtime sanity checks

```bash
go run ./cmd/notriosd -config config/config.example.yaml
curl http://127.0.0.1:8080/api/v1/status | jq          # schema version, storage roots, capabilities
curl -X POST http://127.0.0.1:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'   # MCP tool listing
go run ./cmd/notriosctl doctor                           # environment/config/database diagnostics
```

Relative config paths (the example uses `./data/...`) resolve against the working directory. For clean testing, delete `data/notes.sqlite` (plus its `-wal`/`-shm` sidecars) and restart.

## Agent usage preflight

Before starting a long local profile, release package, or multi-pass validation
from an agent session, inspect the current account windows without making a
model request:

```bash
python3 scripts/test_check_agent_usage.py
python3 scripts/check_agent_usage.py --agent all --minimum-remaining 20
```

The Codex path uses the installed app-server protocol and reports both the
five-hour and weekly windows when available. The Claude path reads only an
explicit local status cache; the current Claude Code 2.1.246 settings do not
provide one, so `unknown` is expected and is advisory by default. Missing or
changed telemetry is never reported as 100%.

Set `NOTRIOS_AGENT_USAGE_GUARD=strict` to make unknown telemetry stop a long
script before it begins, `off` to skip the local probe, and
`NOTRIOS_AGENT_USAGE_MINIMUM` to override the default 20% floor. Model/effort-
specific history is optional and stays outside repository evidence. A pause
means update the repository handoff at the current durable boundary; it does
not make an interrupted non-checkpointed script resumable.

## Cleanup and pre-checkin

`make clean` removes disposable outputs and nothing else:

- `bin/` (Go binaries), `dist/` (release archives), `_site/` (docs site), `web/dist/` (web assets), `.playwright-mcp/` (browser-automation output);
- `coverage.out`/`coverage.*`, `*.test`, `*.prof`, stray root binaries, `notrios-*.zip`;
- `__pycache__/` directories, `*.pyc`, editor `*~` backups.
- Generated G14c snapshot profile databases and packages. Keep them under
  `/tmp` and commit only aggregate JSON from
  `bash scripts/run_snapshot_image_profile.sh <output.json>`.

Deliberately preserved: `data/` and any databases/asset stores (user data), `config/`, `testdata/` fixtures, `web/node_modules/` (use `make clobber` to remove dependencies too), and all sources.

Before committing:

```bash
make clean
make precheck        # shows git status and fails if any tracked file matches .gitignore
git status --short --untracked-files=all
```

`git ls-files -ci --exclude-standard` is the underlying check for tracked-but-ignored files; if it ever lists generated artifacts that were committed by mistake, remove them from tracking with `git rm --cached <path>` (or `git rm` to also delete them) in a dedicated commit.

## Test data

Do not commit private exports. Synthetic fixtures live in importer tests and `testdata/`; derived JSON Schemas for import formats are under `testdata/schemas/`. Local private datasets belong outside the repository or under the ignored `private-testdata/` path.

## Validation before merging

```bash
go vet ./...
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm audit && npm audit fix       # regular, non-forced maintenance pass
cd web && npm ci && npm audit && npm run typecheck && npm test -- --run && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/run_sync_journal_profile.sh /tmp/notrios-g4-journal.json
go test ./internal/syncstate ./internal/store -run 'TestSyncAdmission|TestThreeReplicaModel'
bash scripts/run_joplin_import_profile.sh 100 /tmp/notrios-joplin.json
bash scripts/run_obsidian_import_profile.sh 100 /tmp/notrios-obsidian.json
```

These are the same checks CI runs (plus CI's GUI compile check with `-tags "gui desktop production webkit2_41"`).
