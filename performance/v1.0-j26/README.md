# v1.0 J26: import ChatGPT, OpenAI Privacy Portal and Claude archives as downloaded

The archives are the owner's own, and private. This record holds only counts,
sizes and timings: no message text, names, IDs or bytes from them. Every test
fixture is synthetic.

## What was found (2026-09-15)

The conversation importers from v0.2 had never run on a real archive. Both took
a `conversations.json`, read it whole with `os.ReadFile`, and imported message
text only.

| | ChatGPT export (21,342,993 B) | OpenAI Privacy Portal (154,295,575 B) | Claude (5,775,642 B) |
|---|---|---|---|
| shape | one ZIP: `conversations.json`, `chat.html`, 71 asset files | one ZIP holding **nested ZIPs** under `User Online Activity/`: `Conversations__….zip`, `Files__….zip`, `Ads__….zip` | one ZIP: `conversations.json`, `users.json`, `projects/*.json` |
| conversations | 72 | **147**, across `conversations-000.json` and `conversations-001.json` | 75 |
| assets | `file-<id>-<name>.<ext>` and `file_<hash>-<name>.<ext>` | **225 `file-<id>.dat`**, extensions stripped; plus a 27-file library in its own ZIP | none: attachments carry extracted text, `files` carry a name only |
| before J26 | needed unzipping; no assets imported | **unreadable**: the conversations are inside a nested ZIP | needed unzipping; attachment text, file references and projects dropped |

File IDs use both prefixes, and `file_` is the common one: 48 of the direct
export's 71 assets, 207 of the portal's 225 `.dat` files, 248 of 250
`library_files.json` IDs. `file-service://` pointers name the first form and
`sediment://` the second.

**Owner decisions (2026-09-15).** The file library is imported, with files no
conversation references given notes of their own; each Claude project becomes a
note; code and execution output are rendered while thinking, reasoning recaps
and tool plumbing are counted rather than imported.

## J26-A: one archive reader

J25's ZIP-or-folder reader moved from the Twitter/X importer into
`internal/importers/archivesource`, keeping its bounds, its refusal of names
that escape an archive, and its streaming decode. It gained:
- **`All()`**, to index assets an archive keeps in subdirectories.
- **`Nested()`**, which opens a ZIP held inside a ZIP **in place**. A ZIP needs
  random access, which a compressed entry cannot give, so the nested archive is
  spooled into the instance's own temp space (J22) and removed when it closes;
  with no instance configured, a small one is held in memory.
- A **`Spec`** per format: which files mark its data directory, and the
  importer's own wording when a file is not one of its archives.

The Twitter/X importer runs on it with its behaviour and tests unchanged.

Its own tests cover a folder and a ZIP reading alike, the data directory found
at either depth, unsafe names refused and counted, each bound, a file larger
than it claims, a nested ZIP both in memory and spooled (removed on close), and
the array decoder.

## J26-B and J26-C: the two ChatGPT shapes

`import chatgpt` takes the export ZIP, the Privacy Portal ZIP, an extracted
folder, or a `conversations.json`, and reads **every** `conversations*.json`
shard in order.

- **Assets** are imported as resources and embedded, matched under either id
  prefix, including one in the export's `user-<id>/` subfolder.
- **A `.dat` file gets its name back** from `conversation_asset_file_names.json`,
  then `library_files.json` (which also carries the MIME type), and otherwise
  from its own bytes; the report counts which source named how many.
- **The file library** is imported, and a file no conversation references gets a
  note in a **ChatGPT Files** notebook.
- **`chat.html` is not a source.** It renders the conversations in both shapes,
  but in the portal export it shows the `.dat` names; the JSON is read instead.

## J26-D: Claude

`import claude` takes the export ZIP, a folder holding the `…batch-NNNN.zip`
files of one split export, an extracted folder, or a `conversations.json`.
Attachment text is kept under its file's name, a file the export only names is
marked as absent, and **each project becomes a note** with its description,
prompt template and docs.

## J26-E: what a note contains

`internal/importers/conversationnote` renders both importers' notes: code and
execution output as fenced blocks, browsing results as text, attachments named
where their message is. Thinking, reasoning recaps and tool calls are counted in
the report rather than written into the note.

## The tests

Synthetic fixtures, in both packages:
- the export ZIP and the same files as a folder, read alike, with every shard,
  both id prefixes, code fenced with its language, output fenced, thoughts
  counted and absent from the note
- a path straight to `conversations.json`
- the Privacy Portal shape — an outer ZIP whose `User Online Activity` folder
  holds the conversations and library ZIPs — with `.dat` names from the map,
  from `library_files.json`, and from sniffed bytes, and a stub note for the
  unreferenced library file
- Claude's ZIP: machinery counted not imported, attachment text kept, a named
  but absent file marked, the project note, and batch siblings read in order
- re-import changes nothing, in both importers
- a file that is neither export is refused clearly, in both importers

**A defect the re-import test caught.** An asset whose extension came from
sniffing its bytes was named one way on the first import and another on the
second, because the second found the resource already present and returned the
unsniffed name, so the note was rewritten for nothing. The stored resource's
name is now what every later import uses.

## J26-F: the three real archives

One run of `real_archives_run.sh` per archive: a dry run, an import into an
empty library, and a re-import. HOME, the XDG roots and TMPDIR pointed into the
work directory on `/media/renes/HD2`, never RAM-backed `/tmp`.

| | ChatGPT export | Privacy Portal | Claude |
|---|---|---|---|
| dry run | 2.1 s, 33 MB | 3.2 s, 30 MB | 1.9 s, 24 MB |
| **import** | **13.8 s, 52 MB** | **31.8 s, 42 MB** | **6.9 s, 39 MB** |
| re-import | 1.5 s, 27 MB | 3.7 s, 33 MB | 2.0 s, 26 MB |
| conversations | 72 | 147 | 75 |
| notes imported | 72 | 174 (147 + 27 library stubs) | 76 (75 + the project) |
| messages imported / skipped | 1,303 / 597 | 1,694 / 5 | 728 / 22 |
| code blocks | 299 | 0 | 37 |
| machinery counted, not imported | 163 | 2,240 | 3,339 |
| assets referenced / imported / absent | 81 / 66 / 15 | 185 / 172 / 13 | — |
| `.dat` names: map / library / sniffed | — | 132 / 40 / 0 | — |
| library files seen / imported / stub notes | — | 27 / 27 / 27 | — |
| attachments / file references | — | — | 146 / 109 |
| nested archives read in place | — | 2 | — |
| entries refused | 0 | 0 | 0 |
| library after import | 23,363,584 B | 27,971,584 B | 12,107,776 B |
| **re-import** | unchanged | unchanged | unchanged |

- **Every re-import changed nothing**: no note imported or updated, and the
  library the same size to the byte.
- **Nothing was left in the temp directory** after the runs (J22's rule holds
  for the spooled nested archives too).
- **Memory stays small**: the largest peak is 52 MB, for a 154 MB archive whose
  conversations are a 139 MB ZIP inside it. The shards are streamed, and the
  nested ZIP is spooled to disk rather than held.

### What the numbers mean where they look odd

- **Assets absent from the archive, not missed.** The direct export references
  82 distinct file IDs and carries bytes for 66; sixteen are absent from the
  download itself, under both prefixes. The importer names those in the note and
  counts them. The report says 15 because one absent ID is referenced twice in
  one message and is deduplicated.
- **No code blocks in the portal export.** Its messages are thoughts (1,905),
  text (1,638), reasoning recaps (361) and multimodal parts; it carries no
  `code` or `execution_output` messages at all, where the direct export has 181
  and 121.
- **Nothing needed sniffing.** The asset map named 132 `.dat` files and
  `library_files.json` named the other 40, covering all 172 imported.
- **27 library stub notes.** No conversation references any of the 27 files in
  that export's file library, so each got a note of its own.

### A counting defect the real archive found

The first Claude run reported `machinery_skipped: 0` for an archive holding
1,318 `tool_use`, 1,317 `tool_result` and 631 `thinking` blocks. The renderer
returned as soon as a message's own `text` field held prose — which a real
export populates — so the blocks were never counted. The notes were right; the
number meant to make what was left out visible was not. Blocks are now counted
whichever field carries the prose, and the same archive reports **3,339**. The
numbers above are from the rerun, with notes and timings otherwise unchanged.

## Boundaries kept

- Nothing from the archives is in the repository or in this record: only counts,
  sizes and timings.
- Nothing is extracted from any archive to a path of its choosing; a nested
  archive is spooled only into the instance's own temp directory and removed.
- No network access.
- Re-import stays idempotent and a note the user trashed is never resurrected.
