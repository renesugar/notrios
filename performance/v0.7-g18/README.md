# v0.7 G18 portability handoff evidence

G18 is an evidence and contract slice. It does not implement a shared library,
installer, Flutter client, APK, Wails migration, Mermaid rendering, or physical
device support.

The audit found useful seams but not a finished application facade:

- `store.Store`, `SyncSecretStore`, sidecar interfaces, durable jobs, and
  context-aware operations are reusable without Wails.
- Wails imports are confined to `cmd/notrios/gui_wails.go` behind the `gui`
  build tag.
- `internal/service` still owns `*httpapi.Server` and `*http.Server`, and 24
  production HTTP-adapter files use `net/http`. v0.8 must extract orchestration
  before adding the ABI; wrapping the HTTP handler in a loopback call is not an
  application facade.
- 43 `internal/store` files import C. Linux `c-shared` and `c-archive` probes
  built against the host `libsqlite3`, but the existing main package exports no
  C symbols and therefore emitted no header. The Android/arm64 cgo probe stopped
  at `sqlite3.h` because the installed NDK has no SQLite development boundary.

Evidence files:

- `SOURCE_AUDIT.json` — dependency, route, cgo, build-mode, and seam facts.
- `PLATFORM_MATRIX.json` — finite path/permission/lifecycle ownership matrix.
- `ABI_CONTRACT.json` — proposed v1 symbols, ownership, handles, errors,
  cancellation, polling, streams, and REST-to-facade translation.
- `MERMAID_CONTRACT.json` — honest disabled baseline and exact v0.8 fixture/
  security/performance acceptance contract.
- `ANDROID_FEASIBILITY.json` — installed-tool and cross-compile findings plus
  the bounded emulator gate.
- `validate_evidence.py` and `test_validate_evidence.py` — deterministic source
  and evidence checks. They perform no network access and read no private data.

The Linux build artifacts were generated only under `/tmp` and are not release
libraries. Their hashes show repeatable evidence from this run, not a frozen ABI.
