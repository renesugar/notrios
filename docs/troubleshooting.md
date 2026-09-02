# Troubleshooting

Verified symptoms and fixes, grouped by activity. `notriosctl doctor` diagnoses most environment problems in one shot (see the [CLI reference](cli.md#doctor)).

## Building

| Symptom | Fix |
|---|---|
| `go.mod requires go >= 1.25.0` (or similar toolchain error) | install Go 1.25+ from [go.dev/dl](https://go.dev/dl/); check with `go version` |
| `sqlite3.h: No such file or directory` or `Package sqlite3 was not found` | `sudo apt install libsqlite3-dev pkg-config`; the store is a cgo wrapper, so a C toolchain (`build-essential`) is required |
| `cgo: C compiler ... not found` or builds fail with `CGO_ENABLED=0` | install `build-essential` and don't disable cgo — the SQLite store needs it |
| `Package gtk+-3.0 was not found` / `webkit2gtk-4.1 was not found` (only `make gui`) | `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev` |
| `Wails applications will not build without the correct build tags.` at GUI launch | the binary was built without the GUI tags — build with `make gui`, or run `-no-gui` |
| `vite: not found` / `tsc: not found` / other frontend failures | frontend dependencies aren't installed: `make deps` (runs `npm ci` against the lockfile) |
| npm engine warnings or Vite refusing to run | use Node 22 (what CI tests); Node must be ≥ 20.19 |

## Starting the service

| Symptom | Fix |
|---|---|
| `listen tcp 127.0.0.1:8080: bind: address already in use` | another instance owns the port — stop it, or pass `-addr 127.0.0.1:8081`. A checkout and an installed Notrios no longer collide by default: they use [two different addresses](service.md#the-two-default-addresses) |
| the browser shows the *other* instance's notes, or an empty library | check the address. A checkout serves `127.0.0.1:8099` and an installed Notrios `127.0.0.1:8080` — they are [separate instances](service.md#the-two-default-addresses) with separate libraries. `notriosctl paths` prints which one a given binary resolved |
| `create directory ...: permission denied` | the configured `data` paths aren't writable from this directory; run somewhere writable or point the config at absolute, writable paths |
| `open sqlite: unable to open database file` | the database's parent directory doesn't exist or isn't writable; check `data.database_path` and remember relative paths resolve against the working directory |
| `sqlite exec: ...` errors during startup migration | the database file may be corrupt or written by an incompatible tool — restore from backup ([backup guide](service.md#backup-and-restore)); migrations themselves are automatic and additive |
| `database schema is newer than this build supports` | the library was written by a **newer** Notrios; upgrade rather than opening it with this build. Continuing would re-run old migrations against a schema this build does not understand and record the wrong version, hiding that it happened. The database is left untouched |
| `/` returns `web_ui_not_built` or `web_ui_not_found` | from a checkout, run `make web` and start from the checkout root; for an installed binary the working directory is not searched — copy `web/dist` to the program-assets root or pass `--web-dir` ([where the interface files have to be](installation.md#where-the-interface-files-have-to-be)). The API works regardless |
| `database is locked` during simultaneous CLI + service writes | rare thanks to WAL + a 5s lock timeout; if it persists, stop the service, re-run the CLI command (imports are idempotent), restart |

## Finding your notes

| Symptom | Fix |
|---|---|
| after upgrading to 0.8 the library is **empty** and the notes are gone | they are not gone. Before 0.8 a binary with no configuration file kept its library in `./data`, relative to the directory you launched from; 0.8 resolves the native roots instead. Run `notriosctl paths` from that directory — it names any pre-0.8 library it finds — then `notriosctl migrate --dry-run` and `notriosctl migrate` ([full contract](cli.md#migrate)) |
| `notriosctl paths` reports a pre-0.8 layout but your notes are somewhere else | `notriosctl migrate --from <that directory>` |
| `migrate` refuses: *the resolved location already holds a database* | you have two libraries, and combining them is a decision rather than a copy. Open each with `--db` and move what you want across; nothing was touched |
| a development checkout and an installed copy disagree about which notes exist | that is deliberate. A checkout is a *separate instance* with checkout-local roots under `./data`, so it cannot read or write an installed library. `notriosctl paths` prints which mode each one resolved |
| unsure which database a command will open | `notriosctl paths` for the roots and the mode; `notriosctl config show` for the effective values and where each came from |

## Search sidecar (Recoll)

| Symptom | Fix |
|---|---|
| log: `search sidecar enabled but "recollindex"/recollq not found; continuing with FTS5 only` | Recoll isn't installed — `sudo apt install recoll` — or set `search_sidecar.binary` to its full path. Everything still works via FTS5 |
| log: `recollindex failed; continuing with FTS5 only` | run `recollindex -c <search_sidecar.index_dir>` manually to see Recoll's own error; deleting the index directory forces a clean rebuild on next start |
| field queries (`tag:`, `author:`) miss notes that plain search finds | the sidecar may still be indexing (it syncs ~every 30s) or is disabled; FTS5 alone doesn't index tags-as-fields |
| desktop status shows Recoll `degraded` or a retry backlog | inspect `curl -s localhost:8080/api/v1/status \| jq .search_sidecar`; failed projection jobs back off without blocking later work, and startup/periodic reconciliation repairs drift |

## Importing

| Symptom | Fix |
|---|---|
| `no tweets.js/tweet.js found under ...` | point `import twitter` at the **extracted** archive directory (the one containing `data/`), not the ZIP |
| `not a Notrios archive (missing manifest.json)` | `import archive` needs a directory produced by `export archive` |
| `read ChatGPT export: ... no such file` | pass the `conversations.json` file or the directory that directly contains it |
| Joplin import reports `resources_skipped` with warnings | those attachments were missing from the export's `resources/` directory; re-export from Joplin if they matter |
| `notebook name "X" conflicts with a notebook bound to another data source` | run the archive import's `--dry-run`, edit the generated `import-config.json` renames, then import with `--import-config` |
| import seems to “miss” notes on re-run | notes you moved to the Trash are deliberately not resurrected; unchanged notes count as `notes_unchanged` |
| a Joplin/Obsidian import was interrupted | re-run the same command and option set; if the source inventory fingerprint is unchanged it resumes at the next durable batch, otherwise it replans and skips unchanged item fingerprints |

## Physical snapshot restore

| Symptom | Fix |
|---|---|
| startup reports that a physical restore is in progress | keep the service stopped and re-run the identical `notriosctl snapshot restore --intent ...` command; do not remove the blocker or rename database/assets manually |
| restore says the existing plan names different input or targets | use the original snapshot, intent, database, and asset paths to finish recovery; after it completes, start a separate explicit restore |
| `replace requires the same database id` | use `--intent adopt` only if adopting that compatible library is intended, or use archive-v2 for merge/fork/incompatible-schema recovery |
| restore stopped after cutover | verify the emergency snapshot path printed in the recovery plan remains protected, then re-run the same command; the coordinator rolls forward and verifies installed identity before unblocking startup |
| restored attachments remain unavailable | physical snapshots include only database-declared local bytes; declarations that were remote/unavailable remain so and ordinary materialization fetches them from an enrolled source later |
| a large backup request reaches the ordinary 30-second HTTP deadline | update both peers to a G14e-capable build; authenticated, explicitly permitted snapshot creation has its own bounded two-hour deadline while ordinary API calls remain short |
| a resumable backup reports a wrong range length | update the client and server together; signed backup ranges use one bounded 16 MiB response chunk while ordinary responses remain capped at 8 MiB |

## Documentation site

| Symptom | Fix |
|---|---|
| `npx: command not found` / marked or pagefind download failures | `make docs` needs Node/npm and network access for `npx marked` and `npx pagefind` |
| search box missing or empty on the built site | PageFind assets load from the site's base path — serve `_site/` over HTTP (`python3 -m http.server -d _site`), not `file://`; on GitHub Pages the workflow handles paths |

## Development validation commands

What each check actually verifies:

| Command | Checks |
|---|---|
| `go vet ./...` | Go static analysis |
| `go test ./...` (`make test`) | all unit/integration tests, including importers, store, and API |
| `python3 scripts/check_required_files.py` | required repository files exist |
| `bash scripts/validate-scaffold.sh` (`make validate`) | tests + required files + shell/Python script syntax |
| `cd web && npm run typecheck && npm run build` | frontend type safety and production build |
| `bash scripts/mvp_smoke.sh` (`make smoke`) | boots a real service on a test port and exercises create/update/search/resource/MCP end to end |
| `bash scripts/run_performance_smoke.sh` | generated-dataset store smoke + benchmark |
| `make precheck` | working tree cleanliness; fails if any tracked file matches `.gitignore` |
