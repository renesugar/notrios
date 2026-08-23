# G14b full-corpus physical snapshot findings

## Decision: Option B

Select **Option B** for G14c: add a required, versioned SQLite-image capability
for same-schema full backup and synchronization catch-up, bind a deterministic
packed external-assets/source-bundle payload, and retain packed semantic
archive-v2 for subset export, merge, schema-independent interchange, and
fallback recovery. G14b changes no production format, dependency, schema,
default, encryption, or catch-up behavior.

The decisive recipe-corpus comparison recovered the same 382,206 documents and
passed SQLite integrity, complete canonical fingerprint, exact container, and
source-unchanged checks:

| ext4 phase | Packed archive-v2 | SQLite image + packed assets |
|---|---:|---:|
| Create | 625.9 s / 287.5 MiB | 164.5 s / 30.9 MiB |
| Verify | 441.0 s / 73.0 MiB | 381.6 s / 31.4 MiB |
| ZIP preparation | 37.5 s | 154.6 s |
| Authenticated seal | 40.2 s | 167.8 s |
| Open and verify | 447.2 s / 72.7 MiB | 499.7 s / 31.4 MiB |
| Restore and verify | 2,792.1 s / 196.9 MiB | 516.2 s / 202.6 MiB |
| **Local total** | **4,384.0 s** | **1,884.2 s** |
| Sealed bytes | 1.309 GB | 6.040 GB |
| Google Drive copy + exact reread | 73.7 s | 360.9 s |
| **Total including provider evidence** | **4,457.6 s** | **2,245.1 s** |

The physical path is 2.33x faster locally and 1.99x faster with this provider
copy included. It also writes far less during restore: semantic reconstruction
wrote 28.4 GB to produce a 2.93 GB database, whereas physical install plus
identity rotation wrote 6.04 GB while preserving the source's 6.04 GB
transactional image. The semantic archive is 78.3% smaller on the wire, which
is why option B retains it instead of replacing it.

## Import equivalence was semantic, not source-format fiction

The two recipe imports each contain 382,206 documents. After stripping one
valid leading source-metadata YAML block and using the user-visible leading H1
as semantic title, their title/body/deletion multiset and independent visible
body multiset match exactly; each has 451,964,499 visible body bytes. The raw
database fingerprints do not match. Obsidian has 3,682 stored filename-title
differences, and Joplin alone carries its source notebooks and tags. Those are
reported distinctions, not hidden by declaring the raw imports identical.

## Attachments make a bound external pack mandatory

The attachment workload imported 103,349 documents, 758 resource records, 731
deduplicated blobs, and 111,330 preserved source-bundle items. Five referenced
resource bodies were absent in the source and remained an explicit import
report condition. The image bundle packed 112,062 external files behind one
deterministic tar and restored their complete aggregate exactly. Create used
135.8 MiB peak RSS; open used 33.0 MiB; restore used 158.8 MiB. The complete
sealed artifact was 2.324 GB. A loose-assets SQLite image is therefore not the
selected capability even when its database snapshot mechanism is valid.

## Rejected and retained alternatives

- Loose archive-v2 remains readable but is disqualified as the physical
  transport because its entry count scales with semantic objects. Packing
  bounds the same recipe archive to 46 files and at most 42 entries in one
  directory.
- Stopped copy and SQLite Online Backup creation were effectively tied at
  157.6 and 162.8 seconds. Online Backup is the production-safe live-source
  mechanism; a raw live-WAL file copy is never valid.
- Packed semantic archive-v2 passes every absolute time and memory gate and is
  retained. Its record-level restore cost, larger write amplification, and
  interrupted-writer restart behavior make it the fallback rather than the
  same-schema full-snapshot default.
- A deliberately interrupted packed export preserved its partial tree and
  restarted. G14c must define bounded pack publication/resume; it must not
  claim the current writer resumes a partial pack.
- The existing post-snapshot incremental replay converged, but took 3,039.6
  seconds, reached 3.22 GiB peak RSS, and wrote 31.6 GB. It fails the 512 MiB
  desktop gate independently of snapshot representation. G14d must stream or
  batch that replay rather than carrying this prototype behavior forward.

## Repository references

Restic and Borg provide useful chunking, encryption, repository indexes,
deduplication, retention, and data-check tools. Their first and unchanged
snapshots are measured separately below; an unchanged snapshot is not reported
as full-backup creation speed. Raw restore still recreates every source file and
directory, while canonical backup can exploit the small number of Notrios
state files. Neither tool can substitute Notrios capability admission,
same-schema compatibility, external-object binding, derived-state rebuild,
replica rotation, or semantic subset/merge behavior.

| Recipe input / phase | Restic | Borg |
|---|---:|---:|
| Canonical first snapshot | 183.8 s / 153.2 MiB | 161.9 s / 94.4 MiB |
| Canonical data check | 134.4 s / 57.1 MiB | 97.4 s / 96.3 MiB |
| Canonical unchanged | 1.4 s / +357 B | 97.7 s / +1,480 B |
| Canonical exact restore | 298.3 s / 95.5 MiB | 269.0 s / 96.5 MiB |
| Raw first snapshot | 515.2 s / **2,255.1 MiB** | 1,137.2 s / **967.6 MiB** |
| Raw data check | 186.2 s / **2,270.1 MiB** | 319.5 s / 429.2 MiB |
| Raw unchanged | 235.0 s / **3,300.9 MiB**, +332 B | 699.5 s / **805.7 MiB**, +58,203 B |
| Raw exact restore | 2,134.3 s / **2,882.1 MiB** | 2,783.8 s / **579.0 MiB** |

Both repositories restored the raw 1,237,553-file tree exactly, and Restic
`check --read-data` plus Borg `check --verify-data` passed. Their repositories
bounded physical storage to 72 and 14 files respectively, but raw creation,
unchanged traversal, and restore repeatedly failed memory limits. Raw restore
took 4.13x (Restic) and 5.39x (Borg) the image restore time even though the raw
source held only 1.425 GB apparent bytes versus the 6.040 GB canonical image.
Canonical repository rows were fast and bounded, but they still lack the
application-level manifest/admission and identity semantics required by option
B; they remain useful external backup references rather than the native format.

Repository timing and exact-restore rows are recorded in
`full-corpus-results.json`; detailed private repositories and logs remain in
the external evidence workspace.

## Import scale remains separate performance debt

The imports were required to establish equivalent source-format views, not to
select a physical snapshot. They exposed a distinct limitation: recipe Joplin
took 8,158 seconds and 2.82 GiB peak RSS, recipe Obsidian took 9,861 seconds and
1.65 GiB, and the attachment Joplin import took 1,611.8 seconds and 608.6 MiB.
The two recipe imports exceed the two-hour stage gate and all three exceed the
512 MiB desktop proxy. G14b records those failures without changing importer
code or letting them decide A/B/C; they require a separately planned bounded-
import follow-up.

## Scope and G14c/G14d work

These measurements are on one ext4 host plus a Google Drive FUSE mapping. There
is **no measured exFAT claim**, no physical-mobile claim, and no universal cloud
filesystem claim.

G14c must implement the capability contract in `SQLITE_IMAGE_CAPABILITY.md`:
consistent Online Backup image; exact schema/application range; manifest-bound
database and bounded external packs; reviewed local/transient exclusions;
derived-state rebuild rules; private keys kept outside the image; complete
hash/integrity/capability verification before writes; replica rotation; and
semantic archive fallback. G14d must integrate authenticated framing, emergency
backup, crash-safe staging/cutover, and bounded post-snapshot replay. G14e must
repeat the full-scale acceptance with production code.
