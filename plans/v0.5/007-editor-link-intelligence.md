# v0.5 E5 — Editor-pane link intelligence

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## Why

Two questions an editor has to answer while someone types, and neither had an
answer: *which note could this link name*, and *will the links I have written so
far actually open*. The second matters most before a save, which is exactly when
the link records that would answer it do not exist yet.

## What it does

| Surface | Answers |
|---|---|
| `GET /api/v1/links/suggest` | bounded link-target autocomplete by title |
| `POST /api/v1/links/check` | resolution of every link in an unsaved buffer |

Both are read-only. Neither writes a revision and neither returns a note body.
The web client adds a link picker that inserts a canonical URI at the caret and
a located list of links that will not open.

## Decisions worth recording

**The buffer check takes the body, not a list of targets.** Deciding what counts
as a Markdown link belongs to the canonical extractor. Asking a client to
reimplement it in TypeScript would give an editor markers that disagree with the
link records a save actually writes — worse than no markers, because they would
be confidently wrong. A test asserts the check and the save agree link for link,
status for status, byte for byte.

**Anchors into the note being edited resolve against the submitted body.** While
someone is typing, the buffer is the truth about its own headings. Checking a
just-typed `#new-section` against yesterday's saved blocks would mark a correct
link broken, which is the one failure mode that would make the feature
untrustworthy.

**Schema v16 makes title lookup an index probe.** This started as an
implementation detail and turned out to be the slice's most valuable change.
Resolving a link by title ran `lower(title) = lower(?)`, which no index can
serve — so every link naming a note by title cost a **full scan of the document
table**, once per link, on every save and every lint pass. Moving the comparison
to the NOCASE collation is the same comparison (SQLite's `lower()` folds ASCII
only, exactly as NOCASE does) and makes it a probe. The same index makes
`title LIKE 'prefix%'` a range scan, which is what bounds the suggestion
endpoint, and including `id` in it removes the tie-break sort as well.

**Two suggestion passes, in cost order.** The title-prefix range scan runs first
and is genuinely bounded — it starts at the first match, walks in title order,
and stops one row past the page. Only if that did not fill the page does a
bounded FTS5 pass look for interior words, so "plan" finds "Kitchen Plan". The
second pass is capped at 200 candidates, and the honest consequence is recorded
rather than hidden: for a very common word the interior matches shown are the
first the index yields, not the best. The first pass has no such limitation,
which is why it runs first and ranks above.

**A two-character minimum.** One letter matches so much of a large library that
ranking it means reading the library, and no useful suggestion comes out of it.

**A suggestion is an ID and a title.** Not a snippet, not a body. An autocomplete
dropdown is not a place note content should arrive.

**A typed `%` is text.** Without escaping, `%` in the query would match every
note — a wrong answer rather than an unsafe one, but wrong is enough.

**Failure degrades to nothing.** Both client hooks debounce, cancel the request
they superseded, and on error clear their state rather than raising anything. An
assist that interrupts typing with an error banner is worse than an assist that
is quietly absent. The buffer check also tracks a generation counter, so a slow
reply about older text cannot overwrite a newer result.

**"All 4 links resolve" is said, not implied.** A clean check renders a
sentence; an unfinished or failed check renders nothing at all. Silence must not
be readable as "no problems".

## The gap this slice hands to E6

Half of each feature landed where it was wanted. `md-editor-rt` exposes `insert`
at the caret, so a chosen suggestion goes in where the author is typing. It
exposes nothing about *where* the caret is and accepts no inline widgets, so a
broken link is a located list beside the text rather than an underline inside it.

That is E6's input, stated as a measured limitation rather than a preference:
**in-editor marker placement and caret position are the two things the current
editor cannot give.** Recorded in `UI_DESIGN.md` beside the migration note.

## One latent defect found

`resolveLinkCandidateLocked` treated an anchor-only link (`[x](#section)`) as
resolved even when it had no source document, producing a link whose target URI
was `document://default/documents/` — an ID-less URI naming nothing. It was
unreachable from the save path, which always has a document ID, and became
reachable the moment a caller could check a buffer that has never been saved.
Fixed where it was wrong rather than worked around in the caller.

## Evidence

`performance/v0.5-e5/`, generated at 10k/100k/500k by
`scripts/run_large_library_profile.sh`. Both operations run on a keystroke, so
flatness against library size is the requirement rather than a nicety.

| Tier | Suggest (p95) | Buffer check (p95) | Suggest unindexed | Check unindexed |
|---|---:|---:|---:|---:|
| 10k | 0.51 ms | 1.40 ms | 41.6 ms | 19.5 ms |
| 100k | 1.03 ms | 8.30 ms | 1,324 ms | 1,000 ms |
| 500k | 0.44 ms | 1.40 ms | 5,390 ms | 4,198 ms |

The profile measures the index's value rather than asserting it: each tier drops
`documents_title_idx`, re-measures, and restores it. Indexed is flat across a
fifty-fold library; unindexed is linear, reaching **5.4 seconds per keystroke**
at half a million notes. That cost was already being paid on every save and every
lint pass by any link naming a note by title.

One measurement is honestly not flat and is recorded as such: a query matching
nothing runs both passes to exhaustion and grows 0.62 → 5.83 ms across the
tiers, because an FTS5 prefix lookup walks a term dictionary that grows with the
library. It stays well inside a keystroke; if it ever matters, the fix is an
FTS5 `prefix=` index rather than a change to the two-pass design.

## Validation

`go vet ./...`, `go test ./...`, required-files, plan-loop, scaffold validation,
OpenAPI parse, migration-copy equality, the v15→v16 upgrade fixture, web
typecheck/tests/build, `make gui`, docs-site build, Help reseed, REST/MCP smoke,
performance smoke, and the three generated profiles.
