# v0.5 E3 — Workspace fix

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## Why

E2 reports what has rotted and cannot touch it. Most of what it finds needs a
human — a broken link needs a target, an ambiguous wikilink needs a choice — but
a small subset is mechanical, and doing that by hand across a library is exactly
the work a tool should absorb.

## What it does

`notriosctl fix`, dry-run by default:

| Kind | Repair | Default |
|---|---|---|
| `non_canonical_link_target` | a link that resolved by title or filename becomes the canonical `document://`/`resource://` URI it already points at | on |
| `missing_alt_text` | an image's empty alt text is filled from the resource filename | off |
| `unlocalized_remote_media` | the image is downloaded through the media policy and the note rewritten to `resource://` | off |

## Decisions worth recording

**Dry run is the default and shows both sides.** The plan carries the exact
`before` and `after` bytes for every edit. A fix that cannot be read before it
runs is a fix nobody should run on their own notes.

**Each note is repaired against the revision its plan was computed from**
(`PROJECT_DECISIONS.md` 18). A note edited in between fails and the others still
proceed — per-note outcomes rather than one all-or-nothing transaction, which is
the v0.6 organizer's job. The precondition is not ceremony: the plan's byte
offsets describe one specific body, and applying them to a different one would
cut the note at arbitrary positions.

**Every fix writes an ordinary revision**, so it is visible in history and
revertible by restoring the previous one. There is no silent rewrite path.

**A span whose bytes are not what the plan recorded is skipped**, never applied
at an arbitrary position — the same rule P7's publication rewriter follows, for
the same reason.

**Alt text is opt-in.** A filename is a starting point for a description, not a
description, so asking for it is an explicit choice rather than something a
default run does to every image in a library.

**Remote-media localization is opt-in and goes through the existing engine.** It
reaches the network, so it runs the same code path as `notriosctl localize`:
domain rules, connect-time private-address blocking, size caps, MIME sniffing,
exact-hash policy, quarantine before admission. Fix never fetches anything
itself.

**Wikilinks are left alone.** Rewriting `[[Kitchen]]` into a Markdown link
replaces the syntax the author chose rather than repairing it.

## What the plan listed and this slice does not do

Stale link reference definitions. They are not fixed because they are not
detected: Markdown reference definitions (`[ref]: url`) are outside the link
extractor, so E2 has no check for them. Building a repair for something lint
cannot find would mean fixing blind. The check and its fix belong together in a
later slice, and that is recorded rather than quietly skipped.

## What a test taught me, twice

A fixture written as `[the plan](Kitchen Plan)` did not resolve at all: in
Markdown a space ends an unquoted URL, so the target truncated to `Kitchen`.
That is the same rule that decided E1a's slug form, met from the other side —
title-resolved Markdown links are the space-free ones, and the fixture now says
so in a comment rather than leaving the next reader to rediscover it.

## Validation

- store fixtures: planning changes nothing; the plan carries exact before/after
  and its base revision; applying writes an ordinary revision and preserves what
  the link points at; a concurrent edit fails with a conflict and survives
  intact; a missing precondition is refused; alt text is opt-in and filename
  derived; wikilinks and unresolved links untouched; Help and trashed notes
  skipped; request validation; span refusal for moved text, past-the-end,
  inverted, and negative spans;
- CLI fixtures on the real binary: dry run by default with no applied results,
  `--apply` reporting per note, idempotence on a second run, the repaired note
  keeping its blocks, `--list-kinds`, and refusal of an unknown kind;
- `go vet ./...`, `go test ./...`, required-file, scaffold, OpenAPI parse,
  migration-copy equality, docs site, `npm run typecheck`/`test`/`build`,
  `make gui`, `scripts/mvp_smoke.sh`, `scripts/run_performance_smoke.sh`.

No scale profile: fix is bounded to 1,000 notes per run by design and its cost
is dominated by the per-note revision writes the store already measures. The
whole-library read it depends on is E2's, already profiled.

## Next task

E4, graph traversal and paths, requires user approval.
