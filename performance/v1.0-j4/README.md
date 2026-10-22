# v1.0 J4 — the REST and MCP surfaces 1.0 promises

```sh
python3 performance/v1.0-j4/review_surfaces.py          # write REPORT.json
python3 performance/v1.0-j4/review_surfaces.py --check  # compare, do not write
python3 performance/v1.0-j4/validate_evidence.py        # re-derive and check
```

`DECISIONS.md` is the review. `REPORT.json` is what it was made against:
described, documented, exercised and tested, per member, derived rather than
recalled — because 113 routes read in a list all look equally reasonable, and a
judgement is only worth having if it was made against evidence.

**None of the four facts is a verdict**, and the report says so in its own text.
They give a defensible order to review in and a way to notice the member nothing
refers to.

## What it found

**One thing only a person can settle (D1).** Carrier reads are namespaced and
carrier writes are not, so both two-segment forms collapse onto one OpenAPI path
where the same parameter means different things depending on the method — hence
`{segment1}`/`{segment2}`, because no honest name exists. The implicit namespace
is a real security property; the cost lands on anyone generating a client.
Changing it breaks the sync wire, which this milestone still permits and the
next does not. **Nothing was changed**, and the choice is recorded with both
options.

**Three advertised MCP tools that nothing tested (D2), now fixed.** `plan_sync`,
`request_resource_fetch` and `retry_sync_job` were advertised, documented and
scope-mapped with no test anywhere naming them.
`TestJ4EverySyncToolIsReachableAndGated` covers all three, and MCP is 45 of 45.

**Two things that looked wrong and are not (D3, D4).** `GET /` is absent from
the API contract because it serves the web interface; the sync tools sit in the
read-only tier because they pass a second, orthogonal scope gate. Both are now
recorded, so the next reviewer to raise them finds the answer instead of
rediscovering it.

## Two false findings, and what they cost

**The first draft reported two carrier routes as absent from OpenAPI.** They are
not: the contract describes them with different parameter names. Comparing route
*text* rather than route *shape* had made documentation style look like a
missing route. Chasing the false finding is what uncovered D1, which is the real
one — but the check was wrong and is now shape-based.

**The `tested` check was satisfiable by a comment.** A probe that removed
`retry_sync_job` from D2's test still reported it tested: the bare name appeared
in that test's own doc comment. MCP now requires the name in quotes, because a
tool is called by name and only mentioned in prose. D2's gate depends on this
check, so a gate a comment satisfies was worse than no gate at all.

Both were found by probing checks I had just written and believed. That is the
only reason either is in this file rather than in the report as a fact.

## Limits

- **Behaviour is not reviewed.** Two releases can agree on every route and
  disagree about what one does. J4's boundary says so and this respects it.
- **No response shape was compared** against what a handler returns.
- **Nothing argues 113 is the right number**, because nothing measured which
  routes have users.
- **`tested` counts only `internal/httpapi`**, so a route tested from another
  package reads as untested. It was enough to find three tools tested nowhere;
  it is not coverage.
