# The notriosctl CLI
<!-- notrios:generated:user:the-notriosctl-cli:begin -->
<!-- source: go:github.com/renesugar/notrios/cmd/notriosctl#printHelp -->
printHelp is the finite command and flag usage registry shown by notriosctl.

printHelp renders the whole command line from internal/clispec.

It used to be a string literal that this program, docs/cli.md, the Help
notebook and the documentation coverage gate all depended on, and that
nothing checked against the dispatcher. Eight commands were missing from it.

- notriosctl collections list [--json] [--db ...]
- notriosctl collections show --collection <id> [--json] [--db ...]
- notriosctl compatibility archive-v2 [--reader current-v2|previous-loose-v2] <archive-dir|manifest.json>
- notriosctl config show [--config config.yaml] [--json] [--no-redact]
- notriosctl doctor [--config config.yaml] [--db path] [--asset-store path] [--json] [--no-redact]
- notriosctl export archive [--db ...] [--query "tag:todo"] <out-dir> [--collection id]
- notriosctl export archive-v2 [--db ...] [--target full_archive|subset_transfer] [--notebooks id,id] [--tags a,b] [--query "tag:todo"] [--documents id,id] [--match any|all] [--pack] [--overwrite] [--no-verify] <out-dir> [--max-documents N] [--pack-bytes N] [--records-per-object N]
- notriosctl fix [--db ...] [--kinds a,b] [--document id] [--apply] [--list-kinds] [--allow-review] [--max-documents N]
- notriosctl gc [--config config.yaml] [--db ...] [--asset-store ...] [--dry-run | --apply] [--snapshot <retained-snapshot-dir>]
- notriosctl graph export [--db ...] [--collection id] [--overwrite] <out-dir>
- notriosctl graph report [--db ...] [--collection id] [--limit N] [--write-note] [--quiet]
- notriosctl import archive [--db ...] [--dry-run] [--write-config path] [--import-config path] <archive-dir> [--collection id]
- notriosctl import chatgpt [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook ChatGPT] [--dry-run] <conversations.json|export-dir>
- notriosctl import claude [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Claude] [--dry-run] <conversations.json|export-dir>
- notriosctl import joplin-raw [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--batch-size 100] [--preserve-source] [--dry-run] [--write-config path] [--import-config path] [--localize-media] <raw-export-dir>
- notriosctl import obsidian [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] [--localize-media] <vault-dir> [--batch-size 100] [--preserve-source] [--write-config path] [--import-config path]
- notriosctl import twitter [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Twitter] [--dry-run] <extracted-archive-dir>
- notriosctl jobs cancel [--db ...] <job-id>
- notriosctl jobs list [--db ...] [--kind k] [--state s] [--limit 50]
- notriosctl jobs retry [--db ...] [--reset] <sync-job-id>
- notriosctl jobs show [--db ...] [--command] <job-id>
- notriosctl jobs status [--db ...] [--wait] [--timeout 30m] [--quiet] <job-id>
- notriosctl link [--db ...] [--anchor slug|^block] [--list-anchors] <document-id>
- notriosctl lint [--db ...] [--checks a,b] [--detail-limit 100] [--quiet] [--list-checks] [--collection id]
- notriosctl localize [--config config.yaml] [--db ...] [--dry-run] [--allow-review] [--base-revision rev] <document-id>
- notriosctl migrate [--from dir] [--dry-run] [--json]
- notriosctl notebooks create --name <name> [--parent <id|name>] [--icon <emoji>] [--query <query>]
- notriosctl notebooks list [--json] [--db ...]
- notriosctl notes append --document <id> [--text <text> | --text-file path|-] [--base-revision rev] [--db ...]
- notriosctl notes create --title <title> [--notebook <id|name>] [--body-file path|-] [--body text]
- notriosctl notes delete [--document <id> | --query <query> [--apply] [--mode atomic|best_effort] [--limit N]] [--db ...]
- notriosctl notes duplicate [--document <id> | --query <query> [--apply] [--mode atomic|best_effort] [--limit N]] [--db ...]
- notriosctl notes edit --document <id> [--title <title>] [--body-file path | --body text] [--message <why>]
- notriosctl notes links --document <id> [--direction outgoing|incoming|both] [--output <file>] [--db ...]
- notriosctl notes move --notebook <id|name> [--document <id> | --query <query> [--apply] [--mode atomic|best_effort] [--limit N]] [--db ...]
- notriosctl notes outline --document <id> [--output <file>] [--db ...]
- notriosctl notes prepend --document <id> [--text <text> | --text-file path|-] [--base-revision rev] [--db ...]
- notriosctl notes resources --document <id> [--output <file>] [--db ...]
- notriosctl notes restore [--document <id> | --query <query> [--apply] [--mode atomic|best_effort] [--limit N]] [--db ...]
- notriosctl notes show --document <id> [--json] [--output <file>] [--db ...]
- notriosctl open [--profile name] [--registry path] [--db path] [--launch] <notrios-uri>
- notriosctl paths [--json] [--no-redact]
- notriosctl profile create --name <profile> [--listen 127.0.0.1:8080] [--db ...] [--sync-target none|directory|rest] [--data-dir path] [--public-url url] [--registry path] [--credential-ref ref] [--sync-directory dir] [--sync-rest-url url] [--copied-database-as adopt|fork]
- notriosctl profile forget --name <profile> [--registry path]
- notriosctl profile list [--registry path]
- notriosctl profile register --name <profile> [--db ...] [--registry path]
- notriosctl profile show --name <profile> [--registry path]
- notriosctl profile start --name <profile> [--binary notriosd] [--dry-run] [--registry path]
- notriosctl profile validate [--name <profile>] [--registry path]
- notriosctl publish plan --profile <profile> [--detail-limit 100]
- notriosctl publish profile delete [--name <profile>]
- notriosctl publish profile list [--db ...]
- notriosctl publish profile save --name <profile> [--notebooks id,id] [--tags a,b] [--link-action plain_text] [--description text] [--query "tag:todo"] [--documents id,id] [--match any|all] [--target full_archive|subset_transfer] [--exclude-tags a,b] [--private-tags a,b] [--include-provenance] [--include-source-bundles] [--max-resource-bytes N]
- notriosctl publish run --profile <profile> --reviewed-plan <sha256> <out-dir> [--overwrite] [--no-verify]
- notriosctl purge [--dry-run] [--confirm] [--no-backup] [--backup-dir path] [--json] [--no-redact]
- notriosctl register-url-handler [--apply] [--binary path] [--dir path]
- notriosctl resources add --file <path> [--filename <name>] [--document <id>] [--db ...]
- notriosctl resources get --resource <id> [--output <file>] [--db ...]
- notriosctl resources report [--config config.yaml] [--db ...] [--asset-store ...]
- notriosctl restore archive-v2 --intent replace|adopt|merge|fork [--db ...] [--new-database-id id] <archive-dir>
- notriosctl search [--limit N] [--cursor c] [--count] [--links] [--output <file>] [--db ...] "<query>"
- notriosctl seed-help [--db ...] [docs-dir]
- notriosctl snapshot create [--config config.yaml] [--db ...] [--asset-store ...] <out-dir>
- notriosctl snapshot restore --intent replace|adopt [--db ...] [--asset-store ...] [--emergency dir] <snapshot-dir>
- notriosctl snapshot verify <snapshot-dir>
- notriosctl sync accept --invite <file> --code <code> --out <file>
- notriosctl sync discover [--carrier dir] [--db ...]
- notriosctl sync enroll --acceptance <file> --code <code>
- notriosctl sync exchange --url <base-url> [--materialize N]
- notriosctl sync fetch-backup --url <base-url> --out <dir> [--intent replace|adopt] [--chunk-bytes N] [--emergency dir]
- notriosctl sync handshake --url <base-url> [--db ...]
- notriosctl sync init [--db ...] [--keys path]
- notriosctl sync invite [--ttl 15m] [--offline --out <file>] [--label text]
- notriosctl sync join --url <base-url> --code <code>
- notriosctl sync migrate-credentials --to native|development-file [--dry-run] [--confirm]
- notriosctl sync once [--carrier dir] [--cleanup] [--materialize N] [--db ...]
- notriosctl sync peers [--db ...]
- notriosctl sync retention --snapshot <retained-snapshot-dir> [--apply --confirm-digest <dry-run-digest>]
- notriosctl sync retire --peer <replica-id> [--reason ...] [--confirm retire-peer:<replica-id>]
- notriosctl sync revoke --key <id> [--advance-epoch] [--reason text]
- notriosctl sync start [--carrier dir] [--resource-fetch] [--byte-budget N] [--max-attempts N] [--db ...]
- notriosctl sync status [--db ...] [--keys path]
- notriosctl tags add --tag <tag> [--document <id> | --query <query> [--apply] [--mode atomic|best_effort] [--limit N]]
- notriosctl tags list [--document <id>] [--prefix <branch>] [--limit N] [--db ...]
- notriosctl tags remove --tag <tag> [--document <id> | --query <query> [--apply] [--mode atomic|best_effort] [--limit N]]
- notriosctl tags rename --from <tag> --to <tag> [--db ...] [--include-children] [--apply]
- notriosctl tags show --tag <name> [--db ...]
- notriosctl tasks list [--document <id>] [--notebook <id>] [--state open|done] [--untagged]
- notriosctl templates create --template <id> --title <title> [--notebook <id|name>] [--set name=value ...]
- notriosctl templates list [--db ...]
- notriosctl verify archive-v2 <archive-dir>
- notriosctl version
<!-- notrios:generated:user:the-notriosctl-cli:end -->

`notriosctl` handles imports, exports, diagnostics, and maintenance against the same SQLite database the service uses. Build it with `make build-cli` (output `bin/notriosctl`) or run any command from source with `go run ./cmd/notriosctl <command>`.

General behavior:

- successful commands exit `0`; runtime failures print to stderr and exit `1`; usage mistakes (unknown command/flag, wrong argument count) print usage and exit `2`;
- commands that touch the database accept `--config`, `--db`, and `--asset-store` (overrides win over the config file, which wins over built-in defaults; relative paths resolve against your working directory);
- importers and `export archive` print a JSON report to stdout;
- `notriosctl help` prints the full usage summary; `--help`, `-h` and `help` also work on any
  command or command group (`notriosctl notes --help`, `notriosctl help notes show`). Adding
  `--json` to any of those prints that command's description as data instead of prose --
  `notriosctl help --json` for everything, `notriosctl notes --help --json` for one group,
  `notriosctl notes show --help --json` for one command. An unknown command prints the
  summary and exits 2.

## version
<!-- notrios:generated:user:version:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/version#Version -->
Version is the product version reported by Notrios binaries.
<!-- notrios:generated:user:version:end -->

```sh
notriosctl version
```

Prints the version string and exits 0. No flags. The number is not repeated here: a document that restates the version can only ever be right until the next release, and this one was wrong within a day of it.

## doctor

```sh
notriosctl doctor [--config config.yaml] [--db path] [--asset-store path] [--json]
```

Environment and configuration diagnostics. Checks, in order:

| Check | Kind | What it verifies |
|---|---|---|
| `go runtime` | required | which Go the binary was built with |
| `config` | required | the config file loads (shows which file, or "built-in defaults") |
| `directories` | required | storage directories exist or can be created |
| `database` | required | the SQLite database opens, migrates, and reports its schema version |
| `asset store` | required | the attachment directory is writable |
| `web ui` | informational | whether `web/dist/index.html` exists in the working directory (the browser UI needs it; the API does not) |
| `recoll` | informational | whether the optional `recollindex` binary is on `PATH` |

Exit `0` when all required checks pass, `1` otherwise. Note that doctor *creates* missing storage directories and an empty database if none exists (the same bootstrap the service performs) — point it at your real config to diagnose your real setup.

```text
ok    config           config/config.example.yaml
ok    database         ./data/notes.sqlite (schema version 27)
info  web ui           web/dist missing here; run `make web` or serve API-only
doctor: required checks passed
```

`--json` reports the same run for a monitoring script: every check with its
`state` (`ok`, `failed` or `info`), whether it was `required`, and the same
detail. `required` is beside `state` because the two answer different questions
— what doctor found, and whether it is allowed to be like that — and a script
should not have to infer the second from the wording of the first. The exit code
is the same in both forms.

```json
{
  "checks": [
    { "check": "database", "state": "ok", "required": true,
      "detail": "./data/notes.sqlite (schema version 27)" },
    { "check": "web ui", "state": "info", "required": false,
      "detail": "web/dist missing here; run `make web` or serve API-only" }
  ],
  "failed": false,
  "summary": "required checks passed"
}
```

## paths

```sh
notriosctl paths [--json] [--no-redact]
```

Where this instance keeps things, and how it decided. This is the first thing to
run when Notrios cannot find your notes, or when you are not sure which library
a command is about to touch.

```text
mode: installed
  cache           ~/.cache/notrios
  config          ~/.config/notrios
  data            ~/.local/share/notrios
  program_assets  /usr/local/share/notrios
  runtime         ~/.local/state/notrios/runtime
  state           ~/.local/state/notrios
notices:
  [runtime_dir_unset] XDG_RUNTIME_DIR is not set and the specification names no fallback; using ~/.local/state/notrios/runtime rather than a shared temporary directory
```

`mode` is the layout that was selected:

| Mode | Meaning |
|---|---|
| `installed` | The native per-OS locations. This is a normal installation. |
| `portable` | A `notrios-portable.txt` marker sits beside the executable, so every root is under the executable's own tree. Nothing else selects portable mode — never the current directory, and never that directory being writable. |
| `source` | The binary is running from a Notrios checkout. Every root is checkout-local, so a development build cannot read or write an installed instance's library. |

`notices` explain any decision you did not make yourself: a variable that was
ignored and why, or a fallback that was substituted. Each carries a stable code
so it can be matched in scripts.

The home directory is shown as `~` by default, because this output gets pasted
into issue reports. `--no-redact` prints it in full. `--json` emits the same
information as `{"mode", "roots", "notices", "redacted"}`.

`paths` also reports where a schema migration keeps its copy of the database
(`pre-migration-backups/` beside it) and how many are kept. The value of that
backup is being findable, which matters most exactly when something has gone
wrong; see [schema migrations](service.md#schema-migrations-and-their-backup).

## config show

```sh
notriosctl config show [--config config.yaml] [--json] [--no-redact]
```

The configuration this instance actually resolved, and where each value came
from. `doctor` tells you *which file* loaded; this tells you what is in effect.

```text
source: ~/.config/notrios/config.yaml

  data.directory                       ~/lib                    file
  data.database_path                   ~/lib/notes.sqlite       resolved
  data.state_dir                       ~/lib                    resolved
  remote_media.default_action          block                    file
  server.listen_addr                   127.0.0.1:8080           compiled

origin: file = stated in the configuration file, resolved = resolved from the platform roots, compiled = built-in default
```

The origin column is the useful part. `file` means you wrote it. `resolved`
means nothing said otherwise, so it was derived from the platform roots — which
is why `data.database_path` above sits under a `data.directory` you did set.
`compiled` means the built-in default is still in force.

No secret is printed. `sync.credential_ref` is shown because it is a *reference*
to an entry in a native credential store, never a credential.

## migrate

```sh
notriosctl migrate [--from dir] [--dry-run] [--json]
```

Moves a library left behind by a pre-0.8 Notrios into the resolved roots.

**Most installations need this and do not know it.** Before 0.8 the built-in
defaults were relative to the working directory, so a binary run with no
configuration file wrote its library to `./data` — under whichever directory you
happened to launch from. An 0.8 binary resolves the native roots instead, finds
them empty, and opens a new empty library. Your notes are not gone; they are
where you left them.

Nothing looks that place up, because nothing recorded it. What `notriosctl
paths` does is notice a pre-0.8 library in the directory you are standing in and
say so:

```text
pre-0.8 layout:
  a database from an older Notrios is at ~/notes/data/notes.sqlite
  this instance is not using it; the roots above are what it reads and writes
  run `notriosctl migrate --dry-run` to see what moving it would do
```

Always look before you move:

```sh
notriosctl migrate --dry-run
```

```text
Plan for ~/notes/data (nothing was copied):
  copy     ~/notes/data/assets -> ~/.local/share/notrios/assets (412 file(s), 88118 bytes)
  copy     ~/notes/data/notes.sqlite -> ~/.local/share/notrios/notes.sqlite (1 file(s), 4014080 bytes)
  copy     ~/notes/data/quarantine -> ~/.local/state/notrios/quarantine (3 file(s), 51221 bytes)
  rebuild  ~/notes/data/search-index is derived data and regenerates; it is not copied
  4153419 bytes to copy; 28324495360 bytes free
```

Then run it without `--dry-run`.

### What it guarantees

**Your library is never moved for you.** An 0.8 binary that relocated a library
because it recognised the shape of a directory would be making an irreversible
decision on the evidence of where it was launched from. Detection reports; you
decide.

**The original is copied, never moved,** and every copied file is checked with
SHA-256 against its source before anything is committed. If verification fails,
the migration stops and the original is untouched.

**The old directory is renamed, not deleted** — to `data.migrated-<timestamp>`,
and the command tells you where. It is a complete copy of what you had, so it is
also your backup if the first open migrates the schema. Delete it yourself once
you are satisfied.

**An interruption is resumable.** A journal under `<state>/migration/` records
each category before and after; running the same command again continues from
the last incomplete one. Because the source is intact until the final rename,
the worst outcome of a crash is wasted disk.

**A merge is refused.** If the resolved location already holds a database *with
notes in it*, migration stops and names both paths. Combining two libraries is a
decision, not a copy. If the file there cannot be read as a database at all,
migration also refuses rather than assuming it is empty — a corrupted library is
a recovery problem, not a migration one.

**An empty library at the destination is set aside, not treated as a merge.**
This is the ordinary case rather than a corner: you discover your notes are
missing by running the new binary, and `doctor` (or the service) creates an
empty library at the resolved path in the act of looking. Migration renames it
to `notes.sqlite.unused-<timestamp>` and says so, then continues. As everywhere
else here, it is renamed rather than deleted.

**Derived data is rebuilt rather than carried.** Projections and the search
index regenerate from the library, and a search index copied to a new path would
hold stale absolute paths inside it. The quarantine *is* carried: it is the
record of what a note tried to fetch, which nothing can regenerate.

### When you do not need it

- **You have a configuration file that states its paths.** They are used exactly
  as written and never relocated, so nothing moved and nothing needs to move.
- **You run from a checkout.** A checkout is a separate instance whose roots are
  already `./data`. `migrate` says so and does nothing.

If your library is somewhere other than the directory you are standing in, name
it: `notriosctl migrate --from ~/old-notes`.

## import

Six variants; all support `--dry-run` and the shared `--config/--db/--asset-store/--collection` flags. Full workflows, source-application export steps, report-field explanations, and preserved-metadata tables are in [import & export](import-export.md).

### import joplin-raw

```sh
notriosctl import joplin-raw [shared flags] [--batch-size 100] [--preserve-source] \
  [--dry-run] [--write-config path] [--import-config path] <raw-export-dir>
```

Positional argument: the Joplin **RAW export directory** (not a `.jex` file).
The importer restores nested notebooks and real tags. `--batch-size` is
bounded to 1–500 (default 100), and durable checkpoints resume an interrupted
run when the deterministic source fingerprint still matches.
`--preserve-source` stores exact RAW item bytes, unknown fields, and property
order in a separate content-addressed source bundle.

`--dry-run` uses the real action planner and returns its suggested configuration
inside the JSON report. It writes that configuration only when
`--write-config path` is supplied. Resolve any suggested notebook-path renames,
then pass the file to the real import with `--import-config`. Dry run creates no
source file, import checkpoint, note, resource, notebook, tag, or source-bundle
object.

### import obsidian

```sh
notriosctl import obsidian [shared flags] [--batch-size 100] [--preserve-source]
  [--dry-run] [--write-config path] [--import-config path] <vault-dir>
```

Positional argument: the vault directory. Folder hierarchy becomes nested
notebooks. `--batch-size` is bounded to 1–500 (default 100); matching durable
checkpoints resume interrupted imports. `--preserve-source` captures exact
Markdown/frontmatter and non-Markdown bytes with their relative paths.

Dry run uses the real action classifiers and writes a folder-conflict
configuration (default `<vault>/.notrios/import-config.json`). Apply suggested
path-scoped renames with `--import-config`. Dry run creates no canonical rows,
checkpoint, or source-bundle object.

### import twitter

```sh
notriosctl import twitter [shared flags] [--notebook Twitter] [--dry-run] <extracted-archive-dir>
```

Positional argument: the **extracted** archive directory (the one containing `data/`). `--notebook` (default `Twitter`) names/creates the destination notebook.

### import chatgpt / import claude

```sh
notriosctl import chatgpt [shared flags] [--notebook ChatGPT] [--dry-run] <conversations.json|export-dir>
notriosctl import claude  [shared flags] [--notebook Claude]  [--dry-run] <conversations.json|export-dir>
```

Positional argument: the export's `conversations.json`, or a directory containing it. `--notebook` defaults to `ChatGPT` / `Claude`.

### import archive

```sh
notriosctl import archive [shared flags] [--dry-run] [--write-config path] [--import-config path] <archive-dir>
```

Imports a native Notrios archive. `--dry-run` analyzes notebook-name conflicts and writes an import configuration file (default `<archive-dir>/import-config.json`; override with `--write-config`) instead of importing. A real import optionally takes `--import-config` with notebook renames; it refuses (exit 1) before writing anything if a name still conflicts with a source-bound notebook.

## export archive

```sh
notriosctl export archive [shared flags] [--query "tag:todo"] <out-dir>
```

Writes a native archive **v1** directory: query-scoped, human-readable interchange. `--query` (default empty = all non-trashed notes) scopes the export with the [query language](query-language.md). The report lists `notes`, `notebooks`, `resources`, and any `warnings`. v1 is not a lossless backup — use `export archive-v2` for that.

## export archive-v2

```sh
notriosctl export archive-v2 [shared flags] [--target full_archive|subset_transfer]
    [--notebooks id,id] [--tags a,b] [--query "tag:todo"] [--documents id,id]
    [--match any|all] [--max-documents N] [--records-per-object N]
    [--pack] [--pack-bytes N] [--overwrite] [--no-verify] <out-dir>
```

Writes the lossless [native archive v2](archive-v2.md) snapshot: immutable SHA-256 objects with `manifest.json` published last, verified before the command reports success. `--target full_archive` (the default) is the complete-backup mode and takes no selectors; `--target subset_transfer` requires at least one selector and is never described as a backup. Selectors go through the shared [selection/privacy planner](selection-planning.md), and the manifest binds that plan's digest.

Re-running the command over an interrupted export reuses already-published objects and prunes objects the new manifest does not list; `--overwrite` is required to replace an archive that already has a manifest. `--no-verify` skips the post-publication verification pass (not recommended for backups). The JSON report includes `full_backup`, `commit_sha256`, typed `counts`, object and byte totals, `reused_objects`, `cleared_link_targets`, and `warnings`.

`--pack` writes objects into a few large pack files instead of one file per object (`--pack-bytes` sets the target size, default 256 MiB). On a real 382,206-note library that is 46 files instead of 382,447, and about 1.29× faster, at roughly 11% more disk. Loose storage remains the default because it deduplicates and resumes through the object tree; an interrupted packed export restarts instead.

## compatibility archive-v2

Form: `notriosctl compatibility archive-v2 [--reader
current-v2|previous-loose-v2] <archive-dir|manifest.json>`.

Performs bounded declaration-only admission before a consumer opens archive
objects. It emits JSON naming the identified format/version, selected frozen
reader profile, required and optional capabilities, accept/refuse decision,
and stable reason. `current-v2` is the default. `previous-loose-v2` reproduces
the historical base-capability profile without pretending to execute an old
binary.

An accept exits 0 and sets `full_verification_required: true`; run `verify
archive-v2` before consuming any content. A capability or format refusal exits
1 but still emits the JSON report. Usage/profile errors exit 2. Physical
`notrios-sqlite-image` manifests are always refused without opening
`notes.sqlite`, and the report names `notrios-archive-v2` as the semantic
fallback.

## verify archive-v2

```sh
notriosctl verify archive-v2 <archive-dir>
```

Reads an archive **read-only** and prints what it contains: format/version, snapshot and database identity, capabilities, typed record counts, object and byte totals. It fails on an absent manifest, a missing or damaged object, checksum drift, unknown required capabilities, count or reference inconsistencies, unsafe paths, symlinks, and extra files. Nothing is written and no database is opened.

## restore archive-v2

```sh
notriosctl restore archive-v2 [shared flags] --intent replace|adopt|merge|fork
    [--new-database-id id] [--batch-size N] <archive-dir>
```

Admits a verified archive into a database. `--intent` is required — each choice has a different consequence for the logical database universe and Notrios never guesses one:

| Intent | Target | Result |
|---|---|---|
| `adopt` | must be empty | archive's logical database ID, new replica ID |
| `replace` | existing database | contents replaced; archive's database ID, new replica ID |
| `merge` | existing database | records imported; target keeps its identity and replica ID |
| `fork` | any | new logical database ID from `--new-database-id` |

Verification completes in full before the first canonical write, so a damaged archive leaves the target untouched. Both object layouts are read; every note body, resource, and source bundle is re-hashed as it is used, and attachments are re-sniffed through the ordinary resource admission path rather than trusted from archive metadata.

An interrupted restore leaves a durable marker naming the snapshot it was applying. That library is neither empty nor complete: `adopt`, `merge`, and `fork` refuse it and only `--intent replace` recovers it. The JSON summary reports the intent, resulting database/replica IDs, and applied record counts.

## snapshot create / snapshot verify / snapshot restore

```bash
notriosctl snapshot create [shared flags] <out-dir>
notriosctl snapshot verify <snapshot-dir>
notriosctl snapshot restore [shared flags] --intent replace|adopt
    [--emergency dir] <snapshot-dir>
```

Creates or verifies the same-schema whole-library
`sqlite-image+packed-assets.v1` representation. Creation uses SQLite Online
Backup, securely clears machine-local resumptions in the copy, writes
deterministic uncompressed asset/source-bundle packs with bounded payload and
entry counts, and publishes `manifest.json` last. An interrupted rerun reuses
only complete verified packs and restarts the SQLite image.

Verification is read-only. It checks exact schema/application capability,
every hash and length, SQLite integrity, identity, vector/floors, local-state
clearing, safe paths, and database-to-pack object completeness. Restore accepts
no default intent: `replace` requires the same logical database ID and `adopt`
accepts a compatible library explicitly. It verifies an emergency snapshot,
uses a durable startup-blocking roll-forward plan, rotates the writable replica
ID, preserves vector/floors, and queues derived projections. Stop the service;
after interruption, run the identical command again. Use archive-v2 for subset,
merge, fork, publication, or incompatible-schema recovery.

For compatible whole-library backup/catch-up this is the frozen production
default. It is deliberately not archive-v3: archive-v2 remains the portable,
schema-independent semantic format for previous readers, subset, merge, fork,
and publication workflows.

## link

```sh
notriosctl link [--config config.yaml] [--db path] [--asset-store path]
    [--anchor slug|text|^marker] [--list-anchors] <document-id>
```

Prints the [stable link](stable-links.md) for a note: the portable
`notrios://databases/{database_id}/documents/{document_id}` form that survives
moving the database or renaming the profile. The report also carries the note's
title and internal `document://` URI.

`--anchor` points the link at a heading or a block. Give a heading slug, the
heading's text (normalized to the slug), a `^marker`, or a block ID; an anchor
that does not resolve is refused rather than printed. `--list-anchors` prints
what the note offers, with backlink counts.

## open

```sh
notriosctl open [--registry path] [--profile name] [--db path] [--launch] <notrios-uri>
```

Resolves a stable link on this machine. Without `--db` the [profile
registry](stable-links.md) decides which database answers, and only when the
answer is unambiguous. Exit `0` means the link named a note this machine can
open, `1` means it could not be resolved (unregistered database, several
candidate profiles, or a note that no longer exists), and `2` means the link
was malformed — the codes matter because the desktop protocol handler runs this
without a terminal.

`--profile` settles ambiguity between profiles holding clones of one database;
it cannot redirect a link into a different database. `--launch` opens the
resolved note in the local web UI with `xdg-open`.

## profile

```sh
notriosctl profile create --name <profile> [--listen 127.0.0.1:8080]
    [--db path] [--asset-store path] [--sync-target none|directory|rest]
    [--sync-directory path | --sync-rest-url URL] [--credential-ref reference]
    [--copied-database-as adopt|fork] [--registry path]
notriosctl profile show --name <profile> [--registry path]
notriosctl profile register --name <profile> [--db path] [--asset-store path] [--registry path]
notriosctl profile list [--registry path]
notriosctl profile validate [--name <profile>] [--registry path]
notriosctl profile start --name <profile> [--binary notriosd] [--dry-run]
notriosctl profile forget --name <profile> [--registry path]
```

`create` makes a named runtime profile: a registry entry plus one generated
`0600` config with absolute, isolated database/assets/derived paths, a loopback
listen address, public URL, safe `sync-target none` default, and one bound local
replica identity. `show` redacts the credential-store reference to a boolean.
`validate` refuses stale config/database bindings, shared runtime paths,
duplicate loopback ports, and duplicate replica IDs. `start` validates first
and invokes only `notriosd -config <generated-file>`; no credential or reference
is placed in the server command line. It is a foreground launcher, not a
supervisor.

A raw filesystem copy carries the original replica ID and is refused. On the
copy, `--copied-database-as adopt` preserves the logical database ID and mints a
replica ID; `fork` mints both. The action is accepted only when the registry can
prove the duplicate. Existing archive-v2 adopt/fork restore flows already mint
the appropriate identity before profile creation.

The registry defaults to `~/.config/notrios/profiles.json` and may be overridden
with `--registry` or `NOTRIOS_PROFILE_REGISTRY`. `register` is retained as the
older stable-link routing-only surface: it reads identity from the database and
can describe ambiguity, but its entry has no startable config and cannot
replace an existing runtime binding of the same name. `forget` edits
the registry only and never touches the database or generated config it named.

## register-url-handler

```sh
notriosctl register-url-handler [--apply] [--binary path] [--dir path]
```

Ubuntu/XDG protocol-handler registration for `notrios://`. It prints the
desktop entry and changes nothing unless `--apply` is given, because installing
it changes what happens when you click such a link anywhere on the machine. The
entry claims `x-scheme-handler/notrios` and nothing else.

## publish

```sh
notriosctl publish profile save --name <profile> [--target publication_handoff|subset_transfer]
    [--notebooks id,id] [--tags a,b] [--query "..."] [--documents id,id] [--match any|all]
    [--exclude-tags a,b] [--private-tags a,b] [--link-action plain_text|redact|report|retain]
    [--include-provenance] [--include-source-bundles] [--max-resource-bytes N] [--profiles path]
notriosctl publish profile list [--profiles path]
notriosctl publish profile delete --name <profile> [--profiles path]
notriosctl publish plan --profile <profile> [--detail-limit 100]
notriosctl publish run --profile <profile> --reviewed-plan <sha256> [--overwrite] <out-dir>
```

Saves, reviews, and runs a [publication handoff](publishing.md): a scoped,
sanitized archive-v2 directory containing only the notes you selected, with
current text only — no revision history, trashed notes, provenance, exact source
bundles, or saved searches.

`publish plan` writes nothing and prints the notes, resources, link decisions,
metadata decisions, warnings, and a `manifest_sha256`. `publish run` requires
that digest and re-plans before writing: if anything about the library changed
since the review, it refuses. `--target full_archive` is refused at save time —
a backup must not be reachable through a name that sounds like publishing.

Profiles live in `<data-dir>/publish-profiles.json` (override with `--profiles`
or `NOTRIOS_PUBLISH_PROFILES`) and are written owner-only.

## lint

```sh
notriosctl lint [--config config.yaml] [--db path] [--asset-store path] [--collection id]
    [--checks a,b] [--detail-limit 100] [--quiet] [--list-checks]
```

Read-only [workspace lint](operations.md#finding-what-has-rotted-workspace-lint):
broken document and resource links, ambiguous wikilinks, unresolved block
anchors, duplicate external identities, missing titles, unlocalized remote
media, missing alt text, unreferenced resources, and projection backlog.

Exit `0` when the library is clean and `1` when findings exist, so
`notriosctl lint --quiet` works in a hook. `--detail-limit` caps the examples
printed per check, never the counts, and `report_sha256` covers every finding —
the same library produces the same digest. Nothing is written; fixing is a
separate operation.

Every collection is read. A library you migrated into is one library: notes
that carry a `joplin-…` collection are yours, and a lint that reported the rest
of the library as clean while they rotted was wrong. `--collection` narrows to
one provenance when that is what you want, which is the only thing it does now —
it used to be assumed, and the report said `default` as though you had asked.
The same applies to `fix`, to `graph report` and `graph export`, and to what a
`full_archive` contains.

Since v0.6, content checks skip notes in the read-only builtin notebooks — Help
and Reports. `notriosctl fix` structurally cannot repair them and you cannot
edit them either, so a finding there was noise rather than information. **On a
library with Help seeded this changes the output**: findings inside Notrios' own
documentation stop being reported, and `report_sha256` changes with them.
`projection_backlog` still covers every note, because that one says the search
index is behind rather than that a note needs editing.

## fix

```sh
notriosctl fix [--config config.yaml] [--db path] [--asset-store path]
    [--kinds a,b] [--document id] [--max-documents 1000] [--apply] [--allow-review] [--list-kinds]
```

Repairs the [mechanically safe subset](operations.md#fixing-what-can-be-fixed-mechanically)
of what `notriosctl lint` reports. Dry run is the default and prints the exact
`before`/`after` of every edit; `--apply` writes them one note at a time, each
against the revision its plan was computed from, so a note edited in between
fails while the rest proceed. Every fix writes an ordinary revision.

`non_canonical_link_target` runs by default. `missing_alt_text` and
`unlocalized_remote_media` are opt-in — the first because a filename is not a
description, the second because it reaches the network (through the full media
policy, exactly as `notriosctl localize` does). Findings outside this set stay
reported and unfixed rather than guessed at.

## tags show

```sh
notriosctl tags show --tag <name> [--db ...]
```

One tag, its live note count, and its children. **It exits 1 when no such tag
exists**, so a script can test for a tag without parsing anything:

```sh
if notriosctl tags show --tag todo >/dev/null 2>&1; then
  echo "the tag exists"
fi
```

Tags nest with `/`, and a branch reports what is under it:

```text
{ "tag": "shopping", "notes": 4,
  "children": [ { "tag": "shopping/mall", "notes": 2 } ] }
```

Listing the *notes* carrying a tag is a search — `tag:todo` — rather than
something this reports, so there is one place that pages and orders results
instead of two.

Narrowing exists on the other surfaces too: `GET /api/v1/tags?name=todo` returns
404 when there is no such tag, and the `list_tags` tool takes the same `name`,
`prefix` and `limit`.

## tags rename

```sh
notriosctl tags rename --from <tag> --to <tag>
    [--config config.yaml] [--db path] [--asset-store path]
    [--include-children] [--apply]
```

Renames a tag and, with `--include-children`, every tag under `<from>/`. See
[renaming a tag hierarchy](operations.md#renaming-a-tag-hierarchy).

Dry run is the default. It is not a prediction: the rename runs inside a
transaction and is rolled back, so the report is produced by the statements that
would do the work. `--apply` commits instead.

Exit `0` on a clean plan or a successful apply, and `1` when the tag does not
exist, the destination name is invalid, the rename would touch more than 500
tags — or when a **dry run's plan contains a merge**. That last one is the
useful case in a script: renaming onto a name that already exists combines two
hierarchies, and it should not happen because nobody read the plan.

## Reading JSON output

Most commands print JSON. That is deliberate: one output format is one thing to
keep correct, and anything that can parse JSON can consume it. When you want a
table, render it with a template tool rather than asking Notrios for a second
format.

[gomplate](https://docs.gomplate.ca/usage/) does this in one line. It is not
required by Notrios and nothing here invokes it; any tool that reads JSON works
the same way.

An aligned table from a listing:

```sh
notriosctl collections list --json |
  gomplate -d 'c=stdin:?type=application/json' \
    -i '{{ printf "%-14s %5s  %s" "COLLECTION" "NOTES" "NAME" }}
{{ range (ds "c").collections }}{{ printf "%-14s %5v  %s" .collection_id .notes .name }}
{{ end }}'
```

```text
COLLECTION     NOTES  NAME
default            1  Default
joplin           412  Joplin
```

Use `%v` rather than `%d` or `%f` for numbers: a JSON number arrives as whatever
the decoder made of it, and `%v` prints it either way.

One field, for a shell variable:

```sh
notriosctl notebooks list --json |
  gomplate -d 'nb=stdin:?type=application/json' \
    -i '{{ (index (ds "nb").notebooks 0).notebook_id }}'
```

A filter -- here, the notebooks that are really saved searches, with the query
each one runs:

```sh
notriosctl notebooks list --json |
  gomplate -d 'nb=stdin:?type=application/json' \
    -i '{{ range (ds "nb").query_notebooks }}{{ .search_notebook_id }} -> {{ .query }}
{{ end }}'
```

For a one-off, `jq` and a short Python script do the same job; the point is that
the rendering lives with the person who wants it rather than in every command.

**The exceptions.** `doctor`, `paths`, `config show`, `migrate`,
`notebooks list`, `collections list` and `collections show` print a human form
by default and take `--json` for the structured one. Every other command prints
JSON, and passing `--json` to a command that does not offer it is an error
rather than a no-op.

## collections list and collections show

```sh
notriosctl collections list [--json] [--db ...]
notriosctl collections show --collection <id> [--json] [--db ...]
```

A collection records where a body of notes came from. Notes written in Notrios
are in `default`; an import puts its notes in whichever collection
`--collection` named, creating it if the identifier is new.

`collections list` prints one row per collection with the number of notes that
name it, which is how you tell whether an import landed:

```text
COLLECTION  NOTES  NAME
default        18  Default
joplin        412  Joplin
obsidian        0  Obsidian
```

A collection with no notes, as `obsidian` is above, means the import did not
finish or was pointed somewhere else.

Use an identifier to narrow a search: `collection:"joplin"`. A search covers
every collection unless a `collection:` term says otherwise. See
[the query language](query-language.md).

Listing and showing is all the command line does with collections. Renaming and
deleting are not available on any surface, because what should happen to notes
that name a collection has not been decided.

## search

```sh
notriosctl search [--limit N] [--cursor c] [--count] [--links] "<query>"
```

Find notes with the query language the search box parses — `tag:`, `notebook:`,
`collection:`, dates, phrases, `OR`, negation and the rest, exactly as
[the query language](query-language.md) describes them.

Each hit carries the note's identifier, which is what the rest of the command
line takes:

```json
{ "query": "dusk",
  "hits": [ { "document_id": "doc_hahbr6t…", "title": "Reed beds 2",
              "notebook_id": "nb_notes", "collection_id": "default",
              "updated_at": "2026-09-08T18:51:55Z",
              "uri": "document://default/documents/doc_hahbr6t…",
              "snippet": "Seen at <mark>dusk</mark>, note 2." } ] }
```

`--limit N` bounds a page, up to the API's ceiling of 100, and the response
carries `next_cursor` when there is more; pass it back with `--cursor`.

`--count` answers how many notes match instead of which ones. It is a counting
query over the same compiled predicate — not the hits fetched and tallied, which
would be a lie about cost on a large library.

`--links` reports `notrios://` links rather than `document://` URIs, for pasting
into another machine's library. They are the links `notriosctl link` prints for
the same note.

A search spans every collection unless the query narrows it, and Trash is
excluded unless the query says `is:trashed`. Read-only: it prints what a search
returns and changes nothing, which is what makes it safe in a pipe.

Before this command existed, nothing at a terminal produced note identifiers, so
`notes show`, `notes move` and `tags add` could only be used on an id somebody
already had. The documented workaround was `export archive --query`, which
applies the query language to a *file export* — an answer to a different
question.

**Also on REST and MCP:** `GET|POST /api/v1/search` and the `search_documents`
tool, with the same language.

## notes show, notes outline, notes resources and notes links

```sh
notriosctl notes show --document <id> [--json] [--output <file>]
notriosctl notes outline --document <id> [--output <file>]
notriosctl notes resources --document <id> [--output <file>]
notriosctl notes links --document <id> [--direction outgoing|incoming|both]
```

Reading a note and what it is made of. Find the id with a search, or with
`notebooks list` and the query language.

`notes show` prints the note as a Markdown file with YAML front matter — the
same rendering the Recoll projection writes, so a note exported this way carries
the id, title, notebook, collection, source provenance and tags another
application expects:

```text
---
id: "doc_zjdoaotrxr57tpbrcv57gapkke"
title: "Reed beds"
notebook: "Field notes"
collection: "default"
tags:
  - "wetland"
---

Seen at dusk.
```

`--json` prints the note's fields instead, body included. `--output <file>`
writes to a file rather than standard output, which keeps the exit code that a
shell redirect throws away. `--body` is accepted and ignored: it chose whether
to include the body when this command printed only metadata, and both forms
carry the body now.

`notes outline` lists the headings with the anchor and line of each. **The
anchors are the ones a `notrios://` link resolves against** — the outline is
derived from the same parse that stores them, so an anchor printed here is one
`notriosctl link --anchor` will find.

`notes resources` lists what is attached, with each attachment's id, filename,
type, size and hash. `notriosctl resources get --resource <id> --output <file>`
writes one attachment's bytes; without `--output` they go to standard output, so
the file is exactly the stored bytes and the summary line goes to standard
error.

`notes links` lists the links out of a note, or `--direction incoming` for the
ones pointing at it, or `both` for a page of each. Broken links are reported
rather than filtered out, because a broken link is the interesting one.

**Also on REST and MCP.** These are adapters onto capabilities that already
existed: `GET /api/v1/documents/{id}`, `/body`, `/outline`, `/blocks`, `/lines`,
`/resources` and `/links`, plus `GET /api/v1/resources/{id}/content`; and the
`get_document`, `get_document_outline`, `get_document_blocks`,
`get_note_line_range`, `list_document_resources`, `list_document_links` and
`read_resource` tools. Blocks, line ranges and earlier revisions have no command
of their own yet.

## resources add, notes append and notes prepend

```sh
notriosctl resources add --file <path> [--filename <name>] [--document <id>]
notriosctl notes append --document <id> --text "<text>"
notriosctl notes prepend --document <id> --text "<text>"
```

Attaching a file is three things and only two of them are automatic:

1. the **resource** — the bytes, content-addressed, with an id and a
   `resource://` URI;
2. the **reference** — the note recording that it carries the attachment, which
   is what `notes resources` lists;
3. the **link in the body** — where it renders, which is yours.

`resources add` does the first, and the second when `--document` names a note.
It prints the URI to paste and **never writes to a note body**: putting the link
somewhere of its own choosing would be guessing at the one thing only the writer
knows.

```text
{ "resource_id": "res_rqai…", "uri": "resource://default/resources/res_rqai…",
  "filename": "photo.png", "mime_type": "image/png",
  "size_bytes": 20418, "sha256": "ab444b…" }
```

**The type comes from the bytes, not the extension.** A text file named `.png`
is recorded as `text/plain`, because an extension is what somebody typed.

`notes append` and `notes prepend` place the link. They exist because **nothing
on any surface can patch a range of a note body** — REST and MCP can read one
(`GET /documents/{id}/lines`, `get_note_line_range`) and neither can write one —
so without them, adding a line means reading the whole note, editing it
elsewhere and writing it all back, losing any concurrent edit in between. Both
take `--text` or `--text-file` (or `-` for standard input), and
`--base-revision` pins the revision they apply to; without it they retry once
against whatever is current.

This is local bytes only. A URL belongs to `notriosctl localize`, which goes
through the domain policy, quarantine, hashing and SSRF protections that fetching
requires.

**Also on REST and MCP:** `POST /api/v1/resources` and
`POST /api/v1/documents/{id}/resources/{resource_id}`; `POST /append` and
`/prepend`, and the `append_to_note` and `prepend_to_note` tools.

## notes move

```sh
notriosctl notes move --document <id> --notebook <id|name>
    [--config config.yaml] [--db path] [--asset-store path]
```

Files one note into another notebook. See [choosing which notebook a note goes
in](gui.md#choosing-which-notebook-a-note-goes-in).

There is no dry run and no `--apply`, unlike `gc`, `fix`, and `tags rename`. A
move is neither destructive nor lossy — it changes where one note lives, and
moving it back is the same command with the other notebook — so a confirmation
step would be ceremony rather than safety.

`--notebook` takes an ID, or a name when the name identifies one notebook.
Notebook names are unique only among siblings, so `Contacts/Work` and
`Personal/Work` can both exist; given an ambiguous name the command exits `1`
and lists the matching IDs instead of picking one. Filing a note somewhere
unintended is exactly what this command exists to correct.

## graph report

```sh
notriosctl graph report [--config config.yaml] [--db path] [--asset-store path] [--collection id]
    [--limit N] [--write-note] [--quiet]
```

The shape of the link graph: totals, the most-linked notes by in-degree, the
orphans, and the isolates. `--limit` caps the example lists only; the counts
always describe everything the report covers, which is every collection unless
`--collection` names one.

`--write-note` also renders it as a note in the builtin **Reports** notebook,
with a stable ID, overwritten in place, carrying the time it was generated. The
notebook is read-only, so the note cannot be edited through any surface — a
generated report someone can edit is one that silently stops being true.

Nothing regenerates it on a timer or on save: the scan reads every note, which
would be the one unbounded thing in an otherwise bounded design.

What is measured is the live library you own. Notes in the Trash and notes in
read-only builtin notebooks are neither ranked nor counted, and neither end of a
counted link may be one of them — which is what lets the report live in the
library it measures without changing the answer.

## graph export

```sh
notriosctl graph export [--config config.yaml] [--db path] [--asset-store path] [--collection default]
    [--overwrite] <out-dir>
```

Writes `nodes.csv` and `edges.csv` for tools built to analyse graphs — Gephi,
Cytoscape, NetworkX, igraph all import them. Column names follow Gephi's
convention (`Id`/`Label`, `Source`/`Target`/`Type`) because it is the fussiest
of the four.

Rows stream as they are read, so the export does not grow with what it can hold
in memory. It refuses to replace an existing file unless you pass `--overwrite`.

CLI-only, like archive export: it writes files to a path you named, which is not
a choice a REST caller or an MCP client should make for you. The exported graph
is the same one `graph report` measures.

Centrality, modularity, and community detection are whole fields with software
designed for them; Notrios emits the graph rather than reimplementing any of it.

## jobs

```sh
notriosctl jobs list   [--db path] [--kind k] [--state s] [--limit 50]
notriosctl jobs status [--db path] [--wait] [--timeout 30m] [--quiet] <job-id>
notriosctl jobs show   [--db path] [--command] <job-id>
notriosctl jobs cancel [--db path] <job-id>
```

A long import or archive export records a job, so you can watch it from another
shell or over REST without holding the terminal that started it. The current
GUI's Sync Center lists sync jobs only; it does not show or cancel these
import/export jobs. Dry runs record nothing: they change nothing and finish
quickly.

`status` is the shell-legible half, and its exit codes are the reason no
scheduler is needed — `job-a && job-b` works:

| Code | Meaning |
|---:|---|
| 0 | succeeded |
| 1 | failed |
| 2 | usage error (as everywhere else in this CLI) |
| 3 | queued or still running |
| 4 | cancelled |
| 5 | no such job |
| 6 | interrupted — the process stopped without finishing |

`--wait` blocks until the job settles and then exits with its code; `--timeout`
gives up and exits 3. **Flags must come before the job ID**, as Go's flag
parsing requires; the command says so if you get it the other way round.

6 is separate from 1 on purpose. A failure needs investigating; an interruption
usually just needs the command run again, because the importers resume from
their own durable checkpoints.

**Cancelling is cooperative.** The flag is set at once and the work stops after
its next committed, checkpointed batch, so everything it finished is kept and
rerunning the same command continues from there. Ctrl-C does the same thing —
the first one cancels, a second stops the process immediately and leaves the
record to go `interrupted`.

`show --command` prints the command that reproduces the run:

```sh
$ notriosctl jobs show --command job_01H...
notriosctl import obsidian --collection default --batch-size 25 '/home/you/My Vault'
```

That is rendered from stored *parameters*, not from stored argv. Storing the raw
command line would have captured local paths and any secret that happened to be
on it into the database, and a stored string cannot improve when a flag is
renamed. `list` and the REST routes never show parameters at all; this command
is local, which is where a local path belongs.

## sync

```sh
notriosctl sync init      [--db path] [--keys path]
notriosctl sync invite    [--ttl 15m] [--label ...] [--offline --out <file>]
notriosctl sync join      --url <base-url> --code <code>
notriosctl sync accept    --invite <file> --code <code> --out <file>
notriosctl sync enroll    --acceptance <file> --code <code>
notriosctl sync handshake --url <base-url>
notriosctl sync peers     [--db path]
notriosctl sync revoke    --key <signer-key-id> [--reason ...] [--advance-epoch]
notriosctl sync retire    --peer <replica-id> [--reason ...] [--confirm retire-peer:<replica-id>]
notriosctl sync retention --snapshot <retained-snapshot-dir> [--apply --confirm-digest <digest>]
notriosctl sync status    [--db path] [--keys path]
notriosctl sync discover  [--carrier dir]
notriosctl sync once      [--carrier dir] [--cleanup] [--materialize N]
notriosctl sync migrate-credentials --to native|development-file [--dry-run] [--confirm]
```

Synchronizes two of *your own* libraries — through a folder you both can reach,
or directly over an authenticated connection. Everything that moves is
encrypted and signed, and everything in a shared folder also exists in the
library that published it, so deleting the folder loses nothing.

Sync is off until you turn it on. `init` enrols this library's journal and
creates its key material; nothing before that point writes a single sync record.

### Where the key material is kept

An installed Notrios keeps it in your operating system's own store — GNOME
Keyring or KWallet on Linux, Windows Credential Manager, or the macOS Keychain.
The key file stays where it is and is encrypted; only the key that opens it goes
into the store. Running from a source checkout keeps a plain `0600` file
instead, so a developer's throwaway libraries never land in their real keychain.
`sync.rest.credential_store` overrides both, with `native` or
`development-file`.

A library that already has keys keeps using whatever holds them, whichever way
that setting would otherwise fall. That matters on upgrade: a library enrolled
before this existed has its keys in a `0600` file, and pointing it at a keychain
that does not hold them would strand it. So it carries on, and `init`, `status`
and `doctor` all tell you to move them.

Changing the setting does not move existing keys, and nothing moves them for
you. Key material cannot be regenerated — peers have already published
artifacts your current group key decrypts — so a library that switched stores
automatically and then could not find its keys would be indistinguishable from
one that never had any. Move them deliberately:

```sh
notriosctl sync migrate-credentials --to native --dry-run
notriosctl sync migrate-credentials --to native --confirm
```

The dry run reports the plan and writes nothing. The real run writes the new
copy, reopens it, checks that the signing key and the group key came through
unchanged, and only then removes the old one — so an interruption at any point
leaves the keys readable where they started. `--to development-file` reverses
it, which is the way back if a machine loses access to its keychain.

If the store you name cannot be reached, `init` refuses before enrolling rather
than failing at the first sync, and `doctor` reports it as a failed check.

### Making the second replica

Two libraries created separately are two *databases*, and sync refuses to join
them — that check is the reason a stray copy cannot quietly merge into your
notes. A second replica is made from the first:

```sh
notriosctl export archive-v2 --db first/notes.sqlite /tmp/snapshot
notriosctl restore archive-v2 --intent adopt --db second/notes.sqlite /tmp/snapshot
```

`adopt` keeps the database identity and mints a new replica identity, which is
exactly what a second device is.

### Pairing

Pairing is explicit and deliberate. One replica issues a **code**: short,
single-use, and valid for minutes rather than days. The other spends it, and in
that one exchange the two learn each other's signing keys and the joining side
receives the library's group key — **sealed under the code**, never in readable
text.

Over a network:

```sh
# on the replica that is already set up
notriosctl sync invite --ttl 5m --label "the laptop"
# → reads out a code like ABCD-EFGH-IJKL-…

# on the joining replica
notriosctl sync join --url https://desktop.local:8443 --code ABCD-EFGH-IJKL-…
```

For a replica that has no network path to its peer — one that will only ever
meet it through a folder or a USB stick — the same ceremony splits into a file
and a code that travel **separately**:

```sh
# on the inviting replica: writes the file, reads out the code
notriosctl sync invite --offline --out /tmp/invite.json --ttl 1h

# on the joining replica: needs both halves
notriosctl sync accept --invite /tmp/invite.json --code ABCD-… --out /tmp/accept.json

# back on the inviting replica: enrols what came back
notriosctl sync enroll --acceptance /tmp/accept.json --code ABCD-…
```

**Send the file and the code by different means.** The file alone cannot be
opened, and the code alone is useless once it is spent or expired; together they
are the whole of your library's security, which is why nothing in either half is
a password you might reuse.

To check that pairing worked:

```sh
notriosctl sync handshake --url https://desktop.local:8443
```

That signs a request with this replica's key and asks the peer who it is. A
success means the peer authenticated you as an enrolled replica of the same
database. It carries no note content.

### Seeing and ending trust

```sh
notriosctl sync peers
notriosctl sync revoke --key <signer-key-id> --reason "lost laptop" --advance-epoch
```

`peers` lists every signing key this library has enrolled, whether it is still
active, and whether that replica is configured for admission. `revoke` ends a
key: requests signed with it are refused from that moment, and the refusal is
audited.

`--advance-epoch` is the other half of the answer, and deliberately separate.
Revoking stops a device *signing*; advancing the epoch mints a new group key so
it cannot *read* what is published next. It costs every remaining peer a fresh
pairing, so it is a decision rather than a side effect. The old epoch stays
readable, because a library should not lose its own history in order to exclude
a device.

Credential revocation is deliberately not peer retirement: it stops trust but
keeps that replica holding the history watermark open. To end the replica and
permit future collection, preview first and then repeat the exact confirmation:

```sh
notriosctl sync retire --peer replica_abc
notriosctl sync retire --peer replica_abc --reason "device recycled" --confirm retire-peer:replica_abc
```

The signed decision travels in the ordinary operation log without requiring
every peer online. Old credentials cannot re-enroll; that device must reset and
pair as a new replica. The result lists active peers that have not acknowledged
the decision.

Retention is also review-first. Both commands fully verify the retained
physical snapshot and its database identity at invocation, so a stale database
record cannot stand in for a directory that was deleted:

```sh
notriosctl sync retention --snapshot /safe/notrios-snapshot
notriosctl sync retention --snapshot /safe/notrios-snapshot --apply --confirm-digest <digest-from-review>
```

The configurable default is 90 days. Time alone never authorizes collection:
the safe floor also needs that snapshot and every active peer acknowledgement.
If the state changes between review and apply, the digest is refused. A peer
below an already collected floor needs verified snapshot catch-up; installation
is never automatic.

### Exchanging with a peer directly

If the other replica runs a service you can reach, you can exchange with it
without a folder in between:

```sh
notriosctl sync exchange --url https://desktop.local:8443
```

This is the same exchange, over a different courier. The peer moves artifacts;
your replica merges them, exactly as it does through a folder. Attachments are
fetched afterwards, bounded by `--materialize`.

To fetch a whole library — a new device, or one being rebuilt:

```sh
notriosctl sync fetch-backup --url https://desktop.local:8443 --out /tmp/restore
```

The peer must have explicitly permitted your replica to receive a snapshot;
being enrolled is not enough, because a snapshot is a complete copy of the
library. The download is encrypted, resumable, and verified as a same-schema
physical snapshot before the command reports success — interrupt it and run it again and
it continues from where it stopped.

Without an install intent it stops at a verified snapshot and prints the command that would
restore it:

```sh
notriosctl snapshot restore --intent adopt --db second/notes.sqlite \
  --asset-store second/assets /tmp/restore/snapshot
```

What a restore does to a library is a decision with an intent, not something a
download should make for you.

`--intent adopt` works into a path that holds no library yet -- that is how a
second replica is made -- and reports `emergency_snapshot_skipped` saying there
was nothing to protect. `--intent replace` needs a library to replace and
refuses a fresh path, naming `adopt` as the intent that would work. A file that
exists and is not a Notrios library is refused rather than overwritten.

On a stopped service, `fetch-backup` can perform that same explicit restore
after verification with `--intent replace|adopt [--emergency dir]`. It never
chooses an intent automatically. Re-enroll the fresh replica identity, then run
ordinary incremental sync so only operations after the installed snapshot
vector replay.

### Exchanging through a folder

```sh
notriosctl sync once --carrier /home/you/Drive/notrios --db first/notes.sqlite
```

One round publishes what your peers are missing, reads what they published,
and then materializes up to `--materialize` attachments whose bytes have
arrived. Attachments are fetched only when asked for, so a note referencing a
large file arrives long before the file does, and a note whose attachment nobody
has published stays readable with that attachment marked unavailable.

Run it when you like: from a shell, from `cron`, from a script after an import.
**There is no watcher and no scheduler**, and correctness does not depend on
one — a round is a full scan, so a missed notification costs latency and
nothing else.

`--cleanup` lets this replica remove its own artifacts once every peer has
acknowledged them. It is off by default and the folder stays correct without it;
it never touches another peer's files.

### Seeing who is there

```sh
notriosctl sync discover --carrier /home/you/Drive/notrios
```

Reports the replicas publishing into the folder without publishing, admitting,
or trusting anything. A peer whose signing key you have not paired with shows up
as a key id and nothing else — its artifacts are refused before they are
decrypted, so there is genuinely nothing else to report.

### Keys

Key material lives in a `0600` JSON file, by default under your user config
directory, and never inside the library — a database copied to another machine
must not carry the keys that decrypt its traffic. If the file becomes readable
by other users, `status` reports it and `once` refuses to run. This is a
development secret provider, not a system keychain; every command that touches
it says so.

The file holds secrets only: this replica's signing key and the library's group
key per epoch. **Which peers you trust is in the database**, where enrolling and
revoking are transactional, auditable, and visible to every process at once —
`sync peers` reads it, and `sync revoke` changes it.

## seed-help

```sh
notriosctl seed-help [--config config.yaml] [--db path] [--asset-store path] [docs-dir]
```

Mirrors a documentation directory (default `docs`) into the built-in read-only **Help** notebook: unchanged notes are kept, changed files update in place, and notes whose source file disappeared are removed. Deterministic note IDs make it repeatable. Report: `files_seen`, `notes_created/updated/kept/removed`.

```json
{"files_seen": 10, "notes_created": 10, "notes_updated": 0, "notes_kept": 0, "notes_removed": 0}
```

## localize

```sh
notriosctl localize [--config config.yaml] [--db path] [--asset-store path] [--dry-run] [--allow-review] [--base-revision rev] <document-id>
```

Downloads the note's policy-**allowed** remote media through the quarantine pipeline (domain and redirect-hop checks, private-address blocking, size caps, MIME sniffing, exact hashes), stores it as local content-addressed resources, and rewrites the note's Markdown to `resource://` links in a new revision. `--dry-run` prints the per-URL policy decisions without fetching a byte or writing anything. `--allow-review` also localizes URLs whose decision is `review`; blocked URLs are never fetched. `--base-revision` guards against concurrent edits (default: the note's current revision). The JSON report lists `localized`, `blocked`, `review`, `failed`, and the new `revision_id`.

The `import joplin-raw` and `import obsidian` commands accept `--localize-media` to run the same engine over every imported note after the import completes.

## resources report

```sh
notriosctl resources report [--config config.yaml] [--db path] [--asset-store path]
```

Prints a read-only JSON report of logical resources sharing an exact SHA-256
blob, blobs with no document references, and per-notebook resource usage.
References held by trashed notes still count, so the unreferenced list is safe
input for the retention-aware garbage collector. The perceptual section
is empty by default because Notrios ships no perceptual algorithm; an installed
hook may add review-only policy matches and near-duplicate suggestions.

## gc

```sh
notriosctl gc [--config config.yaml] [--db path] [--asset-store path] [--snapshot retained-snapshot-dir] [--dry-run | --apply]
```

Plans retention-aware resource garbage collection. With no mode flag—or with
`--dry-run`—it only reports:

- `eligible`: unreferenced logical resources whose configured retention window
  and retention gate are satisfied;
- `retained`: unreferenced resources kept, with the exact reason and eligible
  timestamp;
- `removed`: empty during a dry run;
- physical blob/byte totals and warnings.

Only `--apply` deletes. Apply rechecks every reference inside the SQLite
transaction; referenced resources, including resources referenced only by
notes in Trash, are never eligible. A shared physical blob remains until its
last logical resource is removed.

For a sync-enrolled library, `--snapshot` is required even for the plan so the
same physical recovery image is re-verified before the acknowledgement gate is
built. Backup/export sinks never count as active peers.
