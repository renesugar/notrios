# G18f generated documentation and advisory review

**Status:** complete on 2026-08-30.

## Goal and boundaries

G18f makes the source-anchored user/API subset and finite product surfaces
deterministically fresh while using local semantic review only to surface
questions. It does not switch the production documentation site, upload source,
read note/database content, let a model register tests, execute arbitrary model
output, rewrite prose automatically, or make probabilistic output a build gate.

The non-blocking execution decision resolved to the planned default: an
explicit maintainer command against the already-approved loopback
`llama-server`. No hosted model, dependency, recurring charge, external
network, reserve write, push, ISO, or physical burn was used.

## Deterministic generation

`docgen --user` and `docgen --api` use
`docs/docgen/templates.json`, not source order. The strict template places 15
separate blocks (13 user and two API) in five existing Markdown pages. Generated
markers are audience-specific; bytes outside those markers remain manual. The
same frozen `internal/docaudit` parser supplies fragments.

Closed repository adapters derive 412 rendered rows from the real registries:

- 59 configuration keys and 49 default leaves;
- 55 CLI usage forms;
- 109 REST registrations with exact OpenAPI parity;
- 46 MCP tools, four ordinary scopes, 46 ordinary-scope assignments, and seven
  sync-scope assignments;
- 37 G18e GUI journeys.

`TestDocsAreCurrent` regenerates both audiences in memory and compares them to
the committed site/Help Markdown. Focused tests cover stable ordering,
user/API separation, manual-byte preservation, missing sections, template
closure, mutation detection, and exact registry counts. The known G18a stale
schema prose and CLI output were corrected from v20 to the canonical v27.

## Local advisory review

`doccheck` accepts only plain HTTP loopback endpoints. For each G18a calibration
case and each user fragment it resolves the declaration plus at most eight
bounded same-package callees, strips source-adjacent prose, and asks for a blind
source explanation before revealing the claim. Go and TypeScript source slices
are bounded; all 13 repository user slices fit under the conservative 24 KiB
model budget.

The Qwen 2.5 Coder 1.5B model could not reliably choose among three labels in
one prompt. Consistent with the supplied task guidance, comparison was
decomposed into two recorded binary questions: whether a direct conflict
exists, then—only if not—whether the explanation establishes the whole claim.
Those answers deterministically map to contradicted, supported, or
not-determinable. Two repetitions record the model, prompt/source hashes,
tokens, timings, explanations, intermediate answers, final label, and variance.

The final labelled calibration was 7 correct of 16 repeated decisions and was
highly variable. The model contradicted at least one repetition for 11 of 13
actual fragments. Each carries a human disposition that defers to exact
generation or a registered executable claim test. This is intentionally honest
evidence that the small model is an unreliable disagreement generator, not a
release oracle.

Actionability sees prose alone and may propose a command, configuration, API
request, or GUI journey. Nothing is executed directly. Only an exact body in
the closed G18d manifest or exact identity/label in the G18e journey manifest
can be accepted. All 26 recorded attempts failed that exact match and were
rejected. The G18d fixture was independently rerun before report acceptance.

## Evidence and validation

Committed evidence is under `performance/v0.7-g18f/`:

- `REPORT.json` freezes document hashes and registry counts;
- `ADVISORY_REPORT.json` contains the repeated local review;
- `TRIAGE.json` contains reviewed dispositions;
- `MUTATION_MATRIX.json` and Python tests break freshness, markers, audience,
  calibration accounting, triage, closed fixtures, and policy independently.

Validated commands include:

```sh
go run ./cmd/docgen --user --api --check
go test ./internal/docgen ./internal/docaudit ./internal/doccheck ./cmd/docgen ./cmd/doccheck
python3 -m unittest discover -s performance/v0.7-g18f -p 'test_*.py'
python3 performance/v0.7-g18f/validate_evidence.py
python3 performance/v0.7-g18d/validate_evidence.py
```

The final regular validation, release packaging, archive hash, and commit are
recorded in the repository handoff and attempt log.

## Models

- GPT-5/Codex: architecture, implementation integration, safety boundaries,
  human triage, validation, and final review.
- Luna workers: bounded docgen/client/validator scaffolding and routine tests,
  reviewed and corrected by the parent agent.
- Qwen 2.5 Coder 1.5B Instruct Q4_K_M: local advisory inference only.
