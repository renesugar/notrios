# v1.0 J4 — what 1.0 promises, reviewed

The frozen surfaces are **113 REST routes** and **45 MCP tools**. Freezing made
change visible; it did not endorse anything. This is the review, and it is the
last milestone in which a route can be withdrawn without breaking somebody.

`REPORT.json` holds the derived facts — described, documented, exercised,
tested — per member. **None of those four is a verdict**, and the report says so
in its own text: a route with all four may still be a promise 1.0 should not
make, and one with none may be load-bearing and correct. They give a defensible
order to review in. The decisions below are judgements, and are labelled as
such.

| | REST | MCP |
|---|---|---|
| members | 113 | 45 |
| described in the machine-readable contract | 112 | 45 |
| mentioned in a published document | 113 | 45 |
| used by an example this repository **executes** | 39 | 1 |
| named by a test in the serving package | 63 | 45 |

## D1 — `PUT`/`DELETE /api/v1/sync/carrier/{class}/{name}`: **the owner decides**

**This is the one thing in the review that a person has to settle, and it is
recorded rather than decided here.**

The carrier routes have two shapes. Reads are namespaced —
`GET /api/v1/sync/carrier/{namespace}/{class}` and
`.../{namespace}/{class}/{name}` — and writes are not:
`PUT`/`DELETE /api/v1/sync/carrier/{class}/{name}`. The write handler derives
the namespace from the authenticated caller, which is a deliberate and good
security property: you cannot publish into somebody else's namespace because
the URL gives you no way to name one.

The cost is in the contract. Both two-segment forms collapse onto one OpenAPI
path, `/api/v1/sync/carrier/{segment1}/{segment2}`, where **the same parameter
means different things depending on the method** — `{namespace}/{class}` for
`GET`, `{class}/{name}` for `PUT` and `DELETE`. The parameters are called
`segment1` and `segment2` because no honest name exists for a parameter whose
meaning depends on the verb, and a client generated from this contract has
methods taking `segment1` and `segment2`.

- **Leave it.** The implicit namespace is a security property worth having, the
  collision is confined to the description, and the routes are in use by
  `notriosctl sync exchange` between peers that already speak this.
- **Change it before the freeze becomes a promise**, for example to a
  three-segment write path with an explicit marker for "my own namespace", so
  every carrier path has one meaning and honest parameter names. This is a
  breaking change to the sync wire, which is exactly what this milestone still
  permits and the next one does not.

**Nothing was changed.** A breaking change to a wire protocol between peers is
not a call this review makes on its own.

## D2 — three MCP tools nothing tested: **fixed**

`plan_sync`, `request_resource_fetch` and `retry_sync_job` were advertised by
the server, documented, and scope-mapped, and **no test anywhere in the
repository named them**. An advertised tool promises two things — that calling
it does something, and that the scope gates say who may — and neither was
checked.

`TestJ4EverySyncToolIsReachableAndGated` now covers all three: refused when the
sync scope is off, refused as control-tier when only status is allowed, and
dispatched when control is granted. It asserts nothing about what they return,
because J4's boundary is that a name freeze does not freeze behaviour.

Confirmed by adding a status-tier tool to the control-tier list and watching it
fail. MCP coverage is 45 of 45.

## D3 — `GET /` absent from the contract: **reviewed, correct as is**

The only frozen route OpenAPI does not describe. It serves the web interface,
not an API operation, and adding it to an API description would misrepresent
what it is. Kept, exempt, and now recorded rather than merely true.

## D4 — sync tools listed as read-only: **reviewed, kept**

`plan_sync`, `start_sync`, `retry_sync_job`, `cancel_sync_job` and the rest
appear in `mcpToolScopes` as `MCPScopeReadOnly`, which reads alarmingly for
tools that start and cancel work. They pass a second, orthogonal gate —
`mcp.sync_scope` in `mcpSyncToolScopes` — where they are `MCPSyncControl`, and
the code says so in a comment beside them. The arrangement is deliberate and
D2's test is now the evidence that both gates fire.

Recorded because a reviewer reading one map would reasonably raise it, and the
answer should be written down once rather than rediscovered.

## D5 — one MCP tool of 45 in an executed example: **not a surface defect**

The MCP surface is documented and tested; what it lacks is published examples
that this repository runs. That is a documentation-coverage question, it is
measured in `performance/v1.0-j13`, and migrating those pages is J15. Recording
it here so the low number is not mistaken for an untested surface: MCP is 45 of
45 tested and 1 of 45 exercised, and those are different facts.

## What this review did not settle

- **Behaviour.** Two releases can agree on every route and disagree about what
  one does. This is about what is promised.
- **Response shapes.** No schema in `api/openapi.yaml` was compared against what
  a handler returns. `performance/v0.9-i6` checks the OpenAPI document parses
  and `internal/docexec` executes 25 REST examples against a live server, but
  neither is a field-by-field contract check.
- **Whether 113 routes is the right number.** Every one is documented and all
  but one described. Nothing here argues that a smaller surface would be better,
  because nothing measured which routes have users.
- **`tested` is a weak signal, and it was weaker than that at first.** It
  searches `internal/httpapi/*_test.go`, so a route tested from another package
  counts as untested; it is not a coverage measurement and the 63 of 113 should
  not be read as one. For MCP it now requires the tool name **in quotes**,
  because a tool is called by name and only mentioned in prose — the first
  version matched the bare name, and a probe that removed `retry_sync_job` from
  D2's test still reported it tested, having found it in that test's own doc
  comment. A gate a comment satisfies is worse than no gate, and D2 depends on
  this one.
