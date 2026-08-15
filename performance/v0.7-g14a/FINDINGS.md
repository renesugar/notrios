# G14a generated calibration findings

The generated 10k and 100k tiers completed every phase of the current loose
archive-v2 catch-up path on the same ext-family volume. All 18 phase results
passed privacy and arithmetic validation, source metadata was unchanged, exact
archive/wrapper hashes matched, restore matched P4's content-bearing canonical
aggregate, and one post-snapshot journal change converged through the directory
carrier.

## The harness is resumable at the useful boundary

Each phase owns an atomic checkpoint and an immutable result. A deliberately
interrupted unit-test phase resumed as attempt two; rerunning a completed real
inventory returned the original row without invoking work. The real 10k restore
also exercised retry after the first fingerprint design proved invalid. Earlier
completed phases were neither repeated nor overwritten.

## The loose baseline is correct but fails two acceptance defaults

| Phase | 10k | 100k | 100k peak RSS | 100k / 10k time |
|---|---:|---:|---:|---:|
| Foreign import | 118.3 s | 1,063.8 s | 100 MiB | 8.99× |
| Snapshot create | 39.4 s | 322.4 s | 92 MiB | 8.18× |
| Verify | 25.9 s | 196.6 s | 76 MiB | 7.60× |
| ZIP preparation | 7.6 s | 48.2 s | 96 MiB | 6.36× |
| Seal | 2.5 s | 17.4 s | 84 MiB | 6.97× |
| Open/extract/verify | 24.5 s | 218.1 s | 87 MiB | 8.91× |
| Restore | 103.1 s | 831.3 s | 87 MiB | 8.06× |
| One incremental replay | 69.7 s | 621.3 s | **761 MiB** | 8.91× |

Observed time stayed sublinear relative to the 10× tier increase and every
phase finished within two hours. Snapshot open and restore stayed below the
256 MiB receiver proxy. However:

- Loose export produced 10,030 files at 10k and **100,093 files at 100k**.
  Two-level fanout bounded one directory at 256 entries, but total archive and
  ZIP entry counts still grow one-for-one with notes. The current loose
  transport baseline therefore fails the object-per-file acceptance rule.
- Incremental replay reached **797,937,664 bytes peak RSS**, above the 512 MiB
  desktop threshold. Only two carrier artifacts were involved, so this is
  local synchronization-state work rather than directory transport. G14b must
  retain this failure in the baseline; G14a does not optimize production sync.

## ZIP, not encryption, supplies the byte overhead

At 100k, the archive held 235,226,035 apparent bytes. Stored ZIP preparation
produced 261,450,355 bytes: +26,224,320 bytes, or 11.15%, across 100,093
entries. NBK1 sealing then added only 6,004 bytes. The 10k tier independently
showed 11.17% ZIP overhead and 604 bytes of framing. This confirms the G14
hypothesis without claiming that G14a selected the packed layout.

## One fingerprint premise was false

The first restore check compared re-export record-object sets. That is invalid
between an archive source and its restored writable copy because restore must
rotate replica identity, which legitimately changes snapshot-bound records.
The corrected check uses P4's established aggregate: complete canonical counts,
total blob bytes, and ordered exact blob/source-bundle content hashes. The
failed phase left its checkpoint at `started`; attempt two replaced the target,
matched that aggregate, and atomically published the result.

## Scope of the evidence

This is generated calibration, not the G14b candidate decision. It proves stage
boundaries, result recovery, privacy checks, arithmetic, exact hashes, semantic
restore, and scale instrumentation. It does not compare packed archive-v2,
SQLite images, restic, borg, Google Drive, the supplied private corpora, or
controlled cold caches. Those remain G14b.
