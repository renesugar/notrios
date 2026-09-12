# v1.0 J13-A — what the published examples actually are

```sh
python3 performance/v1.0-j13/classify_examples.py          # write REPORT.json
python3 performance/v1.0-j13/classify_examples.py --check  # compare, do not write
python3 performance/v1.0-j13/validate_evidence.py          # re-derive and check the properties
```

The examination before the work, because **"99 of 164 examples are unverified"
is one number covering several different situations**, and acting on it without
separating them would have deleted good documentation to improve a statistic.

## What a block is

| Class | Count | What it is |
|---|---|---|
| recipe | 116 | literal command lines, nothing to substitute |
| synopsis | 45 | optional-argument or metavariable notation: a reference form |
| fragment | 2 | a configuration body, not a command line |
| transcript | 1 | a `$ ` prompt, so a session rather than a command |

A **synopsis** — `notriosctl search [--limit N] … "<query>"` — is not a broken
example. The brackets are optional-argument notation and the angle brackets are
metavariables; it is correctly unexecutable, and replacing it with one worked
invocation would make a reference list *worse*. A **recipe** looks like
something a reader can paste, so a recipe nothing has run is a promise nobody
checked. Crossing the two with the execution registry is the point of this
slice.

## What that cross gives

|  | executed | not executed |
|---|---|---|
| recipe / transcript | 63 | **54** |
| synopsis | 0 | 45 |
| fragment | 2 | 0 |

**Zero executed synopses, and that is the classification's own consistency
check** rather than a coincidence: a block with a metavariable in it cannot have
been run, so a single synopsis marked executed means the classifier is wrong.
The validator asserts it, and it is how three drafts of the metavariable rule
were caught — see below.

**The 54 unverified recipes are not one problem either.** By recorded reason:
19 shared-user-state, 17 host-installation, 9 external-network, 4
interactive-or-long-running, 3 illustrative-placeholder, 2
privileged-host-change. Most of those are honest: a recipe that installs a
package as root, or deletes this user's library, cannot run in a test and runs
in the container matrix or the drills instead. J13-D's target is the ones whose
reason does not survive contact with the block.

## The finding only a cross-reference produces

**Ten recorded reasons contradict their own block.**

- **Seven synopses** are excused as `interactive-or-long-running`,
  `shared-user-state` or `external-network`. A synopsis cannot be run at all, so
  none of those is *why* it was not run. The reason names a cause that could
  never have applied.
- **Three literal recipes** are excused as `illustrative-placeholder`: a
  runnable block filed as decoration. Those are in `docs/api/rest.md`,
  `docs/installation.md` and `docs/operations.md`, and each is either runnable
  or wrong.

## Two gaps, stated in the words the evidence supports

**`docs/configuration.md`: 53 settable keys, 0 examples.** J12 gave the page
prose and a generated key table and no command lines at all, which is the gap
J13-C closes.

**61 of 92 commands have no executed *published* example.** The wording matters
and the first draft of this got it wrong by calling them "never shown working".
Most are exercised heavily — by `cmd/notriosctl`'s own tests, by the I4, I7 and
J3 drills, by the container matrix. The claim is narrowly about documentation: a
reader of `docs/` is shown the form and never shown it run. Conflating "not in
a published example" with "untested" would be exactly the kind of number that
makes a report untrustworthy.

## Why the extraction is trusted

The blocks come from `docs/docaudit/registry.json`, not from a second scanner.
Re-implementing `internal/docaudit`'s fence scanner here would give this tool
its own opinion about what a fenced block is; instead bodies are extracted from
the documents and matched to the registry **by sha256**, and a single mismatch
refuses the whole report rather than reporting on something else. It fired
twice while this was written:

- once because the slug function was not `docaudit`'s. That one removes `.`
  rather than replacing it, so "Upgrading from before 0.8" is
  `upgrading-from-before-08`; the obvious regex gave `…-0-8` and one id went
  missing.
- once, deliberately, as a probe: changing a registered body in
  `docs/query-language.md` is refused by name.

**The metavariable rule took three drafts, and each false positive was a real
pattern in these documents.** One regex for both bracket forms labelled seven
executed examples as synopses: `[a](Kitchen)` is a Markdown link inside a JSON
payload; `.result.tools[].name` is a jq filter; and `"tags":["publish"]` is a
JSON array whose `[` sits outside the quoted key. The rule now counts angle
brackets anywhere — they are not valid shell, JSON or jq here — and square
brackets only outside quoted spans and only when the contents are not JSON-ish.
Blanking a quoted span to `''` was itself the third bug, because `["publish"]`
then read as a bare-word option; the placeholder keeps a quote.

## What this does not do

- It does not judge whether an executed example is a *good* example, only that
  it ran.
- It does not classify fences `internal/docaudit` considers non-executable, so
  a prose code block with no command in it is outside the count. A *new*
  executable fence that is not registered is caught by `docaudit`'s own gate,
  which was confirmed by appending one and watching that test fail — this tool
  deliberately covers the registry rather than duplicating that check.
- `commands_with_no_executed_published_example` matches on the command name
  appearing in an executed body, which is generous. It is a report about where
  to look, not a gate.
