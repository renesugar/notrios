# v0.4 J3 transactional import evidence

This directory contains aggregate-only synthetic and real-corpus performance
evidence for J3. Source paths, filenames, note titles, note bodies, resource
bytes, and imported SQLite databases are intentionally excluded.

## Reproduce

```sh
scripts/run_joplin_import_profile.sh 100 performance/v0.4-j3/synthetic-100.json
scripts/run_joplin_import_profile.sh 10000 performance/v0.4-j3/synthetic-10000.json
scripts/run_joplin_import_profile.sh 100000 performance/v0.4-j3/synthetic-100000.json
scripts/run_real_joplin_profile.sh <label> <raw-dir> /tmp/joplin-dry-run.json
scripts/run_full_joplin_import_profile.sh <label> <raw-dir> /tmp/joplin-full.json
```

The generated tiers all run planning, a real interruption/resume, the final
link pass, and a full revision-stable no-op. The 100k tier completed planning
in 46.0 seconds, canonical interruption/resume in 5m32s, and no-op in 3m08s;
peak process RSS was 174,960 KiB and the temporary manifest was 123,051,400
bytes. These are machine/filesystem measurements, not product SLOs.

## Private recipe corpus

`recipe-full-import.json` covers 1,237,553 RAW items: 382,206 notes, 781
notebooks, 11,753 tags, and 842,813 note-tag relations. The batch size was 500.
The run intentionally stopped after the first 500-note atomic transaction,
resumed to completion, verified search results, and then classified all 382,206
notes unchanged without adding revisions.

- intentional interruption: 7m19s;
- complete resume plus final links: 32m22s (196.8 notes/s including inventory);
- complete no-op plus final links: 18m17s;
- total workflow: 58m11s;
- SQLite database: 3,101,188,096 bytes, 382,206 documents/revisions/FTS rows/
  provenance rows, 842,813 tag relations, WAL, 4096-byte pages,
  `synchronous=2`, foreign keys enabled;
- temporary manifest: 838,699,456 bytes;
- peak RSS: 2,983,720 KiB; Go system memory: 3,037,322,568 bytes;
- hardware: Intel Core i5-9300H, 8 logical CPUs, 65,684,572 KiB RAM, Linux
  amd64, Go 1.26.5.

The separate final dry-run profile completed in 550.497 seconds with 2,809,924
KiB peak RSS, down from J2's 3,769,552 KiB whole-inventory baseline. The indexed
manifest removes whole note/note-tag inventory retention, though transient
parsing/runtime allocation still dominates the measured peak.

The real-scale no-op investigation also found and fixed a correlated global tag
count in the document-tag batch query: the old 500-document query took 54.32
seconds on the 3 GB database; the membership-only batch query took 0.42
seconds. Count-bearing user list APIs remain unchanged.
