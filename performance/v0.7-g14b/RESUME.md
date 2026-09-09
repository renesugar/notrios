# G14b usage-limit checkpoint — 2026-08-16 (resolved 2026-08-22)

This file preserves the interruption handoff that allowed G14b to resume
without repeating immutable private-corpus work. All seven resume steps below
were completed on 2026-08-22. G14b is complete; this is historical evidence,
not the current starting point. Continue from `CODING_CLIENT_HANDOFF.md` and
`PLAN.md`, where G14c is next and unapproved.

No production format, schema, dependency, default, encryption, or catch-up
behavior has changed. Current repository changes are evidence/prototype code,
tests, documentation drafts, and agent logs only.

## Decision selected

The completed evidence selected **option B**: a required versioned SQLite-image
capability with bounded packed external assets for compatible full backup and
catch-up, while packed semantic archive-v2 remains mandatory for subset, merge,
schema-independent interchange, and fallback recovery.

For the 382,206-document recipe workload, complete create-through-restore local
time was 4,383.9 seconds for packed semantic archive-v2 and 1,884.2 seconds for
the SQLite image bundle (2.33x faster). Adding separately measured Google Drive
copy plus exact reread produced 4,457.6 versus 2,245.1 seconds (1.99x faster).
The packed artifact was 1.309 GB and its provider phase took 73.7 seconds; the
image was 6.040 GB and took 360.9 seconds. Both recovered the exact canonical
aggregate and passed integrity, corruption refusal, time, and memory gates.

Semantic restore took 2,792.1 seconds, 196.9 MiB peak RSS, and 28.4 GB of
writes. Image restore took 516.2 seconds, 202.6 MiB peak RSS, and 6.04 GB of
writes. Packed archive creation was deliberately interrupted once; its partial
tree was preserved and the current writer correctly restarted because it has
no durable partial-pack publication boundary.

The loose recipe archive completed with 382,447 files and 448,045 total
entries. The attachment loose-assets image completed with 112,063 files and
166,731 total entries. Both fail the approved non-object-per-file transport
rule and need no downstream phase. The packed attachment image completed its
full round trip and restored 103,349 documents, 758 resources, 731 blobs, and
111,330 preserved source-bundle items from one external tar. Five missing
resource bodies remain an explicit source/import condition.

The existing post-snapshot replay converged but took 3,039.6 seconds, used
3.22 GiB peak RSS, and wrote 31.6 GB. It fails the 512 MiB desktop gate and is
G14d work independent of the selected snapshot representation.

## Completed immutable phase rows

- Both recipe source inventories and imports; semantic comparison proves equal
  382,206-document visible content while preserving 3,682 stored-title and
  source-specific metadata differences.
- Attachment inventory/import.
- Recipe `archive-v2-pack`: create, verify, prepare, seal, open, restore,
  corruption refusal, provider copy.
- Recipe `sqlite-image-bundle`: create, verify, prepare, seal, open, restore,
  corruption refusal, provider copy.
- Attachment `sqlite-image-bundle`: create, verify, prepare, seal, open,
  restore, corruption refusal.
- Recipe `sqlite-stopped-copy`: create, verify, prepare, seal, open, restore,
  corruption refusal, and changed-snapshot replay.
- Recipe `sqlite-online-backup`: create and verify.
- Recipe `archive-v2-loose`: create only; structurally rejected.
- Attachment `sqlite-online-backup`: create only; structurally rejected.
- Corrected recipe `restic-canonical`: first create and `check --read-data`
  verify.

The external diagnostics directory preserves three harness investigations:
the pre-fingerprint-cache timing, a read-only SQLite verifier that initially
created WAL/SHM sidecars before immutable mode was added, and a SQLite ZIP open
that initially omitted the declared empty assets root. It also preserves the
deliberately interrupted packed export and the first Restic repository attempt
described below. None is an accepted result row.

## Last issue fixed and verified

The first Restic canonical repository used an absolute backup operand. Restore
then tried to apply metadata for a synthetic `/home` parent inside its target
and failed under the sandbox. That repository, its result rows, checkpoint,
log, and partial restore were moved intact beneath the external diagnostics
tree. `repository_backup_operand` now runs Restic and Borg from the source
parent with only the relative top-level name. Eleven harness unit tests pass,
including a regression that forbids an absolute operand. A fresh Restic
canonical create (183.8 seconds) and `check --read-data` (134.4 seconds) pass.

## Completed resume order

1. Run corrected `restic-canonical` `unchanged-snapshot` and
   `snapshot-restore`. The restored top-level tree must match the complete
   content fingerprint.
2. Run `borg-canonical` create, `check --verify-data`, unchanged snapshot, and
   exact restore.
3. Run `restic-raw` and `borg-raw` over the immutable recipe Joplin source:
   first create, data verify, unchanged snapshot, and exact restore. Pass the
   same logical source used by the completed recipe source inventory. Do not
   point either adapter at an existing repository.
4. Run the external workspace validator. Then create aggregate-only
   `full-corpus-results.json`, excluding source paths, filenames, content, and
   full-corpus hashes. Finish `FINDINGS.md`; `validate_evidence.py` already
   encodes the required rows and privacy/acceptance checks.
5. Amend G14c's blocking decision to option B with exact capability, fallback,
   compression/pack/resume/migration rules and measurements. Update handoff,
   plan status, roadmap, architecture/policy/testing/docs references; fix the
   duplicated G14a wording in `SYNCHRONIZATION.md`.
6. Run the mandated validation sequence beginning with frontend `npm audit`,
   compatible fix review if any, `npm ci`, repeat audit, frontend tests/build,
   all Go tests, harness/evidence validators, scaffold/docs checks, and
   `git diff --check`.
7. Append the terminal G14b attempt record, archive the item under
   `plans/v0.7/020-full-corpus-physical-snapshot-selection.md`, mark the PLAN
   heading complete with an `Outcome (2026-08-16)` paragraph, commit, and use
   the release-packaging skill/script to produce and verify the G14b ZIP.

Use `scripts/run_archive_full_corpus_benchmark.sh` or the equivalent direct
`harness.py` invocation. Required common arguments are the workspace, workload
`recipe-joplin`, adapter, phase, the already built local `notriosctl` and helper
binaries, and an honest `interleaved-repeat` cache label. The raw adapters also
require the immutable recipe Joplin source root. Do not use `--preflight-
fingerprint` together with a measured phase; it seeds the external fingerprint
cache and exits, so run it separately only if the cache needs rebuilding.
