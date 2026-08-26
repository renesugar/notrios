# G18a documentation-anchor investigation

G18a freezes a finite, source-resolvable method before any manual prose is
moved. `INVENTORY.json` is the checked snapshot of all 15 published Markdown
pages and their Help-notebook mirrors at H1–H3 section granularity, plus the
CLI, config, REST, MCP, and proposed GUI-journey surfaces. Rebuild it only with:

```bash
python3 performance/v0.7-g18a/build_inventory.py --write
python3 performance/v0.7-g18a/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18a -p 'test_*.py'
go test ./performance/v0.7-g18a/go_probe
```

## Findings and recommendation

- Go 1.27 retains recognized directive lines in `CommentGroup.List` and removes
  them from `CommentGroup.Text()`. A space after `//` makes the line ordinary
  prose. `go/ast.ParseDirective` is newer than the module's Go 1.25 minimum, so
  G18c should use a tiny compatibility parser over `CommentGroup.List`.
- Anchors identify declarations, never paths plus line numbers. Go anchors use
  import paths and TypeScript anchors use a repository module plus a named
  top-level declaration. Review is limited to the root and one direct-callee
  hop, with a hard budget; insufficient scope yields `not-determinable`.
- One doc group has one audience. User, API, and maintainer fragments cannot be
  mixed. Rationale remains ordinary unmarked commentary.
- Grades form a strongest-evidence ladder: executed, generated, claimed, then
  unverified. The current manual has no fragment/check links, so every one of
  its 199 non-fenced section units honestly starts unverified. Existing independent tests
  do not silently upgrade prose that never names them.
- The calibration set includes an actual contradiction (`docs/service.md`
  still describes schema v20 while `CurrentSchemaVersion` is 27), a one-word
  Recoll negation pair, rationale, and a direct-callee boundary. Similarity is
  therefore unsuitable as an agreement verdict.

## Finite follow-on contract

G18c implements only directive parsing, declaration resolution, inventory
extraction, four-grade reconciliation, and deterministic lint errors against
these fixtures. G18d attaches existing prose and typed finite registries without
changing prose meaning. G18e executes examples/journeys and asserts results.
G18f may add advisory blind explanation/comparison only after matching this
labelled set; its three-way verdict never fails deterministic CI. G18b and the
later Hugo/Ledger migration remain independent of this source-anchor contract.
