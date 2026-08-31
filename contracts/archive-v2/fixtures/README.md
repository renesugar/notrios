# Archive-v2 public fixtures

These fixtures are sanitized, deterministic contract examples made from an
invented one-of-each-record corpus. They contain no database, private export,
or secret. `loose-schema12`, `packed-schema12`, and
`sync-era-schema27-packed` are complete archives generated independently of
the production exporter; all three pass `VerifyDirectory`. The schema-27
fixture proves that the sync era did not create archive-v3 or add sync-wire
records to the portable archive contract.

The remaining files are narrow admission/refusal probes and deliberately do
not claim to be complete archives. G9's deterministic NCB1/NEV1 sync-wire
vectors remain separately published at `../sync-wire/goldens.json`;
NAR1 uses fresh production entropy and is not mislabelled as a deterministic
archive fixture.

`physical-refusal/manifest.json` is deliberately only a sanitized manifest
shape. It identifies the SQLite-image format and its semantic fallback, so a
portable archive-v2 consumer must refuse it without opening a database image.
Its hashes and descriptor are invented, and `notes.sqlite` and pack bytes are
intentionally absent.
