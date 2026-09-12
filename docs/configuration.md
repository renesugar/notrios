# Configuration

Notrios runs with no configuration file at all. Everything below has a default,
and the defaults are chosen so that a fresh install puts your notes where your
desktop expects them and listens only on loopback. Read this page when you want
to move something, not to get started.

This page is task-shaped: each section is a thing you might want to change. The
complete key table is [at the end](#every-key), generated from the code so it
cannot drift out of agreement with it.

## Where configuration comes from

`notriosd` takes `-config <path>`. When you do not pass one:

1. it loads `config/config.example.yaml` if you are running from a source
   checkout and that file exists, then
2. it falls back to compiled defaults.

So a checkout behaves like the example file and an installed package behaves
like the defaults. `notriosctl config show` prints what is actually in effect,
with your home directory replaced by `~` unless you pass `--no-redact` — that
is the command to reach for before editing anything, because it answers "what
is the current value" without you having to work out which of the three sources
won.

`-addr` and `-db` remain as explicit overrides on the command line, and they
beat the file.

Two flags are worth knowing about together: `notriosctl paths` prints the
resolved roots and how they were resolved, and `notriosctl doctor` checks that
they exist, are writable, and that the database opens. Between them they answer
most "why is it not where I put it" questions without a config change.

## Where your notes are stored

The keys under `data` are the ones people change first, usually to put a library
on another disk. They are separate keys rather than one root because they have
genuinely different lifetimes, and `notriosctl purge` treats them differently:

| Key | Holds | What a purge does with it |
|---|---|---|
| `data.directory` | the storage root the others default under | — |
| `data.database_path` | the SQLite library: every note, notebook, tag and revision | backs it up, then deletes |
| `data.asset_store` | attachments, content-addressed | backs it up, then deletes |
| `data.state_dir` | what must survive a restart but you did not write: sync spools, sync backups, the catch-up inbox, the remote-media quarantine | backs it up, then deletes |
| `data.cache_dir` | what can be rebuilt from the library: projections, the search index | deletes **without** a backup |
| `data.runtime_dir` | work in flight that must not survive a reboot: backup staging, restore review | deletes without a backup |
| `data.projection_dir` | rendered projections | with the cache |

The distinction is not cosmetic. Anything you put in `cache_dir` is deleted
without a copy, deliberately, because a purge that backed up a search index
would spend a user's disk on something rebuildable. Do not move something
irreplaceable there.

A library outside these roots is not deleted by a purge — it is listed and left
alone. See
[Removing your data as well](installation.md#removing-your-data-as-well).

On startup the service creates the data directory, the database's parent
directory, the asset store, the projection directory and the search sidecar's
index directory. It does not create anything under a path you have not
configured.

## Serving the API and the interface

`server.listen_addr` defaults to loopback, and that is the intended
configuration: Notrios is a local-first application and the service is not
hardened for exposure to a network. If you change it, you own what that means.

`server.public_base_url` is what the service believes its own address to be,
which matters for links it generates rather than for what it binds. Set it when
Notrios is behind a reverse proxy and the address a browser uses is not the
address the service listens on.

`server.web_dir` is the built interface — the directory with `index.html`.
Leaving it empty searches the working directory, then the executable's own
directory and its parent, which covers both a checkout and an installed package.
Set it when the binary lives apart from its assets.

`mcp` configures the MCP surface served at `/mcp`, for an assistant that speaks
that protocol.

## Search

`search.default_limit` and `search.max_limit` bound what a query returns.
`max_limit` is a real limit, not a suggestion: a request asking for more is
clamped, so raising it is how you allow larger pages rather than how you make
them faster.

`search_sidecar` configures the optional Recoll-derived sidecar, which adds
field search over front matter and over the text inside attachments. It is
optional in the strict sense — everything works without it, using SQLite FTS5 —
and `search_sidecar.index_dir` is created at startup when the sidecar is
configured. `notriosctl doctor` reports whether `recollindex` was found.

## Sync

`sync` and `sync.rest` configure synchronisation between your own libraries.
The keys here are about transport and scheduling; the identities and keys
themselves are not configuration and never appear in this file. Sync key
material lives in its own store, and a purge deliberately excludes it from the
backup it writes, because that file is the password to your synchronised
traffic.

If you are setting sync up for the first time, the pairing and enrolment
commands in [the CLI guide](cli.md) come before any of these keys.

## Remote media

`remote_media` governs what happens when an imported note refers to an image or
a file on the internet. It is the most security-relevant section on this page,
because localizing remote media means fetching bytes a note asked for.

Fetching goes through a domain policy, a quarantine, exact and perceptual
hashing, MIME sniffing, a size limit and protections against being pointed at
your own network. The keys here tune that; they do not switch it off. If you
are deciding what to allow, read
[the remote-media section of the service guide](service.md) before setting
anything permissive.

## Retention

`retention` decides how long Notrios keeps things it could remove: resource
history, sync history and so on. `notriosctl resources report` shows what the
current settings would collect before you change them, which is the safer order.

## Profiles

`profile` names this library when several exist on one machine. Profiles are
registered with `notriosctl profile register`, and the registry is a separate
file from this configuration — see [the CLI guide](cli.md). A profile can keep
its database anywhere, including outside the roots above.

## Every key

<!-- notrios:generated:config:keys:begin -->
53 settable keys, generated from `internal/config` by `go run ./cmd/docconfig --write`. A dash means the default is empty, which for a path means "work it out from the XDG roots". The recorded configuration surface counts 63, because it includes the 10 section names that group these.

| Key | Type | Default | Notes |
|---|---|---|---|
| `config_path` | string | — |  |
| `profile.id` | string | — |  |
| `profile.name` | string | — |  |
| `profile.registry_path` | string | — |  |
| `server.listen_addr` | string | `127.0.0.1:8080` |  |
| `server.public_base_url` | string | `http://127.0.0.1:8080` |  |
| `server.web_dir` | string | — | WebDir is the directory holding the built web interface (the one with index.html). Empty searches the default locations: the working directory, then the executable's own directory and its parent. Set it when the binary lives apart from its assets. |
| `data.directory` | string | `./data` |  |
| `data.database_path` | string | `./data/notes.sqlite` |  |
| `data.asset_store` | string | `./data/assets` |  |
| `data.projection_dir` | string | `./data/projections` |  |
| `data.state_dir` | string | — | StateDir holds what Notrios must remember across runs but the user did not write: sync carrier spools, sync backups, the catch-up inbox, and the remote-media quarantine. It is backed up before a purge. |
| `data.cache_dir` | string | — | CacheDir holds what can be rebuilt from the library: projections and the search index. A purge disposes of it without backing it up, so nothing irreplaceable may live here. |
| `data.runtime_dir` | string | — | RuntimeDir holds work in flight that must not survive a reboot: backup staging and restore review, which briefly hold decrypted library contents. |
| `sync.target` | string | `none` |  |
| `sync.directory` | string | — |  |
| `sync.rest_base_url` | string | — |  |
| `sync.credential_ref` | string | — |  |
| `sync.rest.enabled` | bool | `false` | Enabled exposes /api/v1/sync/... to authenticated peers. Nothing else on the API becomes remotely reachable or remotely authorized by it. |
| `sync.rest.require_tls` | bool | `true` | RequireTLS refuses to serve the sync surface over plaintext on anything but a loopback address. It defaults to true and should stay true: a peer credential is a signature rather than a bearer token, so plaintext does not leak a reusable secret, but every byte of every note would be in the clear once G14 carries data. |
| `sync.rest.tls_cert_file` | string | — | TLSCertFile and TLSKeyFile enable HTTPS for the whole service. |
| `sync.rest.tls_key_file` | string | — |  |
| `sync.rest.max_body_bytes` | int64 | `0` | MaxBodyBytes bounds an authenticated request body. |
| `sync.rest.requests_per_minute` | int | `0` | RequestsPerMinute and Burst bound one peer; FailuresPerMinute bounds how fast one address can guess. |
| `sync.rest.burst` | int | `0` |  |
| `sync.rest.failures_per_minute` | int | `0` |  |
| `sync.rest.key_file` | string | — | KeyFile is the local sync key material. Empty uses the default path. |
| `sync.rest.credential_store` | string | — | CredentialStore names where the secret that protects KeyFile lives. Empty means the development file, which is v0.7 behaviour and stays the default until a migration exists: switching an installed profile to the keychain without moving its existing material would strand it. |
| `search.default_limit` | int | `20` |  |
| `search.max_limit` | int | `100` |  |
| `mcp.enabled` | bool | `true` |  |
| `mcp.default_scope` | string | — | DefaultScope is the MCP tool scope: search-only, read-only, editor, or organizer. Empty means read-only. |
| `mcp.default_profile` | string | — | DefaultProfile is the deprecated former name of DefaultScope. It is still read so existing configs keep working; when both are set the narrower wins, because a key that silently stops applying must never widen what an agent may do. |
| `mcp.sync_scope` | string | — | SyncScope is orthogonal to the content scope. Empty/disabled is the default; status permits bounded operational reads and control additionally permits ordinary incremental and resource-fetch jobs. |
| `mcp.max_results` | int | `10` |  |
| `mcp.max_document_bytes` | int | `65536` |  |
| `search_sidecar.enabled` | bool | `false` |  |
| `search_sidecar.binary` | string | `recollindex` |  |
| `search_sidecar.index_dir` | string | `./data/search-index` |  |
| `remote_media.default_action` | string | `review` | DefaultAction applies to URLs matched by no domain list: allow, block, or review. Invalid configured values fall back to "review". |
| `remote_media.allow_private_networks` | bool | `false` |  |
| `remote_media.max_redirects` | int | `5` |  |
| `remote_media.fetch_timeout_seconds` | int | `30` |  |
| `remote_media.blocked_schemes` | []string | `file, data, javascript, ftp` |  |
| `remote_media.blocked_domains` | []string | — |  |
| `remote_media.allowed_domains` | []string | — |  |
| `remote_media.review_domains` | []string | — |  |
| `remote_media.max_bytes` | map[string]int64 | `map[image:20971520 pdf:104857600 video:209715200]` | MaxBytes caps download sizes per media class (image/video/pdf), parsed from human-readable values like "20MB". |
| `remote_media.quarantine_dir` | string | `./data/quarantine` | QuarantineDir holds fetched bytes before policy admission to the content-addressed asset store; it is never served. |
| `retention.unreferenced_resource_days` | int | `30` |  |
| `retention.purged_resource_days` | int | `90` |  |
| `retention.sync_history_days` | int | `90` |  |
| `retention.sync_peer_warning_days` | int | `30` |  |
<!-- notrios:generated:config:keys:end -->
