# Command-line journeys

The [CLI reference](cli.md) tells you what each command and flag does. This page
is the other document: a set of tasks someone actually sets out to do, with the
steps that do them and — the part that matters — how you can tell it worked.

Every journey here is executed. A test runs each one against a disposable
library and then checks the state afterwards, so what you read below is a
recording rather than a description. That check is deliberately not "did the
command exit zero": a command can exit zero having done nothing, and it can exit
zero having done the opposite of what a page said. Only looking at what changed
afterwards tells those apart.

Some steps are yours rather than Notrios' — writing a file, plugging in a drive.
Those are marked and not executed, because a catalogue that only described steps
it could run would leave out the parts you are most likely to get stuck on.

## What the command line will not do for you

Three gaps are worth knowing before you start, and they are recorded here rather
than smoothed over.

**There is no command that writes a note.** `notriosctl` imports and maintains a
library; it does not author in one. From the command line a note arrives by
import. Writing happens in the GUI or over the API.

**The Obsidian importer cannot choose a notebook.** It takes `--collection`, not
`--notebook`, so a note imported this way lands wherever the importer puts it.

**`--collection` will not create a collection.** Naming one that does not exist
fails with `FOREIGN KEY constraint failed`, which is the database talking rather
than Notrios. Use `default`, or a collection that is already there.

## The journeys
<!-- notrios:generated:user:the-journeys:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docjourneys#(Catalogue).Lines -->
Lines renders the catalogue for the generated fragment.
- Find out where your notes actually live — See which library this installation is using before doing anything to it. (2 steps, verified by: doctor reports that the required checks passed and names the database it used)
- Write a note from the command line — Create a note, file it in a notebook, and confirm it is there. (3 steps, verified by: the two created notes appear in an export of the whole library) `notes create` was added in v0.8 H14 because this journey could not be written without it: the task went through a one-file Obsidian import, which is what the test suite had been doing for the same reason.
- Find notes with the query language — Select a subset of notes by tag, notebook or text, and see exactly which ones matched. (1 steps, verified by: the export reports a smaller set than the whole library, so the query actually filtered) The command line has no search command; it applies the same query language to an export instead, which is how you see a result set as files.
- Export the library and check the export is sound — Write a portable copy of everything and confirm it is complete before trusting it. (2 steps, verified by: the archive's manifest exists on disk after the export)
- Keep work notes and personal notes apart — Run more than one library on this machine, each with its own database and settings. (2 steps, verified by: the profile registry lists the profile that was just created)
- Prepare a library to synchronize — Turn on synchronization for a library and see where its keys are kept. (2 steps, verified by: status reports the library as enrolled and names the store holding its keys)
- Find what has gone stale in a library — See broken links, orphaned attachments and other rot, and fix what can be fixed mechanically. (2 steps, verified by: lint produces a report rather than changing the library)
- Read the documentation inside your own library — Get the Notrios guides into the library as notes, so they are searchable like anything else. (1 steps, verified by: the seed reports how many documentation files became notes in the library)
- Move sync keys into the operating system's keychain — Take key material out of the development file and put it where the OS keeps secrets. (2 steps, verified by: the dry run reports the plan and says nothing was moved) Nothing about this command is guessable: the subcommand name, the direction flag and the confirmation are all specific to Notrios, which is what makes it able to measure a page.
<!-- notrios:generated:user:the-journeys:end -->


## How this page is kept honest

The list above is generated from `docs/docjourneys/CLI_JOURNEYS.json`, which
holds each journey's steps and the postcondition that confirms it. Three checks
run against it.

Every journey executes, and its postcondition must hold afterwards. Every
journey names a capability from the [features page](features.md), so a task
cannot exist for something the product does not claim to do. And the number of
features with no journey yet is tracked, so that backlog can shrink but not
grow.

Writing this catalogue found the three gaps above, and two flags that do not
exist — `paths --db` and `import obsidian --notebook` — both of which had been
written down from memory of other commands rather than from the usage message.
That is the whole argument for executing a journey rather than describing one.
