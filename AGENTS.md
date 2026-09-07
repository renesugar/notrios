# AGENTS.md

This file provides project instructions for all coding agents (Codex, Claude, aider, swival.dev, etc.). AGENTS.md is intended as a predictable place for agent guidance, similar to a README for agents. `CLAUDE.md` points here.

## Prime directive

Leave the repository in a working state after every task. If interrupted, the next agent should be able to resume using only repository files.

## Before coding

Start with the handoff: `CODING_CLIENT_HANDOFF.md` is the current compressed state of the project and the recommended next task.


1. Read `CODING_CLIENT_HANDOFF.md`, `README.md`, `PLAN.md`, `ROADMAP.md`, `SYSTEM_ARCHITECTURE.md`, `SYNCHRONIZATION.md`, `FLUTTER_GO_CLIENT.md`, `API_SPEC.md`, `DATABASE_SCHEMA.md`, `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `RECOLL_INTEGRATION.md`, `CODING_STANDARDS.md`, `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`, and `CONTEXT_MAP.md`.
2. Read `agent/PLAN_STATUS.md`, `agent/ATTEMPT_LOG.jsonl`, and `agent/MODEL_LOG.jsonl`.
3. Identify the next incomplete task in `PLAN.md`.
4. If the task is ambiguous, write the question into `agent/OPEN_QUESTIONS.md` and ask the user before implementing.

## During each task

- Work in the smallest coherent slice that can be validated.
- Add or update tests with the change.
- Run the relevant validation commands.
- At the start of a regular validation pass, audit the bundled frontend with
  `cd web && npm audit`. Apply available compatible fixes with `npm audit fix`,
  review the lockfile, then use `npm ci` and rerun the audit before testing. Do
  not use `--force` without a separately approved dependency upgrade.
- Do not rewrite large areas without preserving working behavior.
- Do not introduce raw SQL or unrestricted filesystem access in MCP tools.
- Do not make Recoll (or any search sidecar) the canonical document store; Recoll is a derived, optional extraction/search sidecar (see `RECOLL_INTEGRATION.md`).
- Keep all project code and dependencies compatible with an MIT or Apache-2.0 license. GPL tools such as Recoll/Xapian may only be invoked as external processes — never linked, vendored, or redistributed, and never used as a source to derive code from.
- Do not put secrets, medical data, private user exports, or proprietary datasets in the repository.


## Writing plan items

A plan item is read on its own, by whoever is about to approve or implement it.
Anything it needs must be *in* it — a reader does not scroll two hundred lines
to a milestone-level section to discover that the task has an unanswered
question. (This rule exists because that is exactly what happened to v0.6 F1:
its two open decisions sat in a "Decisions required" section far below the item,
and the task was approved without them being seen.)

Every item states its goal, its scope boundaries, and its working state. Beyond
that:

**Declare the item's work as slices, in `docs/docplan/PLAN_SLICES.json`.** A
slice is one implementable piece: small enough to finish in a sitting, large
enough to be worth naming, and stated as an outcome rather than an activity
("the command line can delete and restore a note", not "work on deletion").
Give each one an id prefixed by the item (`H15-C`), a statement, and a state.

Declare them **before** the work, and keep them current as it proceeds. Items
have been written up in slices after the fact -- H4, H9 and H14 all did -- and a
slice named once it is finished is a changelog entry, not a plan: it tells the
next reader what happened and nothing about what is left. That is the failure
this file exists to prevent, so the ledger is checked rather than trusted (see
"Keeping the plan current").

**An item with an unresolved decision carries an `Open decisions` subsection,
inside the item.** List each decision, what the options are, which one you
recommend and why, and what changes depending on the answer. Say plainly whether
it blocks the work:

- *Blocking* — the item cannot be designed without an answer. Say so, and do not
  start until it is answered.
- *Non-blocking* — you will take a defensible default if no answer comes. Name
  the default in the plan, so approving the item is also approving the default.
  Restate it in the completion report.

Never leave a decision implicit because you intend to make it yourself. A choice
recorded before the work is a decision; the same choice explained afterwards is
a justification, and the reader has lost the chance to disagree cheaply.

**An item whose *approach* is uncertain gets an investigation slice before it.**
The distinction is between not knowing what to build (a decision — write it
down and ask) and not knowing how, or whether the premise holds (an
investigation — go and find out). Signals that a spike belongs first:

- two or more approaches with materially different cost, and no way to choose on
  paper;
- a premise that has not been checked against the code or a dependency;
- a claim about performance, size, or capability that nobody has measured.

A spike is a real slice: it is numbered, archived, and produces evidence and a
recommendation rather than a feature. v0.5 E6 is the model — framed as "migrate
to CodeMirror", it found the premise false (the editor already *is* CodeMirror
6) and returned a decision instead of a migration. That outcome was worth more
than the migration would have been, and it was only reachable because the
investigation was a task rather than an assumption inside one.

Prefer a spike over a long item with a fork in the middle. An item that says
"depending on what we find, do A or B" is two items.

**Milestone-level decision sections are a summary, never the only home.** A
decision that affects one item lives in that item; the milestone list may
reference it. When a decision is resolved, say so where it was asked and mark it
resolved rather than deleting it — the reasoning is the useful part.

**A completed item is marked in two places, in one spelling.** Append
`— complete` to the item's `##` heading, and close the item with a paragraph
that begins `**Outcome (YYYY-MM-DD).**` and names its archive under `plans/`.

This rule exists because it was broken. v0.7 accumulated four spellings of the
closing paragraph — `Completion`, `Completion evidence`, `Completed`, and
`Outcome` — and three completed items (G3, G5, G6) had the paragraph but lost
the heading marker, so the plan appeared to show unfinished work that was
finished, archived, and logged. Nothing reads the marker mechanically, which is
exactly why it drifts: only a reader notices, and only later.

Questions with no owning item, or that outlive a milestone, belong in
`agent/OPEN_QUESTIONS.md`.

## Handoff and environment limitations

The scaffold was created in a restricted container. Before large implementation work, read `CODING_CLIENT_HANDOFF.md` and review the documented limitations. In particular:

- the current SQLite adapter is a small local cgo wrapper chosen to avoid external Go module downloads during scaffold creation;
- the Go module path is `github.com/renesugar/notrios` and the license is Apache-2.0 (both set in plan task R2);
- Recoll, Quartz, MCP SDK, and browser automation are documented but not integrated;
- real private import datasets were intentionally not included;
- UI validation has been build/typecheck level only, not Playwright-level browser testing.

In a less restricted environment, consider resolving the SQLite driver, MCP SDK pin, and browser test stack before deep feature work. Do not make these changes all at once unless the active plan calls for a cleanup slice.

## Progress and loop detection

For every attempt, append one JSON object to `agent/ATTEMPT_LOG.jsonl`:

```json
{"timestamp":"2026-07-08T00:00:00Z","task_id":"mvp-01","task":"Service persistence foundation","model":"unknown","status":"started","notes":"..."}
```

On completion, append another object with `status: "completed"` or `status: "blocked"`.

If a task has three blocked attempts with no meaningful file changes or no passing validation improvement:

1. Stop broad attempts.
2. Split the task into smaller subtasks in `PLAN.md`.
3. Add a loop-detection note to `agent/LOOP_DETECTION.md`.
4. Ask the user to review the obstacle.

## Model tracking

At the start of a session, record the model if known in `agent/MODEL_LOG.jsonl`. If the model appears downgraded or performance changes abruptly, note it. Do not assume the model is stable across sessions.

## Agent usage preflight

At the start of a session that may run long validations or model-backed
subtasks, verify the installed-client parser before relying on it:

```bash
python3 scripts/test_check_agent_usage.py
python3 scripts/check_agent_usage.py --agent all --minimum-remaining 20
```

Compare the reported client version and fields with the current fixtures. If a
Codex or Claude Code update makes the result `unknown`, update the probe and
tests before enabling strict mode; never interpret missing telemetry as 100%.
Run the preflight before a long agent-managed phase and pause only at a durable
checkpoint. If it requests a pause, update the handoff and attempt log with the
exact resume command before stopping. See `skills/agent-usage-preflight/SKILL.md`.

The Codex probe must remain model-free (`account/rateLimits/read` through the
local app-server), never `codex exec /status`. The Claude probe remains
local-cache-only and must not invoke `claude -p` merely to estimate quota.

The Claude side needs one machine-local setup step, because Claude Code keeps
rate limits in memory and only hands them to the configured `statusLine`
command. Install `scripts/claude_statusline_usage.py` as that command in
`~/.claude/settings.json`; it records `~/.claude/usage-cache.json` for the
probe to read. Without it the probe reports `unknown` and cannot gate a long
run. A cache older than `--claude-max-age-minutes` (default 30), or one whose
windows are all past `resets_at`, reports `stale` and is treated like `unknown`
rather than trusted.

## Keeping the plan current

`PLAN.md` is a journal: it records what happened, in the order it happened. The
question asked of a plan is *what is left*, and prose cannot be relied on to
answer it -- this plan reached four thousand lines in nine different status
notations, two thirds of the open items' text describing finished work, and
answering "what is unfinished?" meant reading all of it and then checking the
answer against the code. So what is left is declared as data and checked.

When you start a slice, set it `in-progress`. When you finish one:

1. set it `done` and name evidence that resolves -- `go:<import path>#Symbol`,
   `file:<path>`, or `test:<TestName>`;
2. run `go run ./cmd/docplan --write` to regenerate the progress log in
   `PLAN.md`, and commit that with the work;
3. update the item's own narrative with what happened and why, which is a
   different question from what is left. Do not let one become the other.

The rules the ledger enforces, and `go test ./...` fails on:

- every plan item has a ledger entry and every entry a plan item;
- a slice marked done names evidence that exists; unfinished work names none,
  because evidence on unfinished work is how a plan starts describing
  intentions as achievements;
- a blocked slice says what would unblock it -- otherwise it is "not started"
  with a better excuse;
- an item may not call itself complete with slices outstanding, in progress
  with none, or unstarted with work done. The heading and the ledger must agree
  about whether an item is finished; the first run of this check found four
  items whose outcome said complete while their heading did not.

## Keeping the roadmap current

`ROADMAP.md` is the feature inventory and the planning source: it says what a
version *means*, and `PLAN.md` says how far along the active one is. Keep that
line, because the roadmap has crossed it and rotted -- it claimed "H0 is
complete; H1 is the next separately approval-gated item" for a month after
fourteen items had finished.

- **The roadmap does not carry per-item status for the active plan.** The one
  status sentence it needs is generated from the same ledger as the plan's
  progress log, between the markers in its "Agent handoff status" section, and
  `go run ./cmd/docplan --write` writes both.
- **A version's own heading is where its completion is recorded**, with a
  completion note reconciling every bullet against the code -- as v0.5, v0.6 and
  v0.7 do. A bullet that shipped only half says so in the bullet rather than
  being marked done.
- **Scope that moves between versions says where it came from**, in the version
  it moved to: "Moved here from v0.8 (H9) because ...". A bullet that simply
  disappears from one version and appears in another loses the reason, which is
  the only part worth keeping.
- **New work is added to the roadmap when it is added to the plan**, not
  afterwards. The two disagreeing is how a plan item comes to exist that no
  version claims.

`ROADMAP.md` points back here for these rules, and `internal/docplan` fails when
it does not -- for the same reason the plan does.

## Plan archival

Implemented plans must be archived under:

```text
plans/v<major>.<minor>/<NNN>-<slug>.md
```

Use the active product version/milestone. Include validation evidence and the models used.

**Archive an item when the record of executing it has outgrown the plan.**
`PLAN.md` keeps the item's specification and its outcome and links to the
archive; the archive keeps the working record **verbatim**, because a summary of
a finding is not the finding. The test is length and kind rather than age: an
item that is a specification plus a paragraph of outcome reads fine where it is,
while one that has accumulated slice reports, corrections and findings is
describing finished work in a document somebody reads to learn what is left.
H14 was nine hundred lines of it. Say in the plan that the item is archived and
where, or the record is lost to the next reader -- H0 and H1 were archived and
unlinked for a month.

After the current `PLAN.md` completes, create a new `PLAN.md` from `ROADMAP.md`
and ask the user before starting it. A new plan needs a `## Progress` section
that carries the generated block **and points back here**, because these rules
outlive the plan and the plan's own copy of them does not:

```markdown
## Progress

Generated from `docs/docplan/PLAN_SLICES.json` by
`go run ./cmd/docplan --write`, and checked by `internal/docplan`.

**The rules for keeping it current are in [`AGENTS.md`](AGENTS.md)** — under
"Writing plan items", "Keeping the plan current" and "Plan archival".

<!-- notrios:generated:plan:progress:begin -->
<!-- notrios:generated:plan:progress:end -->
```

It also needs a fresh `docs/docplan/PLAN_SLICES.json` seeded with its items;
archive the finished ledger alongside the plan it belonged to. The pointer is
checked rather than trusted -- `internal/docplan` fails when the progress
section does not name `AGENTS.md` -- because an instruction that only exists in
the document it is about disappears with it, which is how this rule came to be
missing in the first place.

## Git workflow

- Work on `develop` until MVP is ready.
- Use small commits aligned with completed working-state slices.
- Do not commit generated build artifacts, private data, or local databases.
- Before committing, run `go test ./...` and any task-specific tests.

## Skills

Reusable workflows live under `skills/<skill-name>/SKILL.md`. If you discover a repeatable process, add or update a skill after the task succeeds. Keep skills short and progressively disclosed.

## Security constraints

- Treat imported notes and downloaded resources as untrusted.
- Remote-media localization must go through domain policy, quarantine, exact hashes, perceptual-hash policy hooks, MIME sniffing, size limits, and SSRF protections.
- Preview HTML must be sanitized.
- MCP tools should default to read-only and context-limited outputs.

## User interaction

When a plan step completes, produce a ZIP snapshot and ask whether to proceed to the next step.

## Feature triage

When a requested feature is not already in the active `PLAN.md`, consult `FEATURE_MATRIX.md` and `ROADMAP.md` before coding. If the feature is not MVP, document it or create a small planning slice; do not silently expand the MVP.

Use `prompts/review_feature_against_roadmap.md` for features that may alter milestone scope.

## Release packaging rule

When producing a ZIP for user handoff or repository import, use `scripts/package_release.sh` and verify with `scripts/check_release_zip.py`. Do not hand off a ZIP that lacks `web/dist/`, and do not include `web/node_modules/`, runtime `data/`, `.git/`, or SQLite database files. See `skills/release-packaging/SKILL.md`.
