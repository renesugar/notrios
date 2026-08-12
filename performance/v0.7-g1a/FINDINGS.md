# G1a findings and recommendation

## Outcome

A Notrios-owned pure-Go Subversion-style matcher and constrained RFC 3284
VCDIFF codec is feasible for both text and binary transfer deltas. Keep the
prototype under `performance/`; do not promote it during G1a. If G2 establishes
acceptable numeric bounds, a later approved G7/G8 slice should independently
review and either rewrite or deliberately promote the design.

The recommended interoperability target is the constrained RFC 3284 default
code table profile, not Subversion svndiff and not the private `NXD1`
comparison container. Algorithm compatibility means the matcher follows the
published Subversion xdelta strategy; it does not mean byte-identical matching
decisions, Subversion API compatibility, or svndiff serialization.

## Measured result

The committed host run exercised 21 deterministic fixtures: 14 text cases
derived from G1's localized, scattered, append, Unicode, Markdown,
control-bearing, and very-long-line shapes at one-hour and 30-day intervals,
plus seven empty, identical, sparse, unrelated, repetitive, NUL-bearing, and
attachment-like binary cases.

- Both containers reconstructed the exact target and encoded deterministically
  in all 21 cases.
- VCDIFF was smaller than the complete target in 19 cases. The exceptions were
  the empty target (five-byte VCDIFF header) and unrelated 256 KiB random bytes
  (262,166-byte delta versus a 262,144-byte complete object). A production size
  gate must therefore retain complete-object fallback.
- VCDIFF was smaller than G1's JSON line delta for all 14 compared text cases.
  The strongest structural examples were a 66,008-byte one-line target encoded
  in 36 bytes rather than G1's 88,271-byte line JSON, a sparse-edited binary
  target of 263,167 bytes encoded in 1,358 bytes, and a repetitive 1 MiB target
  encoded in 32 bytes.
- The private container was typically only 10 bytes smaller than VCDIFF (13
  bytes at most on this matrix; VCDIFF was one byte smaller for empty output).
  That small saving does not justify inventing a Notrios-only parser and format.
- Across these bounded fixtures, VCDIFF median p50 CPU was about 1.67 ms to
  encode and 0.89 ms to decode; maxima were about 29.0 ms and 15.6 ms. Maximum
  measured median allocations were about 1.69 MiB to encode and 2.10 MiB to
  decode. Process peak RSS was about 22 MiB. These are comparative desktop-host
  observations, not protocol budgets or mobile claims.

The RFC profile has a slightly larger parser surface than `NXD1`, but it buys
real interoperability: open-vcdiff and xdelta3 both decoded every Go-emitted
fixture exactly. The constrained Go decoder accepted two of six externally
encoded streams; the other four used valid default-table compound opcodes that
the prototype deliberately rejects. This is an explicit profile boundary: it
is not a general VCDIFF decoder and makes no claim of full RFC 3284 coverage.

## Security and correctness gates

The prototype bounds source, target, window, delta, section, operation, window,
checksum-candidate, varint, expansion, and chain-depth dimensions and exposes
bounded `io.Reader`/`io.Writer` entry points. Tests cover exact golden output,
overlapping target copies, randomized round trips, truncation, trailing data,
invalid source ranges, unsupported opcodes/windows/compressors, every declared
limit, cancellation, short writes, and fuzzed arbitrary byte streams.

Those checks do not make a delta authoritative. Later production admission
must require an immutable named parent, verify the parent's exact identity,
reconstruct into quarantine under G2 limits, verify the complete result hash,
and only then admit canonical bytes. Missing, wrong, refused, oversized, or
non-beneficial deltas fall back to the complete object. Partial/best-effort
materialization is forbidden.

## Text merge and resources

Binary delta encoding remains separate from three-way conflict resolution.
G7 still owns the bounded pure-Go line-first merge with Unicode-aware
word-region refinement selected by G1. VCDIFF neither discovers concurrency nor
resolves conflicts.

G8 may consider VCDIFF for a binary resource only when the resource revision
names a real immutable parent and the delta passes the material-size gate.
Ordinary unrelated content-addressed resources have no inferred ancestry and
continue to use complete or fixed-chunk transfer. The 22-byte loss on unrelated
random bytes demonstrates why that boundary matters.

## Resolved G1a decisions

1. **Format:** recommend a constrained RFC 3284 VCDIFF default-table profile;
   reject `NXD1` as a production format and do not target svndiff bytes.
2. **Provenance:** retain the spec-first independent Go implementation model,
   with Apache Subversion used as an attributed behavioral reference. No GPL
   source was used and no direct source translation is claimed.
3. **Promotion:** keep all G1a code under `performance/`. G7/G8 must make a new,
   explicit promotion or rewrite decision after G2 supplies numeric limits.

No production sync codec, merge engine, schema, runtime dependency, or mobile
safety claim was added by G1a.
