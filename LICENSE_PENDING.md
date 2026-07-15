# License Pending

The license decision has been narrowed: Notrios will use **MIT or Apache-2.0** (final pick is a user decision, tracked in `agent/OPEN_QUESTIONS.md`; replaced in plan task R2).

Consequences already binding on all code:

- All project code and dependencies must be compatible with both MIT and Apache-2.0.
- GPL components (Recoll, Xapian) may only be used as external, user-installed processes — never linked, vendored, redistributed, or used as a source to derive code from (see `RECOLL_INTEGRATION.md`).
- Do not add third-party source code until its license compatibility has been reviewed.

Decision notes: Apache-2.0 adds an explicit patent grant; MIT is maximally simple and compatible with GPLv2-only tooling. Dual-licensing (MIT OR Apache-2.0) is also an option.
