# v1.0 J36: what Obsidian actually resolves, and the worse bug underneath

This item was opened to make a partial path resolve "the way Obsidian does":
`[[topic-00001/index]]` should find `area-01/topic-00001/index.md` instead of
warning that the link is ambiguous. J32's collision corpus produced 101 of those
warnings, which is what put it on the plan.

Obsidian does not do that. Confirming the premise first is what J36-A was for,
and the premise was wrong.

## What Obsidian documents (J36-A)

> To link to a note in a folder, include the folder path before the note name.
> Folder paths start at the vault root and use forward slashes (`/`), even on
> Windows

— <https://obsidian.md/help/Linking+notes+and+files/Internal+links>

A link containing a slash is a path from the vault root. There is no suffix
matching, so `[[topic-00001/index]]` is a broken link in Obsidian too, and the
change this item proposed would have moved *away* from Obsidian's behaviour.

Precedence below that is documented only in the forum, by an Obsidian moderator,
and it is explicit about the reason:

> This is not a bug. it's intentional. Otherwise, where `[[A]]` points to
> depends on which file it is contained.

> We want `[[A]]` to point to the same note across the vault.

— <https://forum.obsidian.md/t/absolute-link-path-has-higher-precedence-than-relative-path/69542>

So the vault-root path wins over the note-relative one. Below *that* there is
nothing: the API contract for resolution is one sentence, "Get the best match
for a linkpath", with nothing about ordering
(<https://docs.obsidian.md/Reference/TypeScript+API/MetadataCache/getFirstLinkpathDest>).

## What Notrios did, measured against those rules (J36-A)

Probing `resolveNoteID` and `resolveAssetID` against a vault with a name shared
between the root and a folder, a name shared between two folders, and a name
unique in the vault:

| link | Notrios before | Obsidian |
|---|---|---|
| `[[topic-00002/index]]`, no such path | unresolved, reported **ambiguous** | unresolved |
| **`[[wrong/path/unique]]`**, path absent, base name unique | **resolved to `only/deep/unique.md`** | unresolved |
| `[[note]]` from `folder/`, with `note.md` and `folder/note.md` | `folder/note.md` | the root `note.md` |
| `[[folder/note]]`, exact root path | resolved | resolved |
| `[[index]]` from `area-01/topic-00001/` | its own folder's note | — |

The second row is the defect this item found, and it is worse than the warning
that opened it: the base-name fallback ran even when the link named a path, so a
link whose path matched nothing landed on a file elsewhere in the vault that
happened to share its last segment. A wrong target is worse than an unresolved
one. Attachments had the same hole.

The first row's *outcome* was right all along. The 101 warnings on the collision
corpus were unresolved links either way — only the stated reason was wrong.

`j36a_resolve_test.go` pins fourteen cases across notes and attachments, each
carrying Obsidian's answer for the same link in its failure message.

## A path that matches nothing is a broken link (J36-B)

`pathShapedTarget` decides whether a target names a path — it contains a slash,
or begins with a dot. When it does and no path matches, both resolvers stop:
the link is broken, the link text is left exactly as it is, and that is what
Obsidian renders. The base-name map is only consulted for a target that names a
file name.

This also retired the misleading warning: `[[topic-00002/index]]` is no longer
reported as ambiguous, because ambiguity was never the trouble.

## The vault root wins (J36-D)

`vaultCandidates` lists the root path before the note-relative one, for notes and
attachments alike, and a bare name counts as a root path. Before this, `[[note]]`
written inside `folder/` reached `folder/note.md` while the same link written at
the root reached `note.md` — the dependence on the containing file that the
moderator's answer says Obsidian exists to avoid.

It also rescues a case that used to fail: a name shared between the root and a
folder, linked from a third folder, could only be called ambiguous by the name
map, and now resolves to the root note.

The candidates are returned as a `[2]string` by value, so the ordering costs no
allocation per link — J32 spent this item's neighbourhood getting allocations
out of link rewriting, and this does not put one back.

## A reimport is the repair (J36-C)

A library imported before these two slices is repaired by reimporting it. No
migration tool is needed, because the importer calls a note unchanged only when
`current.Body == canonical` — the body already in the database against the one
this build would write — so a changed resolution rule shows up as a changed body.

`j36c_reimport_test.go` puts the pre-fix bodies back by hand, byte for byte as an
older build wrote them (`[[uri]]` spliced in place; the note-relative target where
the root one now wins), and pins both halves:

- the reimport reports **two notes updated and four unchanged**, adds exactly one
  revision to those two, and rebuilds their link rows from the new bodies;
- a **no-op reimport adds no revision to any note** — stronger than the counters
  reporting "unchanged", which an earlier test already covered.

**This constrains J39.** J39's skip is allowed to conclude that a reimport cannot
change a note, from the source file and a recorded hash. J36-B and J36-D changed
which note a link resolves to without touching a single vault file, so that skip
would fire and leave the pre-fix link in place — defeating the repair measured
here. J39-B now carries the condition: the recorded state needs a rule version
that moves when link resolution, frontmatter handling or body canonicalisation
moves, and a note whose recorded version is not this build's is not skippable.

## The ambiguity is kept (J36-E)

One divergence remains. Obsidian's rationale reaches further than precedence: a
bare name should not depend on the containing file *at all*. Where a name is
shared between two folders and no note of that name sits at the vault root,
Obsidian still resolves to one of them; Notrios reports the link as ambiguous.

There is nothing there to copy. The chosen candidate comes from the vault scan's
`readdir` order — a filename hash on ext4 with `dir_index`, name order on NTFS,
roughly insertion order on APFS — and the API promises only "the best match".

Two rules were considered. Both are rejected.

**A timestamp tiebreak.** It would not simulate Obsidian, whose order correlates
with nothing temporal. `vaultFile` carries no timestamp today and adding one is
easy, but mtime does not survive a zip extraction, a `git checkout`, an `rsync`
without `-t`, a sync client or a restore from backup — so the first restore would
move a link, rewrite bodies and create revisions with no file content having
changed, breaking what J36-C pinned.

**Shallowest path, then lexicographic.** Deterministic, identical on every
machine, and it agrees with Obsidian where Obsidian is specified, since "root
preferred" *is* "shallowest wins". Measured against today's namespace build,
five counts each, load average 0.76:

| shape | today | (depth, path) order | delta |
|---|---:|---:|---|
| 5,000 notes named `index.md`, 50 alias groups | **161.1 ms** | 185.2 ms | +24.1 ms, +15.0% |
| 10,000 notes, no shared alias | **665.0 ms** | 759.0 ms | +94.0 ms, +14.1% |

Allocations barely move: +3 allocations, +41 KB at 5,000 and +82 KB at 10,000 —
the order slice.

I expected it to be *cheaper*, because it deletes the per-key `sort.Strings`
calls. It is not: sorting a one-element slice early-returns and costs almost
nothing, so the candidate replaced N free sorts with one real O(N log N) sort of
the whole vault, through `sort.Slice`'s reflective swapper. The same intuition
J32-Q and J32-Y punished — that fewer, bigger operations beat many trivial ones.

In import terms it is nothing: the namespace is built once per import, so +94 ms
is **+0.14%** of a 66 s obsidian-10k import and +24 ms is **+0.05%** of a 47 s
collision import, both far under the run-to-run range the measurement rule
compares against. Cost is not why it is rejected.

**It is rejected because it guesses.** The ambiguity is information, and the
person who knows which note was meant is the one reading the note. A rule that
picked silently would spend that information before anyone could be asked. So
the ambiguity is kept and reported, and **J40** was written to spend it: a reader
who clicks an ambiguous link is offered the candidates, picks one, and the note
is repaired for good.

What J40 also had to record is that the reader says nothing about such a link
today. The importer leaves the literal `[[name]]` in the body, and the preview
only rewires *anchors* by URI scheme, so an ambiguous link is indistinguishable
from prose: no click target, no cursor change, no tooltip. The editor underlines
it and explains it, lint counts it, the import warns once. And the two
ambiguities are not the same one — several vault files sharing a *name* at import
time, against several notes sharing a *title* at runtime — which is what decides
what a prompt can offer, and what puts J36-B's pathless links outside J40 as a
"did you mean" search instead.

## Boundaries held

No parser grammar changed: `markdownlinks` recognises the same links. An
ambiguous link still stays unresolved with a warning, and the importer still
never guesses between candidates.

## Archive

`notrios-v1.0-j36-f6b9719.zip`, built from commit `f6b9719` — the commit that
completed this item's work, with the record above in it and the ledger not yet
flipped. 26,456,882 bytes, 2,936 entries, accepted by `check_release_zip.py`,
built by `package_release.sh` with the usage guard on and no override.

There are no measurement directories here. This item's evidence is its tests:
`internal/importers/obsidian/j36a_resolve_test.go`, which states every
resolution case with Obsidian's answer beside it, and
`internal/importers/obsidian/j36c_reimport_test.go`, which pins what a reimport
of a library imported before the fix revises. The one measurement taken — the
rejected tiebreak's cost — is in the J36-E section above, because it decided
nothing and needs no directory of its own.
