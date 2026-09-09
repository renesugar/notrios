# Coding Standards

Use the relevant Google style guide as the default unless this project overrides it:

- Go: https://google.github.io/styleguide/go/
- TypeScript: https://google.github.io/styleguide/tsguide.html
- C++: https://google.github.io/styleguide/cppguide.html
- Python: https://google.github.io/styleguide/pyguide.html

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

## General rules

- Prefer boring, maintainable code over clever abstractions.
- Keep core business logic out of protocol adapters and UI components.
- Use typed request/response structures and avoid `map[string]any` except at boundaries.
- Keep generated code separate and documented.
- Treat imported content as untrusted.
- Do not commit private datasets, local databases, binary caches, or credentials.

## Go

- Standard library first.
- Keep packages small and named by responsibility.
- Use `context.Context` for request-scoped work.
- Avoid panics in server paths.
- Return structured errors at service boundaries.
- Keep SQL parameterized.
- Put database migrations in `migrations/`.
- Add tests near the package under test.

## TypeScript/React

- Use TypeScript strict mode.
- Keep API client code in `web/src/api.ts` or generated clients.
- Sanitize all rendered Markdown/HTML.
- Do not let UI routing depend on filesystem paths as canonical IDs.
- Keep editor integration behind a wrapper so the editor can be replaced later.

## SQL

- Migrations are append-only after a release.
- Prefer explicit indexes for common access paths.
- Keep FTS5 and canonical tables synchronized in transactions.
- Avoid exposing internal rowids as public IDs.

## Markdown docs

- Keep root docs concise and actionable.
- Put deeper topic docs under `docs/` when they grow.
- Which root document owns which fact, and what to do when code changes one, is
  in `AGENTS.md` under "Keeping the reference documents current". It is there
  rather than here because it outlives any one coding standard, and because the
  one rule this section used to carry — update `CONTEXT_MAP.md` when adding a
  package — went unenforced for eleven packages. It is a build failure now:
  `go test ./internal/docrules/` fails when a package under `internal/` or
  `cmd/` has no entry in the atlas.
