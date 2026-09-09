# G14c production physical-snapshot evidence

G14c implements `sqlite-image+packed-assets.v1` for compatible same-schema,
whole-library snapshots. The committed evidence is generated and aggregate
only. No private corpus, filename, path, content hash, database, or asset is in
this directory.

Run the ordinary tests with `go test ./internal/snapshotimage`. Run the opt-in
100k gate with:

```bash
bash scripts/run_snapshot_image_profile.sh /tmp/notrios-snapshot-image-100000.json
```

The format adds no compressor and no dependency. SQLite remains the existing
system library; packs use Go's standard-library USTAR reader/writer.
