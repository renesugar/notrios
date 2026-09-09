# v0.7 G12 — directory-carrier conformance over a mapped cloud folder and removable media

G11 built the shared-directory carrier and measured it on a local filesystem,
which is the one place a protocol designed for hostile carriers is guaranteed to
work. G12 runs the same shipped code against a **real cloud provider** — a
Google Drive folder mounted with `rclone mount` — and against a directory that
is alternately available to each peer, the way a USB drive is.

The harness drives `internal/synccarrier` exactly as `notriosctl sync` does. It
adds no product code: if a phase fails, the protocol is wrong, not the harness.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g12-gocache go vet ./performance/v0.7-g12/...

env GOCACHE=/tmp/notrios-g12-gocache go run ./performance/v0.7-g12/cmd/conformance \
  -carrier <a folder on the mounted provider> \
  -remote  <the same folder as an rclone remote path> \
  -out performance/v0.7-g12/conformance-results.json \
  -notes 12

PYTHONDONTWRITEBYTECODE=1 python3 performance/v0.7-g12/validate_evidence.py
```

`-remote` is optional and enables the visibility probes, which publish through
the provider's API and time how long the mount takes to notice. Without it the
protocol phases still run; the provider measurements are simply absent.

## Evidence

- `conformance-results.json` — provider behavior, eight protocol phases with
  timings, and the resulting carrier shape;
- `FINDINGS.md` — what the provider actually does, which protocol decisions it
  vindicates, and what this run does not prove.

## Safety

Everything written is generated. The libraries themselves stay on local disk —
only sealed artifacts reach the provider — and the committed results record
aggregates and behaviors, never a path, a title, or a note. The harness refuses
to run `rclone sync`, `bisync`, `move`, `delete`, or `purge`; the refusal is
enforced in code and asserted by a phase, not merely written down. The only
rclone verbs it uses are `copyto`, `lsf`, and `copy --immutable`. The folder it
creates on the provider is removed after the run.

## Scope

One host, one provider, one network, one run. These numbers describe Google
Drive through `rclone mount` on the day they were taken; another provider, or a
different mount configuration, will differ. What is not provider-specific is
which protocol properties had to hold for the run to pass at all, and those are
in `FINDINGS.md`.
