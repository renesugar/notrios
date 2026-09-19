# v1.0 J15: the published examples, generated or explained

J13 built the mechanism and used it on one page. This item took it to the rest
— and, where generating would earn nothing, recorded that decision instead of
leaving a reader to wonder why one page is different.

## The measurement that changed the design

172 published examples across 16 documents. **Not one of `docs/api/rest.md`'s
26 examples is a single request.** Every one is a shell recipe: a variable
captured from a previous response, a pipeline into `jq`, several requests that
only make sense in order. The "REST request" kind the plan sketched would have
reproduced none of them, and reshaping the page to fit would have been
migrating to raise a count, which the boundaries forbid.

So the kind added is a **recipe**, and what makes it earn its keep is not
re-typing commands in JSON. It is the declaration beside them.

## The recipe kind (J15-A)

A recipe's steps are the published text. What the tracked set adds is `uses`:
the CLI commands and REST routes that recipe drives, checked against the
surfaces that define them — `internal/clispec`, and the routes
`internal/httpapi` registers. A fence cannot fail when a command it documents
stops existing; this can.

The check runs **both ways**, which is the part that matters:

- a declared route the server does not register fails;
- a declared command that is not in the CLI spec fails;
- a declared route or command the recipe's own steps never run fails, because
  a declaration that drifts from the text beside it tells the reader this
  example shows something it does not show.

It reads a published path the way a reader wrote it: a value where the route
has a `{placeholder}`, a shell variable spliced in through quoting
(`.../resources/'$RES'/content`), a method from `-X`, a `GET` with no flag at
all.

Five tests in `cmd/docexamples/uses_test.go` cover each direction, the
placeholder and method reading, and that the surfaces come from the program —
if either list were empty the check would assert nothing, so that fails too.

## The migration (J15-B)

**106 of 172 examples are now generated, across 14 documents**, and **no
published byte changed**. That is the proof the migration is a declaration of
what was already there: `performance/v1.0-j15/migrate_page.py` reads a page,
writes the tracked set from what it finds, and wraps each fence in markers; the
generator then rewrites the page, and the only difference is the marker lines.

| document | generated | left |
|---|---|---|
| `docs/api/rest.md` | 26 | 0 |
| `docs/cli.md` | 17 | 39 |
| `docs/operations.md` | 17 | 3 |
| `docs/import-export.md` | 12 | 0 |
| `docs/installation.md` | 11 | 15 |
| `docs/configuration.md` | 6 | 0 |
| `docs/stable-links.md` | 5 | 0 |
| `docs/gui.md` | 4 | 2 |
| `docs/archive-v2.md`, `docs/publishing.md` | 2 each | 0, 1 |
| `docs/api/mcp.md`, `docs/query-language.md`, `docs/selection-planning.md`, `docs/troubleshooting.md` | 1 each | 2, 0, 0, 0 |
| `docs/index.md`, `docs/service.md` | 0 | 1, 3 |

**The 66 left are left for a reason, and the reason is recorded**, in
`docs/docexamples/DECISIONS.json`:

| reason | count | why |
|---|---|---|
| `synopsis` | 39 | a synopsis shows the *form* of a command; generating a form from a declaration of that form is a tautology |
| `no-surface` | 24 | names no CLI command and no REST route — `apt`, `make`, `sha256sum` — so a tracked entry would assert nothing the fence does not already say |
| `fragment` | 2 | not a command line |
| `transcript` | 1 | recorded output, not something a reader runs |

The record is derived, not typed: `record_decisions.py` reads J13's
classification and whether an example names a surface. **`cmd/docexamples`
refuses an example that is neither generated nor recorded as left**, and
`validate-scaffold.sh` keeps the record itself current. That is what makes this
a gate rather than a description somebody must remember to update.

`docs/service.md` and `docs/index.md` have no tracked set at all: every example
on them is a literal with no command or route, so a set would be an empty file
pretending to own a page.

## No hand edit survives (J15-C)

`TestAHandEditedFenceIsDetectedOnEveryGeneratedPage` edits a rendered fence on
**every** generated page and requires the page to stop matching its source —
the same comparison `go run ./cmd/docexamples` makes, so an edit fails the
build. The registry hashes move with the fences, by the tool that changes them.

## The troubleshooting rows, and why they went stale (J15-D)

Two rows quoted errors the importers had stopped producing:

| was | now |
|---|---|
| `no tweets.js/tweet.js found under ...`, with advice to use the extracted folder "not the ZIP" | `no tweets.js, tweets-partN.js or tweet.js found under` …, with the ZIP as the primary input, which is what J25 made it |
| `read ChatGPT export: ... no such file` | `is neither a ChatGPT export ZIP nor an extracted export folder`, plus a new row for Claude's own message |

Both had passed every gate for weeks, because nothing tied the quote to the
code. `scripts/check_documented_errors.py` now pins nine quoted messages to the
source that must contain them, and checks **both directions**: a message
reworded in the source fails here rather than in a user's terminal, and a quote
removed from the document fails too, so an entry cannot rot into a pin on
something nobody documents.

It caught a mistake immediately: I pinned the archive message to
`internal/archive/archive.go`, and it is in `read.go`.

## Validation

The whole Go suite passes through `scripts/check_temp_leaks.sh`, which left no
`notrios-*` entry. `validate-scaffold.sh` — now running both new checks — and
`make g18g-validate` pass.

Two evidence records moved with the pages: G18a's inventory and G18f's hashes
for the four documents whose markers changed. One G18f **test** changed: it
mutated "the first generated marker" on a page to prove a marker mismatch is
caught, and since a page can now also carry example markers, it targets the
slot's own marker instead.
