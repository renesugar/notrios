# v0.7 G14a — Archive scalability benchmark contract and resumable harness — complete

Status: **complete**

Date: 2026-08-15

Model: GPT-5 (exact serving variant unavailable)

## Goal and boundaries

Make the physical snapshot decision reproducible before using the supplied
private corpora. This slice adds evidence/prototype code only: it does not
change production formats, schema, dependencies, defaults, database encryption,
or catch-up behavior. Evidence committed to the repository is aggregate-only.

The approved non-blocking defaults were exercised: passes are labelled
`interleaved-first`; none is called cold; correctness and recoverability are
mandatory; the two-hour stage, 512 MiB desktop, 256 MiB receiver-proxy, and
non-object-per-file gates remain in force for G14b.

## Implementation

- `scripts/run_archive_scalability_benchmark.sh` runs or resumes one named
  tier and phase in an external workspace.
- `performance/v0.7-g14a/harness` defines an atomic checkpoint, immutable
  aggregate result, privacy/arithmetic validation, filesystem inventory, and
  complete nine-stage adapter contract.
- Eleven adapters map current loose/packed archive-v2 and catch-up, stopped/
  online/bundled SQLite image candidates, and raw/canonical restic and borg
  references onto identical stage boundaries. Reference commands always create
  new repositories and never mutate the existing `recipedb_repo`.
- Generated 10k and 100k Joplin RAW tiers exercised real import, export,
  verification, ZIP preparation, authenticated framing, open, restore, and
  incremental replay. Each phase can resume independently.

## Calibration evidence

All 18 phase rows completed under two hours, and all 10x tier time ratios were
below 10x. The 100k loose archive produced 100,093 files, failing the
non-object-per-file gate even though fanout limited one directory to 256
entries. Stored ZIP added 26,224,320 bytes (11.15%); authenticated framing
added only 6,004 bytes. Snapshot open and restore remained below 92 MiB peak
RSS and the canonical content fingerprints matched. Incremental replay reached
797,937,664 bytes peak RSS and fails the 512 MiB desktop gate.

An initial restore comparison tried to equate exported record-object sets.
That premise was wrong because a replacement restore deliberately rotates
replica/database identity. The completed harness instead uses the established
P4 canonical aggregate: integrity check, entity counts, and ordered blob/source
bundle hashes. The failed result was never published, and the resumed attempt
completed with the corrected semantic comparison.

## Validation

- `go test ./performance/v0.7-g14a/...`
- `python3 performance/v0.7-g14a/validate_evidence.py`
- `bash scripts/run_archive_scalability_benchmark.sh --validate-results <external-workspace>`
- completed-phase rerun returned the immutable result without repeating work
- full repository validation recorded in `agent/ATTEMPT_LOG.jsonl`

**Outcome (2026-08-15).** G14a is complete. The resumable aggregate-only
benchmark contract, adapter matrix, generated calibration, privacy validator,
and two explicit baseline failures are durable repository evidence. No
production behavior changed. G14b is next and remains approval-gated.
