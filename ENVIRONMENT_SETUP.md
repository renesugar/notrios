# Environment Setup

## Target development OS

Primary target: Ubuntu Linux.

## Required tools for the current scaffold

- Git
- Go 1.22 or newer
- Python 3.10 or newer
- Bash

## Required tools for the web UI

- Node.js 20 or newer
- npm 10 or newer

## Recommended tools

- `go-task` for `Taskfile.yml`
- `make`
- `sqlite3` command-line shell
- `libsqlite3-dev` / SQLite development headers for the Step 4 cgo-backed store
- `ripgrep`
- `jq`
- `xvfb` for headless UI tests
- `sist2` for future integration work
- `git-lfs` only if future Git workflows require large-file support outside go-git

Ubuntu example:

```bash
sudo apt update
sudo apt install -y git golang-go python3 python3-venv make sqlite3 libsqlite3-dev pkg-config ripgrep jq xvfb nodejs npm
```

If Ubuntu packages are too old, install Go and Node from upstream sources or a tool manager.

## Optional Task runner

```bash
go install github.com/go-task/task/v3/cmd/task@latest
```

## Repository setup

```bash
git init
git checkout -b develop
git add .
git commit -m "Initial Notes Companion scaffold"
```

Before pushing to Gitea/GitHub:

1. Create the remote repository.
2. Push `develop` first.
3. Create `main` only when ready, or protect `main` immediately.
4. Configure branch protection for `main`.
5. Require CI before merging to `main`.

## Test data needed later

Do not commit private test data. Store local fixtures outside the repo or under ignored paths.

Needed datasets:

- Small synthetic Markdown vault.
- Small Obsidian vault with Wikilinks, embeds, aliases, headings, block refs.
- Joplin RAW Export Directory fixture.
- Twitter/X archive fixture.
- ChatGPT conversations export fixture.
- Claude conversations JSON fixture.
- Image/PDF sample resources.
- Remote-image localization fixture with local test HTTP server.
- Large generated datasets for performance tests.

See `testdata/README.md`.

## Step 4 SQLite note

The scaffold currently uses a small local cgo adapter over system `libsqlite3` because external Go module downloads were unavailable during scaffold creation. A later implementation step may replace this adapter with a pinned Go SQLite driver if project policy prefers it. Until then, keep `CGO_ENABLED=1`, `pkg-config`, and SQLite development headers available.

## Runtime configuration check

Run the service with the example config:

```bash
go run ./cmd/notesd -config config/config.example.yaml
```

The service creates the configured data, asset, projection, and sist2 index directories before opening SQLite. Use `-addr` and `-db` only as explicit overrides for quick local smoke tests:

```bash
go run ./cmd/notesd -addr 127.0.0.1:8081 -db /tmp/notes-companion.sqlite
```

Confirm runtime state:

```bash
curl http://127.0.0.1:8080/api/v1/status | jq
```

Expected fields include `database_info.schema_version`, `storage.asset_store`, `storage.projection_dir`, and `capabilities.search.fts5`.

## MVP Task 2 development notes

The service now reports SQLite schema version 4 after bootstrap. Existing local development databases from MVP Tasks 1–2 can be opened; bootstrap applies narrow compatibility shims for revision columns, resource indexes, and link graph columns. For clean testing, delete `data/notes.sqlite` and restart `notesd`.

### Resource upload smoke test

```bash
printf 'hello resource' | curl -s -X POST \
  'http://127.0.0.1:8080/api/v1/resources?filename=hello.txt' \
  -H 'Content-Type: text/plain' --data-binary @-
```

The response contains a logical resource ID, `resource://` URI, byte count, and SHA-256 hash. Use `GET /api/v1/resources/{id}/content?download=1` to download the bytes.

### Link/backlink smoke test

Create two notes, then create a source note that links to the target by title or `document://` URI. Inspect outgoing links and backlinks:

```bash
curl 'http://127.0.0.1:8080/api/v1/documents/<source_doc_id>/links?direction=both' | jq
curl 'http://127.0.0.1:8080/api/v1/documents/<target_doc_id>/links?direction=incoming' | jq
curl -X POST http://127.0.0.1:8080/api/v1/graph \
  -H 'Content-Type: application/json' \
  -d '{"roots":["<source_doc_id>"],"direction":"both","max_nodes":20,"max_edges":40}' | jq
```


## Built-in UI production smoke test

After building the web UI, `notesd` can serve it from `web/dist`:

```bash
cd web && npm ci && npm run build
cd ..
go run ./cmd/notesd -addr 127.0.0.1:8080
# open http://127.0.0.1:8080/
```

For iterative frontend development, continue using `cd web && npm run dev`; Vite proxies `/api` and `/healthz` to `notesd`.


## MCP smoke test

After starting `notesd`, list MCP tools with:

```bash
curl -X POST http://127.0.0.1:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

The current MCP adapter is dependency-free for this scaffold. In a normal development environment, Codex may replace it with the official MCP Go SDK after pinning the dependency and updating tests.

## Joplin RAW importer MVP

Run a dry-run scan:

```bash
go run ./cmd/notesctl import joplin-raw --dry-run /path/to/joplin-raw-export
```

Import into the default local database:

```bash
go run ./cmd/notesctl import joplin-raw \
  --db ./data/notes.sqlite \
  --asset-store ./data/assets \
  --collection default \
  /path/to/joplin-raw-export
```

The importer expects a Joplin RAW Export Directory, not a JEX archive. Keep private exports out of Git.


## Obsidian importer MVP

Run a dry-run scan:

```bash
go run ./cmd/notesctl import obsidian --dry-run /path/to/obsidian-vault
```

Import into the default local database:

```bash
go run ./cmd/notesctl import obsidian \
  --db ./data/notes.sqlite \
  --asset-store ./data/assets \
  --collection default \
  /path/to/obsidian-vault
```

The importer expects a directory containing Markdown files and local assets. It skips `.obsidian`, VCS directories, and dependency folders. Keep private vaults and imported asset stores out of Git.


## MVP smoke and packaging commands

After installing Go, Node, npm, and SQLite development headers, the release-candidate validation flow is:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp.zip
```

`mvp_smoke.sh` starts `notesd` on a loopback test port, creates and updates a note, searches it, uploads/downloads a resource, and verifies MCP tool listing. `run_performance_smoke.sh` runs the generated-dataset store smoke test and benchmark.
