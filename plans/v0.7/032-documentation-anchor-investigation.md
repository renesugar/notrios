# v0.7 G18a — documentation anchors, truth grades, and review calibration

**Status:** complete

**Date:** 2026-08-26

**Model:** GPT-5 (exact serving variant unavailable)

## Goal and boundary

G18a investigated how Notrios can make source-linked documentation finite and
auditable before any prose migration or generator exists. It changed no manual
prose, product behavior, schema, API, dependency, docs renderer, or Help seed.
It called no external model and introduced no non-deterministic CI result.

## Inventory baseline

The checked inventory covers all 15 Markdown pages used by both the public site
and read-only Help notebook. Their 199 non-fenced H1-H3 sections are the migration
denominator: G18c may split a section into finer source-adjacent fragments, but
it may not lose a section. Adjacent claim surfaces are also finite:

- 55 `notriosctl` usage forms from `printHelp`;
- 59 JSON-tagged configuration keys;
- 109 OpenAPI operations and zero `operationId` values;
- 46 MCP tools and zero MCP protocol resources (the `read_resource` tool is not
  a `resources/read` capability); and
- nine proposed GUI journeys with explicit action-count targets, still labelled
  proposed and not executed.

All 199 manual sections begin at `unverified`. Independent implementation tests
do not silently upgrade prose that has no fragment or check link. This makes the
future grade delta reviewable instead of retroactively treating existing prose
as proved. Every section has one strongest grade from `executed`, `generated`,
`claimed`, or `unverified`; rationale is removed before the denominator.

## Directive and symbol contract

The grammar is `//notrios:doc`, `help`, `enumerates`, and `claim`. The absence of
a space after `//` is material. Each declaration doc group owns exactly one
audience and one globally unique fragment ID. Help slots are user-only; finite
lists and checks name declarations rather than expressions. Rationale remains
ordinary unmarked commentary.

Go anchors use the module import path plus a package declaration or receiver
method. TypeScript/TSX anchors use a repository-relative module plus a named
top-level declaration. Bare paths, lines, anonymous closures, ambiguous names,
and missing symbols fail. Review includes the root and at most eight statically
resolved direct callees at depth one, within the same Go package or the `web`
npm workspace. Dynamic, third-party, anonymous, and transitive calls are out of
scope; insufficient source yields `not-determinable`.

Go 1.27 confirms that recognized directives remain visible in
`ast.CommentGroup.List` and disappear from `CommentGroup.Text()`, while a spaced
or uppercase spelling stays ordinary text. The module minimum is Go 1.25, but
`ast.ParseDirective` was added in Go 1.26. G18c must therefore use a small
compatibility parser over `CommentGroup.List`, take prose from `Text()`, and run
the same fixtures on minimum and current toolchains.

## Contradiction calibration

Eight human-labelled cases cover all three permitted outcomes: `supported`,
`contradicted`, and `not-determinable`. The real stale statement in
`docs/service.md` names schema v20 while `CurrentSchemaVersion` is 27 and is
preserved as contradicted evidence. A controlled Recoll pair differs only by
the word “not” but has opposite verdicts, demonstrating why cosine or semantic
similarity cannot determine agreement. Other cases cover Trash/restore direct
callees, an immediate-purge mutation, unreviewable SQLite rationale, and a
universal cloud-provider claim outside the bounded source slice.

No model judgement is stored in the calibration set. If G18f later uses blind
code explanation and comparison, it must first report accuracy against these
labels and remain advisory; it cannot fail deterministic CI or rewrite prose.

## Finite follow-on recommendation

- G18c implements only parsing, symbol resolution, inventory extraction, grade
  reconciliation, and deterministic fixture failures.
- G18d attaches current prose and typed registries without changing meaning.
- G18e executes CLI/config/REST/MCP examples and GUI journeys, checks results,
  and records measured action lengths.
- G18f adds optional calibrated three-verdict/actionability review only after
  the deterministic pipeline exists.
- G18b and G18g own Hugo/Ledger independently; raw Markdown remains the Help
  source and source anchoring does not depend on a site theme.

## Evidence and validation

`performance/v0.7-g18a/` contains the deterministic builder, checked inventory,
directive grammar, source-symbol rules, calibration set, ordinary/directive Go
and TSX fixtures, a Go AST behavior probe, validator, and mutation tests.
Validation checks stale hashes and counts, every current anchor, duplicate and
dangling fragments, mixed audiences, dangling Go/TS anchors, one-hop callee
scope, grade reconciliation, negation, and rationale. Standard repository,
frontend, docs, scaffold, release-package, and mandatory evidence-reserve gates
were run before completion; exact results are recorded in the attempt log.
