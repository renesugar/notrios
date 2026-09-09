# Publishing a subset of your notes

Publishing is not backup. A backup reproduces your library; a **publication
handoff** is a deliberately narrowed projection of it — the notes you chose,
with everything you did not choose removed from what goes out.

Notrios produces the handoff as a checksum-verified archive-v2 directory. It
does not build a site: converting the handoff into an Obsidian vault, a Quartz
site, or a Hugo site is the downstream toolkit's job.

## What a publication withholds

By default a publication handoff carries only the current text of the selected
notes:

| Withheld | Why |
|---|---|
| revision history | an earlier revision can contain exactly the sentence you later removed |
| trashed notes | deleting a note is not a request to publish it |
| notes tagged `private`, `draft`, or `confidential` | secure defaults; change them with `--exclude-tags` |
| source provenance and exact source bundles | they describe where the note came from, including local paths |
| revision metadata | application state, not content |
| saved searches | their queries describe the whole library |
| links to withheld or broken targets | see below |

## What happens to links

A published note must not carry a reference a reader cannot follow, and must not
name a note you held back. So each link is classified and handled:

| Link | Result |
|---|---|
| to another published note | kept as a link |
| to an external `http(s)` URL | kept as a link |
| to a note the publication withheld | rewritten |
| unresolved, ambiguous, or pointing at a deleted note | rewritten |

`--link-action plain_text` (the default) leaves the link's display text as
ordinary prose. `--link-action redact` replaces it with `[redacted]`, which
removes the display text too. In both cases the link *record* is dropped as
well, because it would otherwise carry the withheld note's ID, the raw target,
and a context excerpt of the surrounding sentence.

If a note's stored link positions no longer match its body, that link is left
alone rather than cut out at a stale offset, and the report warns you. Re-save
those notes and publish again.

## Saving a profile

```sh
notriosctl publish profile save --name public-site \
  --query 'notebook:"Public"' \
  --link-action plain_text \
  --exclude-tags private,draft,internal
notriosctl publish profile list
notriosctl publish profile delete --name public-site
```

A profile records *what* to publish, never where to put it or what to run
afterwards. Profiles live in `<data-dir>/publish-profiles.json` (override with
`--profiles` or `NOTRIOS_PUBLISH_PROFILES`) and are written owner-only, because
a profile says which of your notes are private.

A profile's selectors reach every collection unless one names a collection. A
note you migrated in is a note you own, so `--query 'notebook:"Public"'` matches
it wherever it came from; if provenance is what you mean to select on, say so
with a `collection:` term or the profile's own collection field. This is worth
knowing when reading a review: a selector written before Notrios spanned
collections may now match more than it did, which is exactly what the review is
for — the counts are shown before anything is written, and publishing refuses
unless the plan you read is still the plan.

`--target full_archive` is refused: a full archive carries trashed notes, every
revision, provenance, and exact source bundles, and it should never be reachable
by a name that sounds like publishing. Use `notriosctl export archive-v2` when
you want a backup.

## Reviewing before publishing

```sh
notriosctl publish plan --profile public-site
```

This writes nothing. It reports the notes it would publish, the resources they
reach, every link decision, the metadata it would strip, warnings, and a
`manifest_sha256` digest over the whole plan. Read it — that is the point of the
step.

## Publishing

```sh
notriosctl publish run --profile public-site \
  --reviewed-plan <manifest_sha256 from the plan> \
  /transfers/public-site
```

The digest is required. Notrios re-plans at publication time and refuses if the
result differs from what you reviewed:

```text
the reviewed plan no longer matches this library: reviewed 37adf55f…,
this library now plans 323fc893…; re-run the plan and review the difference
```

That is not friction for its own sake. A profile is not a promise about a fixed
set of notes: adding a note to the published notebook, or removing a `private`
tag, changes what would go out. "I checked this yesterday" is not a review of
what would be published today.

The result is an ordinary archive-v2 directory — the same manifest-last,
SHA-256-addressed format as a backup, verified before the command reports
success — whose report says `full_backup: false` and whose warnings state
plainly that it is a projection. See [archive v2](archive-v2.md).

## Safety boundaries

- Notrios never executes note content. A profile cannot name a command, and
  publishing runs no build step.
- Publishing is a local CLI operation. No REST or MCP endpoint accepts an output
  path or produces a handoff; `POST /api/v1/selection/plan` exposes the
  read-only review, and nothing more.
- Do not rely on a static-site generator's private-page filter to protect notes
  or attachments. The exclusion happens here, before any file is written.
