# v0.4 P7 — Publication profiles and privacy-reviewed archive handoff

Status: complete on 2026-08-05.

Model: Claude Opus 5 (Claude Code).

## Why

P1 planned publications and P3 refused to write them: `publication_handoff` and
the content-rewriting link actions were rejected by the exporter because both
change what a note says, which a restore-fidelity archive may never do. This
slice builds the writer that is allowed to.

## What it does

`notriosctl publish profile save|list|delete`, `publish plan`, `publish run`.

A profile records selection and privacy decisions only — never an output path,
a command, or anything derived from note content — and lives owner-only in
`<data-dir>/publish-profiles.json`. `publish plan` is the read-only review.
`publish run` re-plans and refuses unless the result still matches the digest
that was reviewed.

The projection itself:

- current revisions only, no trashed notes, no provenance, no exact source
  bundles, no saved searches, and blanked revision metadata;
- links to withheld or unresolved targets become plain text or `[redacted]`;
  links to published notes, and external URLs, are untouched;
- the link *records* for rewritten links are dropped, and retained links keep
  offsets shifted onto the published body;
- the result is an ordinary archive-v2 directory: same manifest-last format,
  same verifier, `full_backup: false`, and warnings that say what it is.

## Decisions worth recording

**A backup may not rewrite content; a projection may.** The refusal moved from
"this link action is never allowed" to "this link action is not allowed for
`full_archive`". A backup's whole promise is that what comes out is what went
in.

**Rewriting the body is not enough.** The CLI test caught this: after the body
was rewritten, the withheld note's ID still appeared in the archive, because a
link record carries the raw target, the resolved ID, and a `context` excerpt of
the surrounding sentence. A published record for a withheld link hands over
exactly what rewriting the body just removed, so those records are dropped and
`context` is cleared for every published link.

**Retained links need shifted offsets.** Rewriting one span moves every byte
after it. Without adjustment, a record for a link the publication *kept* would
point into the wrong place in the published body.

**A stale span is left alone.** Link offsets are recorded when a note is saved.
If the bytes at a recorded span no longer look like the link — wrong prefix,
target no longer present, span past the end, inverted, overlapping — the body is
not touched and the run warns. Cutting at a stale offset would remove an
arbitrary piece of an unrelated sentence.

**Publishing is gated on a reviewed digest, not on a flag.** A profile is not a
promise about a fixed set of notes: adding a note to the published notebook or
removing a `private` tag changes what would go out. `--reviewed-plan` makes "I
checked this yesterday" fail loudly instead of publishing quietly.

**`full_archive` is refused as a profile target** at save time, so the most
dangerous export is not reachable through a name that sounds like publishing.

## A defect the evidence run found

Archive-v1 import never rebuilt links. A note imported before its target
recorded the link `unresolved` and it stayed that way until the note was edited.
The first evidence run therefore classified every internal link as broken —
including a link to a note the publication was publishing — so a publication of
an imported library would have flattened all of its internal links.
`internal/archive.Import` now rebuilds links after every note exists, which adds
no revision, with a regression test covering a note that links to a target
imported after it.

The same run also showed the first version of the CLI test was passing
vacuously: the profile selected by tag, archive-v1 import does not create tags
from that fixture's frontmatter, and a selection of zero notes makes every
"the withheld note is absent" assertion trivially true. The test now asserts the
selection is real (one note, one private link) before checking what was
withheld.

## Validation

- projection fixtures: plain-text and redaction rewriting, records dropped for
  withheld links, shifted offsets for retained links, current-revision-only,
  provenance/bundles/saved searches withheld, revision metadata stripped,
  verification as an ordinary archive, and a selector requirement;
- rewriter fixtures: the five refusal cases and reverse-order application;
- profile fixtures: round trip with `0600`, refusal of `full_archive`, of an
  empty selection, of publishing Trash, and of a corrupt profile file;
- CLI fixture building the real binary: no digest and a stale digest both refuse
  and write nothing, the reviewed digest publishes, and a note joining the
  selection after the review invalidates it;
- `go vet ./...`, `go test ./...`, required-file, scaffold, docs-site,
  `npm run typecheck`/`build`/`test`, `make gui`, `scripts/mvp_smoke.sh`,
  `scripts/run_performance_smoke.sh`;
- live check recorded in `performance/v0.4-p7/publication-handoff-check.md`.

No real-corpus publication run: neither recipe corpus nor the attachment corpus
has the public/private boundary a publication profile exists to separate, and
the export path underneath is the one already measured at 382,206 notes.

## Next task

P8, v0.4 documentation and release wrap-up.
