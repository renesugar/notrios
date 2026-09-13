# v1.0 J16 — every carrier path means one thing

Writes to the sync carrier moved from `/api/v1/sync/carrier/{class}/{name}` to
`/api/v1/sync/carrier/mine/{class}/{name}`, where `mine` is a literal segment.

## Why

v1.0 J4's D1 found two routes with the same shape after `carrier`:

| route | meaning of the two segments |
|---|---|
| `GET /api/v1/sync/carrier/{namespace}/{class}` | a namespace, then a class |
| `PUT`/`DELETE /api/v1/sync/carrier/{class}/{name}` | a class, then a name |

OpenAPI can only describe one path per shape, so both were published as
`/api/v1/sync/carrier/{segment1}/{segment2}`: a parameter whose meaning depended
on the verb, with no honest name. A client generated from the contract got
methods taking `segment1` and `segment2`, and `internal/docgen` carried a special
case to fold the two shapes together. A careful reader of that contract built
the wrong model of what the routes do, which is what made the fix worth a
breaking change.

## What did not change

The authorization. A carrier namespace is
`HMAC(routing key derived from the group key, "replica" ‖ replica_id)`: nobody
chooses it, and a write derives it from the authenticated principal so that a
replica cannot publish as another one. `mine` is a literal, never a parameter,
so the URL still gives a caller no way to name somebody else's namespace.

## Why it was free now

No release has been published, so no peers existed outside this repository. The
old write path is removed rather than kept alongside: keeping both would have
shipped the ambiguity permanently to stay compatible with peers that do not
exist. After a release the same change would have needed a migration window.

## What moved

- `internal/httpapi/sync_data.go` — the two write routes.
- `internal/service/service.go` — the remote-peer allow-list, which must name
  the same routes or a remote peer is refused the new path while the old one no
  longer exists.
- `internal/syncrest/carrier.go` — the client's publish and remove paths.
- `api/openapi.yaml` — one path with two meanings became two paths with one
  each; `{segment1}`/`{segment2}` no longer appear anywhere in the contract.
- `internal/docgen/repository.go` — `normalizeRESTPath` is the identity again.
- `docs/api/rest.md` (regenerated) and `docs/docfeatures/FEATURES.json`.
- `performance/v0.9-i8/FROZEN.json` — the REST surface re-recorded: still 113
  members, with exactly the two write routes moved.
- `performance/v1.0-j4` — the surface review regenerated, and D1 marked
  resolved.

## Proof

- **The round trip.** `internal/syncrest`'s exchange tests publish, list, read
  and remove through the real client against the real server, so they exercise
  the new paths end to end. They pass.
- **Impersonation is still refused, now for both shapes.**
  `TestAPeerCannotPublishIntoAnotherNamespace` tries `PUT` and `DELETE` against
  both the removed two-segment shape and the three-segment read shape with
  another replica's namespace in it. None may return a success status.
- **The contract describes every carrier route under its own path.** J4's
  regenerated review reports all six carrier routes as described in OpenAPI, and
  `GET /` remains the only undescribed REST route.
- `make validate` passes.
