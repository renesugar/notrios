# v0.6 F4 — Note templates and task extraction

Status: complete on 2026-08-07.

Model: Claude Opus 5 (Claude Code).

## What it does

A template is an ordinary note carrying a fenced ```` ```note-template ````
block; `POST /api/v1/templates/{id}/create` fills it in. A task is a checkbox
list item, extracted on read by `GET /api/v1/tasks`. Both reach MCP —
`list_templates` and `list_tasks` under `read-only`, `create_from_template`
under `editor`.

## Decisions worth recording

**The closed vocabulary is the whole safety argument.** A placeholder name is
either a declared `prompt:` or one of `date`, `time`, `datetime`, `title`,
`notebook`. Anything else is an error naming the alternatives. An *open*
vocabulary is an expression language, and an expression language living in note
content is exactly what E7 spent a slice refusing. The placeholder pattern
allows only letters, digits and underscore, so a placeholder cannot look like a
call or a path even by accident.

**Substitution is one pass, and that is a security property rather than an
optimisation.** A supplied value containing `{{date}}` is inserted literally and
never re-scanned, so a value cannot introduce a placeholder, cannot reach an
automatic name it was not given, and cannot recurse. Everything a caller
supplies is data. A test asserts it directly.

**An unknown placeholder is an error on the *template*, reported at listing
time.** A typo like `{{onwer}}` is a broken template, and discovering it when
someone tries to use it is too late. Listing a malformed template with its error
rather than hiding it follows the same reasoning: the author is the person who
needs to find it.

**A missing value is refused; so is an undeclared one.** Producing a note with a
blank where a value belonged is worse than refusing, and silently ignoring a
value the caller thought mattered hides a caller bug.

**An automatic name cannot be prompted for.** Two sources for one name would
make which wins a coin toss.

**Tasks are computed on read, and the reason is concrete.** `document_blocks`
stores a content hash and byte offsets but **not** block text, so there is
nothing to query for "is this item checked" — the body has to be read either
way. That makes extraction a whole-library scan of the same shape and cost as
lint, which is what the decision anticipated: no table until a query needs one a
scan cannot serve. A `LIKE` prefilter skips notes that cannot contain a checkbox
before any body is parsed, and `document_id`/`notebook_id` turn the scan into a
small one for the common case of showing a note's own tasks.

**Counts are complete even when rows are capped**, the same contract lint's
examples have. A capped list that also capped its counts would understate the
library.

## The identity property, and its honest limit

A task is addressed by the v0.5 block model rather than by a line number, so it
keeps its identity when the note is edited *around* it — a test inserts a
paragraph above and below and asserts the block ID is unchanged while the
ordinal moves, so the test would fail if the task had not actually moved.

But a block ID is derived from the block's text, and **the checkbox is part of
that text**: `- [ ] buy milk` reaches the block parser as `[ ] buy milk`. So
ticking a task changes its derived ID. That is the block model working as
documented, not a defect, and the answer is the one the block model already
has — an author-written `^marker` outranks the derived ID and survives the
rewrite.

A test asserts *both halves*: that ticking changes the derived ID, and that the
marker and the anchor URI do not. The first assertion is deliberately worded to
fail loudly if the block model's contract ever changes, because a silent change
there would quietly alter what every task link means.

## Validation

`go vet ./...`, `go test ./...`, required-file, plan-loop, and scaffold checks,
OpenAPI parse, migration-copy equality, web typecheck/tests/build, `make gui`,
docs-site build, Help reseed, REST/MCP smoke, performance smoke, and the
offline-assets check.

No schema change and no migration: templates are notes, and tasks are computed.

No scale profile of its own. Task extraction is a whole-library body read of the
same shape as lint, whose 100k/500k evidence stands in for it, and the prefilter
strictly reduces that work. If extraction ever proves too slow at scale, *that*
is the measurement that justifies a table — which is precisely the condition the
decision named.
