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

## Linking to a section or a block

A link can point inside a note, not just at it. Add an anchor:

```text
notrios://databases/db_qz.../documents/doc_01H...#getting-started      # a heading
notrios://databases/db_qz.../documents/doc_01H...#^blk_7fq3...         # a block
notrios://databases/db_qz.../documents/doc_01H...#^my-anchor           # your own marker
```

Ask a note what it offers, and build the link without typing it:

```sh
notriosctl link --list-anchors doc_01H...
notriosctl link --anchor getting-started doc_01H...
notriosctl link --anchor 'Getting Started' doc_01H...   # heading text works too
notriosctl link --anchor '^my-anchor' doc_01H...
```

An anchor that does not resolve is refused rather than printed. A stable link is
meant to be pasted somewhere permanent, and one that never worked is worse than
no link.

### Headings

A heading anchor is the heading's **slug**: lowercase, spaces to hyphens,
punctuation dropped — `## Install & Setup` becomes `install-setup`. Repeated
headings stay addressable (`notes`, `notes-1`, …).

You can write the heading's text instead and Notrios normalizes it, so
`#Install & Setup` and `#install-setup` reach the same heading. The slug is what
a stable link carries, for a practical reason: in Markdown a space ends an
unquoted URL, so `[x](document://…#Install & Setup)` truncates at the space. A
wikilink (`[[Note#Install & Setup]]`) keeps the text intact, which is the form
Obsidian uses.

If you have used Obsidian, this is the same model with one difference in what
Notrios *writes*. Obsidian percent-encodes the heading text into its URI
(`obsidian://open?vault=V&file=Note%23Heading%20Name`); Notrios always emits the
slug, which needs no encoding at all.

Notrios does **read** a percent-encoded anchor, but only inside a link that
carries one of its URI schemes:

```text
notrios://…/documents/doc_x#Install%20%26%20Setup     ✅ decoded → install-setup
document://default/documents/doc_x#Install%20%26%20Setup ✅ decoded
[in a note](#Install%20%26%20Setup)                    ❌ literal text
```

The scheme is what makes the difference: percent-encoding is defined for URIs,
so a link that declares itself a URI is the one place `%20` means a space. A
bare `#…` in a note is text you typed, and Notrios will not reinterpret it —
which is why a heading called `100% Coverage` keeps working. Link *targets* are
never decoded either; use the canonical URI or a wikilink for a note name with
spaces.

### Blocks

A block anchor is `#^` followed by either your own marker or the block's derived
ID. Write your own by putting `^my-anchor` at the end of a paragraph, preceded
by a space:

```markdown
The paragraph you want to cite. ^my-anchor
```

Precedence when resolving is marker, then block ID, then heading slug — the name
you chose wins.

### What breaks, and when

| You change | The anchor |
|---|---|
| move a block or heading within the note | keeps working |
| edit other parts of the note | keeps working |
| clean up whitespace or line endings | keeps working |
| rewrite the block's text | breaks (`stale_anchor`) |
| rename the heading | breaks (`stale_anchor`) |
| add your own `^marker` | survives rewriting that block |

A broken anchor is reported as `stale_anchor` and Notrios still tells you which
note it was, so a client can offer to open the note. Dropping you at the top of a
note that no longer contains what the link named would be worse than saying so.
`notriosctl lint` reports every anchor in your library that no longer resolves.

## What this is not

This is local routing, not synchronization. Resolving a link opens a note in a
database this machine already has; it never contacts a peer, fetches a remote
note, or copies anything between databases. Sharing content between databases
is [archive v2](archive-v2.md) today and record-level sync later.
