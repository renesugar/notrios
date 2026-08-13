# v0.7 G11 — ephemeral shared-directory protocol and peer discovery

Status: **complete**, 2026-08-13. Product remains 0.6.0 and the canonical schema
remains **v24** — G11 adds no migration. Implemented by Claude Opus 5
(`claude-opus-5`) under Claude Code, after explicit user approval naming G11.

## Goal

Synchronize without a direct connection, through a disposable folder that peers
can recreate and inspect safely.

## What landed

**`internal/synccarrier`** — the carrier and the round that drives it.

- `Carrier` is the transport-neutral surface (initialize, publish, namespaces,
  list, read, remove); `Directory` implements it over a shared folder, and
  G14's REST data plane can implement the same operations without touching the
  round.
- `Round.Run` is one complete exchange: scan advertisements, admit envelopes,
  record acknowledgements, serve requested object bytes, publish what peers
  lack, publish this replica's own requests, advertise last, and optionally
  clean up its own artifacts.
- `Round.Discover` reads and reports without publishing, admitting, or
  enrolling anything, so asking who else uses a folder cannot announce this
  replica to it.
- `Provider` implements the `ObjectProvider` interface G8 defined and left for
  this slice, so lazy attachment materialization now runs over a real transport
  with its verification unchanged.
- `StoreReplica` is the only file that knows a database exists. Everything else
  is written against interfaces.

**`internal/synckeys`** — the locked-file development secret provider v0.7's
resolved decision calls for: one group key per epoch, this replica's Ed25519
signing key, and paired peers' public keys, in a `0600` JSON file that refuses
to open if other users can read it. It satisfies `syncwire`'s `KeyRing`,
`Signer`, and `Verifier`, so the protocol never learns where a key came from.
v0.8 owns the platform secret store; every command that touches this warns.

**`syncwire.CarrierName` and `syncwire.PeekSignerKeyID`** — a distinct blind
domain for path segments, and a way to name an unverifiable artifact's claimed
signing key without decrypting or believing anything else about it.

**`store.SyncPeerEnrolled`, `store.ListSyncPeers`, `store.UnavailableBlobs`** —
three reads the carrier needs and none of it had: whether a replica is
configured for admission, what each enrolled peer has acknowledged, and which
resources are known but unavailable.

**`notriosctl sync init|bundle|pair|status|discover|once`** — the first way to
actually run any of this. `once` performs one exchange and then materializes up
to `--materialize` attachments through the same carrier.

## The layout, and the three corrections it forced

`SYNCHRONIZATION.md` carried an illustrative layout. Implementing it against
G0's metadata budget corrected it in three places, and the document now records
all three:

1. **`objects/sha256/ab/cd/<hash>` publishes plaintext content hashes.** That is
   precisely the leak G0 froze the routing rule to prevent: anyone holding the
   same file could confirm the library holds it. Object addresses are keyed
   blinds under the group key.
2. **`<first>-<last>` in an envelope name publishes a sequence range**, and
   request ranges and acknowledgement positions are on the encrypted side of the
   budget. Names are blinds of what an artifact logically covers.
3. **`acknowledgements/` is not a separate class.** A G5 contiguous vector *is*
   what a replica has durably admitted from every peer, so the advertisement is
   already the acknowledgement. Two artifacts for one fact can disagree.

Every path segment below the layout version is therefore an HMAC blind, which
also makes the layout case-safe: a case-insensitive filesystem cannot fold two
lowercase-hex names together, and a name in any other shape is ignored.

Names are stable rather than derived from sealed bytes because **every seal
draws a fresh salt**. Byte-named artifacts would leave a new file per round on a
shared drive forever; measured over ten quiet rounds at both library sizes, this
layout adds zero.

## Two defects the work found

**A torn artifact could never be repaired.** The first implementation treated an
existing name as proof the artifact was there and skipped the publish — correct
for an immutable name, wrong for a real carrier. A half-copied file keeps its
name, so the peer waiting for it would wait forever while the sender believed it
had sent it. Publishing now asks whether a *readable* copy is there, by reading
its own artifact back and opening it. The truncation test fails without the
change and passes with it, and it is also what keeps a quiet carrier from
growing, since an unchanged artifact is recognized rather than rewritten.

**A fresh carrier was a standoff.** Publishing was driven only by advertisements
found on the carrier, so on an empty folder neither replica had anything to
publish for a peer it had not yet heard from. On removable media that is not a
lost round trip but a physical journey: the drive would have to visit the second
replica once merely to say hello. A round now starts from what the journal
durably remembers about each enrolled peer — the acknowledgement G5 already
records — and a carrier advertisement replaces that memory when one is present,
because the peer is the authority on itself and a stale memory must not suppress
work a reset peer needs.

## Decisions taken, and why

**Any enrolled replica may initialize the carrier**, as the resolved decision
required. A carrier with an owner stops working the day that peer is retired.

**Cleanup is off by default**, and the resolved decision is why: correctness
must survive no cleanup at all. A test runs the whole exchange with cleanup
disabled and a one-operation envelope limit, so the carrier accumulates and
convergence is unaffected. When it is on, a writer removes only its own
superseded advertisements and only those of its own envelopes that *every*
enrolled peer's vector covers — and an envelope's coverage is read from the
envelope rather than inferred from its name, because a naming convention would
break silently the day a batch size changed, by deleting something a peer still
needed.

**Relaying is allowed.** An envelope's sender is the replica whose key signed
it; the operations inside may be authored by anyone. That is what lets a
three-replica group converge through one folder when one member is offline.

**Discovery reports, and never enrolls.** An artifact from an unenrolled signing
key is refused before decryption, so the only thing knowable about that
candidate is the key id — and the candidate says exactly that, claiming no
replica id or database. A signing key that *is* enrolled for a replica that is
not configured for admission yields a richer candidate and still applies
nothing.

**The namespace is bound to the identity inside the artifact.** Both peers hold
the group key, so an enrolled peer could write into another's folder; a
namespace whose blind does not match the advertised replica id is refused.

**Pairing has an order, and it is a real constraint.** Adopting a group key is
refused once a replica has peers, because it would make everything those peers
published unreadable. So the joining replica pairs first and hands out its own
bundle afterwards. The CLI test performs the ceremony in that order and the
error names the reason.

## Validation

Evidence is `performance/v0.7-g11/` (`carrier-results.json`, `README.md`,
`FINDINGS.md`, `validate_evidence.py`), produced by real replicas exchanging
through a real directory.

- 200 and 1,000 notes converge in three rounds; five artifacts hold a thousand
  notes; ten quiet rounds add nothing; a carrier deleted with a batch in flight
  is recovered in two rounds.
- **The carrier layer is about one percent of an exchange**: 100 ms to seal,
  sign, and publish 1,200 operations and 125 ms to read, verify, decrypt, and
  decode them, against 21,838 ms for the whole exchange, which is G5 admission
  committing to SQLite.
- Listing one class took 362 µs at 100 artifacts, 3.9 ms at 1,000, and 50 ms at
  5,000 — where it returned the 4,096 cap rather than everything present.
- `internal/synccarrier` (22 tests): two- and three-replica convergence,
  carrier deletion and republication, truncation, an unenrolled signer, an
  unpaired replica, artifacts moved into a foreign namespace, provider
  sidecars/conflict copies/uppercase names/directories among the artifacts,
  idempotent publication, cleanup refusing another namespace, correctness with
  cleanup disabled, an unavailable mount, content-name verification, the
  no-rename fallback, an entry that vanishes between listing and reading, an
  oversized entry refused before it is read, stable listing order, two
  concurrent writers, removable media in two trips, attachment bytes travelling
  on request, and an object nobody publishes staying unavailable.
- `cmd/notriosctl` (3 tests, multi-process): two compiled processes converge
  through a shared folder after an archive-v2 adopt and a mutual pairing;
  discovery reports an unpaired peer and changes nothing; a world-readable key
  file is reported by `status` and refused by `once`.
- A carrier-wide assertion reads every path and every byte in the folder and
  fails if a note title, a body, a replica id, or the database id appears in
  any of them.

Repository validation ran audit-first per `AGENTS.md`.

## Out of scope, and still owned elsewhere

No cloud provider or removable device is *measured* — G12 owns that evidence,
and a local filesystem must not stand in for it. No snapshot archive travels
over the carrier: the class is reserved and G14 owns the resumable transfer. No
scheduling, watcher, daemon integration, or retry policy — G15 owns durable sync
jobs. No REST, MCP, or UI surface. Peer authentication and a real pairing
ceremony remain G13's; the platform secret store remains v0.8's; carrier cleanup
is not canonical garbage collection, which stays G17's.
