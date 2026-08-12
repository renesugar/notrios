# G2 findings and recommendation

## Result

Select the compact NCB1 canonical operation-record stream in `FORMAT.md`, with
a readable canonical-JSON outer manifest and the pinned deterministic gzip
profile, for G9 implementation review. This invokes the plan's recorded
exception to the JSONL default: the compact candidate has a canonicalization
specification and was materially beneficial.

At 10,000 generated operations on the recorded amd64 Linux desktop proxy:

| Candidate | Raw | gzip | Encode p50 | Decode p50 | Decode allocation |
|---|---:|---:|---:|---:|---:|
| canonical JSONL | 3,090,440 B | 424,041 B | 104 ms | 256 ms | 17.6 MiB |
| compact NCB1 | 1,485,497 B | 355,032 B | 60 ms | 42 ms | 3.7 MiB |

NCB1 is 51.9% smaller raw and 16.3% smaller after compression, decoded about
6.1× faster, and allocated about one-fifth as much in this prototype. All six
100/10k/100k format-tier rows round-tripped exactly and reproduced identical
encoded and compressed bytes. The full 100,000-operation workload became ten
envelopes; the largest NCB1 part was 1,495,878 B raw and 355,858 B compressed,
well below the proposed 16 MiB/4 MiB ceilings.

This comparison does not claim the prototype is production-quality or that a
different optimized JSON decoder could not narrow the CPU gap. The size gap,
canonical minimal-varint specification, and lower allocation still make NCB1
the evidence-backed candidate. G9 owns promotion, per-kind payload schemas,
goldens, compatibility, signature/encryption framing, and fuzzing.

## Envelope and pending bounds for G8/G9

Close an envelope at the first of 10,000 operations, 16 MiB canonical record
bytes, or 4 MiB compressed record bytes. Retain the independent 1 MiB record,
512 KiB payload, 64-dependency, and 64:1 expansion limits. Cheap outer length,
identity, capability, signature/replay checks should precede decompression when
G9's format permits it.

Bound unavailable dependencies per peer at 10,000 operations and 64 MiB of
disk-backed encoded bytes. Overflow stops admission and requests repair or
snapshot catch-up. It does not drop an arbitrary record or keep pending data in
heap. G5/G8 may choose a stricter transaction batch inside an admitted
envelope; the transport ceiling is not a SQLite transaction-size mandate.

The hostile probes rejected operation-count overflow, oversized payload,
compression-ratio expansion, multiple gzip members, non-minimal compact
varints, and unknown JSON fields. They are exemplars, not a replacement for G9
fuzz/property tests.

## Resources and ranges

Keep whole-resource objects below 1 MiB and split resources at or above that
threshold into 1 MiB fixed chunks. In the aggregate corpora this leaves 1,237
of 1,266 recipe resources (97.7%) and 615 of 734 Joplin resources (83.8%) as
one object while bounding retry and streaming work for the long tail.

For the generated 107,951,795-byte attachment-heavy case:

| Fixed chunk | Objects | Manifest proxy | 64 KiB aligned/worst range | Max retry |
|---|---:|---:|---:|---:|
| 256 KiB | 412 | 39,552 B | 256/512 KiB | 256 KiB |
| **1 MiB** | **103** | **9,888 B** | **1/2 MiB** | **1 MiB** |
| 4 MiB | 26 | 2,496 B | 4/8 MiB | 4 MiB |

One MiB is the middle point: four times fewer chunk objects/descriptors than
256 KiB without the 4 MiB retry/range amplification. Preserve archive-v2's 16
GiB maximum resource size, which implies at most 16,384 chunks and about 1.5
MiB of 96-byte descriptor proxy data. Range fetches align to chunks, verify
each exact chunk, and admit the complete resource atomically. **FastCDC remains deferred**:
the available static corpora show attachment sizes, not repeated
binary edit traces, so they cannot demonstrate a bandwidth win.

## Many-small objects and packs

Reuse archive-v2's pack capability rather than creating one file per envelope
dependency. Existing real-corpus evidence collapsed 382,447 files to 46
(8,314×) and 215,484 to 30 (7,183×), with export/verify speedups of about 1.29×
and 2.01× respectively. Bytes vary with filesystem block rounding and format
revision: the first recorded packed run used 10.9% more measured disk bytes;
the final attachment run used 2.3% more apparent archive bytes while occupying
less disk because it avoided 215k small files.

For the sync carrier, use provisional targets of 64 MiB payload and 4,096
contained objects with a hard 4 MiB trailer. These are deliberately below the
desktop archive writer's 256 MiB/65,536-object targets because the current pack
reader loads a trailer whole and G2 has no mobile measurement. G9 must stream
or bound trailers and keep loose compatibility; v0.8 may lower these targets.

## Scope and limitations

- Timings, allocations, and process peak RSS are desktop proxies from Go
  1.26.5 on linux/amd64. The 100k rows share one process, so the reported peak
  RSS (about 250 MiB) is a conservative cumulative process peak, not per-format
  incremental memory.
- Aggregate order statistics support threshold selection but do not recreate
  private per-file distributions. Resource manifest totals in the matrix are
  explicit five-point proxies, not claimed corpus totals.
- Static imports still contain no independent-device history or repeated
  binary-resource edit trace.
- The v0.8 emulator and post-1.0 physical-device checklists are mandatory.
  G2 makes no desktop-to-mobile claim.
- No production sync codec, schema, cryptography, transport, or dependency was
  added. Complete/fixed-chunk objects remain the implementation baseline until
  later approved G7/G8/G9 work lands.
