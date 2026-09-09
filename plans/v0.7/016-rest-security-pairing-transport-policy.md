# v0.7 G13 — REST security foundation, pairing, and transport policy

Status: **complete**, 2026-08-13. Product remains 0.6.0; the canonical schema is
now **v25**. Implemented by Claude Opus 5 (`claude-opus-5`) under Claude Code,
after explicit user approval naming G13.

## Goal

Satisfy the security prerequisites that currently prohibit exposing Notrios
beyond loopback before adding sync endpoints.

## What landed

**`internal/syncauth`** — the peer principal, and everything that decides
whether a request is one. A credential is an Ed25519 signature over the method,
path, database id, replica id, timestamp, nonce, and body hash; the pairing
half is a short-lived single-use code, its HMAC proofs in both directions, and
AES-256-GCM wrapping of the group key under a key derived from that code. It
also holds the signing HTTP client, the replay cache, and the two rate limiters.

**Schema v25** — `sync_peer_keys` and `sync_pairing_invitations`, with enrol,
revoke, list, and a `syncwire.Verifier` implementation over the first, and
create/consume/revoke/list over the second. Consuming is one transaction.

**`internal/httpapi/sync.go`** — three routes and the middleware that guards
them: `GET /api/v1/sync/handshake` (authenticated), `POST /api/v1/sync/pair`
(invitation-gated), and `GET /api/v1/sync/status` (loopback-only, redacted).

**Transport policy** — `sync.rest` configuration, `config.ValidateSyncTransport`,
TLS serving in `notriosd`, and a startup refusal rather than a warning.

**CLI** — `sync invite|join|accept|enroll|handshake|peers|revoke`, replacing
G11's `sync bundle|pair`.

## Why a signature and not a token

A bearer token is a reusable secret that travels on every request, so anything
that logs, proxies, caches, or mis-terminates TLS ends up holding a credential.
A signature over the request is useless once that request is spent, and the
private key never leaves the replica. It also gives the properties this protocol
needs for free: the method and path are covered, so a signed read cannot be
replayed as a write or against another route; the body hash is covered, so the
body cannot be swapped; the database id is covered, so a peer of one library
cannot present the same request to another.

The replay cache is bounded per peer at more nonces than the rate limiter allows
within two skew windows, so an evicted entry is already unusable by the
timestamp check. That relationship between two limits is the argument; either
one alone would be a hole.

## Pairing, and what G11 got wrong

G11 shipped a development bundle containing the library's group key in clear
text. G13's resolved decision says a pairing bundle carries **no reusable
library decryption key in displayable text**, and there is no version of "handle
this file carefully" that fixes a file which *is* the key. Those commands are
gone.

What replaces them is one ceremony with two carriers. A code — short, spoken,
single-use, minutes long — is the transfer secret. Online, `sync join` spends it
in one exchange. Offline, `sync invite --offline` writes a file whose group key
is sealed under the same code, and the two halves travel separately: neither is
usable alone. The inviter enrols what comes back with `sync enroll`, which is
where an offline code is spent, in the same transaction an online one is.

Single use is a transaction rather than a convention: eight goroutines racing
one code produce one winner and seven refusals.

**The limit, stated plainly:** the joining side authenticates the inviter only
through the code. Whoever hands it over is trusted to be who they say. That is
why it is short, spoken, single-use, and expiring, and it is why G18 owns how a
code is presented.

## Two things moved, deliberately

**Peer public keys are database state now.** They are public, so nothing secret
moved; what was gained is that enrolment and revocation are transactional,
auditable, and visible to every process at once. A key file read by a daemon and
rewritten by a CLI is a race with a security outcome. The carrier's verifier
became `syncwire.MultiVerifier{own key, database peers}` in the same change.

**The key file holds secrets only** — this replica's signing key and the group
key per epoch — and gained `AdoptGroupKey`, `AdvanceEpoch`, and `RetireEpoch`.

## Decisions taken, and why

**A refusal, not a warning, at startup.** With the surface enabled, a
non-loopback listener without TLS, a certificate without its key, or unreadable
TLS material makes `notriosd` exit and name the setting. The empty host `:8080`
is covered explicitly, because reading it as "unspecified, therefore local" is
how a library ends up on a coffee-shop network.

**Every refusal says the same thing.** One status family and one message; which
check failed is in the local audit log under a closed vocabulary. A refusal that
explains itself is a refusal that teaches an attacker how to pass. The matrix
asserts this per row.

**A revoked key is reported as unknown.** "We no longer trust this" and "we
never knew this" are the same answer to a caller; the difference is a local
event with its own reason.

**Revocation and epoch advance are separate acts.** Revoking stops a device
signing. Advancing the epoch stops it reading what is published next, at the
cost of re-pairing every remaining peer, and the old epoch stays readable
because a library should not lose its own history to exclude a device.

**The surface is not for browsers.** No CORS header is ever emitted, and any
request carrying `Origin`, `Cookie`, or `Referer` is refused. A cross-site form
cannot set the authorization header, but it can carry cookies, and a surface
that ignored them would eventually be reached by a deputy that has some.

## Validation

Evidence is `performance/v0.7-g13/` (`auth-matrix.json`, `README.md`,
`FINDINGS.md`, `validate_evidence.py`): seventeen cases against a real service,
three authorized and fourteen refused, none of the refusals explaining itself,
plus five transport-policy decisions.

- `internal/syncauth`: every signed field proved binding by mutation, replay,
  clock skew in both directions, a key enrolled for another replica, code
  formatting survived being retyped five ways, the wrapped key needing its code,
  the four proof fields, the failure budget, the closed vocabulary, and the
  client's own refusal to sign over plaintext to a remote host.
- `internal/httpapi`: the authentication matrix as a table test, replay refused
  and audited, per-address rate limiting, a peer credential changing nothing on
  an ordinary route, the disabled surface, loopback-only redacted status
  including a forged forwarding header, and a pairing code spent exactly once.
- `internal/store`: idempotent enrolment, one key refused for two replicas, a
  revoked key invisible to the verifier and unable to return, eight concurrent
  callers racing one invitation, expiry and revocation, the secret never stored,
  and the v25 upgrade leaving earlier tables intact.
- `internal/config`: ten transport-policy rows, the strict defaults, and the
  parsed `sync.rest` block.
- `cmd/notriosctl`: two compiled binaries and a running `notriosd` — an
  unenrolled replica refused, a code issued and spent over the network, the same
  replica then authenticating, the code refused on reuse, an anonymous request
  refused with no CORS header, and the key revoked so it stops working; plus a
  service that refuses to start with the surface exposed without TLS.
- **Mutation-checked:** removing the replay cache, the browser refusal, and the
  admission check each makes exactly the intended test fail.

Repository validation ran audit-first per `AGENTS.md`.

## Out of scope, and still owned elsewhere

No data plane: the authenticated surface carries a handshake and a pairing
exchange, and G14 owns envelopes, objects, and resumable snapshot download. No
multi-user accounts, roles, sessions, or passwords. No ordinary note route
becomes remotely authorized. No public deployment claim, and **no external
security review has happened** — the plan requires one before any such claim.
Private key material stays in the warned `0600` development provider until v0.8
selects a platform store, and how a pairing code is presented to a person is
G18's.
