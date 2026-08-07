# v0.6 F2 — MCP tool scopes

Status: complete on 2026-08-07.

Model: Claude Opus 5 (Claude Code).

## What it does

Four cumulative scopes decide which MCP tools a client sees and may call:
`search-only`, `read-only` (the default), `editor`, `organizer`. They are
enforced at the call site, not only by filtering `tools/list`. The config key is
`mcp.default_scope`, with `mcp.default_profile` kept as a deprecated alias.

## The defect this closes

Before F2, `tools/list` filtered write tools by profile and the dispatcher had a
matching guard — but only for writes. Every *read* tool dispatched
unconditionally. There was no narrower-than-read-only tier, so nothing was
hidden that could be called; the shape of the bug was latent rather than live.

F2 makes it structural instead of incidental. One table maps every tool to the
narrowest scope that may call it, and both the listing and the dispatcher read
it. A tool hidden from a scope is refused when invoked directly, with an error
naming the scope required and the scope in force.

Verified by mutation: deleting the call-site check makes
`TestHiddenToolsAreRefusedWhenCalledDirectly` fail, and the failure is
instructive — `append_to_note` *executes* and complains that `text is required`,
which is precisely "hidden but answers when called".

## Decisions worth recording

**Four scopes, not the roadmap's five.** `administrator` is absent by decision
(recorded in round 1 of the v0.6 open decisions): garbage collection, purge,
archive restore, and publication are unreachable over MCP at any scope, so a
scope naming them would cover an empty set. `TestNoScopeReachesWholeLibraryOperations`
asserts that against tool names that do not exist yet, so adding one later fails
the test and forces the decision to be taken deliberately rather than by
omission.

**"Scope", not "profile".** The word already meant two other shipped things: a
named local database (`notriosctl profile register`) and a saved publication
selection (`notriosctl publish profile save`). Both of those are "a saved named
configuration", which is coherent; a permission tier wearing the same word is
not. The permission tier moved.

**When both keys are set, the narrower wins**, and the service says so. A
configuration key that silently stops applying is bad generally; for this one it
would *widen* what an agent may do, which is the single direction that must
never happen quietly. Taking the narrower makes a half-migrated config strictly
no more permissive than either half intended.

`config.Default()` now leaves **both** keys empty rather than pre-filling the
deprecated one, because a default config that looks half-migrated would emit a
deprecation warning nobody caused.

**An unrecognized value falls back to `read-only` with a warning**, rather than
failing closed to the narrowest scope. Failing closed is defensible in general,
but here a typo would silently disable reading and look like a broken service.
The documented default plus a log line is what an operator can actually debug.

**Every tool must be classified, and an unclassified one is refused.** Not
defaulted to the narrowest scope — refused. A default would let a tool ship
without anyone deciding how much trust it needs, which is the dangerous kind of
safe.

**Scopes are cumulative**, and a test asserts it. Each entry in the table then
answers one question — "how much trust does this need" — rather than "which
tiers include it".

**`search-only` can see structure but not content.** Notebook, tag, collection,
and search-notebook lists carry no note body, and searching by notebook or tag
is unusable without them. The line is drawn at note *content*, which is what
`search-only` exists to withhold.

**F1's batch surface arrives here as `run_batch`, under `organizer`.** F1 shipped
it without an MCP tool on purpose, so bulk mutation would not land on the default
read-only surface before there was a tier to gate it.

## Validation

`go vet ./...`, `go test ./...`, required-file, plan-loop, and scaffold checks,
OpenAPI parse, migration-copy equality, web typecheck/tests/build, `make gui`,
docs-site build, Help reseed, REST/MCP smoke, performance smoke, and the
offline-assets check.

Plus a mutation check on the property the slice exists for: removing the
call-site enforcement must fail the test that asserts it.
