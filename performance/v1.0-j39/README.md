# v1.0 J39: a reimport skips a note it cannot change

Before J39, an Obsidian reimport did almost all the work of a fresh import just
to conclude that nothing had changed. Every note was read from disk again,
canonicalized and hashed, then compared with the stored body:

| case (2026-09-26) | fresh | no-op reimport |
|---|---:|---:|
| obsidian-10k | 65.7 s | 37.7 s |
| collision | 46.4 s | 16.3 s |

J39 skips a note when the import can tell, without reading it, that it would
write exactly what is stored. The rest of this record covers what that proof
is, what it must never get wrong, and what it bought.

## Two decisions, before any code (2026-10-20)

The plan as written needed a new column on `import_item_states`, and only a
reimport of an entirely unchanged vault could skip. Reading the code changed
both, and the owner decided each one.

- **No schema change.** The plan avoided redefining the existing `fingerprint`
  column because, it said, every note of every library would then look changed.
  That is not true for Obsidian notes. `processNotes` calls a note unchanged
  when its title, body and notebook match what is stored, and the fingerprint
  test beside it could only ever agree. A column would have meant schema 29.
  Schema 28's compatible range ends at 28, so sync between a J39 build and any
  schema-28 build would have stopped until both were upgraded.
- **The witness is the link namespace, not the whole vault.** A canonical body
  depends on five things: the note's bytes, its path, the collection, the
  rewriting rules, and the namespace its links resolve against. Keying on the
  whole-vault fingerprint would have turned every skip off the moment any file
  changed. Keying on the namespace lets a reimport with a few edits skip every
  other note, as long as no edit moved a title, an alias or a path.

## J39-A: what a note's state records

A note that did not warn records `recordedFingerprint`, which is `j39:`
followed by one hash over five parts:

- **The file's hash.**
- **The notebook the note lands in.**
- **The canonical body's hash.**
- **The basis.** This is `canonicalRules`, the collection and the link
  namespace, hashed once per run by `canonicalBasis`. The namespace is hashed
  from its inputs, in the inventory's sorted order: each note's path, ID,
  title and aliases, and each asset's path and ID. `buildLinkNamespace` builds
  the namespace from those inputs and nothing else.
- **The resource references the import gave the note**, hashed by
  `store.ResourceRefsDigest`, which does not depend on their order.

**Two kinds of note record the bare J32-AA note fingerprint instead:**
- A note whose links warned.
- Every note a build before J39 wrote.

Neither carries the `j39:` prefix, so the import rules them out by their first
bytes. It fetches no head for them and hashes nothing on their account.

`store.ImportDocumentHeads` answers, for a batch of notes, each one's current
revision's `content_sha256`, its title and notebook, and the digest of its
stored references. It is one statement, it reads no body, and a note in the
trash has no head.

**A library from an older build is read in full on its first reimport, and
skips from the second.** `TestJ39AStateFromAnOlderBuildIsNotSkipped` rebuilds
exactly the state such a build leaves.

**The rule version is pinned to the rules' output.** J36 changed where links
resolve without touching any vault file, so a skip that did not notice would
have kept the pre-fix links. `canonicalRules` is part of every basis.
`TestJ39CanonicalRulesArePinned` holds a digest of what the rules make of a
corpus of 22 edge-case notes, beside the version number. Those notes cover:
- every link form, titles in all three delimiters, angle brackets, parentheses
  and encoded spaces;
- anchors and blocks;
- aliases, frontmatter, CRLF and empty notes;
- an ambiguous name, a path-shaped broken link;
- an asset by name and by path;
- the overlap the one-pass assembly declines.

If the output changes and the number does not, the test fails, and its message
says to move both.

## J39-B: what is skipped, and what never is

A note is skipped only when all three of these hold:
- **Its state is in the skip form.**
- **The skip form, recomputed, equals what was recorded.** It is recomputed
  from the stored revision's content hash, the stored references' digest and
  this run's basis.
- **The stored title and notebook are what this run would write.**

A skipped note still sends its provenance and its item state, as an unchanged
note always has. What it does not get is the read, the canonicalization and the
hashing. Its body is not read on the store side either: the store's "unchanged"
path now checks only that the note is there, where it used to read the whole
document and discard it. And the provenance lookup is now made only for notes
their deterministic ID did not find. Those are legacy imports and notes in the
trash, so a reimport, which finds every note, no longer asks.

`TestJ39BEveryDivergenceIsRead` covers the plan's repair cases, and two more.
In each one, the notes that diverged are read and repaired and the others are
skipped. The import after the repair then skips every note except those that
warned, and those the table names:

| change after the first import | read | the repair |
|---|---:|---|
| a note edited inside Notrios | 1 | the vault's text is back |
| a resource detached from a note | 1 | attached again |
| a resource purged, as a peer's deletion leaves it | 2 | recreated and attached to both notes |
| a note moved to another notebook | 1 | moved back |
| the rule version changed | all | every note under the new rules |
| a new note makes a name ambiguous | all | the link warns and loses its old target |
| a note moved to another vault folder | all | the links to it follow it |
| an asset moved to another vault folder | all | both notes embed it at its new path; both are read on every import after (see below) |
| another note's title changed | all | the namespace moved |
| one note edited in the vault, no name moved | 1 | that note revised |

The two "moved" cases keep the number and the order of the notes as they were,
so only the paths and IDs in the basis can notice them.

**A note that warned is read on every import**
(`TestJ39BANoteThatWarnedIsAlwaysRead`). The report's warnings therefore stay
those of a full pass. The test also checks that its state is the one without
the `j39:` prefix.

**Each condition has a test that fails without it.** Every guard was removed in
turn, and these tests failed:

| guard removed | failing tests |
|---|---|
| the notebook comparison | moved to another notebook |
| the references, from both the record and the skip | a resource detached; a resource purged; J39-C |
| the warned note's bare form (warned notes recorded in skip form) | a note that warned |
| the notes, from the basis | a note moved to another vault folder; another note's title changed |
| the paths and IDs, from the basis | a note moved to another vault folder |
| the assets, from the basis | an asset moved to another vault folder |
| the rule version, from the basis | the rule version changed |
| the `j39:` prefix test | none, as expected |

The prefix is a cost measure, not a guard. A state without the prefix could
never match the recomputed skip form anyway.

Obsidian notes never record a conflict, so the plan's condition about one has
nothing to act on here. The Joplin importer records no canonical hash, so it
gets no skips.

## J39-C: what the reimport still does

The link pass still refreshes every note and the checkpoint still completes.
`TestJ39ANoOpReimportSkipsEveryNote` checks both on a reimport that skips every
note, and checks that no note gains a revision.

`TestJ39CSkipsLeaveTheSameLibrary` puts two libraries through the same history:
- a note edited in Notrios;
- a resource detached from a note;
- a note edited in the vault.

The last reimport then skips in one library and reads every note in the other.
The two must hold identical bodies, titles, notebooks, revision counts, link
rows, references, provenance and item states.

**The report changes in two counters.** `notes_skipped` is new, and counts the
unchanged notes that were not read. A skipped note's links are not counted in
`links_rewritten` or `attachments_created`, because counting them would mean
reading it. `docs/import-export.md` says what a skip means for a re-run.

## J39-D: the measurements

**The harness gained two things first, in its own commit (`1351041`):**
- **A `read_bytes` metric:** the bench process's `rchar` from
  `/proc/self/io`, taken around the import.
- **A `reimport-changed` scenario.** The seed library is imported from the
  corpus. The measured import then reads the same vault path after one note
  in a hundred has had a plain line added at its end, which moves no title,
  alias or path.

The baseline binary was built from that commit, so both sides of every
comparison report `read_bytes`. Every case below is five measured runs per
binary, alternating, after a warm-up each, on HD2 (ext4), with the load average
at 1.0 or below before each run. `compare.py` applies J32's rule.

**The skip took four forms, and only the last is measured below.** J32's
precedent is that a form which regresses is replaced rather than excused, and
each earlier form was measured, found to regress on a count, and replaced:

| form | commit | what the measurement found | replaced by |
|---|---|---|---|
| 1 | `508506b` | obsidian-10k reimport −32.7%; collision reimport flat with hash calls +54%, hashed bytes +109%, statements and allocations up: its 5,000 notes that warn still paid for a head and two hashes before being read | the `j39:` prefix, so a note that cannot be skipped is ruled out by its state; one hash, not a hash of a hash; the provenance lookup only for notes not found |
| 2 | `9221c7f` | collision reimport: allocations, statements and hash calls better, hashed bytes still +32%; fresh imports +500 hash calls | the read path hashed a non-warning note twice; the basis now hashes the namespace's inputs, not the maps; an empty reference list gets a constant digest |
| 3 | `e143816` | wall time held everywhere, but a fresh obsidian-10k import allocated 5% more; a profile named the basis writing each part as a copied string, and a hasher per note in the reference digest | one buffer for the basis, one sum for the digest |
| 4 | `a81ef60` | below | — |

The first and second forms' results, and the invalid run described next, are
kept on HD2 under `notrios-j39/`, not in the repository.

**A harness mistake, found and fixed while measuring (`4a73bd6`).** As first
written, `reimport-changed` imported the edited copy from its own directory.
An import keys its item states by the vault's path, so that made the edited
copy a new source with no states, and nothing could be skipped. The first
collision/reimport-changed result showed `notes_skipped: 0` and was discarded.
The case now gives each run one vault directory, which holds the corpus while
the seeds are made and the edited copy after.

### The fourth form, against the harness commit

| case | wall, before → after | change | verdict under J32's rule |
|---|---:|---:|---|
| obsidian-10k/reimport | 36.42 s → **24.11 s** | **−33.8%** | improved, no regression |
| obsidian-10k/reimport-changed | 37.28 s → **24.42 s** | **−34.5%** | improved, no regression |
| collision/reimport | 15.89 s → 15.66 s | −1.4%, within noise | worse: hashed bytes +12.0%, read bytes +0.1% |
| collision/reimport-changed | 16.18 s → 15.85 s | −2.0%, within noise | worse: hashed bytes +11.9% |
| collision/fresh | 44.82 s → 44.12 s | −1.6%, within noise | worse: allocated bytes +0.6%, mallocs +0.8%, hashed bytes +11.8%, one hash call |
| obsidian-10k/fresh | 62.81 s → 61.21 s | −2.5%, within noise | worse: allocated bytes +3.7%, mallocs +2.5%, hashed bytes +16.4%, one hash call |

On the two obsidian-10k reimports, everything else moves the same way:

| metric | no-op | 1% changed |
|---|---:|---:|
| prepared statements | −19.9% | −19.4% |
| hash calls | −49.9% | −48.6% |
| hashed bytes | −41.3% | −40.1% |
| allocated bytes | −33.6% | −33.0% |
| mallocs | −39.6% | −39.0% |
| read bytes | −3.0% | −2.2% |

Peak RSS, live heap and commits are within noise on every case.

**Why collision gains no time.** 5,000 of its 5,500 notes are `index.md` in
different folders, and their links warn as ambiguous. A note that warned is read
on every import, by design, so only 500 notes are skipped. What the case shows is
that the skip costs such a vault no time. It still costs it the basis hash and
the head of each note that might be skipped.

**Why `read_bytes` moved so little where wall time moved so much.** The
inventory still reads every Markdown file twice before any note is considered:
once to hash it, and once to take its title and aliases. The skip removes the
third read, in the notes phase, and its canonicalization. That is 3% of the
bytes read, but a third of the time. Merging the inventory's two reads would
be a separate change, and under J8 it is not made here.

### What the rule leaves to the owner

By J32's rule as written, J39 is **not shown to improve**. The declared metric
improved by a third on both obsidian-10k reimports and regressed nowhere. But
four cases are worse on a count:

- **`hashed_bytes`: +12% to +16% on every case that is not an obsidian-10k
  reimport.** This is the price of the witness itself. Every import hashes the
  namespace's inputs once, and every note's fingerprint now covers the basis
  and its references. That is at most 2.4 MB more on these corpora, against
  14 MB a fresh obsidian-10k import already hashed.
- **`hash_calls`: +1 on a fresh import.** This is the basis.
- **Allocated bytes +0.6% to +3.7%, and mallocs +0.8% to +2.5%, on fresh
  imports.** The fingerprint is longer, and a note with references gets a
  digest of them.
- **`read_bytes`: +0.1% on collision's no-op reimport,** the head statement's
  pages.

None of these moved a wall time beyond its noise. The same kind of trade was put
to the owner in J32-K, and kept on that basis: bounded batches cost 18% more
statements on `many-large`, and halved peak RSS.

**Decided 2026-10-21: kept.** The owner kept the fourth form on these numbers,
as J32-K's trade was kept: a third off every obsidian-10k reimport, for counts
that cost no measurable time. J39-D is done on that decision.

## What this does not prove

- **The namespace argument is checked, not proved.** A canonical body is a
  function of the note's bytes, its path, the collection, the rules and the
  namespace because `canonicalBody` and `planObsidianLinks` take nothing else.
  A future change that made them read something more would also have to add it
  to the basis. The pin catches a change to what the rules produce, but not a
  new input to them.
- **The pin is only as good as its corpus.** A rule change that alters no note
  in it would pass. The corpus collects the cases J32 to J43 had to decide.
- **An import has never removed a reference it no longer gives.** This predates
  J39. When an asset moves, or a note stops linking an image, the old reference
  stays on the note. J39 reads such a note on every import, because its stored
  references are no longer the ones the import gave. That is the safe
  direction, but each such note is a skip lost. Whether an import should remove
  references it no longer gives is a question about what a reimport repairs,
  and under J8 it is recorded here, not answered.
- **Only the Obsidian importer skips.**

## Archive

`notrios-v1.0-j39-cd14448.zip`, built from commit `cd14448`, with the ledger not
yet flipped. 26,659,678 bytes, 2,991 entries, accepted by `check_release_zip.py`.
No usage override was needed: the seven-day bucket had 94% remaining and the
five-hour bucket had 100%.

The first build, from `f265415`, the commit that records the owner's decision,
stopped at `npm audit` in `web/`, on three `launch-editor` advisories against
vite published the day before. J46 cleared them by moving vite from 8.1.4 to
8.1.6 in the lockfile, so this archive also carries J46-A. That changes nothing
J39 claims: J46-A changes no Go code and leaves the built UI byte for byte the
same, and J39's tests pass unchanged at this commit.
