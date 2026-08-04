# v0.4 P3a archive-v2 large-library container evidence

Aggregate-only evidence that the revised container can archive the corpora this
project is actually tested against. No note titles, bodies, resources, paths,
databases, or archives are committed.

Before P3a, `manifest.json` listed every object inline at roughly 645 bytes per
descriptor. The 4 MiB manifest bound — not the nominal 10,000-object limit —
capped an archive near **6,500 objects, about 6,400 single-revision notes**, so
neither supplied corpus could be archived at all.

## Real corpora (Intel i5-9300H, Go 1.26.5, linux/amd64, NVMe)

Both corpora hold the same 382,206 notes in different source formats.

| | Joplin RAW | Obsidian vault |
|---|---|---|
| Source items | 1,237,553 | 382,206 Markdown files |
| Notes imported | 382,206 | 382,206 |
| Notebooks | 781 | 780 |
| Tags / tag relations | 11,753 / 842,813 | 0 / 0 |
| Objects archived | 382,407 | 382,321 |
| Index chunks | 39 | 39 |
| **Manifest bytes** | **16,212** | **16,203** |
| Archive bytes | 1,142,623,391 | 1,145,497,934 |
| Export + verify | 48m 26s | 1h 02m 27s |
| Peak RSS | 298 MiB | 331 MiB |
| `verified` | true | true |

Every archived count matches its import exactly. `full_backup` is true for both.

The manifest stays ~16 KB at 382k objects because it lists 39 index chunks
rather than 382,407 objects. The inline form would have needed roughly
**247 MB**, about 59× over the 4 MiB bound.

Peak RSS is whole-process and is dominated by the selection planner's document
identity set, not by the archive: object hashes, index entries, and record
identities all stream through external-sorted spools.

### Coverage limits, stated plainly

Neither recipe corpus carries resources or source bundles
(`resources: 0`, `source_bundles: 0`), and the Joplin import ran without
`--preserve-source`. The blob path is therefore exercised at 382k scale by note
bodies only. Resources and exact source bundles are covered by the synthetic
tiers and by the smaller attachment-bearing Joplin archive (111,330 items, 763
resources), not at this scale.

The Obsidian corpus produced no tags because the vault stores none in the form
this importer maps; the Joplin corpus covers tag relations at 842,813.

## Throughput is bound by file operations, not bytes

The Joplin export moved 1.14 GB in 48m26s — about 131 objects per second — and
the Obsidian export 1.15 GB in 1h02m. Cost tracks one
`create + write + fsync + rename` per object rather than the bytes copied.

That is poor for a backup, and worse for v0.7 sync, which carries this
container over REST and folder/rclone transports where every object becomes a
round trip. This measurement resolved `agent/OPEN_QUESTIONS.md` question 18 in
favor of a packed layout, scheduled as plan task **P3b**. The discriminated
`location` field this slice introduced is what allows that to arrive as an
optional capability rather than another format break.

## A bug only real data could find

The exporter inherited `PlanSelection`'s default document cap, which is the
REST/MCP-facing 100,000. A 382,206-note full backup was refused outright with
`selection exceeds max_documents=100000`. Export is an in-process Store caller
and now defaults to the direct planner bound. Every synthetic tier was at or
below 100,000, so no generated dataset would have surfaced it.

## Reproducing

The corpora are private and not committed. With an imported database:

```bash
notriosctl export archive-v2 --db <db> --asset-store <assets> <out-dir>
```

Generated tiers, which do cover resources:

```bash
bash scripts/run_archive_export_profile.sh 100|1000|5000|100000 <output.json>
```
