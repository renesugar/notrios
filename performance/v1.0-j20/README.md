# v1.0 J20: a fresh J5 baseline, and the rest of the Obsidian inventory memory

## J20-A: the baseline under the current code

`baseline_profile.sh` is J5's `scale_profile.sh` with user and system CPU
recorded. It runs the same CLI import into a new library on the same HDD, then
the same three search probes, cold and warm. The runs:
- import the Obsidian vault first, then the Joplin RAW export, one at a time
- run with nothing else on the machine, from a clean tree at `17aa250`
- read from a warm page cache, since there is no passwordless `sudo` to drop it

**Imports.** Both completed with J5's counts: 382,206 notes each, 780 and 781
notebooks, 11,753 Joplin tags, and checkpoints `completed`.

| 382,206 notes | J5 wall | J20-A wall | J20-A user / sys | J5 peak RSS | J20-A peak RSS |
|---|---|---|---|---|---|
| Obsidian vault | 16,261.5 s (4.52 h) | **5,245.5 s (1.46 h)** | 4,881.3 s / 389.4 s | 1,720.5 MiB | **1,197.0 MiB** |
| Joplin RAW | 6,194.1 s (1.72 h) | **5,907.9 s (1.64 h)** | 5,235.6 s / 799.9 s | 2,880.7 MiB | **290.7 MiB** |

- **Obsidian: 3.1× faster and 30% less peak memory.** J17-B batched the
  commits, J18 removed the full-text scan from writes, and J19 cloned the
  inventory's substrings. This is the first full-corpus measurement of those
  changes together. It is not an attribution: they were not measured
  separately at this size.
- **Joplin: 4.6% faster and 90% less peak memory.** The memory matches J19's
  inventory measurement: 1,301 MiB of the old peak was the inventory pinning
  every note's file. The time is close to J5's, as expected, because the Joplin
  importer already batched.
- **The J5 asymmetry has reversed.** The Obsidian import is now 11% faster than
  the Joplin import of the same notes, where J5 found it 2.6× slower.
- **Provenance.**
  - The Obsidian run records commit `17aa250`. The Joplin run records `7c04d12`,
    because the J20 README was committed while it ran.
  - That commit changed only this README. Both runs used one `bin/notriosctl`,
    built before either started.
  - Each run shows zero tracked changes.

**Search, measured after each import: slower than J5, not yet explained.**
See below.

## J20-B: what the remaining Obsidian candidates must preserve

These come from reading how the importer uses each `vaultFile` field, before
anything is changed.

- **`Fingerprint` (hex SHA-256 of the file) must keep its exact text wherever
  it is compared or composed.**
  - It is written as hex into the inventory fingerprint hash
    (`ItemKey + "\x00" + Fingerprint + "\x00"`), which decides whether a
    checkpoint resumes.
  - It is composed as hex into every note fingerprint (`noteFingerprint`).
  - It is compared with the hex `SHA256` of stored resources and bundle items,
    and with `sha256Hex(raw)` in `readExact`.

  Holding it as `[32]byte` is possible only if every one of those uses encodes
  it back to the identical hex. The inventory fingerprint and item-state
  comparisons prove whether that holds.
- **`FrontmatterSHA` is read once**, into a note source's `metadata_json`. It
  could be held as bytes and encoded there.
- **`AbsPath` is `root` joined with `RelPath`**, and is read only to open files.
  It could be derived instead of stored.
- **`Files` is read only by the source-bundle phase**, and when target IDs are
  copied across after source-ID lookups. Notes appear in both `Files` and
  `Notes` as full struct copies. `Files` needs the order sorted by `RelPath`,
  and a note's `TargetID` must match in both.

No candidate is adopted until `TestJ19InventoryMemory` shows held memory falling
on the full vault. Each must also pass the J17 library comparison, including
item states, and keep the inventory fingerprint identical.
