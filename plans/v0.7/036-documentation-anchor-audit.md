# v0.7 G18c — documentation anchor audit and executable-claim registry

Date: 2026-08-27
Model: GPT-5 (exact serving variant unavailable)
Working state: complete; G18d remains unapproved

## Goal and boundary

Make drift in the documentation/source/check graph deterministic and visible
before any bulk prose migration, example execution, renderer change, or model
review. This slice does not generate or relocate documentation, correct the
preserved schema-v20 prose, execute documentation examples or GUI journeys,
change product behavior/schema/API/dependencies, call an external model, push a
remote, alter the evidence reserve, or burn physical media.

## Implemented graph

`cmd/docaudit` and `internal/docaudit` parse the frozen no-space
`//notrios:doc`, `help`, `enumerates`, and `claim` grammar directly from Go AST
comment groups, which preserves the Go 1.25 minimum. A directive must attach to
exactly one named declaration and one audience. Fragment and claim IDs are
global lowercase kebab-case identifiers. Help slots are user-only and must
resolve to one of the 199 G18a Markdown H1-H3 template sections.

Go anchors resolve exactly one package declaration or receiver method in the
current module. TypeScript/TSX anchors pass through
`scripts/docaudit_ts.mjs`, which loads the exact compiler locked by the web
workspace and resolves exactly one top-level named function, class, interface,
type, enum, or variable. Bare paths, line numbers, anonymous, missing,
ambiguous, external, and repository-escaping forms fail. TypeScript 7 exposes
the needed scanner through its unstable AST module; real `App`, `EditorPane`,
and `SyncCenter` regression cases cover nested template-literal interpolation
and prevent the two hangs found during parent review of the delegated resolver.

The strict JSON registry maps four claim IDs to executable repository tests,
131 detected executable-shaped Markdown fences to exact path/section/language/
SHA-256 records, and all nine G18a GUI journeys to current TS/TSX owner
declarations. Unknown JSON fields fail. Executed examples or journeys require a
resolving check; the initial entries remain honestly unverified for G18d/G18e.
Every detected entry must be registered, and every registered entry must still
exist.

## First source-adjacent anchor set

Twelve fragments cover the highest-risk finite starting surfaces:

- product version and canonical schema claims;
- configuration keys and canonical defaults;
- CLI command/flag usage forms;
- the REST registration surface and exact destructive resource confirmation;
- MCP tool names, scope names, ordinary tool-scope assignments, and orthogonal
  sync tool-scope assignments; and
- the GUI peer-retirement confirmation handler.

The G18a inventory remains the checked extractor for 109 OpenAPI operations
with zero `operationId` values and zero MCP protocol resources. Those absences
stay visible rather than being invented as declaration anchors. The first
source set is intentionally small and adds comments plus one version assertion;
it changes no runtime branch or public contract.

## Honest baseline

`performance/v0.7-g18c/REPORT.json` reconciles 351 units:

- 199 existing Markdown sections, all unverified;
- 12 new source fragments: eight generated and four claimed;
- 131 executable-shaped Markdown fences, all unverified; and
- nine proposed GUI journeys, all unverified.

The total is therefore 0 executed, 8 generated, 4 claimed, and 339 unverified.
Grades belong to individual units, not nearby prose. The manual
`docs/service.md` schema-v20 statement consequently remains an unverified unit
beside the separately claimed canonical schema-v27 declaration. The audit is
green because every graph edge is explicit, not because every statement is
true; G18d and later slices must improve grades with result-bearing evidence.

## Deterministic failures and integration

The Go suite exercises 20 graph mutations: unknown/mixed audiences, malformed
or unattached directives, duplicate fragments and claims, missing template
slots and declarations, dangling claims, orphan checks, deleted production and
test anchors, unknown anchor kinds, and unaccounted/duplicate/orphan executable
and journey entries. It separately proves that adding a generated source unit
does not hide the known-drift manual section. The web suite covers eight
TypeScript resolver classes, including missing/ambiguous/nested declarations,
path refusal, deterministic ordering, and the three real component regressions.

`make docaudit` is the local command. The CI web job installs the locked npm
workspace plus Go and validates the exact checked report. Release packaging
runs the same freshness validator after its locked frontend install, so the
source ZIP cannot silently carry a stale graph. The ordinary Go job needs no
Node dependency: it runs the complete Go graph tests with a deterministic TS
resolver seam, while the web job exercises the actual compiler path.

## Validation evidence

- Required npm audit before and after `npm ci`: zero vulnerabilities.
- Focused Go documentation-audit suite: pass, including all 20 mutations and
  the known-drift before/after fixture.
- Focused TypeScript resolver suite: one file/eight tests pass; exact CLI
  resolution of every registered GUI owner passes; frontend typecheck passes.
- Exact current `docaudit` report and `performance/v0.7-g18c/validate_evidence.py`:
  pass with 351 reconciled units and no stale graph edge.
- Full repository, frontend, docs/offline, scaffold, evidence pre-push, release
  ZIP, and clean-extraction validation results are recorded in the completion
  attempt log.

Evidence lives under `performance/v0.7-g18c/`. No generated site, private data,
secret, model output, runtime database, or build artifact is committed.
