# v1.0 J23: keep existing sync peers syncing after both upgrade in place

## The defect (found by J7-B)

`performance/v1.0-j7/upgrade_in_place.sh` paired two replicas on 0.7.0, 0.8.0
and v0.9. In each case:
- **Before the upgrade**, they converged.
- **While their versions differed**, they moved nothing.
- **Once both ran 1.0, they never converged again.** Each published and each
  refused the other on every round.

The cause:
1. **Pairing pins the peer's exact handshake.** `ConfigureSyncAdmissionPeer`
   stores it in `sync_peer_compatibility`: schema 27, range 24–27.
2. **Upgrading does not touch that row.** After both upgraded, each library was
   at schema 28 and still held its peer as 27, range 24–27.
3. **Admission requires an exact match.** `validateConfiguredSyncPeerLocked`
   called `sameSyncCompatibility`. The upgraded peer reported 28, range 24–28,
   and every batch was refused: "peer compatibility differs from explicit
   configuration".

## J23-A: the failing test, then the fix

`TestJ23UpgradedPeerIsAdmittedAndItsStoredCompatibilityUpdated`
(`internal/store/j23_peer_upgrade_test.go`) builds a real peer pairing with this
build's handshake. It then rewinds the stored row to what an upgraded library
holds (schema 27, range 24–27), because a 1.0 build cannot pair with a schema-27
handshake. Then it admits a batch from the peer's current handshake.

**On the unchanged code it failed**, with the error J7-B saw:

```
an upgraded peer was refused: revision conflict: peer compatibility differs from explicit configuration
```

**The fix.** `validateConfiguredSyncPeerLocked` still requires an exact match,
with one exception, `schemaRiseOnly`:
- the peer's schema version is **higher** than the stored one
- its compatible-range **ceiling** is equal or higher
- **protocol major and minor range, required and optional capabilities, and the
  range floor are unchanged**

Mutual range admission is not decided by this rule. `ValidateHandshake` already
runs first on every path, and refuses a peer whose range excludes this build's
schema, or whose schema is outside this build's range.

When the exception applies, the check returns the stored record. The caller then
runs `upgradeSyncPeerCompatibilityLocked` **inside its own transaction**:
- the row takes the new schema version and ceiling
- a `peer.compatibility_upgraded` audit event records
  `from_schema_version`, `to_schema_version`, `from_max_compatible_schema` and
  `to_max_compatible_schema`

A batch that fails after the update rolls the update back with it. Both callers
apply the rule:
- `AdmitSyncOperations`
- `RecordSyncPeerAcknowledgement`, so a round that only acknowledges is not
  refused either

With the fix the test passes. The peer is admitted, the stored row holds its new
schema and ceiling, and exactly one audit event is written. A second batch
admits without recording another upgrade.

Pairing itself is unchanged. `ConfigureSyncAdmissionPeer` still refuses to
re-pair a peer whose stored compatibility differs. J23's scope is admission,
and that path is noted here rather than changed.

## J23-B: what an explicit pairing pins is still refused

`TestJ23PinnedCompatibilityChangesAreStillRefused` gives each case its own
replica pair and the same rewound row. Each must be refused, with the stored row
unchanged and no audit event:

| peer handshake | result |
|---|---|
| a schema **lower** than the stored one, with a range that still admits this build | refused (`ErrConflict`) |
| a schema rise **and** a changed optional capability | refused (`ErrConflict`) |
| a schema rise **and** a changed protocol minor range | refused (`ErrConflict`) |
| a schema rise **and** a lower range floor | refused |

All four passed on the unchanged code, which refused every difference, and all
four still pass with the fix. The existing
`TestSyncAdmissionRequiresConfiguredCompatiblePeerAndExactReplay` also still
refuses a changed capability with no schema change.

**The store package's full suite passes (89 s)**, including the existing sync
admission, security, catch-up and retention tests.

**The sync packages built on admission also pass with the fix:**
`internal/synccarrier` (16.8 s), `internal/syncrest` (12.7 s),
`internal/syncstate` and `internal/httpapi` (30.8 s).

## J23-C: the drill that found it now converges

`performance/v1.0-j7/upgrade_in_place.sh` was re-run against the fixed build
(`6bf70d2`, no tracked changes). For each version:
- two replicas on that version pair offline and converge
- one is upgraded to 1.0, and nothing moves while the versions differ
- the other is upgraded, and each makes a new edit
- both must converge on their original pairing

| step | 0.7.0 | 0.8.0 | v0.9 |
|---|---|---|---|
| both on the older version: sync converges | pass, 2 rounds | pass, 2 rounds | pass, 2 rounds |
| upgrade the first to 1.0 (v27 → v28) | pass | pass | pass |
| mixed versions: nothing moves, nothing corrupted | pass | pass | pass |
| upgrade the second to 1.0 | pass | pass | pass |
| **both on 1.0: sync converges on the original pairing** | **pass, 2 rounds** | **pass, 2 rounds** | **pass, 2 rounds** |

In every case, both replicas' pre-upgrade edits are still present after the new
edits converge. The same drill failed all three final rows before the fix
(`performance/v1.0-j7/README.md`). Nothing named for Notrios was written outside
the drill's work directory.

The run shared the machine with J7-C's detached disaster-recovery drill. This
drill is a correctness check with no timing claims, so the overlap affects
nothing recorded here.
