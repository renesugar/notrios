# v0.7 G18d — executed CLI, configuration, REST, and MCP documentation examples

Date: 2026-08-28
Model: GPT-5 parent; GPT-5.6 Luna workers for bounded registry classification and fixture helpers
Working state: complete; G18e remains unapproved

## Goal and outcome

Every one of the 131 non-GUI executable-shaped fences frozen by G18a/G18c now
has a strict bidirectional registry contract. Sixty-three literal examples run
against isolated scratch state and 68 remain explicitly unverified under six
finite reviewed reason codes. There are no missing, orphaned, duplicate, stale-
hash, or provisional entries. Twelve of the thirteen command-bearing topics
execute; the index topic is the reviewed external clone/build exception.

## Execution harness

`internal/docexec` builds the real `notriosctl` and `notriosd` binaries, creates
fresh non-vacuous SQLite stores, starts per-example loopback REST/MCP services,
applies only declared substitutions, and dispatches through closed adapters.
Each executed entry pins its literal fence hash, fixture, surface, adapter,
expected typed result, and semantic postcondition. Status alone never passes.

REST examples validate both request and response against the shipped OpenAPI
document, including path parameters, bodies, content types, destructive
confirmation, and undocumented status refusal. MCP examples validate JSON-RPC
envelopes, tool schemas, scope, and bounded read results. Configuration examples
parse through the production loader and assert documented overrides plus
defaults. CLI examples inspect canonical rows, tags, Trash state, archives,
snapshots, graph exports, profile files, stable links, dry-run parity, import
plans, and preserved state.

Real minimal Joplin RAW, Obsidian, Twitter, ChatGPT, Claude, and native archive
fixtures replace private input. The harness never accesses an external network,
user database, user path, shared state, or secret.

## Contract defects found and corrected

Execution exposed four documentation/contract contradictions:

- two archive-v1 commands targeted the same refusing output directory; the
  query-scoped example now writes `todo-archive`;
- the Obsidian apply example consumed a review config its dry run did not write;
  the dry run now names `--write-config` explicitly;
- the optimistic-concurrency REST recipe reused a stale revision for its final
  delete; it now rereads the current revision;
- the resource download recipe used content-disposition naming where an
  existing local file made the copied sequence collide; it now uses an explicit
  output filename.

OpenAPI execution also pinned already-required server behavior that had drifted:
quoted flow descriptions, missing sync path parameters, create-document
collection/notebook shape, resource PNG/range schemas, health content type, and
an empty warnings array rather than `null`. These changes align implementation
and schema with the approved existing behavior; no new product feature or API
surface was introduced.

## Mutation and evidence gates

Manifest tests separately reject changed fence bodies, unknown fields, missing
or unused adapters, unresolved substitutions, wrong result status, and failed
semantic postconditions. OpenAPI mutations reject bad request types, missing
confirmation, bad response shapes, and undocumented statuses.

`performance/v0.7-g18d/REPORT.json` records the checked per-topic counts and a
positive local runtime. Its validator reruns all 63 examples and compares every
deterministic field while allowing runtime to vary. CI, scaffold validation,
required-file checks, and release packaging invoke the evidence gate. The G18c
report remains its frozen zero-execution baseline; G18d owns current execution
freshness.

## Validation evidence

- 63-entry real repository harness: pass; checked execution runtime about 20
  seconds, total Go test wall time about 30 seconds.
- 131-entry registry: 63 executed, 68 reviewed unverified, zero provisional.
- G18d evidence validator and helper test: pass.
- G18c frozen baseline validator: pass after transfer of current-state ownership
  to G18d.
- Full repository, frontend, documentation/offline, scaffold, packaging, and
  clean-extraction validation are recorded in the completion attempt log.

No private corpus, runtime database, generated site, external service, model
output, Git remote, evidence reserve, ISO, or physical medium was changed. No
push or burn was performed.
