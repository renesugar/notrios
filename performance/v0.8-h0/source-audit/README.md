# H0 current-source audit

`audit.py` scans production Go sources (tests excluded) and records the
handler, route, service/store, SQLite, dependency-direction, and Wails-isolation
signals that determine the application-facade migration cost. It compares
numeric fields with the v0.7-g18 baseline without embedding a current commit
or expected count. The evidence records a SHA-256 over sorted production source
paths and bytes, so it remains valid across commits until audited source changes.

From the repository root:

```sh
python3 performance/v0.8-h0/source-audit/audit.py --write
python3 performance/v0.8-h0/source-audit/audit.py --check
```

`SOURCE_AUDIT.json` is generated evidence. The scan is intentionally
text-structural; `go test` and the H0 ABI/SQLite probes remain the authority for
compile-time and runtime behavior. No production source is modified.
