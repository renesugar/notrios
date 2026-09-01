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

**Configuration discovery:** with no `-config` flag, the service looks for your own `config.yaml` in the Notrios config directory (`$XDG_CONFIG_HOME/notrios` on Linux, defaulting to `~/.config/notrios`). If there is none, and the binary is running from a source checkout, it reads that checkout's `config/config.example.yaml`. Otherwise it uses compiled defaults.

An installed binary never reads a configuration file from the directory it happens to be launched in. That file decides the database location, the listen address, the public base URL, and the remote-media policy, so taking it from the current directory made those depend on where you were standing. A malformed config file is an error rather than a silent fall back to defaults: starting with a policy you did not choose is worse than not starting.

**Precedence:** command-line flags override the config file, which overrides the built-in defaults.

## Configuration reference
<!-- notrios:generated:user:configuration-reference:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/store#CurrentSchemaVersion -->
CurrentSchemaVersion is the canonical SQLite schema understood by this
build. Archive-v2 manifests record this source schema but never include
derived FTS5 or Recoll state.
<!-- source: go:github.com/renesugar/notrios/internal/config#Config -->
Config contains the runtime settings used by notriosd and notriosctl.
It intentionally avoids third-party YAML dependencies until the project
chooses and pins the long-term configuration library.
- config_path
- data
- data.asset_store
- data.database_path
- data.directory
- data.projection_dir
- mcp
- mcp.default_profile
- mcp.default_scope
- mcp.enabled
- mcp.max_document_bytes
- mcp.max_results
- mcp.sync_scope
- profile
- profile.id
- profile.name
- profile.registry_path
- remote_media
- remote_media.allow_private_networks
- remote_media.allowed_domains
- remote_media.blocked_domains
- remote_media.blocked_schemes
- remote_media.default_action
- remote_media.fetch_timeout_seconds
- remote_media.max_bytes
- remote_media.max_redirects
- remote_media.quarantine_dir
- remote_media.review_domains
- retention
- retention.purged_resource_days
- retention.sync_history_days
- retention.sync_peer_warning_days
- retention.unreferenced_resource_days
- search
- search.default_limit
- search.max_limit
- search_sidecar
- search_sidecar.binary
- search_sidecar.enabled
- search_sidecar.index_dir
- server
- server.listen_addr
- server.public_base_url
- server.web_dir
- sync
- sync.credential_ref
- sync.directory
- sync.rest
- sync.rest.burst
- sync.rest.enabled
- sync.rest.failures_per_minute
- sync.rest.key_file
- sync.rest.max_body_bytes
- sync.rest.requests_per_minute
- sync.rest.require_tls
- sync.rest.tls_cert_file
- sync.rest.tls_key_file
- sync.rest_base_url
- sync.target
<!-- source: go:github.com/renesugar/notrios/internal/config#Default -->
Default returns the canonical local-development defaults for every runtime
configuration group.
- config_path = ""
- data.asset_store = "./data/assets"
- data.database_path = "./data/notes.sqlite"
- data.directory = "./data"
- data.projection_dir = "./data/projections"
- mcp.default_profile = ""
- mcp.default_scope = ""
- mcp.enabled = true
- mcp.max_document_bytes = 65536
- mcp.max_results = 10
- mcp.sync_scope = ""
- profile.id = ""
- profile.name = ""
- profile.registry_path = ""
- remote_media.allow_private_networks = false
- remote_media.allowed_domains = null
- remote_media.blocked_domains = null
- remote_media.blocked_schemes = ["file","data","javascript","ftp"]
- remote_media.default_action = "review"
- remote_media.fetch_timeout_seconds = 30
- remote_media.max_bytes = {"image":20971520,"pdf":104857600,"video":209715200}
- remote_media.max_redirects = 5
- remote_media.quarantine_dir = "./data/quarantine"
- remote_media.review_domains = null
- retention.purged_resource_days = 90
- retention.sync_history_days = 90
- retention.sync_peer_warning_days = 30
- retention.unreferenced_resource_days = 30
- search.default_limit = 20
- search.max_limit = 100
- search_sidecar.binary = "recollindex"
- search_sidecar.enabled = false
- search_sidecar.index_dir = "./data/search-index"
- server.listen_addr = "127.0.0.1:8080"
- server.public_base_url = "http://127.0.0.1:8080"
- server.web_dir = ""
- sync.credential_ref = ""
- sync.directory = ""
- sync.rest.burst = 0
- sync.rest.enabled = false
- sync.rest.failures_per_minute = 0
- sync.rest.key_file = ""
- sync.rest.max_body_bytes = 0
- sync.rest.requests_per_minute = 0
- sync.rest.require_tls = true
- sync.rest.tls_cert_file = ""
- sync.rest.tls_key_file = ""
- sync.rest_base_url = ""
- sync.target = "none"
<!-- notrios:generated:user:configuration-reference:end -->

The authoritative, always-current example is `config/config.example.yaml` in the repository. The parser accepts a deliberately small YAML subset (two-level sections of scalar values); unknown keys are ignored so configs stay forward-compatible.

| Section / key | Default | Meaning |
|---|---|---|
| `profile.id` / `profile.name` / `profile.registry_path` | *(unset)* | generated local runtime-profile binding; use `notriosctl profile create`, do not copy it to make a second writable replica |
| `server.listen_addr` | `127.0.0.1:8080` | bind address — see the security note below |
| `server.public_base_url` | `http://127.0.0.1:8080` | base URL advertised to clients |
| `server.web_dir` | *(unset)* | directory holding the built web interface. Unset searches `web/dist` under the working directory, then under the executable's own directory and its parent. `--web-dir` overrides it. See [where the interface files have to be](installation.md#where-the-interface-files-have-to-be) |
| `data.directory` | `./data` | root data directory |
| `data.database_path` | `./data/notes.sqlite` | the canonical SQLite database |
| `data.asset_store` | `./data/assets` | content-addressed attachment bytes |
| `data.projection_dir` | `./data/projections` | Markdown mirror of your notes for the search sidecar |
| `sync.target` | `none` | local transport choice: `none`, `directory`, or `rest`; a non-none managed profile establishes the local journal boundary and G15 runs only explicitly queued durable jobs |
| `sync.directory` | *(unset)* | absolute ephemeral carrier path required only for `directory` |
| `sync.rest_base_url` | *(unset)* | absolute peer URL required only for `rest` |
| `sync.credential_ref` | *(unset)* | reference to a native credential-store item, never a credential value; redacted from CLI profile output |
| `search.default_limit` | `20` | search page size when the client sends none |
| `search.max_limit` | `100` | hard cap on requested page size |
| `mcp.enabled` | `true` | mount the `/mcp` endpoint |
| `mcp.default_scope` | `read-only` | cumulative `search-only`, `read-only`, `editor`, or `organizer` MCP tool scope. `mcp.default_profile` remains a deprecated alias; if both are set, the narrower wins ([MCP guide](api/mcp.md)) |
| `mcp.sync_scope` | `disabled` | orthogonal `disabled`, `status`, or `control` sync permission; control is limited to bounded incremental/resource jobs and never includes keys, backup/restore, retirement, purge, or reset |
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
| `retention.sync_history_days` | `90` | minimum sync-operation/tombstone age; snapshot and active-peer acknowledgements are also mandatory |
| `retention.sync_peer_warning_days` | `30` | warning interval before an active peer reaches the history horizon |

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
older databases automatically. The current schema is version 27.
`/api/v1/status` reports the active runtime profile name/ID (when managed), resolved paths, database state, schema version,
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

The default bind address is loopback-only. Ordinary REST, MCP, and the web UI
have **no user-authentication layer**, so the service admits them only from a
loopback connection carrying a local Host. Binding to a non-loopback address
does not publish those routes: it admits only the finite authenticated
peer-sync endpoints when that surface is enabled. Forwarding headers and an
unknown `/api/v1/sync/` path do not widen the set.

To use `-gui-only` or another ordinary client across machines, keep the service
on loopback and use an access-controlled tunnel such as SSH:

```sh
ssh -L 8080:127.0.0.1:8080 your-server   # then use http://127.0.0.1:8080 locally
```

Browser mutation requests must also be exact same-origin (or originate from
the Wails shell). JSON/MCP requests are limited to 8 MiB and exactly one JSON
value; raw resource uploads use the 16 GiB object ceiling. These request checks
are containment controls, not accounts or authorization. The separately
authenticated/TLS peer surface is documented in
[Data safety and maintenance](operations.md#exposing-the-sync-surface-to-a-peer).

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
`notriosctl export archive-v2`, `verify archive-v2`, and `restore archive-v2
--intent replace|adopt|merge|fork` implement this. A v2 snapshot is a complete
backup — every revision, trashed notes, provenance, resources, and exact source
bundles — so it is an alternative to the file copy above rather than a subset
export. See [archive v2](archive-v2.md) and `SYNCHRONIZATION.md`.

The G14c-G14e production path provides `notriosctl snapshot create|verify|restore` for faster exact-schema
whole-library recovery: a SQLite Online Backup image plus deterministic bounded
asset packs. Physical restore is local-filesystem only, requires explicit
`replace` or `adopt`, verifies an emergency snapshot before the first rename,
and leaves a durable startup blocker until the installed identity passes.
Re-run the same command after interruption to roll forward. The encrypted REST
catch-up path now carries this physical representation; packed archive-v2
remains the portable/subset/merge/incompatible-schema alternative.

**Portable alternative:** a [native archive export](import-export.md#exporting-a-notrios-archive) (`notriosctl export archive`) captures notes, notebooks, tags, and attachments in a human-readable form. It is ideal for moving a *subset* between machines, but it is not a byte-identical backup: revision history and provenance rows are not included, and re-imported notes become plain local notes.

## Data-safety rules (always on)

- Every note edit writes a durable revision; deleting a note moves it to the Trash.
- Notes imported from external sources can never be permanently deleted — only local notes can be purged from the Trash.
- Attachments are content-addressed (deduplicated by hash) outside the database.
