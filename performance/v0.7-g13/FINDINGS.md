# G13 findings — a credential that authorizes one thing

## The matrix

Seventeen cases against a real service with one enrolled peer. Three were
authorized; fourteen were refused. Every refusal was uniform, and none of them
explained itself.

| Case | Route | Status | Audited as |
|---|---|---:|---|
| an enrolled peer | handshake | 200 | `peer.auth_ok:ok` |
| no credential at all | handshake | 401 | `malformed` |
| a signature that is not the enrolled key's | handshake | 401 | `bad_signature` |
| a revoked credential | handshake | 401 | `unenrolled_key` |
| a signature for another path | handshake | 401 | `bad_signature` |
| a signature for another body | handshake | 401 | `bad_signature` |
| a stale timestamp | handshake | 401 | `stale_timestamp` |
| a browser, by `Origin` | handshake | 403 | — |
| a browser, by `Cookie` | handshake | 403 | — |
| an enrolled key whose replica is not admitted | handshake | 403 | `not_enrolled_for_admission` |
| a peer credential on an ordinary note route | documents | 404 | — |
| pairing with no code | pair | 401 | `malformed_code` |
| pairing with a code nobody issued | pair | 401 | `unknown_or_expired` |
| pairing with a valid code | pair | 200 | `pairing.consumed` |
| status from loopback | status | 200 | — |
| status from elsewhere | status | 403 | — |
| status from elsewhere claiming to be loopback | status | 403 | — |

Three rows deserve their own sentence.

**A revoked key audits as `unenrolled_key`, and that is the design.** "We no
longer trust this" and "we never knew this" are the same answer to a caller.
The difference is in the local log, where the revocation is its own event with
its own reason.

**A peer credential on an ordinary note route did nothing.** The route answered
404 for a note that does not exist — exactly what it answers to an anonymous
local caller, asserted side by side. That is G13's resolved decision made
mechanical: sync authentication is not a login, and no ordinary route becomes
remotely authorized because a peer surface exists.

**A forwarded-for header did not make a remote request local.** The loopback
check reads the connection's own address, because a header is written by
whoever is talking to us.

## Transport policy, decided at startup

| Listen address | TLS | Service starts |
|---|---|---|
| `127.0.0.1:8080` | no | yes |
| `0.0.0.0:8080` | no | **refused** |
| `:8080` | no | **refused** |
| `192.168.1.10:8080` | no | **refused** |
| `0.0.0.0:8443` | yes | yes |

A refusal rather than a warning, and the message names the setting to change.
A service that starts and *then* turns out to have been serving plaintext to a
network is worse than one that does not start: the first failure is discovered
by someone else.

An empty host is in the table because it is the case an operator most often gets
wrong. `:8080` binds everywhere, and reading it as "unspecified, therefore
local" is how a library ends up on a coffee-shop network.

## What pairing is

A short-lived, single-use **code**, and nothing else. It is not a library key;
it is a transfer secret whose whole job is to carry trust once. The group key
travels back sealed under a key derived from it, so:

- the file half of an offline pairing cannot be opened without the spoken half;
- the code is worthless the moment it is spent or expires;
- no artifact anywhere contains a reusable library key in readable text.

That last property is why G11's development bundle is gone. It carried the group
key in clear text, which is exactly what G13's resolved decision says a pairing
artifact must not do, and there is no version of "carry this file carefully"
that fixes it.

Single use is a transaction, not a convention: eight callers racing one code in
a test produce one winner and seven refusals, and the winner is recorded.

**What pairing does not prove.** The joining side authenticates the inviter only
through the code. Whoever hands the code over is trusted to be who they say —
which is why it is short, spoken, single-use, and expiring. A stronger mutual
proof needs a channel this protocol does not have, and G18 owns how the code is
presented.

## Two things that moved

**Peer public keys are database state now**, not entries in a key file. They are
public, so nothing secret moved; what was gained is that enrolling and revoking
are transactional, auditable, and visible to every process at once. A key file
read by a daemon and rewritten by a CLI is a race with a security outcome.

**The key file holds secrets only**: this replica's signing key and the group
key per epoch. Which peers you trust is `sync peers`; ending that trust is
`sync revoke`.

## What this does not claim

- **No data plane.** The authenticated surface carries a handshake and a pairing
  exchange. Envelopes, objects, and snapshots are G14's, and the body limit is
  set for the small requests that exist today.
- **No multi-user anything.** There is no user, no role, no session, and no
  password. One credential means one replica of one database.
- **No public deployment claim.** `SECURITY_REVIEW.md` says so directly, and
  G13's exception to the loopback posture is exactly one surface wide.
- **No external review.** Every control here was designed and tested inside the
  project. The plan requires an external security review before any public
  deployment claim, and this is not one.
- **Not a platform secret store.** Private keys still live in the warned `0600`
  development file; v0.8 owns that.
