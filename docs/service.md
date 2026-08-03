# The Notrios service

`notriosd` is the headless service: one process serving the REST API (`/api/v1`), the MCP endpoint (`/mcp`), and the built-in web UI. Build it with `make build-service` (output `bin/notriosd`) or run it from source:

```sh
go run ./cmd/notriosd -config config/config.example.yaml
```

The `notrios` desktop binary embeds the same service; `notrios -no-gui` behaves exactly like `notriosd`. See the [GUI guide](gui.md).

## Command-line flags

| Flag | Meaning |
|---|---|
| `-config <path>` | configuration file to load |
| `-addr host:port` | overrides `server.listen_addr` |
| `-db <path>` | overrides `data.database_path` (`:memory:` gives a throwaway database) |

**Configuration discovery:** with no `-config` flag, the service loads `config/config.example.yaml` if it exists in the working directory (convenient in a source checkout) and otherwise falls back to compiled defaults. **Precedence:** command-line flags override the config file, which overrides the built-in defaults.

## Configuration reference

The authoritative, always-current example is `config/config.example.yaml` in the repository. The parser accepts a deliberately small YAML subset (two-level sections of scalar values); unknown keys are ignored so configs stay forward-compatible.

| Section / key | Default | Meaning |
|---|---|---|
| `server.listen_addr` | `127.0.0.1:8080` | bind address — see the security note below |
| `server.public_base_url` | `http://127.0.0.1:8080` | base URL advertised to clients |
| `data.directory` | `./data` | root data directory |
| `data.database_path` | `./data/notes.sqlite` | the canonical SQLite database |
| `data.asset_store` | `./data/assets` | content-addressed attachment bytes |
| `data.projection_dir` | `./data/projections` | Markdown mirror of your notes for the search sidecar |
| `search.default_limit` | `20` | search page size when the client sends none |
| `search.max_limit` | `100` | hard cap on requested page size |
| `mcp.enabled` | `true` | mount the `/mcp` endpoint |
| `mcp.default_profile` | `read-only` | `read-only` hides/rejects MCP write tools; `editor` enables them ([MCP guide](api/mcp.md)) |
| `mcp.max_results` | `10` | default MCP search page size |
| `mcp.max_document_bytes` | `65536` | truncation limit for document bodies returned to MCP clients |
| `search_sidecar.enabled` | `false` | activate the optional Recoll sidecar |
| `search_sidecar.binary` | `recollindex` | Recoll indexer binary (`recollq` is looked up next to it) |
| `search_sidecar.index_dir` | `./data/search-index` | generated Recoll config + index location |
| `remote_media.default_action` | `review` | policy for domains matched by no list: `allow`, `block`, or `review` (report only, never auto-download); invalid values fall back to `review` |
| `remote_media.allow_private_networks` | `false` | whether downloads may reach private/link-local addresses |
| `remote_media.max_redirects` | `5` | redirect-hop cap (policy re-checked per hop) |
| `remote_media.fetch_timeout_seconds` | `30` | per-download timeout |
| `remote_media.blocked_schemes` | `file, data, javascript, ftp` | URL schemes never fetched |
| `remote_media.blocked_domains` / `allowed_domains` / `review_domains` | empty | domain patterns (e.g. `*.wikimedia.org`) forcing block/allow/review |
| `remote_media.max_bytes.<class>` | `image: 20MB`, `video: 200MB`, `pdf: 100MB` | download size caps, human-readable sizes accepted |
| `remote_media.quarantine_dir` | `./data/quarantine` | staging area for fetched bytes before policy admission |
| `retention.unreferenced_resource_days` | `30` | recovery window after an unattached upload or final explicit detach |
| `retention.purged_resource_days` | `90` | longer recovery window for resources orphaned by permanent note purge |

The `remote_media` policy is reported by `/api/v1/status` under `media_policy`. It drives the remote-media scan (`POST /api/v1/documents/{id}/remote-media/scan` — per-URL decisions, nothing downloaded) and localization (`POST …/remote-media/localize`, `notriosctl localize` — quarantine fetch with redirect-hop and connect-time address checks, size caps, MIME sniffing, exact hashes, then rewrite to `resource://` links in a new revision). Blocked domains and blocked schemes are never fetched; `review` means report-only until explicitly opted in.

Resource reference health is available from
`GET /api/v1/resources/reports/reference` or
`notriosctl resources report`: exact duplicate logical resources,
unreferenced physical blobs, direct per-notebook usage, and optional
review-only perceptual suggestions. Notrios ships no perceptual algorithm, so
that hook is inert by default.

Retention-aware deletion is separate: `GET /api/v1/admin/gc/report` is always
read-only, while `notriosctl gc` is dry-run by default and requires `--apply`
to delete. Resources referenced by any current or trashed note are protected.

**Relative paths resolve against the working directory** of the process, not the config file's location. Use absolute paths for anything you run outside the repository checkout.

**Automatic creation:** on startup the service creates every configured
directory and, if absent, the database itself, applying schema migrations to
older databases automatically. The current schema is version 11.
`/api/v1/status` reports the resolved paths, database state, schema version,
capability flags, search limits, remote-media policy, and optional Recoll
backlog/sync/reconciliation state.
Search capabilities include `search.boolean` and `search.category_alias`; the
limits block reports the 4,096-byte, 256-token, and 16-level expression-parser
bounds so clients can validate before submitting a query.
P1 adds `selection.plan` plus advertised selector, explicit-ID, selected-note,
and detail limits. The planner is read-only and content-free; REST and MCP use
the same canonical Store operation. See [selection
planning](selection-planning.md).

**The browser UI is loaded from `web/dist/` relative to the working directory** (build it with `make web`). Without it, `/` returns a `web_ui_not_built` error while the API and MCP endpoints work normally.

## Network exposure

The default bind address is loopback-only. The service has **no authentication layer** — anyone who can reach the port can read and modify notes. If you bind to a non-loopback address (for example so `notrios -gui-only -remote ...` on another machine can connect), do it only on a trusted network, or keep the loopback bind and tunnel with SSH:

```sh
ssh -L 8080:127.0.0.1:8080 your-server   # then use http://127.0.0.1:8080 locally
```

## Search sidecar

With `search_sidecar.enabled: true` and [Recoll](https://www.recoll.org/)
installed, the service mirrors notes into `data.projection_dir` through a
transactional outbox, generates a Recoll configuration and indexes titles,
authors, tags, timestamps, and bodies. Startup and a periodic 10-minute pass
compare exact canonical renderings with the projection, repairing missing or
stale files and removing orphaned files. A 30-second worker drains at most 20
200-job batches; failed jobs use durable exponential backoff. Search responses
deduplicate FTS5/Recoll results and expose per-hit `sources`.

The desktop header polls status every 30 seconds and shows whether Recoll is
off/unavailable/active/degraded, its pending backlog, and last projection sync.
Detailed repair counts and errors are available in
`GET /api/v1/status` under `search_sidecar`.

Recoll is strictly optional and strictly external: if the binaries are missing or any step fails, the service logs it and continues with the built-in FTS5 search. Recoll is GPL software; Notrios only ever invokes the user-installed executables.

## Data directories and generated files

| Path | Contents | Disposable? |
|---|---|---|
| `data/notes.sqlite` (+ `-wal`, `-shm`) | **everything canonical**: notes, revisions, notebooks, tags, links, provenance | **no — this is your data** |
| `data/assets/` | attachment bytes, content-addressed by hash | no — attachments live here |
| `data/projections/` | Markdown mirror for the search sidecar | yes — regenerated automatically |
| `data/search-index/` | generated Recoll config and Xapian index | yes — regenerated by `recollindex` |

## Backup and restore

What must be backed up: the **database** and the **asset store** (plus your config file). Projections and search indexes are derived and regenerate.

Safest procedure (implementation-verified — the database runs in WAL mode):

1. Stop `notriosd` / the GUI. A clean shutdown checkpoints the WAL, so `data/notes.sqlite` is complete on its own.
2. Copy `data/notes.sqlite` — and, if they exist, `data/notes.sqlite-wal` and `data/notes.sqlite-shm` — together with `data/assets/`.
3. Restart.

If you cannot stop the service, use SQLite's online backup instead of copying a live file:

```sh
sqlite3 data/notes.sqlite ".backup '/backups/notes-$(date +%F).sqlite'"
cp -r data/assets /backups/assets-$(date +%F)
```

**Restore:** stop the service, preserve an emergency copy of the current
database/assets, verify the backup, put the database file and asset directory
back at the configured paths, and start the service — migrations bring an
older backup's schema forward automatically. Do not delete live WAL/SHM files
until the exact stopped database target and verified backup are resolved.

The native archive-v2 identity contract defines explicit
replace/merge/adopt/fork choices with no default. Replacing or adopting a full
snapshot preserves its logical database ID, merging keeps the existing target
database/replica IDs, and forking requires a new database ID. Replace, adopt,
and fork mint a replica ID for the resulting writable copy.
The P3/P4 commands are not implemented yet. See
[archive v2](archive-v2.md) and `SYNCHRONIZATION.md`.

**Portable alternative:** a [native archive export](import-export.md#exporting-a-notrios-archive) (`notriosctl export archive`) captures notes, notebooks, tags, and attachments in a human-readable form. It is ideal for moving a *subset* between machines, but it is not a byte-identical backup: revision history and provenance rows are not included, and re-imported notes become plain local notes.

## Data-safety rules (always on)

- Every note edit writes a durable revision; deleting a note moves it to the Trash.
- Notes imported from external sources can never be permanently deleted — only local notes can be purged from the Trash.
- Attachments are content-addressed (deduplicated by hash) outside the database.
