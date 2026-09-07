# v0.8 H14 — Documentation actionability investigation (opencode, free models, zvec-grep)

Date: 2026-09-03 to 2026-09-04
Status: complete. The generation half shipped; the evaluation half is stopped on
this item's own exit criteria.
Model: Claude Opus 5 (`claude-opus-5`) via Claude Code, parent-owned throughout;
no subagents were used. The evaluated models are named in the record below.

## Why this is archived

The item ran to nine hundred lines in `PLAN.md` — a third of everything written
under items that were still open — and all of it describes work that is
finished. The plan keeps the specification and the outcome; the record of
executing it, including every finding and every correction, is kept here in
full and verbatim.

## Outcome

Complete, and the two halves ended differently.

*The generation half shipped and stands on its own*, as the exit criteria said
it would whatever the models did: `docs/features.md` with a coverage gate at
zero backlog, three catalogues, a computed surface comparison, and eight defects
found by writing them — two documented flags that did not exist, three product
gaps including a raw `FOREIGN KEY constraint failed` on a plausible user action,
a command missing from the CLI registry, and two containment defects in the
harness itself. `notriosctl notes create` exists because writing the catalogue
found the gap. None of that needed a model.

*The evaluation half is stopped, on its own criteria.* Zero of four tasks were
credited: the no-prose control arm kept succeeding, because the task statement
paraphrases the command it asks for. The tags case was flagged, so that
criterion held. The first criterion — a free model beating the recorded 7/16 on
the existing calibration — was never run, and this says so rather than leaving
it looking pending: the criteria are conjunctive, one had already failed, and
running a roster at 71 to 377 seconds a call to confirm a conclusion already
reached would have been spending hours to learn nothing.

*So the "if it succeeds" clause does not fire, and no follow-up evaluation item
is written.* Making the no-prose arm a real control needs task statements that
convey intent without the command's own vocabulary, and it is not obvious such
a statement exists for most tasks. That is a research problem rather than a
documentation one, and a faster or stronger model makes it worse rather than
better: a better guesser satisfies the control arm more often, so fewer pages
earn credit.

## Evidence

Under `performance/v0.8-h14/`: `ACTIONABILITY.json` (24 runs, three arms, two
repeats, one paid model with its authorisation recorded), the harness that
produced it, and the GUI journey capture. Nothing here is a build gate and
nothing was added to `make validate`, as the item required of itself.

## Follow-ups

- H18 rewrote `docs/features.md` into prose sections with a generated
  capability-by-surface table, and records what remains of its prose quality.
- H19 specifies `notriosctl search`, the gap the surface table made visible.

## The record, as it was written

Everything below is the item's own text as it stood in `PLAN.md`, kept verbatim.

## H14. Documentation actionability investigation (opencode, free models, zvec-grep) — complete

**Ordering.** Independent of the H5-H11 installer chain and can run at any
point. If it succeeds and the follow-up evaluation is approved, that evaluation
should land before H12, because documentation quality is part of what a release
claims.

**Goal.** Decide whether hosted free models driven by `opencode`, with local
semantic search from `zg` (zvec-grep), can answer a question the repository
currently cannot: **can a reader act on this page?** Concretely -- given only the
prose for one task, can a model produce a command line that actually runs and
does the thing? A page whose reader cannot take an action is a page that needs
work, however fresh, hashed and internally consistent it is.

**Why the existing machinery does not answer it.** Every documentation gate in
this repository checks *consistency*: docgen regenerates fragments, docaudit
anchors sections to source, G18a freezes an inventory, G18f hashes content and
counts enumerations, G18d executes registered examples. All of them can be green
while a page fails its reader, and that is not hypothetical -- H4 slice D found
`docs/installation.md` documenting a superseded asset search order, and slice E
found it again on another page, both with every gate passing. The gates check
that documentation is generated consistently and hashed, not that it is still
true or that anyone can use it.

**The known defect this must find, verified by hand.** Ask the documentation how
to create, edit or delete a tag on a note:

- `docs/cli.md` documents exactly one tag-mutating command, `notriosctl tags
  rename`. Tags otherwise appear only as filters (`--tags a,b`, `tag:todo`).
- `docs/gui.md` shows tags only as sidebar navigation with note counts and
  "click anything to search it".
- Yet the capability exists: REST has `POST` and `DELETE
  /api/v1/documents/{document_id}/tags/{tag}`, and MCP has `tag_note` and
  `untag_note` in the editor scope.

So a reader of either the CLI or the GUI guide cannot learn to tag a note, and
neither page points at the surface that can. This is a ready-made positive
control: a method that cannot find it is not worth adopting. It also raises a
separate product question recorded below -- whether the CLI is *meant* to have no
add/remove-tag command.

**Scope, in two parts.** *First*, write the documentation topics the repository
does not have: a features page answering "what can I do with this?", and from it
a catalogue of user journeys for the command line and then for the GUI, the GUI
ones illustrated with annotated screenshots. *Second*, stand up `opencode` and
`zg` locally; index the repository with a **local** embedding; reuse the
existing 8-case, 16-run contradiction calibration in
`performance/v0.7-g18f/ADVISORY_REPORT.json` to score candidate free models
against the recorded Qwen 2.5 Coder 1.5B baseline of 7/16; then build a small
task-to-command harness and measure whether generated command lines run in a
sandbox. The evaluation runs over the new pages as well as the existing ones,
which is the point of the ordering: a quality pass over documentation that does
not yet exist measures nothing.

*This reverses one of the item's own boundaries and that is deliberate.* H14
previously said "change no prose", because an investigation that rewrites what
it is measuring cannot report on it. That still holds for the evaluation: it
proposes, and a person decides. It does not hold for the generation, which is
now the first half of the item. The two are kept apart -- new prose is written
and reviewed before any model scores it, and no model output is committed as
documentation.

**The experiment must control for the model's own knowledge, and this is the
design point everything else depends on.** A capable model can produce a
plausible `notriosctl` invocation from familiarity with command-line conventions
alone, never having read the page. Scoring "did the command work?" would then
measure the model and report it as documentation quality. Every task therefore
needs three arms:

1. **prose-only** -- the page section and the task name;
2. **no-prose** -- the task name alone;
3. **misleading-prose** -- the section with one detail mutated, such as a renamed
   flag or an inverted default.

A page earns credit only when arm 1 succeeds *and* arm 2 fails. Arm 3 catches a
model that is ignoring the text it was given. Without arm 2 the whole exercise is
unfalsifiable.

**Why an executed command beats a label.** G18f's advisory asks a model to
choose among `supported`, `contradicted` and `not-determinable`, and the
recorded run scored 7/16 with both negation cases wrong -- the failure the
proposal names, where prose and its negation sit close together in the vector
space. An executed command sidesteps that entirely: nothing has to distinguish
"do X" from "do not do X" in an embedding, because the command either does X or
it does not. Scoring becomes deterministic, which is also what makes cheap models
usable -- they are being asked to draft, not to judge.

**Exit status is not the oracle; the observed state change is.** A command can
exit zero having done nothing, and -- the case that matters -- it can exit zero
having done the *opposite* of what the page described. Restoring a note and
purging it are both successful commands. So every task declares the state it
expects, and the sandbox database is inspected before and after, giving three
outcomes rather than two:

| Outcome | What it says about the page |
|---|---|
| the intended change happened | the prose is actionable |
| nothing happened, or the command failed | the prose is unclear or incomplete |
| the opposite or another destructive change happened | the prose actively misleads |

The third is the most valuable result and the one a pass/fail oracle would
record as a plain failure, indistinguishable from a typo. It is also where the
negation weakness resurfaces on the *documentation* side rather than the model
side: a page that reads as "notes in Trash are removed after 30 days" and a page
that reads as "notes in Trash are removed immediately" produce different commands
with different observable effects, and only the state check tells them apart.
`internal/docexec` already models this -- its registered examples carry a
`postcondition` describing what must be true afterwards, not merely an expected
exit status -- so the shape exists and needs reusing rather than inventing.

**Reuse the sandbox rather than build a second one.** `internal/docexec` already
runs 63 of 137 registered examples against a seeded loopback fixture with
substitutions for the base URL, binaries, seeded ids and scratch directories. The
difference here is only the source of the command: docexec runs commands
*transcribed from* the docs, this runs commands *synthesised from* the prose. The
gap between those two is exactly the thing being measured, so the fixture,
adapters and substitution machinery should be shared.

**The features page should be half generated, and the generated half is what
makes it trustworthy.** A features list is usually written by hand and quietly
goes stale. This repository already carries four anchored, counted surface
registries -- 59 CLI usage forms behind `printHelp`, 63 configuration keys, 109
REST operations and 46 MCP tools -- each with an owner anchor and a pinned count
that fails when it moves. So the inventory of *what exists* can be generated the
way every other fragment is, and only the description of *what it is for* is
written by a person.

That split buys a check nothing currently performs: **every anchored surface
must be claimed by at least one feature entry, and an unclaimed surface fails.**
A capability that no feature names is by definition a capability no reader can
discover, which is precisely the tags defect recorded above -- REST and MCP can
tag a note, and neither the CLI guide nor the GUI guide says so. Today that was
found by hand. With a coverage gate it would have been found by the build. This
is the most valuable thing in the item and it does not need a model at all.

*Flags are not features, and the mapping is the editorial work.* Reading
`cmd/notriosctl` yields switches, not answers to "what can I do with this?".
`--materialize N` is a flag; "sync two of your own libraries through a folder
you both can reach" is a feature. The generator produces the surface inventory
and the coverage obligation; a person writes the capability prose against it.
Any attempt to generate the prose from flag names would produce a second copy of
the reference documentation and call it a features page.

**Journeys: the command line first, and it is the specification.** The
instruction to start with the command line is right for a reason worth
recording: a CLI journey can be *executed*, deterministically, against a seeded
library, and `internal/docexec` already does exactly that for 63 registered
examples with postconditions describing what must be true afterwards. A GUI
journey needs a browser and is slower, flakier and harder to assert. So the CLI
journey is written and executed first, and it becomes the statement of what the
task *is*; the GUI journey is then checked against it rather than invented
beside it.

The starting catalogue, which is a floor rather than a ceiling: create, update
and delete a note in a named notebook; search, demonstrating every query-language
feature with a worked example; notebooks defined by a query; import from Joplin;
import from Obsidian; export the library; create a profile such as
`personal_notes` or `work_notes`; synchronize with a replica on another drive,
including a cloud folder mapped locally; back up and restore the library; and
how Recoll is used. Tagging a note is deliberately on the list too, because at
the time of writing the command line cannot do it -- the journey is the thing
that makes that visible instead of arguable.

**Comparing the two catalogues is a defect finder, not a formatting exercise.**
Each GUI journey is compared to its command-line counterpart, and the comparison
has three possible outcomes, all of which are findings: the GUI can do something
the CLI cannot, the CLI can do something the GUI cannot, or the two do the same
thing by different names. The tags case is already a worked example of the
second, and it generalises -- this comparison is the systematic version of the
hand-found positive control. Every difference is recorded as either a documented
deliberate asymmetry or a product gap, and the investigation does not decide
which; it presents them.

**Screenshots: reuse the runner, and derive the marker from the click.** The
machinery is largely present. `performance/v0.7-g18e/browser_journeys.mjs`
already launches headless Chromium through Playwright, drives journeys with
`getByRole` and `locator` and **already takes screenshots** -- it simply writes
them to `/tmp` and records the paths, one per viewport, at the end of a run.
What is missing is per-step capture, annotation, and a place for them to live.

The annotation should be drawn from the locator that is about to be clicked,
using its bounding box, rather than placed at coordinates written down by hand.
A hand-placed circle is a second description of the interface that drifts the
moment a button moves; a circle derived from the element is correct by
construction, and if the locator stops matching, the journey fails rather than
producing a confident picture of the wrong place. Draw it by injecting an
overlay into the page before the capture rather than compositing afterwards:
that renders at the page's own device pixel ratio, needs no second imaging
toolchain, and keeps the marker in the same coordinate space as the element.
Each step then carries the screenshot and a sentence saying what the user is
doing and why -- the descriptive text is the documentation, and the picture
supports it.

**Boundaries.** No prose is rewritten automatically, no model output is executed
outside the existing sandbox, no probabilistic result becomes a build gate, and
nothing is added to `make validate`. No screenshot is ever taken of a real
library: every capture comes from the seeded fixture, because a screenshot of a
GUI is a picture of somebody's notes and this repository does not carry those. No subscription and no recurring charge.

*The zero-cost boundary was relaxed on 2026-09-03, deliberately and with
numbers.* Free endpoints proved too slow and too throttled to run a matrix: 71
to 377 seconds a call where they answered at all, and two of the four tried were
rate-limited on contact. The user authorised `openrouter/z-ai/glm-5.3-flash` at
$0.075 in and $0.25 out per million, which answers in 18 seconds. Cost is not
the constraint at any plausible multiplier. The whole documentation corpus is
293,267 characters -- roughly 55,000 to 73,000 tokens -- but the harness sends
one task's prose per call, so a full pass over every journey and arm is about
3,250 tokens of prompt, and a run with repeats stays under a cent even allowing
an order of magnitude for the agent's own system prompt and tool round-trips.
The harness still refuses a slug without `free` unless `--allow-paid` is given
**with a written reason**, and that reason is recorded in the evidence, so a
paid run cannot happen by accident or without saying why. `zg` uses a local embedding model and its remote-data path stays off.
Notes, databases, evidence archives and anything under `data/` are never sent
anywhere. No change to docgen, docaudit, or any G18 gate.

**Dependencies.** None in this milestone. It reads the frozen G18a inventory,
the docaudit registry and the G18f calibration, all of which are already
committed.

**Working state.** A features page whose surface inventory is generated and
whose capability prose is written, with every anchored surface claimed; a
command-line journey catalogue whose steps execute against a seeded library; a
GUI journey catalogue with annotated per-step screenshots; a recorded comparison
of the two catalogues naming every asymmetry; and then a recorded model run over
a page sample, with per-model calibration scores, per-task three-arm results,
the exact prompts and their hashes, and a written recommendation on whether to
proceed -- including "no" as an acceptable outcome.

**Validation and evidence.** For the generation half: the surface-coverage
check, failing on any anchored surface no feature claims; every command-line
journey executed with its postcondition asserted; every GUI journey run in the
browser with its screenshots produced from locators that still match; and the
catalogue comparison, with each asymmetry classified as deliberate or a gap. For
the evaluation half: calibration scores for each candidate model on the same 16
runs the Qwen baseline used, so the comparison is like-for-like; the three-arm
results per task; every generated command with its exit status and what it did;
prompt and source hashes for reproducibility; and the tags case as a positive
control that the method must flag. Evidence under `performance/v0.8-h14/`,
validated the way other evidence directories are.

**Open decisions**

**Slice A complete 2026-09-03: the coverage check, and it found the control by
itself.** `internal/docfeatures` loads `docs/docfeatures/FEATURES.json` and
compares it against the four surface inventories, which it gets from a new
`docgen.Surfaces` rather than reading the sources again -- the same extractors
that already produce the pinned counts of 59 CLI usage forms, 109 REST
operations, 46 MCP tools and 37 GUI journeys. A second extraction would be a
second opinion about what exists, and the value of a coverage check is that
there is only one.

*It reproduces the tags defect mechanically.* Run against the repository with a
registry containing one feature, it reports `asymmetry tag-a-note has=[rest mcp]
missing=[cli gui]`. That gap was found by reading pages by hand; it is now found
by the build, with no model involved. `TestTheTagsAsymmetryIsStillReported`
fails if it stops being reported, and says which of the two possible reasons the
reader should check.

*It checks both directions, and only one of them is ratcheted.* An unclaimed
surface is a capability no reader can discover; there are 210 of them today, so
that check is a **ratchet** -- the backlog may exist and may not grow. A gate
demanding zero on the day it was written would have been red immediately and
switched off within a week. A phantom claim, a feature naming a surface that no
longer exists, is absolute: there is no backlog of those, and a page describing
something that was removed is a defect from the moment it happens. It is also
exactly what a consistency check cannot see, because such prose is perfectly
consistent with itself.

*Both were confirmed by mutation.* Dropping a claim pushes REST from 107 to 109
unclaimed and fails the ratchet; adding a claim on a nonexistent tool is
reported by name. The ratchet also fails when a count goes **down** without the
baseline being lowered, so an improvement is locked in rather than left as slack
for the next regression to spend.

*What slice A does not do.* The registry holds one feature. Writing the other
capability entries -- the editorial half, which is the part no generator can do
-- is the next slice, and each one lowers a baseline.

**Slice B, first half, complete 2026-09-03: the registry is written and the
ratchet is now a gate.** All 29 features are recorded and every one of the 214
surfaces is claimed -- 60 CLI usage forms, 109 REST operations, 46 MCP tools.
The baselines are zero, which turns the ratchet into an absolute gate: a new
command, operation or tool added without a feature entry now fails, rather than
being tolerated as backlog. Clearing it in one slice was possible because the
grouping is editorial rather than laborious; twenty-nine capabilities cover a
surface area that reads as much larger when listed as flags.

*It found a defect I had introduced myself, three commits earlier.* `notriosctl
sync migrate-credentials`, added in H9 slice D, went into `printSyncUsage` and
`docs/cli.md` but never into `printHelp` -- which is the anchored, counted
registry. `notriosctl --help` did not mention it, the count stayed at 59, and no
gate fired, because nothing was inconsistent: the command simply was not
claimed anywhere the machinery looks. That is precisely the class of defect this
item exists to find, and it found one on its author. Registering it moved the
pinned count 59 to 60.

*And it caught three endpoints that do not exist.* Writing the registry, I
claimed `GET /api/v1/resources`, `GET /api/v1/selection/plan` and `GET
/api/v1/documents` -- all plausible, none real. The phantom check named all
three by feature. This is the direction a consistency gate cannot see, and the
first time it ran against real prose it caught the author inventing API.

*The asymmetry report is now a document in its own right.* Fourteen features
offer a capability on some surfaces and not others, each carrying a
`surface_note` saying whether that is deliberate. Most are: importing reads
directories on this machine, so it is command line only; profiles are about this
machine, so a service answering for one must not reach another; credential
migration is command line only because this milestone forbids a credential REST
surface. The one with no note is `tag-a-note`, which is the gap rather than a
decision, and the test that guards it says so.

**Slice B complete 2026-09-03: `docs/features.md`, and what one page costs.**
The page is written and its capability list is generated from the registry, so
the surfaces a reader is told about are the checked ones. It says plainly that
the four surfaces are not equivalent and why -- the command line owns anything
touching this machine, the GUI owns writing, REST and MCP are for other programs
-- and it names the one asymmetry that is a gap rather than a design.

*The page moved eleven pinned counts, and that is the finding.* G18a's document
count and grade denominator, the docs-site staging count and its test, G18b's
prototype builder, docaudit's manual sections, fragments, generated and
unverified counts and its denominator, G18f's slot count, its user/api split and
its unique-fragment count, G18g's repository-file count and its idempotency
assertion, and helpdocs' seed counts -- because `helpdocs` seeds every Markdown
file under `docs/`, so a new page becomes a new Help note automatically. Nothing
here was wrong; every one of those is a gate noticing a real change. But it is
worth recording what a documentation page costs in this repository before the
journey catalogues add several more.

*Two frozen v0.7 records had to be relaxed, and both relaxations are narrower
than they look.* G18c pinned `len(topics) == len(documents) == 15`; a frozen
v0.7 record cannot be edited to claim it always knew about a page written in
v0.8, so the document count became a floor -- the same shape the executables
assertion beside it already used. G18f asserted that its recorded advisory
reviews exactly matched the user fragments; those reviews are a record of an
actual Qwen run, and writing an entry for a new fragment would mean **inventing
model output that never existed**. So it became a subset check with the
direction that matters kept -- a recorded review naming a fragment that no
longer exists still fails -- plus an explicit assertion that `feature-surface`
is the one unreviewed fragment. It is recorded as unreviewed rather than assumed
to have passed, which is the same shape H8 uses for a row it cannot execute.

**Slice C complete 2026-09-03: eight executed command-line journeys, and five
defects found by running them.** `docs/docjourneys/CLI_JOURNEYS.json` holds each
journey's steps and the postcondition that confirms it; a test runs all eight
against disposable libraries and checks the state afterwards, not the exit
status. `docs/journeys-cli.md` is generated from the same catalogue, so the page
and the execution cannot drift. Every journey names a feature, and the count of
features with no journey is its own ratchet -- 24 today, allowed to shrink and
not to grow. That is a different gap from an unclaimed surface: one is a
capability nobody can discover, the other is one someone can find but has not
been shown how to use.

*Two flags I documented do not exist.* `paths --db` and `import obsidian
--notebook` were both written down from memory of neighbouring commands rather
than from the usage message, and both exit 2. That is precisely the failure mode
the executed-journey design exists to catch, arriving on the first run, in prose
written by someone who had just read the whole CLI surface to build the features
registry.

*And three product gaps, one of which is a defect rather than a design.* There
is no `notriosctl` command that writes a note, so a command-line note arrives by
import -- the test suite already worked around this, with a comment saying so.
The Obsidian importer cannot choose a notebook. And **`--collection` will not
create a collection**: naming one that does not exist fails with `FOREIGN KEY
constraint failed`, which is SQLite talking, not Notrios. A plausible user
action producing an internal error message is a defect, and it is recorded here
rather than smoothed over in the prose.

*A schema change the catalogue forced.* Requiring every step to be a
`notriosctl` command made "write a Markdown file" into a placeholder, which is
dishonest about what a task involves. Steps can now be marked manual: the reader
does them, the runner does not, and they appear in the page. A catalogue that
could only describe steps it can run would leave out the parts a reader is most
likely to get stuck on.

**`notriosctl notes create` added 2026-09-03, closing the gap slice C could not
document around.** The case for it was made by evidence rather than argument:
this repository's own sync tests already created notes by writing a Markdown
file and importing it as a one-file Obsidian vault, with a comment saying the
CLI could not do it; the features registry recorded `write-notes` as having no
command-line surface; and the journey catalogue hit the same wall when it tried
to write the task down. `runNoteMove` had already been added for exactly this
shape of gap -- reachable from the store, REST and MCP and from neither surface
a person uses -- so there was precedent as well.

The body comes from an argument, a file, or standard input, and standard input
is the one that matters: it makes a note the end of a pipeline rather than
something staged on disk first. A notebook is resolved before the write and
refused rather than guessed when the name is ambiguous, for the reason `notes
move` gives: notebook names are unique only among siblings. The refusal names
what the user asked for -- which is the contrast slice C found, where `import
--collection` fails the same case with a raw `FOREIGN KEY constraint failed`.

The journey is now a real one: create a note, file it by notebook, see the
refusal, and confirm both notes in an export. `write-notes` gains a command-line
surface and the usage-form count moves 60 to 61.

- **How the click marker is positioned -- Resolved before implementation:
  derived from the element, never written down.** Each step captures the
  bounding box of the locator it is about to click and draws the marker there,
  as an overlay injected into the page before the screenshot rather than
  composited afterwards. Three consequences follow and all three are the reason.
  A moved button moves the circle, so the picture cannot drift from the
  interface while still looking authoritative. A locator that stops matching
  fails the journey instead of producing a confident image of the wrong place --
  the failure is loud rather than silent, which is the property a screenshot in
  documentation otherwise lacks entirely. And drawing in-page keeps the marker
  in the element's own coordinate space at the page's device pixel ratio, so no
  second imaging toolchain is involved and no scaling arithmetic can be wrong.
  Hand-placed coordinates are refused outright: they are a second description of
  the interface, and a second description is a thing that disagrees with the
  first.
**Slice D complete 2026-09-03: three GUI journeys, five annotated screenshots,
and the failure the design was written to prevent -- caught, at last, by an
accident.** `performance/v0.8-h14/gui_journeys.mjs` drives the real interface,
resolves each step's locator, draws the marker from that element's bounding box,
photographs it, then acts. `docs/journeys-gui.md` is generated from the same
catalogue, so the sentence beside a picture is the string the runner used when
it took it.

*The marker function silently did nothing, and nothing noticed.* It was passed
to `page.evaluate` as a **string**, which evaluates the expression, constructs
the arrow function, ignores the argument and never calls it. Five screenshots
came out with no marker on them, no error anywhere, and every gate green -- a
confident picture of nothing, which is precisely the failure the derived-marker
decision was recorded to prevent. What caught it was two steps pointing at
different elements producing byte-identical files. That comparison is now a
check rather than a coincidence: **two steps with different locators may not
produce identical images**, because if the marker stops being drawn every step
in the same app state photographs the same way.

*The first working marker was also wrong, and visibly so.* Sized to the element,
it drew a 244-pixel ring over a full-width sidebar row: it swallowed five rows
and pointed at nothing. The fix separates two claims that had been conflated --
a thin outline for the element, which is how much of the screen is clickable,
and a fixed 40-pixel circle at its centre, which is where the click actually
goes. Both derived from the same box. It took looking at the picture to see
this, which is worth recording: the hash check proved the marker existed, and
only a person could tell it was useless.

*The gate runs without a browser.* Capture is opt-in behind
`NOTRIOS_GUI_JOURNEYS=1`, but a missing screenshot, a stale one, or a step that
starts pointing at a different element all fail an ordinary `go test` run, by
comparing committed images against the hashes recorded when they were taken.
Both halves were confirmed by mutation. Five images total 708 KB, well inside
the bound the decision below asked for.

*One dependency finding.* The Playwright browser journeys do not run from a
clean checkout: `playwright` is not a dependency of this repository, and the
runner resolves it from a sibling project through `PLAYWRIGHT_MODULE`. The
browsers are in the shared cache, so this machine works and a fresh one would
not. Recorded rather than fixed, because pinning a browser automation stack is a
dependency decision rather than a documentation one.

**Slice E complete 2026-09-03: the comparison, computed, and it finds the
control on its own.** `internal/doccompare` puts the two surfaces side by side
from three things that already exist -- which surfaces each capability claims,
which have a command-line journey, which have an interface journey -- and
reports 28 differences over 29 features. It is computed rather than written,
because a hand-maintained list of differences is a list that stops being true.

*Two kinds of disagreement, and they are not the same problem.* A **capability**
difference is something one surface can do and the other cannot: 15 command-line
only, 2 interface only, 7 reachable from neither. A **documentation** difference
is a capability both surfaces offer where only one journey is written: 4 of
those. The first is about the product, the second about these pages, and
conflating them would let a missing journey look like a design decision. The
comparison deliberately drops a feature's `surface_note` when the difference is
documentary, for exactly that reason.

*Exactly one difference is unexplained, and it is the one the item named at the
start.* Tagging a note is reachable over REST and MCP and from neither the
command line nor the interface, and no `surface_note` accounts for it. Every
other capability difference carries its reason -- importing reads directories on
this machine, profiles are about this machine, credential migration is forbidden
a REST surface by H9. That gap was found in this item's opening paragraph by
reading pages by hand; it is now produced by comparing two catalogues, and the
generated page prints it in bold as a gap rather than a decision.

*It is a ratchet, and it bites.* Removing the explanation from `profiles` makes
the unexplained list `[profiles tag-a-note]` and fails. So a capability that
stops reaching a surface must either gain a reason or be recorded as a gap; it
cannot arrive quietly.

- **Where the screenshots live -- Resolved 2026-09-03: `docs/images/journeys/`,
  committed.** They are generated artifacts, which this repository does not
  normally commit, and they churn on every interface change. The earlier
  recommendation here was not to commit them at all. That is revised, because
  checking the alternatives showed the objection was weaker than it looked and
  the cost of not committing was higher.

  **Not `data/assets`.** That is a user's asset store: `data/` is gitignored,
  the `data` root is `backup_and_verify`, and a purge deletes it. Documentation
  images living there would be backed up as though they were somebody's notes,
  destroyed by a purge, and in an installed profile would be written into the
  user's real library. The whole point of H3's six roots is that product content
  and user content are not the same thing.

  **Not `assets/` either, though it is the right *kind* of place.** It is
  committed, already holds `assets/icons/*/notrios.png`, and is staged into
  `program_assets` by the packaging -- but `build_deb.sh` stages `assets/icons`
  specifically rather than the directory wholesale, so putting journey images
  there would be relying on that narrowness to avoid shipping them.

  So `docs/images/journeys/`: committed, beside the pages that reference them,
  outside the path the packaging stages. A screenshot that is not committed does
  not appear when someone reads `docs/gui.md` on a git host, which is where most
  readers are, and that was the cost the earlier recommendation accepted too
  readily. The manifest survives the change of home and still earns its place:
  step id, locator and image hash are committed alongside, so a stale screenshot
  fails the gate rather than quietly misleading. Bound it deliberately -- one
  fixed viewport width, a cap on count and dimensions -- rather than discovering
  the repository size afterwards.
- **How much the new pages move the pinned counts -- Non-blocking but noisy.**
  One section and one example moved five pinned counts in H9 slice D. A features
  page plus two journey catalogues is a large multiple of that, across G18a's
  inventory and denominator, the docaudit surface, G18d's registry and the G18f
  hashes. Recommended: land the generation in slices, one catalogue at a time,
  and re-pin each time rather than once at the end, so a count that moves for the
  wrong reason is still findable.
- **Whether the journey catalogues are new pages or new sections -- Non-blocking,
  decide before writing.** `docs/cli.md` and `docs/gui.md` are already long, and
  every section in them is anchored to an owner. Recommended: separate pages,
  `docs/features.md` and two journey pages, because a journey catalogue has a
  different shape from a command reference and mixing them makes both worse --
  and because a separate page can be regenerated without re-hashing a reference
  page nobody changed.
- **Whether an executed GUI journey is required, or only an executed CLI one --
  Blocking for the gate design.** The browser journeys are opt-in today,
  behind `NOTRIOS_G18E_BROWSER=1`, because they need Playwright and a real
  browser. Making illustrated GUI journeys a committed claim while their
  execution stays optional would mean shipping pictures nothing verifies.
  Recommended: keep browser execution opt-in for `make validate`, but require it
  for the evidence bundle, so the claim is only made when it has been run --
  the same shape H8 uses for rows it cannot execute everywhere.

- **Sending repository documentation to a hosted model -- Resolved 2026-09-03:
  permitted for `docs/` prose and generated command text.** The user authorised
  it on the terms recommended below. Everything else stays loopback, and the
  endpoint is recorded per run. The exposure was always small -- the
  documentation is Apache-2.0 and written to be published -- but it is a change
  to G18f's recorded `endpoint_scope: loopback-only` and is recorded as one.
- **(superseded) Sending repository documentation to a hosted model.** G18f's
  recorded policy is `endpoint_scope: loopback-only`,
  `source_scope: repository-source-only; no notes or databases`, `cost_usd: 0`.
  Using hosted free models changes the first of those. The documentation is
  Apache-2.0 and written to be published, so the exposure is small, but it is a
  policy change and must be recorded as one rather than assumed. Recommended:
  permit hosted calls for `docs/` prose and generated command text only, keep
  everything else loopback, and record the endpoint used per run.
- **Which free models -- Non-blocking, and not decidable from a list.** The
  supplied recommendations describe capabilities that appear to be inferred from
  the model names rather than measured -- a `-fin` suffix read as "financial", a
  `-reasoning` suffix read as "a dedicated internal reasoning token track", a
  vendor read as "purpose-built for software engineering". Several named models
  cannot be verified from here at all. This is exactly what the calibration set
  is for: run the candidates on the same 16 runs and let the score decide.

  **Trial roster, in this order.** Order is a guess at capacity and nothing
  more; the calibration score replaces it as soon as there is one.

  1. `openrouter/z-ai/glm-5.2:free`
  2. `openrouter/nvidia/nemotron-3-ultra-550b-a55b:free`
  3. `openrouter/google/gemma-4-31b-it:free`
  4. `openrouter/nvidia/nemotron-3-super-120b-a12b:free`
  5. `openrouter/google/gemma-4-26b-a4b-it:free`
  6. `openrouter/minimax/minimax-m3:free`
  7. `openrouter/cohere/north-mini-code:free`
  8. `openrouter/thinkingmachines/inkling:free`

  Every one of them runs the same 16 calibration runs and the same three-arm
  tasks, so a claim about any of them is answered by a number. Exclude
  `openrouter/nvidia/nemotron-3.5-content-safety:free`, whose name at least is
  unambiguous. A model that cannot beat 7/16 is dropped rather than tuned.

- **Every model gets the same, deliberately small context -- Blocking for the
  harness design.** One task, one page section, no repository access, and no
  `zg` retrieval into the answering prompt. `zg` selects which sections to test
  and helps a human read the results; it does not enrich the prompt under test.

  This is a property of the measurement, not a limitation being worked around.
  The question is whether **the prose alone** is sufficient to act, so a model
  that has also read `cmd/notriosctl` will produce a correct command whether the
  page is any good or not -- scoring the codebase while appearing to score the
  documentation. That is the "measures the model, not the docs" failure the
  no-prose arm exists to catch, arriving through the context window instead of
  through the model's memory.

  It follows that context capacity is irrelevant here, and a model recommended on
  the strength of it earns no credit for that. Feeding a repository to a large
  context to cross-reference code against prose is a sound technique for a
  different question -- "does the documentation match the code?" -- which the
  existing anchored inventory and generated fragments already answer
  deterministically and for free.
- **Rate limits and spreading work -- Non-blocking.** Free tiers throttle. The
  harness must be resumable, cache by prompt hash the way the existing advisory
  report already records `explanation_prompt_sha256`, and record which model
  answered which task so a mixed run stays attributable. The supplied material
  predicts that the GLM free endpoint in particular returns 429 under sustained
  sequential use while the Gemma endpoints are steadier. That is a testable
  claim, so record observed throttling per model as a result rather than
  designing around it in advance: a model that cannot complete a run is unusable
  here however well it scores on the runs it does complete.

- **Whether GLM-5.2 is actually free at the tier used -- Resolved 2026-09-03,
  and the rule generalises.** A model is free when its slug contains `free`;
  the paid GLM-5.2 is a different slug without it. `opencode models` lists
  every model, so the free set is a filter rather than a judgement, and the
  harness refuses any `--model` whose slug does not contain `free` rather than
  trusting the caller. The account does not pay for overage automatically, so an
  accidental paid call fails rather than bills -- which makes this a checkable
  property rather than a promise. `cost_usd: 0` stays a recorded property of the
  evidence.
- **(superseded) Whether GLM-5.2 is actually free at the tier used.** The supplied material contradicts itself, tabulating GLM-5.2
  as "Paid (~$0.49/M input)" in one comparison and describing a working
  `:free` endpoint in another. The constraint on this whole item is zero cost,
  so the endpoint must be confirmed free at the point of use, and the run
  aborted if any call would be billed. `cost_usd: 0` stays a recorded property
  of the evidence, as it is in G18f.
- **Whether the CLI is meant to have no add/remove-tag command -- Answered
  2026-09-03 by adding one.** `notriosctl tags add`, `tags remove` and `tags
  list` close the command-line half of the gap this item opened with. `tags
  list` exists because the other two would otherwise be unverifiable from the
  surface that performs them: a command that changes something and offers no way
  to see the change asks its caller to take it on trust.

  Two refusals were written to the standard this milestone criticised elsewhere.
  The store answers a missing note and a missing tag with the same bare "not
  found", so the commands name what the user asked for instead -- the same
  failing that made `import --collection` report a raw `FOREIGN KEY constraint
  failed`. And `ListDocumentTags` returns an empty list for a note that does not
  exist, so `tags list --document` checks the note first: "this note has no
  tags" and "there is no such note" are different answers, and a command whose
  job is verifying an edit must not conflate them.

  The gates moved as they should. `doccompare`'s unexplained-gap list is now
  **empty** -- every capability one surface has and another lacks carries a
  written reason -- and the ratchet demanded that be locked in rather than left
  as slack. The features registry records the GUI half as still open, so it is
  an explained asymmetry rather than an unaccounted one. The slice A guard was
  narrowed rather than deleted, exactly as its own comment instructed a future
  reader to do. And the tagging journey, which could not be written before, now
  exists and executes.

  **The GUI half was still open when this was written, and was closed in H15**:
  the editor toolbar has a tag control beside the notebook control, and
  `tag-a-note` is a captured interface journey. The sentence that stood here
  said tags were sidebar navigation with no way to add or remove one, and it
  stayed after that stopped being true -- in the item whose subject is prose
  that no gate can catch, which is the joke and also the point.
- **(superseded) Whether the CLI is meant to have no add/remove-tag command.** REST and MCP can tag a note and
  the CLI cannot. If that is deliberate the CLI guide should say so and point at
  the surfaces that can; if it is an oversight it is a product gap rather than a
  documentation one. The investigation records the question; it does not answer
  it.

**Slice F, pilot complete 2026-09-03: the harness works, and the first complete
task failed the ablation.** `performance/v0.8-h14/actionability.py` builds the
three arms from the journey catalogue -- which already holds a task, its prose
and its postcondition -- asks a free model for one command, and runs it against
a disposable library. It refuses any `--model` whose slug does not contain
`free`, refuses to execute an argument vector containing anything a shell would
interpret, and replaces whatever paths the model names with the sandbox's own,
so the model does not choose which library it touches.

*The result that matters.* On `find-your-library`: the prose arm produced
`notriosctl paths` and the postcondition held; the **no-prose arm produced the
same command and the same result**. Under this item's own rule -- a page earns
credit only when the prose arm succeeds and the no-prose arm fails -- the task
scores **zero**, and correctly so: the model did not need the documentation, it
guessed a conventional command name. A two-arm version of this method would have
reported that page as actionable on the strength of a model that never read it.
That is the failure the ablation exists to catch and it caught it on the first
complete task, which is the most useful thing the pilot could have done.

*One run per arm is not enough, demonstrated by accident.* A manual no-prose
call to the same model on the same task returned `notriosctl config`, which does
not exist; the harness's returned `notriosctl paths`, which does. Same model,
same arm, opposite outcomes. The existing G18f calibration used two repeats per
case for this reason, and any real run of this method needs repeats before a
single result is read as a fact.

*Findings about the instrument, which is the other half of what a pilot is
for.* `opencode run` is an agentic CLI rather than a completion endpoint. Its
permission configuration asks before touching an external directory, every
sandbox is external, and a non-interactive run cannot answer -- so the call
**hangs until the timeout instead of failing**, which cost three arms before it
was diagnosed. `--pure --auto` is therefore required rather than preferred, and
the isolation this item depends on ("no repository access") rests on the sandbox
being empty rather than on the tool refusing. `opencode` also exits **zero** on
an upstream rate limit, so the harness reads the text rather than the status.

*Free endpoints throttle, as predicted, and it is a result rather than an
obstacle.* `openrouter/z-ai/glm-5.2:free` and
`openrouter/google/gemma-4-31b-it:free` were rate-limited upstream on first
contact; `opencode/nemotron-3-ultra-free` and
`openrouter/nvidia/nemotron-3-super-120b-a12b:free` answered. Latency ran 71 to
377 seconds per call, so the full roster over eight journeys and three arms is
several hours of wall clock -- which is why this is a pilot of one task and says
so.

*`zg` was indexed locally and used for selection, not for answering.* `zg index
--embedding local/potion-retrieval-32m` over a copy of `docs/` alone; no notes,
no database, no remote embedding. Querying it for "how do I add a tag to a note"
returns the features page, the registry and "Renaming a tag hierarchy" -- and
nothing that answers the question, because nothing does. The retrieval
corroborates the tags gap from a reader's angle, independently of the comparison
in slice E.

*Against the exit criteria, the honest reading is: not yet.* The criteria
require the ablation to separate arm 1 from arm 2 on a page known to be good,
and on the one task run it did not separate at all. That is a result about the
task rather than about the method -- `notriosctl paths` is guessable and a task
whose command is guessable cannot measure a page -- but the criteria are not met
and no amount of further running changes that for this task. Recommended before
any full evaluation: choose tasks whose commands are *not* conventional, repeat
each arm at least twice, and treat a task where no-prose succeeds as evidence
about the task rather than the documentation. If those do not produce
separation, the recommendation is to stop, and the investigation will still have
been worth doing -- it has already produced a features coverage gate, three
executed catalogues and five product defects without a model being involved at
all.

**Slice F, full run 2026-09-03: repeats, deliberately unguessable tasks, a
faster model -- and still nothing separates. That is the answer.** 24 runs: four
journeys marked `notrios-specific`, three arms, two repeats each, on
`openrouter/z-ai/glm-5.3-flash` at 18 to 30 seconds a call against the free
models' 71 to 377. **Zero of four tasks credited.**

| task | prose | no-prose |
|---|---|---|
| search with the query language | acted, acted | refused, **acted** |
| export and verify an archive | no-change, no-change | no-change, no-change |
| read the built-in help | acted, unavailable | **acted, acted** |
| move sync keys to the keychain | no-change, no-change | no-change, no-change |

*The no-prose arm keeps winning, and marking tasks "unguessable" did not stop
it.* The model produced `seed-help` twice with no documentation at all, and
`export archive --query` once. Guessability was recorded in the catalogue
precisely to remove this, and it did not, which points at something the pilot
did not show: **the task statement itself paraphrases the command.** Every arm
receives the journey's title and goal, and "Get the Notrios guides into the
library as notes" is very nearly a definition of `seed-help`. The no-prose arm
is therefore not a clean control -- it is the prose arm with the steps removed
but the answer still in the framing. That is the method's real limit, and it was
invisible until tasks chosen to defeat guessing failed to defeat it.

*Two tasks failed on the prose arm for reasons that are mine, not the pages'.*
`export archive-v2` produced the right command with the model's own output path,
and the postcondition checks a path the harness chose, so it recorded
`no-change` for a command that worked. A postcondition that depends on a path
the model picks cannot be written this way. The keychain task saw the model emit
a literal `{config}` placeholder it invented, which the harness stripped, leaving
a command that could not do the thing. Both are harness defects surfaced by
running it, and both are recorded rather than tuned away.

*A containment gap the run found in the harness itself.* One run produced
`export archive-v2 /backups/notrios-2026-08-04` -- a **positional path outside
the sandbox**, which the harness passed straight through. It failed only because
`/backups` does not exist; a model naming `/tmp` or a path under the user's home
would have been written to. Root flags were being stripped and positional paths
were not. Any argument that looks like a path is now redirected under the
sandbox, keeping its base name. Separately confirmed: `notriosctl` is not on
`PATH`, so the agent could not have read the usage text, and the no-prose
successes are genuine guesses rather than tool-assisted discovery -- which
matters, because that would have invalidated every result here.

*Recommendation: stop, and keep what the item already produced.* The exit
criteria require the ablation to separate arm 1 from arm 2 on a page known to be
good. With repeats, with tasks chosen to be unguessable, and with a model fast
enough to run a matrix, it separated on nothing. Making the no-prose arm a real
control needs task statements that convey a user's intent without paraphrasing
the command, and it is not obvious that such a statement exists for most tasks --
a task named without its own vocabulary may not be a task a reader would
recognise either. That is a research problem, not a documentation one.

The investigation was still worth doing, and not as consolation. Its
model-free half produced a features page with a coverage gate at zero backlog,
three catalogues -- nine executed command-line journeys, three photographed
interface journeys -- a computed surface comparison, and eight defects: two
documented flags that do not exist, three product gaps including a raw
`FOREIGN KEY constraint failed` on a plausible user action, a command missing
from the CLI registry, and two containment defects in the harness itself. The
`notes create` command exists because writing this found the gap. Not one of
those needed a model.

**Corrections 2026-09-03, from reading the pages as a reader rather than as
their author.** Five faults, and the last is the one that explains the others.

*The page contradicted the product.* `docs/journeys-cli.md` still said there is
no command that writes a note, three commits after `notes create` was added. The
page that this item built to catch stale documentation had gone stale, and no
gate noticed, because nothing was inconsistent -- the sentence was merely untrue.

*It mentioned tags and documented nothing about them.* The tagging journey was a
placeholder with the actual tagging as a manual step. It is now a real journey:
add two tags, take one off, and read the note back, with a postcondition that
asserts the removed tag is **absent** rather than only that the kept one is
present.

*The generated list showed titles and no commands.* A reader learned which tasks
existed and was left no better able to do any of them. The fragment now emits
every step with its command, which is what makes the page documentation rather
than a table of contents.

*The commands it would have shown were unusable anyway.* They carried the
sandbox plumbing every journey needs to run in isolation -- `--db {db}
--asset-store {assets}` -- which no reader ever types. Those flags are now
stripped when rendering, and remaining placeholders become angle-bracket
metavariables, because `{note}` is a substitution and `<note-id>` is an
instruction.

*And both pages read as test instructions.* They explained hash manifests,
mutation results, regenerate commands and the development history of their own
bugs. All of that is true and none of it belongs in front of someone trying to
tag a note; it belongs here. The pages now say only what a reader needs, name
each other so the two surfaces can be compared, and state plainly that tagging
is unavailable in the interface -- as a gap, in a sentence, rather than as a
registry classification.

*A capability finding fell out of the rewrite.* Making the two catalogues
parallel required asking, task by task, whether each surface can do the thing.
Tagging is the only capability the command line has and the interface does not,
and now both pages say so in the same words.

**Formatting corrections 2026-09-03, from looking at the rendered page.** The
journey catalogue rendered as an unbroken wall of text, and three separate
faults were behind it -- none visible in the Markdown source, all obvious in a
reader.

*No blank line before the list.* `docgen` wrote a fragment's prose sentence and
then its items with nothing between, so Markdown folded the sentence and every
bullet into one paragraph. Short enumerations survived that; a catalogue of ten
tasks with five steps each did not. Fixed in the renderer, so every generated
fragment benefits.

*Every item was forced to the same level.* The renderer prefixed `- ` to each
value unconditionally, so tasks and their steps sat side by side and a reader
could not see where one task ended. A value that already carries its own marker
is now written as it is, which is what lets a resolver nest.

*The published sentence was written for the wrong reader.* A fragment's opening
line comes from the Go doc comment, and all three catalogue comments began
"Lines renders the catalogue for the generated fragment" -- an accurate
description of a function, printed to someone trying to tag a note. They now
read "Each task below lists the steps that do it, in order."

*One thing was tried and rejected.* Putting each command in a fenced `sh` block
looked better and made every step a registered executable example, whose hash
would change whenever a narrative was reworded. That is churn with no reader
benefit, since the journey runner already executes those commands. An indented
code span reads the same and costs nothing.

**The interface tagging journey is a standing check rather than a note.**
`docs/journeys-gui.md` has no tagging journey because the interface cannot tag,
and a journey for something a surface cannot do would be a lie with pictures.
`TestTaggingGainsAGUIJourneyWhenTheInterfaceCanTag` fails the moment the feature
records a `gui` surface without a matching journey, and says what to add and how
to capture it. "Add the journey later" is the kind of intention that survives in
a plan and not in a repository.

**Exit criteria.** The generation half stands on its own and is not conditional
on the model work: the features page, the two journey catalogues and the
comparison are worth having whether or not any model turns out to be usable, and
the surface-coverage check is worth having whether or not the pages are ever
scored. If the evaluation half is abandoned, the generation half still ships.

The evaluation method is worth adopting only if all of these hold: at
least one free model scores materially better than the 7/16 baseline on the
existing calibration; the three-arm ablation separates arm 1 from arm 2 on a page
known to be good, so the test can tell prose from prior knowledge; and the tags
case is flagged. Failing any of them, the recommendation is to stop, and the
investigation is still worth having done.

**If it succeeds.** Add a follow-up plan item to evaluate the documentation with
the method: a full pass over the user-facing pages, a ranked list of task topics
a reader cannot act on, and prose fixes for the worst of them -- with the
evaluation itself staying advisory and out of `make validate`. That item is not
written yet, deliberately: it should be scoped by what the investigation actually
finds rather than by what it is hoped to find.

**Outcome (2026-09-04).** Complete, and the two halves ended differently.

*The generation half shipped and stands on its own*, as the exit criteria said
it would whatever the models did: `docs/features.md` with a coverage gate at
zero backlog, three catalogues, a computed surface comparison, and eight defects
found by writing them -- two documented flags that did not exist, three product
gaps including a raw `FOREIGN KEY constraint failed` on a plausible user action,
a command missing from the CLI registry, and two containment defects in the
harness itself. `notriosctl notes create` exists because writing the catalogue
found the gap. None of that needed a model.

*The evaluation half is stopped, on its own criteria.* Those criteria are
conjunctive, and the ablation failed outright: with repeats, with tasks chosen
to be unguessable, and with a model fast enough to run a matrix, **zero of four
tasks were credited** -- the no-prose arm kept succeeding, because the task
statement paraphrases the command it is asking for. The tags case was flagged,
so that criterion held. The first criterion -- a free model beating the recorded
7/16 on the existing calibration -- was never run, and this says so rather than
leaving it looking pending: it could not change the verdict, because the
criteria are conjunctive and one had already failed, and running a roster at 71
to 377 seconds a call to confirm a conclusion already reached would have been
spending hours to learn nothing.

*So the "If it succeeds" clause does not fire, and no follow-up evaluation item
is written.* Making the no-prose arm a real control needs task statements that
convey intent without the command's own vocabulary, and it is not obvious such a
statement exists for most tasks -- a task named without its vocabulary may not
be a task a reader would recognise either. That is a research problem rather
than a documentation one, and a faster or stronger model makes it worse rather
than better: a better guesser satisfies the control arm more often, so fewer
pages earn credit.

Evidence under `performance/v0.8-h14/`: `ACTIONABILITY.json` (24 runs, three
arms, two repeats, one paid model with its authorisation recorded), the harness
that produced it, and the journey capture. Nothing here is a build gate and
nothing was added to `make validate`, as this item required of itself.

