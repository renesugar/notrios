# v1.0 J10: publish the user-authorized release

The plan is in `PLAN.md` under J10. This directory records each step as it
happens, in the order the plan gives.

```sh
python3 performance/v1.0-j10/plan_volumes.py     # the seal's volumes, held to the plan
```

## J10-A: the seal, planned

v1.0's 49 archives, 1,267,409,548 bytes, need two volumes. `plan_volumes.py`
derives `VOLUME_PLAN.json` from `performance/v1.0/MANIFEST.json`, so it needs no
evidence directory and runs in validation:

| Volume | Items | Archives | Payload bytes | Under the budget |
| --- | --- | --- | --- | --- |
| `NTR-EV-0005` | J1 to J26 | 25 | 636,677,987 | 44,896,413 |
| `NTR-EV-0006` | J27 to J50 | 24 | 630,731,561 | 50,842,839 |

J10 is in neither: its archive is taken at its close, after the release. The
cut is the one in item order that makes the larger volume smallest. A greedy
fill would have put J27 in the first volume, too, leaving it 19 MB under the
budget rather than 45.

Before anything was sealed, the pre-push evidence gate passed against the
reserve, and the reserve verified four volumes, `NTR-EV-0001` to `NTR-EV-0004`.
It listed 55 artifacts as unsealed: the 49 above and the six superseded v0.8
archives.

## J10-A: the seal

**Rehearsed first, 2026-10-22.** Both volumes ran end to end against a throwaway
ed25519 key, in a GnuPG home of their own on a scratch reserve, with a scratch
copy of the catalog. The rehearsal chained onto the real volume-0004. Each build
took about three and a half minutes. Both ISOs were built twice and came out
byte-identical, and both were read back and verified. The scratch catalog grew
to six entries, with the real four-entry catalog a byte-exact prefix of it. No
rehearsal file entered the repository.

**Sealed, 2026-10-22, on the owner's authorization.** Both volumes are signed by
the production subkey `4ABEB98A…68264005` and timestamped by DigiCert under the
pinned policy `2.16.840.1.114412.7.1`:

| Volume | Archives | ISO bytes | Blocks | Timestamp | Content commit |
| --- | --- | --- | --- | --- | --- |
| `NTR-EV-0005` | 25 | 637,296,640 | 311,180 | 2026-10-22T14:31:20Z | `8706d77` |
| `NTR-EV-0006` | 24 | 631,343,104 | 308,273 | 2026-10-22T14:39:42Z | `3eb36a8` |

`NTR-EV-0005` chains onto `NTR-EV-0004` and `NTR-EV-0006` onto `NTR-EV-0005`,
each by the hash of the previous checkpoint document. The catalog is at six
entries, and each append left the earlier entries a byte-exact prefix.
`plan_volumes.py` finds each volume's committed signatures equal to the plan.

The volumes are on the reserve drive only. Burning them to disc is a separate
step that the owner has not asked for.
