# G14e resumable acceptance checkpoint

G14e is the active, user-approved item. Its private full-corpus workspace is
`/home/renes/evidence/notrios/g14e-full-workspace`; the preserved G14b inputs
and frozen repository baselines are under the adjacent
`g14b-full-workspace`. Neither location is committed.

Run `bash scripts/run_archive_production_acceptance.sh` to continue. Each
phase writes an atomic immutable result before the next phase begins, so a
stopped run resumes without repeating completed work. The script builds the
current binaries, reuses the already imported canonical libraries read-only at
the content level, and deliberately does not repeat the unchanged 5.5-hour
foreign-import pass. Import counts, timings, and source-format equivalence are
revalidated against the preserved G14b results instead.

The first run exposed and fixed one harness-only edge case: a library with no
external objects serializes `external.packs` as JSON `null`. The unchanged-run
comparison now normalizes that to an empty list. The product snapshot completed
and verified; because the harness failed before publishing its phase metrics,
only `recipe-physical-unchanged` is repeated on resume.

The first two full catch-up attempts then exposed two G14d scale defects, both
fixed with targeted tests. Backup creation is synchronous but the shipped
client and server write deadline were 30 seconds; only this authenticated,
separately permitted request now has the existing two-hour G14 stage bound.
Next, the downloader requested a bounded 16 MiB range while the generic client
truncated all responses at 8 MiB. Ordinary replies keep the 8 MiB ceiling and
signed Range replies now have their own 16 MiB ceiling, matching the downloader
without widening unrelated allocations or doubling calls past the rate limit.

The third attempt completed both 6 GB carrier restores and cutover, then exposed
a third G14d scale defect in post-snapshot replay. Admission unconditionally ran
the whole-library metadata and attachment reconcilers for every operation; a
single body edit therefore climbed past 3 GiB RSS and remained in a global
projection for more than an hour. Admission now selects only the convergence
engines named by the newly contiguous record families. A document operation
that advances only `current_revision_id` stays with the already bounded revision
reconciler, while any real metadata or lifecycle field still invokes metadata
convergence. The production acceptance fixture now places its generated note in
the snapshot and edits its body after the boundary, with a focused scope test
and the small REST/directory catch-up test passing. The disqualifying attempt was
interrupted; its private log remains append-only outside the repository.

The fourth attempt then exposed a directory-carrier scale defect that the small
G14d interruption fixture could not show. Every bounded publish call compared
the complete durable prefix byte-for-byte again, so a transfer split into 16 MiB
calls performed quadratic source reads. A `Directory` instance now verifies the
durable prefix once, remembers the exact length it synced, and resumes later
calls from that position. A restarted process still revalidates its existing
prefix once, and the completed staging file is still hashed in full against the
authenticated expected digest before publication; carrier mutation can cause a
restart but cannot admit different bytes. The repeated-call test asserts one
prefix scan. Attempt four is retained as timing diagnostics, and catch-up must be
rerun with the optimized binary before its result is accepted.

Resume at `catchup`; all fourteen preceding results are complete.

The remaining completion work after the private run is:

1. Copy the privacy-sanitized summary produced by the harness to
   `performance/v0.7-g14e/FULL_SCALE_ACCEPTANCE.json` and validate it.
2. Record findings and freeze the capability/default throughout archive,
   synchronization, user, operations, troubleshooting, testing, security, and
   compatibility documentation. Feed the capability contract into G19.
3. Run the complete audit-first repository validation, archive G14e, update
   the handoff/logs, commit on `develop`, and produce a verified release ZIP.

No private database, archive, resource, source-bundle byte, repository, path,
filename, content hash, or command log belongs in the repository. A failed
gate reopens G14c or G14d; it is never waived here. G15 remains blocked until
G14e is complete and separately approved.
