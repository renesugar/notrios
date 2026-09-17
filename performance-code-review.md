# Performance Code Review: movenotes-v3

**Repository reviewed:** Notrios (`/home/renes/projects/notrios`).  
**Review snapshot:** 2026-09-17. **Reviewer:** Union Alpha.  
**Scope:** Joplin RAW and Obsidian import, their canonical SQLite write path, and shared Markdown processing.

The requested heading is retained, but this repository is Notrios, not movenotes-v3. Its production importers are **Go**, not Python scripts. Python examples below are explicitly illustrative before/after equivalents; they are not quotations from this repository or proposals to rewrite it in Python. Go changes belong behind the existing Store interface, never in direct importer SQL.

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current". This is a dated review snapshot, not the implementation plan. Supersede it rather than silently updating its historical measurements.

## 1. Executive Summary

The importers already have substantial, measured optimization work: bounded canonical transactions, batched lookups, resumable checkpoints, streamed resource input, and fixes for retained substrings. Joplin additionally spools note metadata and note/tag joins to indexed temporary SQLite; Obsidian retains an in-memory namespace to resolve paths, names and aliases. Neither importer is fully constant-memory.

**Do not begin by adding transactions, enabling WAL, compiling regexes, pooling hash buffers, or moving FTS to a background worker.** Transactions, WAL and compiled regexes already exist; J19 measured buffer pooling as ineffective, and the owner explicitly chose to keep FTS inline so a committed note is searchable.

### Recorded results, not measurements made by this review

| Evidence / workload | Result | Interpretation |
|---|---|---|
| [J17, corrected clean runs](performance/v1.0-j17/README.md), 10,000 Obsidian notes, HDD, warm source cache | Old per-note commits: mean 29.1 ms/note; batched: 13.2 ms/note, about 2.2× | Already implemented. The earlier 1.46× result was contaminated by concurrent work. |
| [J20-A](performance/v1.0-j20/README.md), 382,206 notes | Obsidian 5,245.5 s / 1,197.0 MiB peak RSS; Joplin 5,907.9 s / 290.7 MiB | Obsidian was about 11% faster, reversing J5's older comparison. Different source formats still perform different work. |
| [J20-C](performance/v1.0-j20/README.md), full Obsidian run after inventory compaction and J21 | 5,278.2 s / 842.6 MiB peak RSS, approximately 72.4 notes/s | Memory improved; no demonstrated import-time improvement over J20-A. One full run per configuration. |
| [J19 profile](performance/v1.0-j19/README.md), 10,000 Obsidian notes | SQL preparation 35% of CPU samples; links/blocks rebuild 50%; block extraction 51% of total allocation | Cumulative shares overlap. They cannot be added. These are historical profile samples, not current wall-time fractions. |

**Top recommendations:**

1. Reuse prepared statements in repeated link/block inserts, taking the already-working manifest insertion loop as the local model.
2. Stop collecting Obsidian property-order arrays when source preservation is off; investigate high-collision name buckets and avoid needless whole-document reads for existence-only checks.
3. Replace repeated whole-body link splicing and repeated prefix scans for link coordinates with bounded, linear-pass processing, preserving all existing parser semantics.
4. Investigate avoiding the second block rebuild without compromising the invariant that blocks and links describe the same revision.
5. Improve no-op work and checkpoint placement using Joplin's existing patterns, with explicit compatibility and crash tests.

**Expected total impact:** no defensible combined speedup is available without new experiments. A planning scenario that removes half of the historical SQL-preparation CPU cost implies a 17.5% CPU reduction (about 1.21× CPU throughput), not a guaranteed wall-time gain. Property-order omission could reclaim tens of MiB on property-rich large vaults. Link-dense notes could benefit much more from removing O(K·B) work than the recipe corpus, which had only two indexed links in J17's 10k sample. Do not multiply independent estimates or promise 10×–50× import speedups.

## 2. Optimizations Comparison Matrix

**Impact labels distinguish implemented historical results from unmeasured candidates.** B = body bytes, K = links in a note, N = note count, D = notebook depth.

| Optimization Technique | `Joplin` Status | `Obsidian` Status | Cross-Applicable? | Estimated Impact |
| :--- | :--- | :--- | :--- | :--- |
| Canonical note transactions | Implemented, 100 default / 500 maximum | Implemented through the same store batch API | Already ported Joplin → Obsidian | Historical ~2.2× at 10k, not remaining gain |
| Final link rebuild transactions | Implemented | Implemented | Already shared | No missing-transaction finding |
| Scoped bulk reads / item-state writes | Implemented | Implemented | Already shared | Avoids per-note round trips; row work remains |
| Prepared-statement reuse | Manifest `Put` resets/rebinds one statement; canonical inserts do not | Canonical inserts prepare/finalize repeatedly | Manifest → shared Store | High-priority CPU candidate; historical preparation share 35% |
| Minimal no-op provenance/state writes | `SkipSource` / `SkipState` under guarded conditions | Flags not set by importer | Joplin → Obsidian, with stronger metadata equivalence checks | Removes up to two mutation calls per stable note; time unmeasured |
| One checkpoint commit per canonical batch | Counters advanced before atomic notes/links write | Atomic checkpoint followed by another checkpoint save | Joplin → Obsidian | Up to 50% fewer commits in those phases, not 50% less import time |
| Indexed disk inventory / keyset pages | Implemented for notes, bundles and note/tag joins | Notes/assets/namespace retained on heap | Conditional Joplin → Obsidian spike | Large-memory potential; extra disk queries can hurt throughput |
| Bounded directory enumeration | `ReadDir(1024)`, no full directory name sort | `filepath.WalkDir`, sorted directory enumeration | Conditional Joplin → Obsidian | Reduces peak enumeration memory on huge flat vaults; ordering compatibility required |
| Inventory hash from already-read Markdown | Implemented | Hash stream followed by a separate `ReadFile` | Joplin → Obsidian | Removes one Markdown inventory read, not canonical verification |
| Substring detachment | Lean parser clones retained fields and IDs | Clones titles, aliases, properties, relative paths | Already applied both ways | J19 Joplin inventory 1,301.4 → 51.9 MiB, historical |
| Derived paths/item keys and raw digest storage | Strings/paths retained in manifest/resource representations | Implemented, `[32]byte` digests | Selective Obsidian → Joplin | Smaller per-resource/batch footprint; likely modest for note-only Joplin |
| Conditional preserve-source inventory | Bundle manifest and ordered properties conditional | `Files` conditional; `PropertyOrder` still always built | Joplin → Obsidian | J20 already saved 80 MiB omitting `Files`; additional properties are a new candidate |
| Single-pass rewrite output | `strings.Builder` | Repeated reverse-order string splices | Joplin → Obsidian | O(K·B) copying → O(B + replacement bytes), plus sort |
| Precompiled regex / literal scanning | RAW metadata and Joplin targets use literal scans; shared parsers precompiled | Shared link/block regexes precompiled; frontmatter literal scans | Do not replace grammars blindly | No regex-compilation quick win remains |
| Resource streaming / unchanged resource skip | Implemented; content rehashed before store upload | Implemented; same additional verification | Already shared | Bounded attachment memory, but multiple read passes |
| Existing FTS rowid lookup | Shared rowid map avoids full index scan on update/delete | Same | Already shared | J18's large update improvement is not a new finding |
| Bounded parallel parse pipeline | Not implemented in reviewed import path | Not implemented | Both, only after profiling | Conditional; database work serializes, HDD reads can worsen |

## 3. Cross-Pollination Analysis

### 3.1 Applying `Joplin` import Optimizations to `Obsidian` import

#### A. Assemble rewritten text once

**Evidence:** `joplinraw/joplinraw.go:411–460` uses a grown `strings.Builder`; `obsidian/obsidian.go:1113–1116` sorts replacements descending and concatenates a new body for every replacement.

Retain Obsidian's resolver, anchors, attachment classification and warnings. Change output assembly, not the Joplin `:/id` grammar. For validated non-overlapping ranges, sort ascending and append unchanged spans and replacement text to a builder. `markdownlinks.Extract` returns Markdown and wiki matches in separate groups, not global source order. Mixed/nested match ranges require a defined compatibility policy before shipping; do not silently discard overlaps.

```python
# BEFORE: illustrative Python equivalent of the current splice strategy.
for start, end, replacement in sorted(changes, reverse=True):
    body = body[:start] + replacement + body[end:]

# AFTER: valid, non-overlapping character offsets only.
parts, cursor = [], 0
for start, end, replacement in sorted(changes):
    if not cursor <= start <= end <= len(body):
        raise ValueError("overlapping or invalid ranges")
    parts.extend((body[cursor:start], replacement))
    cursor = end
parts.append(body[cursor:])
body = "".join(parts)
```

Go must use byte offsets (`StartByte`, `EndByte`), not Python character offsets. For a 1 MiB note with 1,000 replacements, the old loop can copy on the order of 1 GiB cumulatively; the builder produces roughly one output-sized allocation. This is an operation-count illustration, not measured peak RSS or a 1,000× end-to-end prediction.

#### B. Do not retain source-only property order by default

**Evidence:** Joplin `scalable.go:435–468` retains ordered properties conditionally. Obsidian `obsidian.go:426–437` always builds/clones them, although the source-bundle phase is their consumer. Keep aliases, title and frontmatter digest because canonicalization/provenance still need those.

```python
# BEFORE (illustrative)
record.property_order = parse_property_order(frontmatter)
# AFTER
record.property_order = (
    parse_property_order(frontmatter) if preserve_source else None
)
```

Go target: guard `cloneStrings(frontmatterPropertyOrder(...))` with `retainFiles`. J20 identifies roughly 72–81 MB of property-order slices in sampled profiles. Treat that as a candidate scale, not a guaranteed saving; measure live heap and preserve-source controls. No source hash composition should change.

#### C. Hash Markdown bytes from the inventory read

**Evidence:** Joplin `scalable.go:429–459` parses and hashes `raw`; Obsidian `obsidian.go:408–437` first streams the file into SHA-256, then reads Markdown again.

```python
# BEFORE (illustrative)
digest = hash_file(path)
raw = path.read_bytes()
metadata = extract_metadata(raw)

# AFTER: bounded read; retain the later canonical-phase revalidation.
with path.open("rb") as stream:
    raw = stream.read(MAX_NOTE_BYTES + 1)
if len(raw) > MAX_NOTE_BYTES:
    raise ValueError("note too large")
digest = hashlib.sha256(raw).digest()
metadata = extract_metadata(raw)
```

Keep streaming hashes for large assets. This removes one open/read pass for Markdown inventory, not the read needed to confirm source bytes during canonical import. Across inventory plus canonicalization, common non-preserving Markdown reads fall from three to two. Warm cache means fewer logical reads may save little physical I/O.

#### D. Port the no-op and checkpoint structure, not just flags

Joplin `scalable.go:1277–1302` suppresses stable source/state mutations; `1316–1328` advances counters before the atomic write. Obsidian `896–1011`, `1178–1240` still writes source/state for stable notes and saves a second checkpoint after the atomic one.

```python
# BEFORE (illustrative orchestration)
commit_batch(mutations, checkpoint(old_counters, next_position))
counters = advance(old_counters, batch)
save_checkpoint(checkpoint(counters, next_position))

# AFTER: proposed counters and rows become durable together.
proposed = advance(old_counters, batch)
commit_batch(mutations, checkpoint(proposed, next_position))
counters = proposed  # publish only after successful commit
```

The shared store's source/state skip guard permits skips only for unchanged actions. For Obsidian, verify expected provenance metadata too: aliases, frontmatter digest, source mapping and target ID. A canonical body match alone must not suppress repair of missing/stale provenance. Preserve action/report semantics; item-state `action` and timestamps currently change on reimport. Do not remove resource attachment reconciliation indiscriminately.

For 382,206 notes at batch size 100, two extra checkpoint saves per batch pair are about **7,646 additional commits** in notes plus links. Eliminating them saves only their measured cost; canonical SQL and parsing remain. Inject failures between commit and progress publication, and verify resumed counters as well as data.

#### E. Consider a disk-backed inventory only after smaller changes

Use Joplin's manifest/keyset design for very large Obsidian vaults only if property omission and compact namespaces do not meet memory targets. Preserve path priority, aliases, case-insensitive collision detection, deterministic IDs and checkpoint fingerprints.

```python
# BEFORE (illustrative)
records = list(all_metadata)
for batch in slices(records, 100):
    process(batch)
# AFTER: conceptual indexed spool; schema-specific API is hypothetical.
for batch in bounded_batches(all_metadata, 500):
    spool.insert_batch(batch)
last_key = ""
while batch := spool.page_after(last_key, limit=100):
    process(batch)
    last_key = batch[-1].key
```

Do not substitute `OFFSET` on every page. Joplin pays one indexed offset lookup when resuming, then returns to keyset pages. Obsidian's ordered inventory hash and collision suffix assignment depend on traversal ordering; an unordered `ReadDir` port cannot reuse the existing fingerprint unquestioned. A pending-directory stack is also not strictly constant memory.

### 3.2 Applying `Obsidian` import Optimizations to `Joplin` import

#### A. Apply compact representations selectively

Obsidian's J20 pattern stores raw digests, derives item keys/absolute paths, and detaches relative paths. Joplin's `inventoryItem` (`scalable.go:40–50`) holds several strings and paths, but most **notes** are already on disk rather than retained as full structs. The best initial target is a resource-heavy inventory, not a blanket Joplin rewrite.

```python
# BEFORE (illustrative persistent in-memory record)
record.digest_hex = hashlib.sha256(raw).hexdigest()
record.absolute_path = str(root / record.relative_path)
# AFTER: keep wire/database encodings unchanged.
record.digest = hashlib.sha256(raw).digest()
# At the existing storage boundary:
stored_digest = record.digest.hex()
absolute_path = root / record.relative_path
```

One digest's payload falls from 64 textual bytes to 32 bytes; Go struct layout, allocator overhead and serialization determine actual savings. Do not change JSON manifest encodings to an array accidentally, or compose fingerprints from raw bytes where existing code composes hex text. J20's 47.5 MiB saving was for Obsidian's two hashes per note, not a transferable Joplin result.

#### B. Carry notebook routing data forward instead of reconstructing it per note

Obsidian retains `NotebookPath` and resolves it through `folderIDs`; Joplin `buildDocumentBody` calls `notebookPath` for every note. That helper prepends each ancestor (`joplinraw.go:551–563`), copying growing slices, while folder planning already calculates cached paths (`scalable.go:663–683`).

```python
# BEFORE (illustrative): repeat for every note.
parts = []
while parent_id and parent_id not in seen:
    seen.add(parent_id)
    parts.insert(0, folders[parent_id].title)
    parent_id = folders[parent_id].parent_id
path = "/".join(parts)

# AFTER: reuse paths computed once for this immutable inventory.
path = source_path_by_folder_id[note.parent_id]
```

Avoid O(D²) prepend copying and repeated traversal for N notes; a simple append/reverse is a smaller first change. Preserve missing-parent, cycle, empty-title and source-vs-renamed-path semantics: the current planner's fallback names are not automatically identical to `notebookPath` output. On shallow recipe notebooks expect small gains; benchmark deeply nested notebooks separately. Cache memory grows with folder count and total path length.

#### Architectural limits on cross-pollination

- Joplin uses source IDs and explicit note/tag relation records; Obsidian must resolve relative paths, basenames and aliases with ambiguity rules. A Joplin ID scanner is not a replacement Obsidian parser.
- Joplin RAW's metadata can trail the body and includes OCR control characters. Preserve physical CR/LF splitting, UTF-8 validation and leading-metadata fallback. Generic Unicode `splitlines()` is not equivalent.
- Both store the whole canonical body for revisions and FTS. Streaming only frontmatter cannot remove the need to process the body.
- Both retain global IDs/names and return `DocumentIDs`; batching does not mean O(1) total memory.
- Obsidian performs legacy source-ID resolution before constructing its namespace. Joplin remaps IDs during batches. Reordering these phases needs identity/forward-reference tests, not merely a timing comparison.

## 4. New Performance Findings & Bottlenecks

### 4.1 SQLite Operations & Transaction Handling

#### F1 — Repeated SQL preparation is the strongest measured remaining CPU lead

**Evidence:** `sqlite.go:2754–2780` prepares/finalizes on each `execPreparedLocked` call. `sqlite_blocks.go:251–264` inserts every block this way; `sqlite.go:2337–2356` does the same for every link. In contrast, `import_manifest.go:111–133` prepares once, resets, clears bindings and rebinds for a batch.

**Recommendation:** start with batch-scoped reusable statements for repeated block/link insert SQL, finalized on all exits while holding the existing store mutex. Prefer this over an unbounded global cache; handle reset/binding/step errors and nested use explicitly. Transaction batching does not imply statement reuse.

```python
# BEFORE: toy equivalent; cached_statements=0 models the Go wrapper's behavior.
con = sqlite3.connect(temporary_db, cached_statements=0)
with con:
    for row in block_rows:
        con.execute("INSERT INTO demo_blocks(id, body) VALUES (?, ?)", row)

# AFTER: Python executemany reuses the statement for the bounded iterator.
with con:
    con.executemany(
        "INSERT INTO demo_blocks(id, body) VALUES (?, ?)", block_rows
    )
```

This is an isolated toy schema, not an instruction to bypass Notrios. In Go use SQLite prepare/reset/clear-bindings/step, as the manifest already does. At 25 blocks/note across 382,206 notes and two rebuilds, there are roughly 19.1 million block insert executions/preparations; statement reuse cuts compilation count, not inserted rows. The 25-block figure is an illustrative scale consistent with J17's sample, not an observed full-corpus mean.

Historical 35% CPU share gives an idealized ceiling of 1/(1−0.35) ≈ 1.54× CPU speedup if all preparation disappeared. It cannot disappear completely; measure a fresh profile and prepare count. CPU share is not SQLite IOPS.

#### F2 — Blocks and links are derived twice for newly changed notes

`ApplyImportDocumentBatch` first rebuilds changed bodies (`sqlite_import_batch.go:124–130`); the final importer pass invokes the same rebuild again. The final pass is needed for forward targets. The shared rebuild also deletes/recreates blocks, whose text does not depend on later target existence.

**Investigation:** retain first-pass transactional visibility; consider a revision-checked final **link-resolution refresh** that reuses current blocks. Do not simply skip the first rebuild or all unchanged-note link work. Namespace changes can make a previously unresolved link resolve without any change to its source note.

```python
# BEFORE (illustrative)
for doc in committed_batch:
    rebuild_blocks_and_links(doc)
# After all targets exist:
for doc in imported_docs:
    rebuild_blocks_and_links(doc)

# AFTER: hypothetical contract, requiring a checked implementation.
for doc in committed_batch:
    rebuild_blocks_and_links(doc)
for doc in imported_docs:
    with store.transaction():
        current = store.get_current_revision(doc.id)
        if store.blocks_describe(current):
            store.refresh_link_resolution(current)
        else:
            store.rebuild_blocks_and_links(current)
```

Potentially removes one of two block parse/write passes on fresh imports, **not half of total import time**. Blocks and links must still describe the same current body under concurrent edits, cancellation and resume. J19's 50% rebuild CPU share and 51% block allocation share motivate the investigation but overlap with F1.

#### F3 — Body-bearing queries used only for existence

`GetDocuments` (`sqlite_imports.go:49–79`) joins revisions and materializes full bodies. Both importers' `currentDocumentIDs` need only presence. Obsidian `processLinkRebuild:1023–1032` loads bodies before the store batch reloads them.

```python
# BEFORE (illustrative)
present = {d.id for d in store.get_documents(ids)}
# AFTER: proposed bounded typed Store API, not currently implemented.
present = store.existing_live_document_ids(ids)
```

Preserve deleted-note filtering and expected collection semantics. For N bodies averaging b bytes, each eliminated pass avoids about N·b bytes of body materialization plus conversion/allocation, although cache hits mean this is not necessarily disk traffic. Do not replace the notes-phase full-body comparison, which determines updates and preserves user edits correctly.

#### F4 — Other phases are bounded in scheduling, not uniformly in writes

Both resource/source-bundle phases call per-item store methods; Joplin also calls `UpsertTag`, and both create/update individual notebooks. `PutSourceBundleItem` streams a copy/hash and then performs a metadata upsert (`sqlite_imports.go:446–533`), including on preservation reimports. Deduplicated content storage does not avoid reading/copying the incoming stream.

Introduce a resource/source-bundle **staging** API only after attachment-heavy profiles justify it. Stage and verify blobs outside the canonical transaction, then commit bounded metadata/state/checkpoint batches. Preserve quarantine, MIME checks, expected hashes and orphan cleanup. Existing notebook/resource APIs have no drop-in bulk equivalent (J17-C).

```python
# BEFORE (illustrative)
for asset in assets:
    store.upload_and_commit_one(asset)
# AFTER: hypothetical staging API, not available today.
staged = [store.stage_and_verify(a) for a in bounded_asset_batch]
try:
    store.commit_staged_batch(staged, checkpoint)
finally:
    store.release_staging(staged)
```

Commit counts could fall from O(asset count) to O(asset count / batch size); file bytes, validation and durable content handling do not disappear. New store API, not a quick importer patch.

#### PRAGMA and index recommendations

| Setting / technique | Current evidence and recommendation |
|---|---|
| `journal_mode=WAL` | Already set in `sqlite.go:196`; retain it. WAL does not create multiple concurrent writers. |
| `foreign_keys=ON`, `busy_timeout=5000` | Already set; retain integrity checks. More waiting is not a throughput optimization. |
| `synchronous` | The cited open path does not explicitly override it. Record actual runtime value with the existing `ImportMetrics` API. Keep durable canonical operation; do **not** recommend `OFF` for user data. |
| `synchronous=OFF` | Only a separately owned disposable spool experiment could justify it, with discard/rebuild on interruption. Never change the canonical connection or reuse a damaged spool. Not recommended as a production change here. |
| `cache_size` | Benchmark the current value against, for example, `-32768` and `-65536` (32/64 MiB target cache). Negative units are KiB. Additional caches compete with the Obsidian namespace and OS cache. |
| `temp_store=MEMORY` | Benchmark only against explicit memory budgets. It affects SQLite temporary structures, not ordinary tables such as `manifest_records`, and is not automatically faster. |
| Defer indexes | Do not drop canonical PK/unique/FK-supporting/link/query indexes during an online import. Joplin's disposable manifest has two indexes maintained during inventory; building them after all inserts is an isolated candidate, preserving readiness before paging and lookup. |
| FTS placement | Keep inline. J19's owner decision rejects delayed visibility; J18's rowid map already fixes document-ID full scans. |

PRAGMA variants should be separate disposable benchmark databases, not toggles on a user's active library. No universal cache-size or index-deferral speedup is claimed. Inspect `EXPLAIN QUERY PLAN`, statement timings, WAL bytes and storage latency before prescribing indexes.

### 4.2 File System I/O & Memory Efficiency

**F5 — Count-bounded batches are not byte-bounded.** Both importers cap source note size at 64 MiB based on inventory metadata, then use `ReadFile` again later. A changed/grown file can allocate before the hash mismatch is rejected. A batch of 500 near-limit notes has a theoretical raw payload near 31.25 GiB, before canonical strings and existing bodies.

Recommend bounded reads on an opened file and a second bound on total batch canonical bytes; allow a single legitimate large note to form its own batch. Keep durable item-position semantics unchanged. Reject oversize input without truncating silently.

```python
# BEFORE (illustrative)
for batch in count_batches(notes, 500):
    mutations = [canonicalize(n.read_bytes()) for n in batch]
# AFTER: proposed byte-and-count-aware batch producer.
for batch in byte_and_count_batches(notes, max_count=100, max_bytes=32 << 20):
    mutations = [canonicalize(read_checked(n, limit=64 << 20)) for n in batch]
```

Helpers above are architectural placeholders; the single-note exception, cancellation and post-normalization size accounting must be implemented. A 32 MiB batch budget is a benchmark candidate, not a new format limit. Existing `GetDocuments` can also return large **stored** bodies and must be considered in the memory bound.

**F6 — Resources incur multiple hash/read passes.** Inventory hashes content, the resource phase rehashes, then upload reads it again. Keep exact validation but investigate staging and hashing the actual upload stream once, comparing its digest **before** committing metadata. A prehash followed by reopen is not equivalent to validating bytes actually stored. Scope this with F4, rather than removing verification as an optimization.

**F7 — Joplin retains duplicate ID maps.** `scalable.go:310–312` copies `inv.NoteIDs` into `run.noteIDMap`; the original map remains inside the run's inventory. Investigate ownership transfer of the map because live mappings are subsequently mutated. This can remove one map's buckets, not the underlying shared string bytes.

```python
# BEFORE (illustrative ownership)
run.note_ids = dict(inventory.note_ids)
# AFTER: only if no other reader needs the original snapshot.
run.note_ids = inventory.note_ids
inventory.note_ids = None
```

Preserving imports use the full Joplin parser rather than the lean cloning path. Check heap retention of folder/resource fields and tag titles there; this review does not claim the same measured J19 improvement applies to preservation mode.

**Do not pool the 32 KiB hash buffer without new evidence.** J19 found it stack-resident: 6 allocations / 312 B per operation either way. Likewise, `mmap` is not a default improvement: rewritten strings still allocate, while mapping mutable source files complicates truncation, lifetime and portability. Retain streamed attachments and whole-body bounded note processing.

### 4.3 Parsing & Text Processing

#### F8 — Shared coordinate decoration is O(K·B) on link-dense notes

`markdownlinks/parser.go:107–109` calls `lineColumn` for every candidate; `132–146` scans from the beginning of the body each time. This work is separate from the Obsidian replacement-copy loop and also affects Joplin's stored canonical Markdown through the shared store parser.

Extract matches, process their positions in ascending order, and advance a single rune/line cursor; restore the existing result ordering if callers depend on it. Alternatively, build a newline index and count runes within the relevant line. Beware long single-line notes, where repeated rune counting still becomes quadratic.

```python
# BEFORE: illustrative character-offset equivalent.
positions = [(text[:p].count("\n") + 1) for p in starts]
# AFTER: each prefix traversed once; also maintain rune-column in Go.
line, cursor, by_offset = 1, 0, {}
for p in sorted(set(starts)):
    line += text.count("\n", cursor, p)
    by_offset[p] = line
    cursor = p
positions = [by_offset[p] for p in starts]
```

Go positions are byte offsets but columns count runes. Preserve this distinction, context snippets, anchors and extraction order in goldens. Sorting costs O(K log K), then scanning O(B); no claimed recipe-corpus speedup follows without a link-dense benchmark.

#### F9 — Alias/basename collisions create quadratic namespace construction

Obsidian `buildLinkNamespace:621–648` uses `appendUnique`, which linearly scans each bucket. If many notes share a title/basename, building that bucket performs roughly N(N−1)/2 comparisons. Sorting each bucket follows. Resolution usually needs zero, one, or multiple candidates, but do not change representation without testing repeated alias/ID semantics.

```python
# BEFORE (illustrative)
if target_id not in names[key]:
    names[key].append(target_id)
# AFTER: deduplicate within a set, preserving externally required ordering.
names[key].add(target_id)
# At the boundary, only if sorted lists remain part of the contract:
ordered = {key: sorted(ids) for key, ids in names.items()}
```

Use sets selectively for collision-heavy buckets or represent unique/ambiguous states compactly; sets everywhere may worsen memory on normal unique-name vaults. Benchmark 100k distinct titles versus 100k files all named `index.md` in different folders. This is an input-shape scaling finding, not evidence that ordinary imports currently take quadratic time.

#### F10 — Reduce redundant parsing/allocation without changing identity

- `splitSourceTitle` (`joplinraw.go:356–366`) splits and rejoins the whole normalized body merely to remove its first line and optional blank line. Use first-newline indexes/slices with exact trailing-newline equivalence.
- `markdownTitle` scans `strings.Split(body, "\n")` even when the first heading is near the start. A line iterator can stop early.
- Obsidian's string `splitFrontmatter` converts through byte slices and returns strings; J19 attributed 7% of allocation to this wrapper, **not** the comparison-only string conversions in `splitFrontmatterBytes` (measured zero allocations). A string-native delimiter scanner is a candidate; still normalize CR/CRLF consistently and clone retained values.
- `markdownblocks.Extract` builds line/offset arrays, block text, occurrence keys, normalized copies and hashes. Start with replacing `listItemRE.MatchString` followed by `ReplaceAllString` with one prefix match and slice; preserve normalization and block identity. Broader parser rewrites need evidence and golden tests.

```python
# BEFORE: illustrative list-prefix handling.
if list_item_re.match(line):
    content = list_item_re.sub("", line).strip()
# AFTER: one anchored match, no replacement engine.
match = list_item_re.match(line)
if match:
    content = line[match.end():].strip()
```

```python
# BEFORE: normalized LF Joplin title separation, illustrative.
lines = body.split("\n")
title = lines[0].strip()
start = 2 if len(lines) > 1 and lines[1] == "" else 1
rest = "\n".join(lines[start:])
# AFTER: same normalized-input operation without whole-body line array.
first, _, rest = body.partition("\n")
title = first.strip()
if rest.startswith("\n"):
    rest = rest[1:]
```

Regexes in both shared parsers are already package-level `MustCompile`. Historical profiles label some runtime work “regexp backtracking”; that does not imply Python-style catastrophic backtracking in Go's standard regexp engine. Literal fast-path guards are reasonable only when they cannot exclude recognized input. Obsidian's frontmatter handling is a small manual parser, not repeated general YAML deserialization; switching YAML libraries would change behavior, not simply accelerate an existing YAML decoder.

### 4.4 Parallelism & Concurrency

The importer loops are serial. Shared SQLite mutations hold `s.mu`, and `BEGIN IMMEDIATE` takes the writer lock; more writer goroutines do not remove that bottleneck. Cumulative SQLite time and shared parser costs should be reduced before launching parallel I/O.

A bounded Go worker pool could parse/hash a few notes ahead of one ordered writer. Keep the namespace immutable during that phase; Joplin's batch legacy-ID remapping means workers must not concurrently read a mutating map. Do not let workers update reports/checkpoints. Respect total queued bytes, cancellation and file descriptor limits.

```python
# BEFORE (illustrative)
for item in batch:
    parsed.append(parse_and_hash(item))
# AFTER: only one already-byte-bounded batch in flight.
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
    parsed = list(pool.map(parse_and_hash, batch))
# Single owner classifies and commits, in deterministic input order.
store.apply_batch(parsed)
```

Python threads suit I/O that releases the GIL; CPU-heavy pure-Python parsing would need a process pool, whose serialization/startup costs can exceed the work on small notes. Neither `concurrent.futures` nor the GIL governs the actual Go implementation. Do not share a Python sqlite connection across threads as a shortcut, or create several canonical writer connections.

Measure 1, 2 and 4 workers on SSD and HDD independently. If only 20% of wall time is parallelizable, four ideal workers yield at most 1/(0.8+0.2/4) ≈ 1.18×; seek overhead and memory regressions too. This is an Amdahl example, not a measured fraction for this repository.

## 5. Prioritized Action Plan

### 1. High Impact / Low Effort (Quick Wins)

- **Property-order omission** on non-preserving Obsidian inventory: strongest small memory candidate; retain exact fingerprints and preserve-source behavior.
- **Single-pass replacement assembly**: high impact for link-dense large notes; first pin behavior for overlapping mixed-format matches.
- **Reuse inventory Markdown bytes for hashing**: low-complexity reduction in logical reads; verify cold/warm performance separately.
- Treat the title-splitting and duplicate ID-map ownership changes as smaller wins, not substitutes for profiled CPU work.

### 2. High Impact / Moderate Effort

- **Batch-scoped prepared block/link statements (F1)**: first CPU experiment, with before/after prepare counts and equivalence tests.
- **Coordinate decoration and collision-heavy namespace construction (F8/F9)**: targeted scaling tests, Unicode/range goldens.
- **Byte-bounded batches and bounded rereads (F5)**: memory robustness; preserve item-position checkpoints and large-note support.
- **Obsidian transactional counters/checkpoint consolidation and stable metadata skips**: independent changes, crash/resume and provenance tests.
- **Revision-checked second-pass block reuse (F2)**: investigation first; affects cross-component derived-state invariants.
- **Existence-only batch Store reads (F3)**: no loss of deleted-note semantics; avoid full bodies only where actually unnecessary.

### 3. Medium/Low Impact Optimizations

- Resource/source-bundle staging and batch metadata commit when attachment-heavy profiling justifies the API work.
- Selective compact Joplin resource representations and cached source notebook paths.
- Disposable manifest initialization/index-build experiment; do not relax canonical durability.
- Disk-backed Obsidian namespace and parallel parsing only if smaller changes miss measured targets.
- Do not re-propose J19's rejected buffer pool, streamed fingerprint concatenation or comparison-conversion “fixes” without evidence of a changed compiler/workload.

Each candidate needs a declared investigation/implementation slice and owner approval where it changes product semantics. This review does not start or complete any roadmap item.

## 6. Benchmarking & Verification Strategy

### Benchmark matrix

Run each change alone, alternating baseline/candidate, at least five measured repetitions after warm-up where practical. Do not run package tests, builds, reviewers or other imports alongside timed runs; J17 demonstrated how badly that contaminates results.

| Dimension | Cases |
|---|---|
| Scale | 100 smoke, 10k, 100k, then 382,206-equivalent or 500k synthetic where supported |
| Import state | Fresh DB; no-op reimport; 1% changed notes; interruption/resume; user-edited/trashed notes |
| Content | Small recipe notes; long single-line notes; 1 MiB link-dense notes; near-limit large notes; many blocks |
| Namespace | Unique titles; repeated basenames/aliases; ambiguous/missing targets; forward/backward links across batch boundaries |
| Layout | Flat huge directory; deep hierarchy; wide hierarchy; symlink refusals |
| Resources | None; many small files; few large attachments; shared references; preservation on/off; source changed between phases |
| Platform | HDD vs SSD; warm vs explicitly controlled cold cache; same filesystem and SQLite build |
| Batch | 25/100/500 notes plus explicit byte budgets; record commit latency and interactive-read blocking |

Report median, range and dispersion for wall/user/sys, notes/s, input MiB/s, allocations/op, total allocated bytes, live heap after GC, peak RSS, statements prepared, commits, SQL step count, bytes read/written, WAL peak and checkpoint time. Count file passes separately from physical IOPS. Record binary revision, schema, runtime PRAGMAs, compiler, corpus digest, cache state and source-preservation settings.

### Existing Go harnesses and suggested commands

These are **reproduction instructions, not commands run by this review**. Use disposable output/library directories; no private content in committed evidence.

```sh
# Existing small generated profiles (scripts run usage preflight themselves):
bash scripts/run_joplin_import_profile.sh 100 /tmp/opencode/joplin-review-100.json
bash scripts/run_obsidian_import_profile.sh 100 /tmp/opencode/obsidian-review-100.json

# Focused functional suites:
go test ./internal/importers/joplinraw ./internal/importers/obsidian ./internal/markdownlinks ./internal/markdownblocks

# Existing microbenchmarks; compare outputs with benchstat when installed:
go test ./internal/importers/joplinraw ./internal/importers/obsidian -run '^$' -bench . -benchmem -count=5

# Existing opt-in real-vault profiling harness, on a disposable DB:
NOTRIOS_J17_VAULT=/path/to/synthetic-vault NOTRIOS_J17_DB=/path/to/disposable.sqlite \
  go test ./internal/importers/obsidian -run '^TestJ17ImportRealVaultOnDisk$' \
  -count=1 -v -cpuprofile /tmp/opencode/import.cpu -memprofile /tmp/opencode/import.heap -timeout 60m

go tool pprof -top /tmp/opencode/import.cpu
go tool pprof -sample_index=alloc_space -top /tmp/opencode/import.heap
```

CPU/heap profiles may reveal note-derived strings or private paths; keep them local unless generated entirely from synthetic data. Supplement Go pprof with native `perf`/SQLite counters to distinguish C execution and prepare cost. Use `/usr/bin/time -v` for RSS and `hyperfine` for repeated **already-built** CLI commands, with a fresh disposable DB each run. Building/starting the store must be measured separately from import; J21 fixed startup work that had contaminated prior search measurements.

For actual Python harness overhead, `python -m cProfile -o harness.prof ...` and `tracemalloc` are appropriate. They will not profile Go heap allocation or SQLite C CPU inside the production executable. Python `executemany` toy timings are not evidence of the custom Go wrapper's improvement.

### Correctness gates

- Compare canonical bodies, titles, notebook placement, provenance/metadata, tags, attachment references, source bundle bytes/hashes and property order.
- Compare links including byte spans, rune columns, contexts, display text, anchors and resolution status; compare blocks including IDs, occurrence numbers, hashes, slugs and offsets.
- Verify inventory/item fingerprints and checkpoint positions/reports; stop after each phase/batch and resume; inject errors before and after durable commit.
- No-op must not create revisions or projection jobs. Preserve intentional handling of user edits and trashed documents; test namespace changes that resolve formerly missing links.
- Maintain search visibility immediately after each committed batch, FTS/rowid-map consistency, FK integrity, rollback behavior and resource policy checks.
- Run `performance/v1.0-j17/j17_compare.py` on independently imported synthetic libraries; justify only expected timestamp/random-identifier differences. Do not normalize away meaningful provenance/checkpoint differences.
- Add parser edge cases: CR/LF/CRLF, Joplin OCR control characters, BOM/invalid UTF-8, escaped/fenced/inline-code links, overlapping match ranges and non-ASCII offsets. Preserve existing grammars; do not silently “fix” parser semantics during a performance refactor.
- Run race tests for any worker pool and full `go test ./...` before implementation commits. Follow AGENTS' frontend audit policy at the beginning of a regular validation pass.

## 7. Execution Guidelines

### Evidence coverage and limitations

Read in full for this review:

- `internal/importers/joplinraw/joplinraw.go` (633 lines) and `scalable.go` (1,624 lines): complete production Joplin package files located for the review.
- `internal/importers/obsidian/obsidian.go` (1,619 lines): complete production Obsidian importer file.
- `internal/store/sqlite_import_batch.go`, `sqlite_imports.go`, `import_manifest.go`, `sqlite_blocks.go`, `fts_rowid.go`, `sqlite_cgo.go`.
- `internal/markdownlinks/parser.go`, `internal/markdownblocks/parser.go`.
- Both generated import-profile shell wrappers, and J17/J19/J20 performance READMEs including their corrections and final results.

Focused inspection, **not an entire-repository audit**: relevant `sqlite.go` initialization, link rebuild/resolution and prepare/bind helpers; the separate `sqlite_fix.go` query at line 315. That line is a fix-planner query, not the importer hot loop. All of `sqlite.go`, resource policy/storage dependencies, every migration and all importer tests were **not** read in full. Accordingly the request's literal “all shared utility files” coverage is not claimed, and resource API redesign/index choices remain investigation proposals. No runtime benchmark was run or new numerical result fabricated.

Workspace semantic search reported possibly stale results; current source and the complete, corrected performance records above take precedence. In particular, obsolete findings that Obsidian lacks batching or still has J19's original retained-body problem are not repeated as current defects.

### Work performed and validation status

- Documentation-only performance review; no importer, Store, parser, dependency or SQL tuning changes applied.
- Usage-parser preflight passed **37 tests**. The usage probe found no Claude/Codex ancestor and fell back to checking all: Codex reported pause (five-hour remaining 0%, reset `2026-09-17T19:11:28Z`); Claude cache was stale. These values do not establish Union Alpha/OpenCode quota.
- An attempted delegated checkpoint failed with the provider's monthly-budget limit. No independent reviewer result was obtained.
- Long model-backed work, full validation and fresh profiling were not run. The document is a static review using repository-recorded measurements. Suggested examples are explanatory, not tested production patches.
- Remaining verification: new candidate microbenchmarks, clean full import comparisons, complete relevant dependency review before each implementation, and canonical-store invariant review for changes to transaction or derived-state boundaries.

Use the supplied Python examples to communicate the mechanism, then implement and test the actual Go paths. Preserve durability, exact source validation, privacy boundaries and format compatibility even when a less safe benchmark looks faster.
