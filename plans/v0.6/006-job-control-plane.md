# v0.6 F6 — Job control plane for bulk work

Status: complete on 2026-08-08.

Model: Claude Opus 5 (Claude Code).

## What it does

Schema v18 adds a `jobs` table. The two batching importers and archive export
record a run into it; `notriosctl jobs list|status|show|cancel`, three REST
routes, and two MCP tools read and stop it. `GET /api/v1/jobs/{job_id}`, a stub
answering `"status": "unknown"` since the scaffold, is now real.

## The one place this is narrower than the plan asked

The plan bullet says *"MCP may start and watch a job"*. **Starting is not
offered — not over MCP and not over REST.** Every kind this build runs
(`import_joplin_raw`, `import_obsidian`, `export_archive_v2`) takes a filesystem
path, and the standing decision since v0.4, restated in `SECURITY_REVIEW.md` and
hardened in F3 and F5, is that no REST or MCP surface accepts an archive path or
streams archive bytes. **Putting a job record around an operation does not
change what the operation does**, so a `start` route would have reopened exactly
the boundary F3 recorded a reason for closing.

The same bullet's second half — "it may never carry archive or blob bytes; REST
and object transfer stay the data plane" — is the reason to read it this way. A
control plane that watches and stops is what remains once starting is off, and
that is what F6 built.

Cancelling is also withheld from MCP, separately: it is safe for the data, but
stopping a four-hour import a person started is not a model's decision. REST and
the CLI both offer it.

## Decisions worth recording

**`interrupted` is derived, never stored.** A process that dies cannot write its
own epitaph, so a job left `running` with a heartbeat older than two minutes is
reported as interrupted at read time. A sweeper that *wrote* the state would
have to decide another process is dead — and two Notrios processes against one
database, `notriosd` serving the GUI while `notriosctl` imports, would take
turns declaring each other's work over. Deriving it also means a job that was
merely slow can come back and finish normally, which a test asserts.

**Records persist across a restart; the work does not.** There is no resume
column, because an interrupted import already resumes through its own
`import_checkpoints` row. Two resume mechanisms would give two answers to one
question. **Verified on real data:** a 4,000-note import cancelled from another
process stopped at 1,225 notes, and rerunning the same command started at 1,250
and finished all 4,000.

**Cancellation is cooperative, and the existing seam was already the right
one.** `AfterBatch` runs *after* a batch is committed and its checkpoint saved,
and aborting it aborts the import — so progress reporting and the stop signal
share one hook, and stopping leaves a state the next run resumes from. Killing
the work mid-batch would leave the same durable state and a worse story about
it.

Archive export has no such boundary, so it gets the other mechanism: the runner
owns a context, cancelling it aborts at the next store read, and because the
manifest is written last and *is* the completion marker, a stopped export leaves
nothing that could pass as a complete archive. **Both routes classify
identically** — a cancelled run is `cancelled`, never `failed` — because they
are the same event arriving two ways, and an operator should not be sent hunting
for a fault that does not exist. Mutation-verified.

**Parameters are stored; the command is rendered.** Storing raw argv would have
captured local paths and any secret that happened to be on the command line into
the database, and a stored string cannot improve when a flag is renamed. Each
parameter carries a `path` flag, which is what lets `notriosctl jobs show
--command` print a runnable command locally while REST and MCP return no
parameters at all. Tested against `/tmp/it's here; rm -rf /`, because a rendered
command exists to be pasted into a shell.

**No scheduler, and the exit codes are why.** `notriosctl jobs status` exits 0
succeeded, 1 failed, 2 usage, 3 running, 4 cancelled, 5 no such job, 6
interrupted; `--wait` blocks until settled. That is enough for `job-a && job-b`.
**6 is deliberately not 1**: an interruption usually just needs the command run
again, which is not the response to a failure. A test asserts no state collides
with 2, since 2 is usage everywhere else in this CLI.

**Ctrl-C tells the truth.** Without a handler it would leave a record saying
`running` for two minutes and then only `interrupted`. The first signal requests
cancellation, so the record settles as `cancelled` at the next checkpoint; a
second signal still kills the process, which is what pressing it twice means.

**The MCP view drops two fields REST keeps**, both because their values
routinely contain a local filesystem path: the parameters, and the free-text
error — a filesystem failure reads like `open /home/someone/private/x:
permission denied`. The state is still reported, with a pointer to
`notriosctl jobs show <id>`. A summary crosses, because it is counts by
construction.

**Dry runs record nothing.** They change nothing and finish quickly; a control
plane full of records for runs that did nothing is harder to read for no gain.

## Two defects found while building

**Listing was not actually newest-first.** `CURRENT_TIMESTAMP` has one-second
resolution, so jobs started in quick succession share a timestamp and the
tiebreak fell to the random job ID — an arbitrary order that looked like
chronology. Listing now orders by `rowid`, which *is* insertion order. Caught by
the listing test.

**`jobs status <id> --wait` exited 2 with a bare usage dump.** Go's flag package
stops parsing at the first positional, which is standard and matches every other
command here — but this is the one command written to be driven from a shell,
and it is the obvious thing to type. The refusal now names the flag and says it
must come before the ID.

## Validation

`go vet ./...`, `go test ./...`, `make validate`,
`python3 scripts/check_required_files.py`, `python3 scripts/check_plan_loops.py`,
OpenAPI parse (63 paths — and a **duplicate `JobStatus` schema** was found and
removed, where the stale placeholder definition was silently overriding the new
one), `npx tsc --noEmit`, 155 web tests, `npm run build`, `make gui`,
`bash scripts/build_docs_site.sh`, `notriosctl seed-help`,
`bash scripts/mvp_smoke.sh`, `bash scripts/run_performance_smoke.sh`,
`bash scripts/run_offline_assets_check.sh`.

End to end against a 4,000-note vault: watched from a second process, cancelled
mid-run, resumed by rerunning; hard-killed and shown as `interrupted` once the
heartbeat went stale; read over REST and MCP with the vault path absent from
both.
