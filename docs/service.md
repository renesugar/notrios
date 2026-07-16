# The Notrios service

`notriosd` is the headless service: a single process serving the REST API, the MCP endpoint, and the built-in web UI.

```bash
notriosd -config config.yaml       # explicit config
notriosd -addr 127.0.0.1:8080      # quick overrides
notriosd -db ./data/notes.sqlite
```

The `notrios` desktop binary embeds the same service; `notrios -no-gui` behaves exactly like `notriosd`. See the [GUI guide](gui.md) for the other modes.

## Configuration

```yaml
server:
  listen_addr: "127.0.0.1:8080"

data:
  directory: "./data"
  database_path: "./data/notes.sqlite"
  asset_store: "./data/assets"
  projection_dir: "./data/projections"

search:
  default_limit: 20
  max_limit: 100

mcp:
  enabled: true
  default_profile: "read-only"   # "editor" enables MCP write tools

search_sidecar:
  enabled: false                 # requires Recoll installed
  binary: "recollindex"
  index_dir: "./data/search-index"
```

On startup the service creates the storage directories, opens (or creates) the SQLite database, runs migrations, and bootstraps the default "Notes" notebook, the read-only "Help" notebook, and the builtin "All notes" and "Trash" search notebooks.

## Search sidecar

With `search_sidecar.enabled: true` and [Recoll](https://www.recoll.org/) installed, the service mirrors your notes into a filesystem projection, generates a Recoll configuration (including a front-matter handler that indexes titles, authors, tags, and timestamps as searchable fields), and merges Recoll results into search responses. Recoll is optional: without it, everything falls back to the built-in FTS5 search. Recoll is GPL software and is only ever invoked as an external program.

## Data safety

- Every note edit writes a durable revision; deleting a note moves it to the Trash.
- Notes imported from external sources can never be permanently deleted — only local notes can be purged from the Trash.
- Attachments are stored content-addressed (deduplicated by hash) outside the database.
