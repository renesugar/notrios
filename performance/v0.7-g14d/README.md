# G14d scalable restore and catch-up evidence

G14d makes `sqlite-image+packed-assets.v1` the encrypted whole-library REST
catch-up payload and adds explicit, crash-safe physical `replace`/`adopt`.
Packed semantic archive-v2 remains the subset, merge, portable interchange,
incompatible-schema fallback, and fork path.

The committed evidence is synthetic and aggregate-only. It contains no private
content, database, artifact, pathname, filename, content hash, or key.

Run the ordinary coverage with:

```bash
go test ./internal/snapshotimage ./internal/syncbackup \
  ./internal/synccarrier ./internal/syncrest
```

Run the opt-in generated 100k restore gate with:

```bash
bash scripts/run_snapshot_restore_profile.sh \
  /tmp/notrios-snapshot-restore-100000.json
```

`RECOVERY_STATE_MACHINE.md` records the durable stages and recovery contract.
`ANDROID_EMULATOR_CHECKLIST.md` records the deferred mobile validation without
mislabeling desktop measurements as emulator or physical-device results.
