# G14d recovery state machine

Physical restore is a stopped-service, local-filesystem operation. The caller
must say `replace` or `adopt`; merge, fork, incompatible schema, and automatic
destructive recovery are refused. Snapshot paths never come from REST or MCP.

The adjacent owner-only plan advances durably through:

1. `initialized`
2. `emergency_verified`
3. `assets_staged`
4. `prepared`
5. `cutover_blocked`
6. `previous_assets_moved`
7. `assets_installed`
8. `previous_database_moved`
9. `installed`
10. `verified_installed`
11. `complete`

Before the first destructive rename, the coordinator creates and completely
verifies a G14c emergency snapshot of the current canonical database and local
assets. It then writes a startup blocker. Ordinary database open refuses while
that blocker exists; only the restore coordinator's narrowly scoped verifier
may open the installed candidate. Re-running the same command rolls forward
from the recorded stage. A different input, intent, or target is refused.

The staged image rotates the allocator to a new replica ID, retains the source
as a real peer, installs the authenticated snapshot vector and catch-up floors,
clears copied peer acknowledgements, and queues every document for external
index projection. Same-schema SQLite FTS/link/block state is retained but
non-authoritative; Recoll is rebuilt from the outbox. Database rows that declare
remote/unavailable resources remain declared unavailable and do not cause an
invented local object.

After installed identity verification, the blocker is removed. The emergency
snapshot remains; redundant raw previous paths are removed. The active plan is
renamed to a unique `.completed.json` record so a later explicit restore can
start. A failure injected after recording `complete` still leaves the canonical
plan in place and a retry archives it.

The durable restore workspace has bounded shape: one staged incoming snapshot
plus the old canonical state and its verified emergency snapshot. External
objects remain in 256 MiB/65,536-entry USTAR packs in transit; install expands
only the canonical content-addressed asset fanout. The wrapper itself is a
sequential deterministic USTAR stream, not a semantic trust boundary.
