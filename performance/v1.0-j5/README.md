# v1.0 J5 — the library at 382,206 notes

```sh
bash performance/v1.0-j5/scale_profile.sh <label> <joplin-raw|obsidian> <source> <work-dir>
bash performance/v1.0-j5/backup_restore_profile.sh <label> <work-dir>
python3 performance/v1.0-j5/validate_evidence.py
```

Three real libraries the owner supplied, measured on one machine, sequentially,
so the numbers describe the work rather than three jobs contending for a disk.
**Nothing here is generated and nothing is extrapolated.** J5's open decision
about how to build a bulk corpus is closed by not needing one: the real corpora
reach the target size, so no synthetic data was produced and nothing has to be
labelled synthetic.

## Import

| corpus | notes | elapsed | rate | peak RSS | database |
|---|---|---|---|---|---|
| Joplin export with resources | 103,349 | 27.1 min | 63.5/s | 525 MiB | 916 MB + 956 MB assets |
| Obsidian vault | 382,206 | **4.52 h** | 23.5/s | 1,721 MiB | 7.28 GB |
| Joplin RAW (same notes) | 382,206 | **1.72 h** | 61.7/s | 2,881 MiB | 6.04 GB |

## Export and restore

| corpus | export archive-v2 | restore | restore peak RSS |
|---|---|---|---|
| Joplin export with resources | 772 s | 499 s | 44 MiB |
| Obsidian vault | 3,017 s | 1,655 s | 58 MiB |
| Joplin RAW | 3,206 s | 2,767 s | 54 MiB |

A full round trip at 382,206 notes is **~5.8 h** through the Obsidian importer
and **~3.4 h** through the Joplin one.

## What the pair of importers showed

The recipe corpus exists in both formats, so the store does identical work both
times and any difference is the importer. **The Joplin import is 2.6× faster on
the same 382,206 notes**, while reading 3.2× more files and 8.5× more bytes, and
while additionally applying 842,813 tag applications the vault has none of.

The code says why, and the counters only hinted at it:

| importer | link rebuild |
|---|---|
| Joplin | `RebuildImportDocumentLinksBatch(ctx, {DocumentIDs: […]})` — one call per batch, 3,823 calls |
| Obsidian | `RebuildDocumentLinks(ctx, note.TargetID)` — one call per document, **382,206 calls** |

The Obsidian importer batches its *reads* — `GetDocuments` over a slice — and
then rebuilds links one document at a time inside that loop. The batch API it
needs already exists and its sibling already uses it.

**This was a strong lead and J17-A measured it. It is not the cause.**

`TestJ17LinkRebuildTransactionCost` runs both paths over the same 2,000
documents in a copy of this library:

```
per-document: 19.095s total, 9.547ms per document (2000 transactions)
batched:      12.639s total, 6.320ms per document (4 transactions)
ratio: 1.51x
```

Collapsing 500 commits into one buys **34%**, not the 2.6× the importers differ
by. Most of the cost is the rebuild itself, not the transaction around it: even
batched, a document costs ~6.3 ms, so 382,206 of them is about 40 minutes of
work neither importer can avoid — real, and far short of the 2 h 48 m gap.

**The arithmetic published here before that measurement was wrong.** It reasoned
that ~378,000 extra commits across a 10,068-second gap implied ~27 ms per
commit. The measurement puts a commit at ~3.2 ms of the 9.5 ms per document.
That was a number fitted to a hypothesis rather than a test of it, and it is
left in this record with its correction rather than quietly replaced.

**A second batching theory was measured and also failed.** The Obsidian importer
writes each document with per-document calls where the Joplin importer uses
`ApplyImportDocumentBatch`; running 400 documents each way gave **0.99×** — 400
transactions against 1 changed nothing. See `performance/v1.0-j17`.

**Characterised since, and still unexplained.** The Obsidian importer costs
~40 ms per note and the Joplin importer ~16 ms, both stable across corpus size,
so the 2.6× is a constant factor rather than a scaling defect. Five candidate
causes have been tested and rejected — see `performance/v1.0-j17`.

**So the gap this section reports is, as of now, unexplained.** Two call-site
differences that looked like causes are not causes. The measurements did find
something larger on the way — every document write scans the whole full-text
index, which is `performance/v1.0-j18` — but that is a property of writing to a
*full* library, and an import writes to one that starts empty. The 2.6× stands
as a measured fact with no established mechanism, which is the honest state of
it.

## Two structural results

**Restore streams; import does not.** Restore peak RSS is 44–58 MiB whatever the
corpus size. Import peak RSS scales roughly with it: 525 MiB at 103k notes,
1,721 MiB at 382k. After the first corpus I wrote that the importer "is
streaming, not accumulating"; the larger runs disproved it, and the record says
so rather than quietly dropping the claim.

**Export is insensitive to which importer built the library** — 3,017 s versus
3,206 s for the same notes — which is consistent with the archive reading the
store rather than replaying an import.

## What degraded

**Search is slow at this size.** On 382,206 notes, from the command line: ~9 s
for a common term (187,518 hits), ~4.4–5 s for a rare one (385 hits). Cold and
warm passes are within noise of each other, so nothing useful is being cached.
Both passes are recorded separately rather than averaged, which is what makes
that visible.

**Five resources in the Joplin export have no content.** `resources_seen: 763`,
`resources_imported: 758`, `resources_missing_content: 5`. The import completed
and reported them rather than failing or dropping them silently.

**`unresolved_links: 14`** on the Joplin RAW import, and a 839 MB temporary
manifest during it.

## The external comparison

`movenotes-v3` `joplin2sql` at `cb7ddc0`, cloned from GitHub rather than the
owner's working copy, on the same corpus: **2,186 s and 1,250 MiB peak**, versus
Notrios's 6,194 s and 2,881 MiB. **2.83× faster, 2.3× less memory.**

It is not the same job. `movenotes` builds two tables — `settings` and `notes` —
with eight B-tree indexes and **no full-text index**; Notrios builds a
`documents_fts` index whose data alone is 374 MB, plus revisions, links,
notebooks, 842,813 tag applications and a content-addressed asset store. So the
number bounds how much of the import is inherent — walking 1,237,553 files and
parsing them — rather than showing the same work done better.

One trade worth naming: `movenotes` ran with a **1.4 GB write-ahead log**, one
long transaction. Fast, and a crash mid-import loses everything; Notrios commits
in 3,823 checkpointed batches.

## Three harness bugs, all mine

Recorded because each looked briefly like a product defect and none was.

- **`--intent fork` without `--new-database-id`** failed all three restores.
  The refusal is *correct*: forking means becoming a new database, and letting
  the tool invent the identity would produce two libraries that disagree about
  who they are.
- **`--output` takes a directory**, and my first fix changed the path without
  creating it, so the comparison failed twice on the same argument.
- **A step called "verify"** was a reader-capability preflight over the
  manifest, which is why it returned in hundredths of a second on a 3 GB
  archive. Renamed: the content verification runs inside `export archive-v2`
  and is already inside the export timings.

And one outside the harness: every `until ! pgrep -f "name"` waiter in this
session never fired, because **`pgrep -f` matches the waiting shell's own
command line**. Two shells waited 12 and 17 hours on conditions that could never
become false. `pgrep -f "[n]ame"` is the fix.
