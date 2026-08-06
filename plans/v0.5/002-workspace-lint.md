# v0.5 E2 — Workspace lint

Status: complete on 2026-08-05.

Model: Claude Opus 5 (Claude Code).

## Why

`WORKSPACE_MAINTENANCE.md` has listed these checks since the scaffold, and
nothing implemented them. A library accumulates broken links, orphaned
attachments, and remote images that hit the network on every preview, and until
now the only way to find any of it was to open notes.

Read-only first, exactly as resource garbage collection was: a report that
cannot mutate is safe to run on a library nobody has backed up, which is
precisely when someone wants to know what is broken. Fixing is E3.

## What it does

Ten checks: `broken_document_link`, `broken_resource_link`, `ambiguous_link`,
`unresolved_block_anchor`, `duplicate_source_id`, `missing_title`,
`unlocalized_remote_media`, `missing_alt_text`, `unreferenced_resource`, and
`projection_backlog`. Exposed as `notriosctl lint` and
`GET /api/v1/admin/lint/report`, with no apply endpoint anywhere.

`unresolved_block_anchor` is the check E1 made possible: before block rows
existed there was nothing to compare an anchor against.

## Decisions worth recording

**Findings locate; they do not quote.** A finding carries a document or resource
ID, a line and column, a stable reason code, and a SHA-256 fingerprint of the
offending target. The raw target stays out, because a broken wikilink's text is
frequently the title of a private note — the same boundary P1 drew for the
selection planner. A line number is enough to find the problem in the note.

**The library is read once.** The six checks whose findings come from
`document_links` share a single ordered scan rather than scanning that table
six times. See the measurement below — this was not a premature optimization but
a response to a number.

**The cap hides examples, never counts.** Every check streams its rows: each is
counted and folded into the digest, then discarded unless it is one of the first
`detail_limit` examples. So counts and `report_sha256` describe the whole
library at any cap, memory stays flat, and a changed digest means the library
actually changed.

**The exit code is the answer.** `notriosctl lint` exits 0 clean and 1 with
findings, so `--quiet` works in a hook or a script without parsing JSON.

**One check is deliberately absent.** Heading anchors (`#section-title`) cannot
be verified: a heading anchor is a slug, and block rows store a content hash
rather than heading text (E1's privacy-driven row shape), so there is nothing to
compare against. Reporting every heading anchor as unresolved, or none of them,
would both be wrong. Checking them needs a stored heading slug, which is a
schema question for its own slice — recorded rather than guessed at.

## Two things the tests corrected

1. **`![alt](x)` is recorded as an `embed`, not an `image`.** The alt-text check
   filtered on `relation_type = 'image'` and therefore found nothing. It now
   accepts both names, so a future relation rename cannot silently empty the
   check again.
2. **`CreateDocument` substitutes "Untitled" for a blank title**, so the
   missing-title state cannot be produced through the ordinary API at all. It
   arises from an importer or a restore whose title-derivation pass did not
   complete — which is exactly what the check exists for — so the fixture now
   produces it that way instead of pretending the API can.

## The measurement that changed the implementation

The first implementation ran each check as its own statement and measured
**61.3 s at 500,000 notes**. Per-check timings — added to find this, and kept
because "which check is expensive" is something an operator wants — showed six
checks each scanning `document_links` separately, about 10 of the 12 seconds at
100k. The indexed checks were already free; the pass simply read the largest
table six times.

The six link checks now share one ordered scan that classifies each row into
whichever checks it violates, preserving every check's semantics including rows
that violate more than one:

| Tier | Six scans | One scan | Speedup |
|---|---:|---:|---:|
| 100k | 10.9 s | 3.2 s | 3.4× |
| 500k | 61.3 s | 18.9 s | 3.2× |

Every lint test passed unchanged across the rewrite, which is the signal that
matters: identical behaviour, one pass instead of six.

## Measurements

`performance/v0.5-e2/`, at 10k/100k/500k. Full lint: 0.34 s / 3.20 s / 18.9 s,
peak RSS 26 / 87 / 376 MB — flat against the same tier without lint, since only
the capped examples are held. Cost is linear in links. That is a maintenance
command's cost rather than an interactive one, and the documentation says so.

## Validation

- store fixtures: one instance of every problem plus healthy content that must
  not be reported; content-free findings; determinism and zero revisions
  written; the cap hiding examples without changing counts or the digest; check
  selection and limit validation; trashed notes excluded;
- REST fixtures: the report, input validation, no content in the response, and
  no write method routed;
- CLI fixture building the real binary: exit 0 clean, exit 1 with findings,
  quiet mode silent, check selection narrowing the result, `--list-checks`;
- `go vet ./...`, `go test ./...`, required-file, scaffold, OpenAPI parse,
  migration-copy equality, docs site, `npm run typecheck`/`test`/`build`,
  `make gui`, `scripts/mvp_smoke.sh`, `scripts/run_performance_smoke.sh`;
- generated scale profiles under `performance/v0.5-e2/`.

## Next task

E3, workspace fix, requires user approval. Per `PROJECT_DECISIONS.md` 18 it
stays single-note and revision-preconditioned; anything bulk belongs to the v0.6
organizer.
