# G14b full-corpus physical snapshot investigation

This directory contains the aggregate-only contract and reviewed
summary for G14b. Private source paths, databases, archives, repository
contents, command logs, and detailed phase rows stay in an external ext4
workspace. G14b changes no production format, default, schema, dependency, or
catch-up behavior.

## Result

G14b selected option B: a required, compatible same-schema SQLite image plus
bounded packed external assets for whole-library full backup/catch-up. Packed
semantic archive-v2 remains required for subset, merge, schema-independent
interchange, and fallback. On the 382,206-document recipe workload, the image
path completed the exact local create-through-restore boundary in 1,884.2
seconds versus 4,384.0 seconds for packed semantic archive-v2. See
`FINDINGS.md`, `SQLITE_IMAGE_CAPABILITY.md`, and the 57 sanitized rows in
`full-corpus-results.json`.

Validate the committed summary independently:

```bash
python3 performance/v0.7-g14b/validate_evidence.py
```

## Harness

Build the pinned local CLI/helper and run one immutable phase:

```bash
bash scripts/run_archive_full_corpus_benchmark.sh \
  --workspace <external-ext4-workspace> \
  --workload recipe-joplin \
  --adapter archive-v2-pack \
  --phase snapshot-create
```

Source inventory and foreign import additionally take `--source-root` and
`--source-kind joplin|obsidian`. Each invocation first publishes a `started`
checkpoint. A completed aggregate result is linked into place without replace;
rerunning returns it without repeating work. Private command output goes only
to the workspace log for that phase.

The full boundaries are inventory, foreign import, snapshot create, snapshot
verify, transport prepare, transport seal, snapshot open, snapshot restore,
unchanged second snapshot for repository tools, corruption refusal, and an
encrypted closed-artifact provider copy. Passes are labelled
`interleaved-first`, `interleaved-repeat`, or `uncontrolled`; none is called
cold.

Validate every published row:

```bash
bash scripts/run_archive_full_corpus_benchmark.sh \
  --validate-results <external-ext4-workspace>
```

The recipe imports must be compared before either becomes the canonical
snapshot workload:

```bash
bash scripts/run_archive_full_corpus_benchmark.sh \
  --compare-imports <external-ext4-workspace>
```

The comparison strips one valid leading source-metadata YAML block, projects a
user-visible leading H1 as the semantic title when present, and hashes the
complete multiset of semantic title/body/deletion tuples. It also requires the
visible body multiset and byte total to match independently. Source-specific
stored titles, notebook/tag/provenance counts, and raw database fingerprints
are reported separately and are not falsely required to match across Joplin
and Obsidian.

## Candidate meanings

- `archive-v2-loose` and `archive-v2-pack` use the production exporter,
  verifier, restore semantics, stored-ZIP catch-up wrapper, and NBK1 framing.
- `sqlite-stopped-copy` checkpoints and closes the isolated source before a
  byte copy; it is never a raw live-WAL copy.
- `sqlite-online-backup` uses SQLite's Online Backup API for the image and
  binds a loose external-object tree behind a versioned manifest.
- `sqlite-image-bundle` uses the Online Backup API plus a deterministic bounded
  tar of external resources/source bundles behind the same manifest.
- Restic repositories are always newly initialized with a private workspace
  password, then checked with `check --read-data`.
- Borg repositories are always newly initialized with `repokey-blake2`, then
  checked with `check --verify-data`.

Restic and Borg raw rows back up the immutable source tree. Canonical rows back
up a checkpointed Notrios database plus its external objects. First and
unchanged snapshots are separate rows, because repository deduplication is not
full-backup creation speed. No command ever points at the existing
`recipedb_repo`.

SQLite-image restores install only into a new ext4 destination, verify the
database and external manifest, then mint a new replica identity. They remain
prototypes: G14b selected the state-class contract in
`SQLITE_IMAGE_CAPABILITY.md`; G14c must review the local-only/derived tables and
implement their exclusion or bounded rebuild before the capability becomes
production.

## Provider boundary

Google Drive evidence copies an already sealed, closed artifact into a new
G14b provider folder, waits for size/hash visibility, and records that copy as
a distinct phase. It never restores into the FUSE mapping and never reports
provider latency as archive creation, verification, or restore time. The host
has no exFAT target, so this investigation makes no measured exFAT claim.
