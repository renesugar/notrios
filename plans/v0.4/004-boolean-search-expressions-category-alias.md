# v0.4 Q1 — Boolean search expressions and category alias

Status: complete (2026-08-03)

Model: GPT-5

## Scope

Replace the flat search parser with one bounded, backend-neutral expression
tree supporting uppercase `OR`, implicit `AND`, prefix `-`, grouping, quoted
phrases, typed fields, and `category:` as an exact `notebook:` alias. Preserve
URLs, hyphenated words, unknown colon tokens, recursive notebook semantics,
All-notes behavior, emoji, and query-bound pagination.

## Implementation

- Added a 4,096-byte, 256-token, 16-level parser with stable canonical AST
  serialization. Implicit `AND` binds more tightly than `OR`; invalid syntax
  and unsupported Trash shapes are rejected as bounded input errors.
- Compiled the AST to parameterized SQLite SQL/FTS5. Positive text-only
  subtrees retain FTS relevance; mixed, grouped, negated, metadata, and emoji
  expressions use exact set predicates, with mandatory positive text anchors
  where available. Cursor fingerprints bind to canonical structure instead of
  raw spelling.
- Compiled supported shapes to fully parenthesized Recoll syntax, lowering
  grouped negation through De Morgan because installed Recoll does not accept
  negated groups. Shapes Recoll cannot honor exactly—Trash, unbounded All
  notes, or unsupported emoji combinations—use an explicit SQLite-only path,
  never an approximation.
- Projected recursive notebook ancestry and stable Unicode-symbol keys for
  live Recoll parity. Added exact SQLite/Recoll fixtures for grouping,
  negation, category recursion, title/author fields, and emoji.
- Validated saved search notebooks on creation and exposed the grammar/limits
  through REST status, MCP schemas/descriptions, GUI help, OpenAPI, and living
  documentation.
- Extended the generated large-library profile with boolean/category
  correctness and latency measurements under `performance/v0.4-q1/`.

## Validation

- `go vet ./...`
- `go test ./...`, including installed live Recoll parity
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- Python frontmatter-handler compilation and OpenAPI YAML parse
- `cd web && npm ci && npm run typecheck && npm test -- --run && npm run build`
  (5 files, 39 tests)
- `bash scripts/build_docs_site.sh` (11 pages)
- `make gui`
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
- `bash scripts/run_large_library_profile.sh 10000 /tmp/notrios-v0.4-q1-10k.json`
- Real Chrome/Playwright interaction: `(alpha OR beta) -tag:private 😀`, no
  console warnings/errors
- Xvfb/Openbox native resize at 1024×768, 1400×900, and 1920×1080: no body
  overflow and equal editor/preview resizing
- `git diff --check`

The 10k profile recorded ordinary first/next/deep p95 values of 3.128,
3.468, and 2.974 ms. Text `OR` was 1.504 ms p95 and `category:` was 3.331 ms
p95. A deliberately nonselective negated-field query measured 288.542 ms p95
and is reported separately rather than represented as an ordinary-page claim.

## Next task

P1, the shared selection and privacy planner, remains unstarted and requires
explicit user approval.
