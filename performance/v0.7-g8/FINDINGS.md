# G8 findings

## Metadata arrives without the bytes

Eight attachments per size, shipped between two real replicas:

| Object size | Chunks | Metadata admitted | Note readable first | Fetched on its own | Reason |
|---|---:|---:|---|---:|---|
| 64 KiB | 1 | 79 ms | yes | 8 of 8 | `eager_small_object` |
| 1 MiB | 1 | 106 ms | yes | 8 of 8 | `eager_small_object` |
| 4 MiB | 4 | 82 ms | yes | 0 of 8 | `above_eager_threshold` |
| 16 MiB | 16 | 85 ms | yes | 0 of 8 | `above_eager_threshold` |

The admission time is flat across a 256× range of attachment size, which is the
point: what travels at admission is the object's identity, length, type, and
transfer shape, not the object. In every case the note, its body, and its
reference to the attachment were usable before a single attachment byte moved.

The policy split falls exactly where G2 put the whole-versus-chunked boundary.
One megabyte is fetched automatically; one megabyte and one byte is not. Reusing
that threshold rather than inventing a second one means there is only one number
to explain, and it is already the number that decides how the object travels.

## Pinned transfer, resume, and exactness

| Object size | Pinned transfer | Time | Refetched on resume | Exact |
|---|---:|---:|---:|---|
| 64 KiB | 524,288 B | 37 ms | 0 of 1 | yes |
| 1 MiB | 8,388,608 B | 384 ms | 0 of 1 | yes |
| 4 MiB | 33,554,432 B | 2,125 ms | 3 of 4 | yes |
| 16 MiB | 134,217,728 B | 7,988 ms | 15 of 16 | yes |

Every materialized object was compared byte for byte against the original and
matched. The resume column is a transfer deliberately cut off after one chunk
and then restarted: it refetched everything except the segment it had already
verified. A whole object shows `0 of 1` because one chunk *is* the object — the
interrupted pass completed it, so the resuming pass had nothing to ask for.

Halving the large-object time was a change made because of this measurement. The
first run took 16.6 s for the 16 MiB row, because installing a materialized
object rebuilt its manifest by reading the whole file again — a file whose every
chunk hash had just been verified on the way in. Passing the manifest through
removed one full pass over the data.

## Three limits these numbers do not remove

**A note can be read before its attachment; an archive cannot.** Exporting a
library that holds unmaterialized objects now refuses, naming the object, rather
than producing an archive-v2 container that silently omits bytes it claims to
contain. Pinning and materializing first is the fix. That refusal is new in G8
and is the honest reading of "full snapshot".

**The provider here is in-process.** These timings include hashing, staging,
assembly, and installation, but no network, no encryption, and no carrier
framing. G9 owns the codec, G11 the shared directory, and G14 the REST data
plane; all three will change the wall-clock numbers and none of them changes
what is verified.

**Bounded concurrency is a bound, not a measurement.** `MaxConcurrentFetches` is
declared and the per-pass limit is enforced and tested, but this harness fetches
sequentially from a local provider, so it does not demonstrate parallel transfer
throughput. That belongs with a real carrier.

## What a receiver refuses

Every one of these is a fixture, and each leaves the object exactly as it was —
known, referenced, and unavailable — rather than half-installed:

| Case | Recorded reason |
|---|---|
| No peer currently holds the object | `no_source` |
| A chunk is corrupted in flight | `corrupt_chunk` |
| The manifest is internally consistent but is not the one the operation named | `manifest_digest_mismatch` |
| The manifest does not describe the object | `invalid_manifest` |
| Every chunk verifies and the whole still is not the named object | `object_mismatch` |
| The content type contradicts what was advertised | `mime_mismatch` |
| The local staging budget is exhausted | `deferred_staging_budget` |

The manifest digest case is the one worth naming. A source that could supply its
own chunk hashes would be choosing what it is later checked against, so the
manifest is verified against a digest the resource operation already carried
before a single chunk is requested.

`mime_mismatch` is deliberately narrow. `http.DetectContentType` is inconclusive
for most formats — it answers `application/octet-stream` or `text/plain` for
anything it does not specifically recognize — so demanding equality would reject
ordinary attachments rather than hostile ones. A PDF advertised as a PNG is
refused; an OpenDocument file that sniffs as opaque bytes is not.

## Bounds this slice fixed

| Bound | Value | Why |
|---|---|---|
| Whole-object threshold | 1 MiB | G2's selection; also the eager-fetch threshold. |
| Chunk size | 1 MiB fixed | G2 deferred FastCDC: static corpora hold no repeated binary-edit trace to justify content-defined chunking. |
| Chunk ceiling | 16,384 | With 1 MiB chunks this is the 16 GiB resource limit Notrios already enforced. |
| Concurrent fetches | 4 | A metered mobile link is the constraint; v0.8's emulator work revisits it. |
| Staging budget | 512 MiB | Exceeding it defers rather than fails, so the object stays visible and askable. |
| Eager budget | 64 MiB per pass | Beyond it everything waits for a pin or a request. |

## What stays unavailable, forever if necessary

When no peer has an object's bytes, the resource keeps its metadata and a
visible `unavailable` state indefinitely. Nothing drops the reference, nothing
substitutes an empty file, and nothing writes a placeholder into the
content-addressed store — a structural trigger refuses any blob row whose
availability contradicts whether it has a storage path, in both directions.
