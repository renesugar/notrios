# G14a archive scalability benchmark contract

This directory contains generated, aggregate-only evidence for the resumable
archive benchmark harness. It selects no production format. Full private-corpus
candidate comparison belongs to G14b.

## Run one phase

Use an absolute ext4 workspace outside the repository. A completed phase result
is immutable and is returned immediately on a later invocation. An interrupted
phase retains a `started` checkpoint; rerunning increments its attempt and uses
the underlying importer/archive recovery behavior.

```bash
bash scripts/run_archive_scalability_benchmark.sh \
  --workspace /absolute/private/benchmark-workspace \
  --tier 10000 \
  --adapter archive-v2-loose \
  --phase snapshot-create \
  --cache-state interleaved-first
```

The phase order is `inventory`, `foreign-import`, `snapshot-create`,
`snapshot-verify`, `transport-prepare`, `transport-seal`, `snapshot-open`,
`snapshot-restore`, and `incremental-replay`. `--describe-adapters` emits the
machine-readable mapping for all 11 candidates/references. Generated execution
in G14a supports the loose and packed archive-v2 adapters; G14b supplies the
private source/canonical roots and implements the remaining candidate commands.

Validate every completed row without reading corpus content:

```bash
bash scripts/run_archive_scalability_benchmark.sh \
  --validate-results /absolute/private/benchmark-workspace
python3 performance/v0.7-g14a/validate_evidence.py
```

## G14b command definitions

Every repository destination must be new and explicitly named. In particular,
never point a comparison at the existing `recipedb_repo`. Password/passphrase
files live in the private benchmark workspace and are never committed.

| Adapter | Comparable create/verify/open/restore commands |
|---|---|
| stopped SQLite copy | stop and checkpoint the isolated Notrios store; copy `notes.sqlite` plus the asset roots into a versioned staging directory; run `sqlite3 <image> 'PRAGMA integrity_check'`; pack/seal/open through the same boundaries as archive-v2 |
| SQLite Online Backup API | `sqlite3 <live-db> ".backup '<new-image>'"`; run `PRAGMA integrity_check`; bind the image and external-object manifest before pack/seal/open |
| SQLite-image bundle | Online Backup API image plus a versioned canonical manifest and bounded packs for resources/source bundles; verify every declared size/hash and schema range |
| restic raw/canonical | `restic -r <new-repo> init`; `restic -r <new-repo> backup <source>`; unchanged and changed second snapshots are separate; `restic -r <new-repo> check --read-data`; `restic -r <new-repo> snapshots`; `restic -r <new-repo> restore <snapshot> --target <new-ext4-target>` |
| borg raw/canonical | `borg init --encryption=<recorded-mode> <new-repo>`; `borg create --stats <new-repo>::<new-archive> <source>`; unchanged and changed second archives are separate; `borg check --verify-data <new-repo>`; `borg list <new-repo>`; from a new ext4 target, `borg extract <new-repo>::<archive>` |

Raw rows use the source directory. Canonical rows use a stopped/checkpointed
database plus external assets. Restic/Borg integrated packing, compression,
deduplication, and encryption remain marked `integrated`; they are not reported
as zero-cost Notrios-equivalent stages. Google Drive measurements copy a closed
artifact to a new provider folder and observe visibility separately; restore is
never performed in the mapping. No pass is called cold because caches were not
controlled.

## Privacy and recovery

Results include counts, bytes, directory histograms, timings, resource usage,
tool/environment versions, and generated artifact hashes. They contain no
paths, filenames, titles, bodies, source keys, warnings, or command arguments.
The privacy validator rejects those fields and local path-like strings. Private
G14b hashes/results stay in its external workspace; only reviewed aggregate
summaries may be committed.

The external generated workspace for the committed calibration retains all 18
phase results and their checkpoints. No private corpus was read. The compact
committed copy is `calibration-results.json`.
