# Plan Archive

Completed plans are archived here by product version/milestone.

Format:

```text
plans/v<major>.<minor>/<NNN>-<slug>.md
```

Each archive should include the goal, status, files changed, validation evidence, model history, and follow-up tasks.

An item is archived when the record of *executing* it has outgrown the plan. `PLAN.md` keeps the item's specification and its outcome, and links here; the archive keeps the working record verbatim, because a summary of a finding is not the finding. The test is length and kind rather than age: an item whose text is a specification plus a paragraph of outcome reads fine in place, while one that has accumulated slice reports, corrections and findings is describing finished work in a document somebody reads to learn what is left.

Pre-v0.1 milestone reports are archived by milestone name:

- `plans/scaffold/` — scaffold-era creation plan and step reports (`SCAFFOLD_*.md`).
- `plans/mvp/` — v0.1 MVP task and release reports (`MVP_*.md`).

These are historical records; they intentionally keep the old "Notes Companion" naming and superseded design decisions.
