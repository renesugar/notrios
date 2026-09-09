# v0.7 G16 — Sync, pairing, catch-up, encrypted-backup, and conflict UI

Completed: 2026-08-24

Model: GPT-5 (exact serving variant unavailable)

## Outcome

G16 adds a responsive, comprehensible local Sync Center over the existing G3-
G15 synchronization contracts. It adds no schema, mobile build, periodic sync,
retention decision, silent merge, automatic restore, peer retirement, or purge.

## Implemented contract

- The header and dialog name the active profile, logical library, and replica.
  Setup supports `none|directory|rest`; native local Wails mode binds a directory
  chooser, while browser and GUI-only remote modes expose no misleading local
  picker. Configuration is owner-only, requires restart, and never enrolls or
  starts work implicitly. Inbound peer service is a separate checkbox.
- Discovery reports without enrolling. Pairing consumes one short-lived code.
  Complete-snapshot permission is separately granted/revoked per active peer;
  pairing alone cannot hand another replica a full library.
- The dashboard projects bounded job progress, cancellation/retry, offline and
  peer behind/retired state without returning raw parameters, target paths,
  credentials, keys, or content-bearing audit detail.
- Lazy resources show unavailable/pending/local state. Download and pin write
  local intent and queue only a bounded resource-fetch job; a REST request/
  serve/retry flow materializes bytes only after all existing G8 checks pass.
- Conflict review shows the common base, current displayed head, and other head.
  Saving creates one immutable revision naming both conflicting parents, so the
  answer converges instead of merely hiding the conflict.
- Notebook repair rows remain visible. Catch-up/reset creates a durable local
  catch-up job; REST catch-up resumes, authenticates, decrypts, and verifies a
  physical snapshot into a private inbox. Neither action installs it. Reset and
  uploaded-backup restore stay behind a separate destructive review/shutdown
  handoff.
- NPB1 is a password-portable wrapper for the same deterministic, verified
  physical snapshot and NBK1 sealed payload used by peer catch-up. Argon2id
  wraps a fresh random payload key; exact sealed length/hash and physical
  admission are checked before review. Passwords are bounded, never persisted
  or copied to the clipboard, and cleared after success or failure. Wrong
  password reports that nothing changed and leaves retry/cancel explicit.

## Secret-provider correction

The approved v0.7 default remains an injectable interface backed by an explicit
warned `0600` development file—not a claim of native keychain support. The live
two-process test found that the service originally opened separate key-file
objects and captured the group key at startup. Pairing updated disk but a
running worker retained the pre-pair key. G16 now shares one provider instance
between local UI, inbound peer security, catch-up, and jobs, and resolves the
current group key at every carrier attempt. v0.8 still owns native providers.

## Validation evidence

- Focused store tests prove three-way detail, deterministic two-parent
  resolution and convergence, resource intent, and repair reports.
- Portable-backup tests prove exact round trip and typed wrong-password refusal.
- HTTP tests prove loopback-only access, explicit profile identity, no secret
  disclosure, owner-only configuration/restart, HTTPS policy, explicit inbound
  setting, separate snapshot permission, and wrong/correct backup review with
  no application.
- Six React flows cover active identity/Escape and labels, explicit start/retry,
  native directory selection, three-way resolution, pairing-code-only
  clipboard, and password show/clear/retry/cancel.
- A real two-daemon product flow passes pairing, exchange, overlapping edits,
  lazy attachment transfer, resolution, backup failure/retry, explicit snapshot
  permission, and verified non-installing catch-up. See
  `performance/v0.7-g16/`.
- Browser plugin unavailable; regular Playwright 1.63 + Google Chrome passes
  content-free 1440×960 and 390×844 sweeps with exact viewport containment,
  keyboard Escape, touch controls, and zero console errors.
- Full repository validation, audit, build, smoke, OpenAPI parity, and package
  evidence are recorded in the completion attempt log.

## Boundaries retained

Conflict, retention, and restore decisions remain human choices. The web layer
cannot receive key material or arbitrary native paths; only the local Wails
bridge can choose a directory. The password is never remembered. Uploaded
backup review and downloaded catch-up staging cannot mutate canonical state.
G17 owns peer retirement, horizons, tombstone/resource collection, and repair.
