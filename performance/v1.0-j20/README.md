# v1.0 J20: a fresh J5 baseline, and the rest of the Obsidian inventory memory

## J20-A: the baseline under the current code

`baseline_profile.sh` is J5's `scale_profile.sh` with user and system CPU
recorded. It runs the same CLI import into a new library on the same HDD, then
the same three search probes, cold and warm. The runs:
- import the Obsidian vault first, then the Joplin RAW export, one at a time
- run with nothing else on the machine, from a clean tree at `17aa250`
- read from a warm page cache, since there is no passwordless `sudo` to drop it

*Runs in progress; results follow.*

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
