# v0.7 G3 — Profiles, replica identity, and multi-instance isolation

Status: complete on 2026-08-12.

Model: GPT-5 (exact variant not exposed).

## What shipped

The existing owner-only stable-link registry is now format version 2 and can
hold two deliberately different kinds of entry:

- a compatibility routing-only entry made by `profile register`, which keeps
  `notrios://` ambiguity visible but is not startable; and
- a runtime profile made by `profile create`, with a random local `profile_id`,
  one bound `database_id`/`replica_id`, and one generated `0600` config.

Generated configs resolve absolute database, asset, projection, search-index,
quarantine, and config paths; a loopback listen address and public URL; and the
local `none|directory|rest` sync target plus an optional credential-store
reference. `none` is the default and cannot carry dormant transport or
credential fields. No sync engine or credential-store implementation landed.

`notriosctl profile create|show|list|validate|start` supplies the G3 lifecycle.
`show` reports only whether a credential reference exists. `start` validates
and launches a foreground `notriosd -config <path>` command, with no credential
or credential reference in argv; it is not a supervisor. The existing
`register|forget` and stable-link routing behavior remains compatible.

The service revalidates a generated config against its registry entry and the
database identity before opening canonical state. Validation refuses exact or
nested runtime-path sharing, duplicate loopback ports, stale/missing config or
database files, changed identity bindings, and duplicate replica IDs. An
unrotated filesystem copy is refused. `--copied-database-as adopt` preserves
the logical database ID and rotates the replica; `fork` mints both. The option
is accepted only when an already registered duplicate proves that the action is
actually operating on a copy. Archive-v2 adopt/fork restores already satisfy
the same identity rule.

`GET /api/v1/status` now reports the active local profile name and ID, and the
built-in UI includes the profile name in its status line. Replica identity is
still absent from REST because it has no stable-link meaning. Stable-link open
now uses a runtime profile's public URL rather than the default service URL.

No schema migration, sync journal, transport, MCP surface, filesystem scan,
external dependency, or non-loopback authentication claim was added. Product
version remains 0.6.0 and schema remains v18.

## Validation evidence

Focused fixtures cover fresh identities; owner-only registry/config files and
redaction; path/port/replica collisions; raw-copy refusal plus explicit adopt
and fork; stable-link ambiguity across valid replicas; and stale config,
database, and identity bindings. A real CLI fixture builds `notriosctl` and
`notriosd`, starts two profiles simultaneously on different loopback
ports/databases, checks their active-profile status, refuses a third port
collision, and routes a stable link through the selected profile URL.

Repository validation passed:

- `GOCACHE=/tmp/notrios-g3-gocache go test ./...`
- `GOCACHE=/tmp/notrios-g3-gocache go vet ./...`
- `python3 scripts/check_required_files.py` (65 files)
- `python3 scripts/check_plan_loops.py`
- `GOCACHE=/tmp/notrios-g3-gocache bash scripts/validate-scaffold.sh`
- `cd web && npm run typecheck`
- `cd web && npm test -- --run` (15 files, 155 tests)
- `cd web && npm run build`
- `bash scripts/build_docs_site.sh` (15 pages)
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
- `git diff --check`

The first combined frontend invocation was run from the repository root and
correctly reported that root `package.json` has no frontend scripts. It was an
invocation error; all three commands were rerun from `web/` and passed.

G4 is next and remains unapproved.
