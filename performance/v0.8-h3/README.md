# v0.8 H3 evidence — installed paths, XDG, migration, destructive lifecycle

Investigation only. No path default changed, no data moved, no Make target
added, nothing deleted.

Read [`REPORT.md`](REPORT.md) first.

## Reproduce

```bash
go test ./performance/v0.8-h3/pathprobe/     # characterizes current behavior
python3 performance/v0.8-h3/test_resolve_model.py    # regenerates RESOLUTION_TABLE.json
python3 performance/v0.8-h3/test_purge_oracle.py     # regenerates PURGE_ORACLE_FIXTURES.json
python3 performance/v0.8-h3/validate_evidence.py     # cross-checks all of it
```

## Files

| File | What it is |
| --- | --- |
| `REPORT.md` | The findings, the proposed contract, and what is not verified |
| `PATH_CONSUMERS.json` | 24 path consumers, each anchored to an exact source substring |
| `LAYOUT.json` | The proposed per-OS root matrix, precedence, install variables, manifest schema |
| `RESOLUTION_TABLE.json` | Generated: 14 environments resolved through the proposed rules |
| `PURGE_ORACLE_FIXTURES.json` | Generated: 30 purge decisions |
| `resolve_model.py` | Executable model of the proposed resolver |
| `purge_oracle.py` | Reference decision procedure for "may Notrios delete this?" |
| `pathprobe/` | Go tests characterizing today's behavior |
| `validate_evidence.py` | Re-checks anchors and cross-checks the documents |

## The probe tests are meant to fail later

`pathprobe/` asserts what Notrios does *today*, including the parts H3
recommends changing. Each such test names the H4 change that should break it and
says so in its failure message. When H4 lands, they should be deleted or
inverted along with the fix — the failure is the reminder, not a regression.
