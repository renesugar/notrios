# v0.4 P7 — publication handoff check

Date: 2026-08-05. Aggregate only, from a throwaway three-note library built for
this check: no private corpus, no local paths, no note content beyond the
fixture text written for the run.

P7 is a correctness and privacy slice rather than a scale one — the export path
it uses is the same P3/P3a/P3b writer already measured on the 382,206-note
corpus. What follows is the live behaviour check that the projection publishes
what it promises and withholds what it promises.

## Library

Three notes: `doc_guide` (notebook `Public/Guides`), `doc_setup` (`Public`), and
`doc_secret` (`Internal`). The guide links to all three kinds of target plus an
external URL:

- an internal link to the withheld `doc_secret`;
- an internal link to the published `doc_setup`;
- a link that never resolved;
- `https://example.com/docs`.

Profile: `--query 'notebook:"Public"' --link-action plain_text`.

## Review step (`publish plan`, read-only)

| Reported | Value |
|---|---|
| selected documents | 2 |
| internal links (retain) | 1 |
| private links (plain_text) | 1 |
| broken links (plain_text) | 1 |
| external links (retain) | 1 |
| policy | provenance, source bundles, private metadata, and Trash all off |

Two warnings appear *before* anything is written: one that a link targets a note
outside the selection, one that a link is unresolved.

## Publication (`publish run`)

Refused against a digest the operator had not reviewed:

```text
the reviewed plan no longer matches this library: reviewed 0000…, this library
now plans 323fc893…; re-run the plan and review the difference
```

With the reviewed digest:

| Reported | Value |
|---|---|
| target | `publication_handoff` |
| `full_backup` | false |
| verified | true |
| documents / revisions | 2 / 2 (current only) |
| link records | 2 |
| rewritten links | 2 |
| skipped rewrites | 0 |

Published body of the guide:

```markdown
See the internal runbook and the [setup guide](document://default/documents/doc_setup).
Also missing and [upstream](https://example.com/docs).
```

The withheld link and the broken link became prose; the published internal link
and the external link are untouched. A search of every file in the published
archive for the withheld note's ID and body text returns nothing: it is absent
from the records, from the bodies, and from the link records.

## Defect this check found

Archive-v1 import never rebuilt links. A note importing before its target
recorded the link as `unresolved` and it stayed that way until the note was
edited, so this fixture's first run classified all three internal links as
broken — including the link to a note the publication *was* publishing. A
publication would then have flattened every internal link in an imported
library. `internal/archive.Import` now rebuilds links after every note exists,
which adds no revision, and a regression test covers a note that links to a
target imported after it.

## Not covered

No real-corpus publication run: neither recipe corpus has the public/private
structure a publication profile exists to separate, and the attachment corpus
carries no such boundary either. The export path underneath is the one already
measured at 382,206 notes.
