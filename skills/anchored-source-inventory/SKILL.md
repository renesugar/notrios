---
name: anchored-source-inventory
description: Build an audit inventory that breaks when the code it describes changes, instead of silently going stale, by anchoring each entry to a source substring that must occur exactly once.
---

# Anchored source inventory

Use when an investigation slice inventories many places in the source — path
consumers, permission sites, error paths, feature flags — and a later slice will
change them.

The failure this prevents: an inventory is written, the code moves, and the
inventory keeps describing code that no longer exists. Nobody notices, because
nothing reads it mechanically. The same drift produced four spellings of a plan
closing paragraph in v0.7 and three completed items that looked unfinished.

## The rule

Each entry carries `{file, anchor}` where `anchor` is an exact source substring
that must occur **exactly once** in that file. A validator re-checks every
anchor and fails on zero matches *and* on two.

```python
count = open(os.path.join(REPO, entry["file"]), encoding="utf-8").read().count(entry["anchor"])
require(count == 1,
        f"{entry['id']}: anchor occurs {count} times in {entry['file']}; "
        "the consumer has changed and the inventory must be revisited")
```

Failing on two matches matters as much as failing on zero: a duplicated line
means the thing being inventoried has been copied, and an inventory that names
one of two copies is worse than one that names neither.

Pick an anchor that is *about* the behavior — the line that computes the path,
not a nearby comment or import. Prose changes should not break the inventory,
and a behavior change should.

## Cross-checks worth adding beside it

- Every category, owner and defect id in the inventory is declared somewhere
  else in the evidence, so two documents cannot drift apart.
- Every declared owning change owns at least one entry. A change nobody needs is
  a change the next slice would implement for no reason.
- The investigation boundary: assert that the defaults the slice promised *not*
  to change are still there. An investigation that quietly implemented itself is
  the failure mode worth a test.
- If a report cites test names, assert each one exists.

## Verify the validator bites

Before trusting it, break each rule on purpose and confirm the failure: change
an anchor, cite a nonexistent test, orphan an owning change. A validator nobody
has seen fail is a validator nobody knows works.

## Example

`performance/v0.8-h3/` — 25 path consumers, `PATH_CONSUMERS.json` plus
`validate_evidence.py`, wired into `scripts/validate-scaffold.sh`.
