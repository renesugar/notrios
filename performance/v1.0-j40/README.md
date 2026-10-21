# v1.0 J40: show ambiguous links, and report them with the notes they might mean

The plan is in `PLAN.md` under J40, with the owner's decisions of 2026-10-18.

## J40-A: what is recorded today (2026-10-21)

The tests are `internal/importers/obsidian/j40a_ambiguity_test.go` for the
library and `web/src/__tests__/j40a-preview.test.tsx` for the preview. Each case
holds one link and records, for that link, what the note keeps, the stored row,
the editor's buffer check (`POST /api/v1/links/check`), lint's `ambiguous_link`
and `broken_document_link` counts, and whether the importer warned.

| how the link became ambiguous | note keeps | row | editor | lint | importer |
|---|---|---|---|---|---|
| two notes titled `Shared`, written in Notrios | `[[Shared]]` | ambiguous | ambiguous | `ambiguous_link` 1 | — |
| `a/Foo.md` and `b/Foo.md`, titled by their file names | `[[Foo]]` | ambiguous | ambiguous | `ambiguous_link` 1 | warned |
| `a/index.md` and `b/index.md`, titled by headings (J36-E; J32's collision corpus) | `[[index]]` | **unresolved** | **unresolved** | `broken_document_link` 1 | warned |
| two notes claim the alias `Shared` | `[[Shared]]` | **unresolved** | **unresolved** | `broken_document_link` 1 | warned |
| `a/pic.png` and `b/pic.png` | `![[pic.png]]` | ambiguous | ambiguous | `ambiguous_link` 1 | warned |

**The expected surprise is confirmed, and is narrower than the plan expected.**
The importer decides ambiguity by file name, and then by alias. It leaves an
ambiguous link as written (J36-E). The store re-resolves that text the only way
it can, by note title, so the row depends on how the notes that share the name
were titled:

- **Titled by their file names:** they share the title too, and the row says
  `ambiguous`.
- **Titled by a heading or by front matter,** as every note in J32's collision
  corpus is: no note has the title, and the row says `unresolved`.
- **An alias:** the store never resolves aliases, so an alias two notes claim is
  `unresolved`.
- **An attachment name:** a resource keeps its file name, so it is `ambiguous`
  to both the importer and the store.

A report built only from rows would therefore miss the vault-name case whenever
titles come from headings, and the alias case always. Those are the cases that
put this item on the plan.

**The editor and lint agree with the row.** The buffer check runs the store's
extractor and resolver, so it can say nothing the row does not. Lint reads the
rows. A link the importer found ambiguous, but the store calls `unresolved`, is
counted by lint as a broken document link, not as an ambiguous one.

**Library Health shows lint's counts and nothing else.** It fetches the capped
findings but renders only each check's count (`LibraryHealth.tsx`, the
`health-checks` list). So it can say "1 ambiguous link" or "1 broken document
link", but never which link or which note. When a check is truncated, it adds
"showing the first N", where N is the number of findings it fetched, though it
shows none of them. No test pins either behaviour.

**The preview says nothing about any of them.** The renderer has no wiki-link
rule, so `[[index]]` is printed as prose, brackets included. A Markdown link
whose target is not a scheme the preview routes keeps its `<a>` element and
loses its `href`. It then has the same class as a working link and no title. The
preview renders the body alone and never consults the service.

**The editor's check runs only for an editable note.** `useBufferLinks` is
enabled by `editable`, so a note in a read-only notebook, Help or Reports, is
never checked. J40-B's marking is planned to come from that check. For a
read-only note there would then be nothing to mark from, unless the preview
gains a check of its own. J40-B's design has to say which.

### What the library keeps that J40-E can use

The plan says that deriving the candidates later "needs the source vault, which
the library does not keep". That is not quite so. Every Obsidian import writes
each note's provenance (`noteSource`, into `document_sources`). Its
`external_id` is the note's vault path, and its `metadata_json` holds
`relative_path` and `aliases`. That is what the importer's namespace was built
from: names come from paths, and aliases from front matter. Provenance is
written on every import, including for a skipped note (J39), and it travels by
sync. An attachment keeps its file name on the resource.

What provenance does not keep is **which vault** a note came from. Two vaults
imported into one collection would be one namespace to a derivation, where the
importer saw two. Item states (`import_item_states`) do record the vault, by
source key, but they are local, and a library reached by sync has none.

### What the owner decided on this evidence (2026-10-21)

The plan records these as decisions 9 to 11.

- **J40-E derives candidates from provenance,** and records nothing new at
  import. There is no schema change, and the one-namespace-per-collection limit
  is stated where it applies.
- **A read-only note is not marked.** The preview makes no request of its own.
- **J40-B renders wiki links, and J47 renders embeds.** A probe for this
  decision found that the preview shows every link an import resolved as raw
  text: `[[document://default/documents/doc_x|shown]]` renders as exactly those
  characters inside a `<p>`, and so does `![[resource://…]]`.
