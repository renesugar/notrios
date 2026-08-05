# Stable links between notes and machines

A `document://` URI identifies a note inside one database. A **stable link**
identifies it from outside:

```text
notrios://databases/{database_id}/documents/{document_id}
```

It carries the *logical database identity* — not a file path, not a profile
name, not a hostname. Move the database, rename the profile, restore it onto
another machine, and the link still names the same note. Paste it into an
email, a task tracker, or another note.

Get one with:

```sh
notriosctl link --db data/notes.sqlite doc_01H...
```

```json
{
  "document_id": "doc_01H...",
  "title": "Kitchen renovation",
  "document_uri": "document://default/documents/doc_01H...",
  "stable_uri": "notrios://databases/db_qz.../documents/doc_01H..."
}
```

Clients can build the same link themselves: `GET /api/v1/status` reports
`database_info.database_id`. The per-copy replica ID is deliberately not
reported — it means nothing in a shared link.

## Telling this machine where your databases are

A stable link names a database, so something has to know which database that is
on *this* machine. That is the profile registry: a small file (by default
`~/.config/notrios/profiles.json`) that you fill in explicitly.

```sh
notriosctl profile register --name work --db /srv/work/notes.sqlite
notriosctl profile register --name personal --db ~/notes/notes.sqlite
notriosctl profile list
notriosctl profile forget --name work        # registry edit only; the database is untouched
```

`profile register` reads the database ID **out of the database**; you cannot
assert one on the command line. The registry is written with owner-only
permissions because it records local paths.

## Opening a link

```sh
notriosctl open 'notrios://databases/db_qz.../documents/doc_01H...'
```

The exit code is part of the contract, because a desktop protocol handler runs
this without a terminal:

| Exit | Meaning |
|---:|---|
| `0` | the link named a note this machine can open |
| `1` | it could not be resolved: unregistered database, ambiguity, or a note that no longer exists |
| `2` | the link itself was malformed |

Notrios never guesses which database answers a link:

- **Unregistered database** — reported as `unregistered_database`. It does not
  fall back to the only database you happen to have.
- **Several copies** — if two profiles hold clones of one logical database
  (a restored backup, a synced copy), the result is `ambiguous_database` and
  every candidate is listed. Choose one with `--profile <name>`.
- **Wrong database** — `--profile` settles ambiguity but cannot redirect a link
  into a different database. Checking a link against an explicit `--db` that
  holds another database returns `foreign_database` rather than matching a
  local note that happens to share the ID. Document IDs are unique per
  database, not globally.
- **Stale target** — the link is well formed and points here, but the note is
  gone: `stale_target`. A note in the Trash is reported as `trashed`, which is
  a different thing from missing.

`--launch` opens the resolved note in the local web UI with `xdg-open` (the
service has to be running; this command does not start one).

## Registering the desktop handler (Ubuntu)

```sh
notriosctl register-url-handler            # prints the desktop entry, changes nothing
notriosctl register-url-handler --apply    # installs it and registers the scheme
```

Printing is the default because this changes what happens when you click a link
anywhere on the machine. `--apply` writes
`~/.local/share/applications/notrios-url-handler.desktop` and runs `xdg-mime`
and `update-desktop-database`. The entry claims `x-scheme-handler/notrios` and
nothing else — it never becomes your browser or file handler. Only Ubuntu/XDG
desktops are implemented and tested.

## Links inside notes

A `notrios://` link written into a note body is treated as a first-class link:

- naming **this** database and a live note, it resolves exactly like
  `document://` — it appears in the link graph and in backlinks;
- naming **another** database, it is recorded as `external`; nothing local is
  opened for it;
- naming a note this database no longer has, it stays `unresolved`;
- malformed, it is recorded as `invalid` rather than silently searched for as a
  note title.

Clicking one in the preview asks the service what it names. If it belongs to
another database, the UI says so instead of opening a similarly-numbered local
note.

## Linking to a block, not just a note

A link can point at one block inside a note — a heading, a paragraph, a list
item, a code block, or a table — by adding an anchor:

```text
notrios://databases/db_qz.../documents/doc_01H...#^blk_7fq3...
document://default/documents/doc_01H...#^blk_7fq3...
```

List a note's blocks, with the number of links pointing at each, with
`GET /api/v1/documents/{id}/blocks`.

**A block's ID comes from its text.** That has two consequences worth knowing
before you paste one somewhere permanent:

- Move the block around the note, or edit anything else in the note, and the
  link keeps working.
- Rewrite the block's text and the link stops resolving. Notrios reports this
  as `stale_anchor` and still tells you which note it was: the anchor named
  exactly that text, and quietly dropping you at the top of a note that no
  longer contains it would be worse than saying so.

If you want an anchor that survives rewriting, write your own marker at the end
of the block, Obsidian-style:

```markdown
The paragraph you want to cite. ^my-anchor
```

Then link to `#^my-anchor`. Author-written markers are names you chose, so they
outrank the derived ID and follow the block through edits. Notrios keeps both:
the marker for stability, the content hash for precision.

Whitespace cleanup is safe either way — line endings and trailing spaces are
normalized before the ID is computed, so an editor that tidies your file on save
does not break every anchor in it.

## What this is not

This is local routing, not synchronization. Resolving a link opens a note in a
database this machine already has; it never contacts a peer, fetches a remote
note, or copies anything between databases. Sharing content between databases
is [archive v2](archive-v2.md) today and record-level sync later.
