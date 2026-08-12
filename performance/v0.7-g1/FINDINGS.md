# G1 findings and G7 selection

## Decision

G7 should store every immutable revision as a **complete UTF-8 body object with
its result hash and parent IDs**. It may additionally carry a **named-parent,
line-token edit script** only when generation stays inside a preflight budget,
exact reconstruction and UTF-8 validation succeed, and the encoded delta is
materially smaller than the complete body. A delta is never the only recovery
representation. Missing/wrong bases, malformed operations, limit overruns,
invalid UTF-8, and wrong result hashes are refusals followed by complete-object
fetch, never best-effort patching.

Three-way merge should be **bounded line-first with word-token refinement of a
conflicting line/region**. Disjoint line edits merge cheaply. A line conflict
may be retried as Unicode-aware word/punctuation/whitespace tokens only below
explicit byte, token, operation, and CPU/cancellation bounds. Same-token edits,
delete/edit, malformed inputs, or a refinement that exceeds a bound become a
durable typed conflict holding base/local/remote revision IDs and bodies. Byte merge is rejected: it is slower on the small cases, requires a low token limit
on long lines, and offers no useful semantic boundary beyond word refinement.

The preferred G7 library candidate is
`github.com/epiclabs-io/diff3` at commit
`3b1669897fb1aa7c1fb2699a3c6a45bbb46e9ec1`. It is a small MIT Go package with
no runtime external imports, a generic token API, selectable Myers diff, and
inspectable conflict records. Its upstream tests and the G1 probe passed for a
clean disjoint Unicode-token merge, a same-token conflict, and exposed
local/base/remote conflict values. It has no observed tag or release, so G7
must use an exact pseudo-version, retain Notrios-owned conformance/property
tests, and recheck maintenance before adding it. G1 does not add the dependency.

## Corpus shape

The complete aggregate scan observed:

| Corpus | Bodies | Body p50 | p95 | p99 | max | Non-body files | non-body p95 | max |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Joplin RAW | 103,349 | 293 B | 2,878 B | 8,283 B | 326,359 B | 734 | 3,825,632 B | 107,951,795 B |
| Recipe Markdown | 1,619,768 | 224 B | 3,850 B | 5,346 B | 209,873 B | 1,266 | 369,969 B | 153,175,560 B |
| Twitter vault | 339,944 | 1,446 B | 4,811 B | 4,919 B | 5,719 B | 13,269 | 433,430 B | 82,969,993 B |

“Non-body” is a filesystem-shape observation, not a claim that every file is a
Notrios resource. The Joplin scan classified 103,349 type-1 bodies, six
notebooks, 763 resource records, 322 tags, and 6,890 note/tag relations; 29
files were invalid or unclassified. No corpus contained a type-13 revision
item. Static Markdown corpora likewise provide one current body per file, not a
history. Therefore no real revision depth or human conflict rate is claimed.

The size distributions do support two bounded conclusions: the overwhelming
majority of bodies are small (all three p99 values are below 9 KiB), while rare
large bodies and resources are real and must take a bounded full-object path.

## Transfer-delta measurements

The ordinary cases are localized, scattered, append, Unicode, Markdown, and
valid UTF-8 control-bearing (“binary-looking”) edits. The long-line case is
shown separately because a line delta necessarily degenerates there.

| Offline interval | Synthetic edits/side | Line average/full (ordinary) | Word average/full | Byte-span average/full (ordinary) |
|---|---:|---:|---:|---:|
| 1 hour | 1 | 2.49% | 1.34% | 1.48% |
| 1 day | 4 | 4.73% | 1.75% | 6.59% |
| 1 week | 12 | 7.96% | 2.39% | 13.93% |
| 30 days | 32 | 10.90% | 3.13% | 20.61% |

At 30 days, the line method's median per-case CPU was 105.6 ms and its maximum
was 425.6 ms; word-token generation's median was 816.6 ms and maximum 1,364.9
ms. Peak worker RSS was 23,356 KiB versus 25,480 KiB. Word deltas save more
bytes but spend enough CPU on this modest host and 74–75 KiB workload that they
are not the default transfer encoding. A single byte span is attractive for a
localized edit but reached 82.74% of a full body for scattered 30-day edits.

The 60 KiB very-long-line case sent 133.73% of the result as a line delta and
119.69% as a byte-span delta at 30 days; the selected size-benefit gate sends
the complete body instead. A separate 1 MiB generated note reconstructed
exactly: line delta was 390 bytes at 218.6 ms and 31,544 KiB peak RSS; the word
candidate hit its token ceiling; complete-body fallback remained available.
These are host observations, not permanent protocol limits—G2/G7 own numeric
envelope and runtime ceilings.

Every successful delta record carries matching result and reconstruction
SHA-256 values. Seven negative cases reject missing base, wrong base hash,
wrong result hash, cursor overrun, too many operations, too many inserted bytes,
and invalid UTF-8 output.

## Merge classifications

- Independent paragraph edits merge cleanly at all granularities.
- Disjoint words on one line and neighboring Unicode-word edits conflict at
  line level but merge cleanly at word level.
- Same-token edits and overlapping Markdown URL edits conflict at every useful
  granularity and must remain visible.
- A 50 KiB repeated-token line took about 3.8 ms at line granularity and 181.9
  ms at word granularity; byte merge exceeded the explicit 32,768-token bound.
  This is why refinement is regional and bounded.
- Title versus body and move versus rename are independent field merges.
  Delete/edit becomes a durable conflict. Notebook cycles require G6's visible
  deterministic repair.
- Concurrent add/remove of the same tag is the material LWW-per-element case:
  one intent loses according to protocol order. Static corpora cannot estimate
  its frequency. It does not justify an unbounded observed-remove dot set, but
  G6 must retain both operations in the journal/audit trail and test every
  delivery order.

## G7 acceptance consequences

G7 must turn this selection into production invariants rather than copy the
spike implementation:

1. name and verify the parent/base and complete result before admission;
2. cap input bytes, tokens, operations, delta-chain depth, CPU/cancellation,
   conflict output, and inserted bytes before allocation-heavy work;
3. generate a delta only as an optimization and discard it unless the encoded
   result clears a documented benefit threshold;
4. use line merge first and word refinement only on bounded conflict regions;
5. persist an ordinary merge revision for clean results and a typed durable
   conflict for unresolved results;
6. test both delivery orders, Unicode/Markdown/long-line controls, exact hashes,
   candidate-library upgrades, and complete-body fallback.

G2 may refine numeric object/envelope bounds; it must not change the full-body
recovery invariant or turn a transfer delta into canonical state.
