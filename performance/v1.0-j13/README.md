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

---

# J13-C — the configuration page's use-case examples

Four executed examples and one that says why it cannot be, added to
`docs/configuration.md`, which J12 left with 53 documented keys and no command
lines at all.

Each answers the question the key table structurally cannot: **what do I use
this for, and what else has to be set with it.** So each writes a small file
with the keys that go *together* and then runs the command that proves they
took effect:

| Section | Keys that must be set together | Why one alone is not enough |
|---|---|---|
| where your notes are stored | `data.directory`, `data.database_path`, `data.asset_store` | `directory` is only where the others *default*; set alone it moves new roots and leaves an existing library where it was |
| serving | `server.listen_addr`, `server.public_base_url` | they have to disagree — loopback for the proxy to reach, the public address for the links |
| search sidecar | `enabled`, `binary`, `index_dir` | `enabled` alone has nothing to run and nowhere to index |
| remote media | `default_action`, `allowed_domains`, `allow_private_networks` | an allow-list needs a default to be an exception to |

## The check is derived from the example, not restated beside it

`config-show-origin-file` reads the fence's own heredoc, collects the leaf keys
it sets, and requires each one back from `config show` with **origin `file`**
and the value the example wrote. So an example cannot claim a key it does not
set, and cannot quietly stop setting one.

`origin` is the column that carries the weight. A value alone could be the
compiled default agreeing by accident — `server.listen_addr: 127.0.0.1:8080` in
the proxy example *is* the default — and `file` is what says the file did it.
Proved by breaking it: dropping `--config` from one example fails with
`data.directory is "~/data" and the example set "/mnt/library/notrios"`.

**It also refuses an example that checks nothing.** `config show` summarises 20
of the 53 keys, so a fence setting only unreported keys would pass with zero
assertions — the shape of gate this repository keeps finding, one that stopped
running rather than started failing. Proved by replacing an example's YAML with
`search.default_limit`: `none of the 1 key(s) this example sets is reported by
config show, so it asserts nothing`.

## The fifth example, and the honest gap behind it

Search page limits get a bare `yaml` fragment and no command, because there is
no command that would check it: **`config show` reports 20 of the 53 keys** and
`search.default_limit`/`max_limit` are not among them, while `notriosctl search`
takes `--db` and not `--config`. The effect is visible through `/api/v1/status`.
The page says so. Showing `notriosctl config show --config bigger-pages.yaml`
there would have been an example that appears to verify something it does not,
which is the exact defect J13 exists to remove — and it is what the first draft
of this slice did before the output was actually read.

## What J13-B now knows it needs

Writing these by hand first was the point of doing J13-C before the generator,
and three requirements came out of it that an imagined design would have missed:

1. **The fixture must support the idiom, not the reverse.** These examples need
   `cat` and a heredoc, because that is how a person writes a config file. One
   coreutil joined the fixture's PATH rather than the published example being
   contorted into `printf` calls.
2. **The postcondition should be derived from the example body.** Restating each
   example's expectations in Go would be a second copy to drift; reading the
   fence's own YAML cannot.
3. **A verification command has to be checked against real output before it is
   published.** Two of the five candidate examples were unverifiable and one was
   silently so.

## Limits

- The fixture's `jq` is a stub that passes input through except for one filter,
  so these examples deliberately do not pipe through `jq`; a reader can.
- The heredoc reader handles one level of nesting and list members, and refuses
  anything deeper rather than guessing. A configuration example that needs more
  nesting than that has outgrown what a reader can take in.
- `allowed_domains` membership is not checked: `config show` does not report the
  list, so the example's third key is written and not asserted.
