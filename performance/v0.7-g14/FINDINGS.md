# G14 findings — the same protocol, a different courier

## Exchange: REST costs almost nothing over a folder

| | 100 notes | 500 notes |
|---|---|---|
| operations | 200 | 1,000 |
| exchange through a folder | 1,142 ms | 5,929 ms |
| exchange over an authenticated peer | 1,318 ms | 5,933 ms |
| REST overhead | +15.4% | +0.07% |
| both converged | yes | yes |
| transcript identical | yes | yes |

At 500 notes the two are indistinguishable, and at 100 the difference is a
handful of HTTP round trips against a small amount of real work. This is the
same shape G11 measured: **SQLite admission dominates, and the courier is
close to free.** It is also the reason the plan's "REST never becomes the merge
implementation" is not a sacrifice — moving the merge to the server would
optimize the part that was never the cost.

**The transcript is identical because it is the same code.** G11 wrote the
exchange against a `Carrier` interface; G14 implements that interface over
HTTP. There is no second protocol implementation to keep in step, and the
artifacts a REST peer stores are the same sealed `NAR1` bytes with the same
blinded names a folder would have held — asserted directly, not inferred.

## Backup: the container costs more than the crypto

| | 100 notes | 500 notes |
|---|---|---|
| archive-v2 bytes | 107,576 | 528,780 |
| sealed bytes | 134,468 | 660,472 |
| sealing overhead | +25.0% | +24.9% |
| produce | 343 ms | 995 ms |
| download | 7 ms | 12 ms |
| bounded requests at 256 KiB | 1 | 3 |
| verified as archive-v2 | yes | yes |
| records verified | 207 | 1,007 |

**That 25% is the ZIP container, not the encryption.** Frame authentication adds
28 bytes per mebibyte; what the overhead actually is is per-entry headers over
an archive made of many small content-addressed object files. The fix already
exists and was not applied here: P3b's `--pack` layout collapses a 382,206-note
archive from 382,447 files to 46, and packing before sealing would remove nearly
all of this. It is left as a stated cost rather than a silent one, because the
decision belongs with whoever measures it at real scale.

**Resume is the file's own length.** Nothing is remembered between attempts: the
client asks for `bytes=<local length>-<bounded end>`, so a transfer interrupted
anywhere continues from exactly where it stopped, and a retry after a crash
needs no state. The 500-note tier took three bounded requests and resumed twice.

## The defect the first data-plane run found

**The failure budget was being spent on success.** G13 charged a token from the
per-address failure limiter at the top of the middleware — before knowing
whether the request would be refused. With one request per authentication that
was invisible. A data-plane round makes a dozen or more, so the second round of
the first REST exchange came back `429`, and a legitimate peer had throttled
itself out of its own library.

The fix separates the two questions the limiter answers. The budget is now
*checked* before any work and *spent* only on a refusal, so it bounds guessing
without bounding a peer that is behaving. Pairing is the deliberate exception —
a pairing attempt *is* a guess at a secret, so it spends whether it succeeds or
not. The per-peer request budget rose with it, from 120 requests a minute to
600: a round is tens of requests, and the old figure was chosen when a peer only
ever said hello.

## What the layers refuse, and in what order

A downloaded snapshot passes four gates before a single canonical row is
written, and the order is the resolved decision made literal:

1. the transport's own hash — the sealed bytes must be what the peer declared;
2. the frames — each authenticates its index and whether it is the last, so a
   truncated or reordered transfer cannot pass as a complete backup;
3. the container — ZIP entries that would escape their destination are refused,
   because a peer is authenticated rather than trusted;
4. **archive-v2's verifier**, which is what decides whether the result is a
   snapshot at all.

ZIP parsing is never the trust boundary. A tampered byte is caught at gate one,
before the container is opened — asserted by a test that checks *which* layer
refused, not merely that something did.

## What this does not cover

- **No network.** Loopback HTTP on one host; latency, bandwidth, and packet loss
  are not modelled. G12 measured what a real carrier's own cadence costs, and it
  dwarfs anything here.
- **No large corpus.** Two library sizes, because this measures a transport.
  G20 owns the large-corpus convergence run.
- **No scheduling.** Every exchange and every download is an explicit command.
  G15 owns durable jobs, retries, and backpressure.
- **No restore decision.** `fetch-backup` stops at a verified archive and prints
  the restore command; what a restore does to a library stays an explicit act
  with an explicit intent.
