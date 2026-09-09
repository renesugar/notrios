# v0.7 G18c.1 — Agent usage preflight and resumable workflow guard

Status: **complete 2026-08-27**

## Goal and outcome

This workflow amendment adds a practical remaining-usage check at boundaries
where an agent can stop cleanly. It does not attempt to predict local CPU test
time as model usage. Codex account telemetry is read through the installed
client's model-free app-server `account/rateLimits/read` request; Claude is read
only from an explicit local status cache while a Claude process is active.
Missing or changed telemetry is `unknown`, never an inferred 100%.

The proposed `codex exec /status --json` approach was rejected because
`codex exec` treats `/status` as a model prompt and does not emit the assumed
quota structure. The proposed Claude health prompt was rejected because it
spends model usage and still supplies no stable numeric quota contract.

## Implementation

- `scripts/check_agent_usage.py` normalizes current multi-window and legacy
  Codex payloads, reports the binding window, reset and client version, and
  reaps the complete helper process group on success or timeout.
- Claude process detection matches executable arguments instead of arbitrary
  command text. Cache values accept explicit used/remaining forms; an absent
  process is neutral and an absent/malformed cache is unknown.
- Exit 0 means proceed/advisory unknown/not running, exit 2 means pause, exit 3
  means strict unknown, and exit 1 means invalid invocation.
- Optional locked JSONL before/after samples stay outside repository evidence.
  Only completed pairs with the same agent, model, effort, operation, run ID,
  duration and reset window contribute. The maximum observed drop plus a five-
  point margin may raise, but never lower, the explicit 20% floor.
- `scripts/agent_usage_preflight.sh` supplies advisory, strict and off modes.
  The resumable G14b/G14e harnesses call it after completed-result reuse and
  before writing the next `started` checkpoint. Long profile and release
  scripts call it once before disposable setup. No quota output is persisted
  in evidence workspaces.
- `AGENTS.md`, environment/testing guidance, the context map, handoff and the
  repository skill describe version checks, safe pause behavior and exact
  resume documentation.

## Validation evidence

- Live installed-client check: Codex CLI 0.150.1 returned its five-hour and
  weekly windows through the app-server contract in under two seconds; no
  helper remained. Exact percentages/reset timestamps were kept only in a
  temporary external sample. Claude was not running and was reported as such.
- `python3 scripts/test_check_agent_usage.py`: 17 tests pass.
- `bash scripts/test_agent_usage_preflight.sh`: pass.
- `python3 performance/v0.7-g14b/test_harness.py`: 15 tests pass.
- `python3 performance/v0.7-g14e/test_harness.py`: 11 tests pass.
- `npm audit`, `npm ci`, `npm audit`: zero vulnerabilities.
- `npm run typecheck`, `npm test -- --run`: 17 files / 171 tests pass.
- `npm run build`: pass.
- `go test ./...` and `go vet ./...`: pass.
- `bash scripts/validate-scaffold.sh`: pass.
- `python3 scripts/check_required_files.py`,
  `python3 scripts/check_plan_loops.py`, `go run ./cmd/docaudit`,
  `bash scripts/run_offline_assets_check.sh`, skill validation and
  `git diff --check`: pass.

Models used: parent GPT-5 for contract discovery, architectural decisions and
final review; GPT-5.6 Luna workers for bounded probe/test, wrapper/harness and
profile integration assignments. Parent review consolidated duplicate harness
logic and removed quota output from evidence workspaces.

## Boundaries

No product behavior, application quota, schema, REST/MCP API, dependency,
credential, private dataset, Git remote, signed reserve, ISO, or physical
medium changed. CI validates fixtures but never queries a developer account.
No push was performed. G18d remains next and unapproved.
