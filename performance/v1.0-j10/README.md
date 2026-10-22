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
