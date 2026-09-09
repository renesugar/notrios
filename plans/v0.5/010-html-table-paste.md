# v0.5 E6b — Paste normalization: HTML tables to Markdown

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## Why

Pasting an HTML table already *looked* right: the preview rendered it and the
sanitizer stripped scripts, handlers, and inline styles. What landed in the note
**source** was still raw HTML, and that is where it cost:

- `internal/markdownblocks` saw the table as paragraphs, so nothing in it was
  block-addressable;
- `internal/markdownlinks` does not read `<a href>` inside HTML, so those links
  were invisible to the graph, to lint, and to `notriosctl fix`;
- a publication handoff carried the raw HTML downstream to `movenotes-v3`,
  Obsidian, and Quartz — the least portable thing a note can hold.

Converting on paste puts the content into the format the rest of the system
already understands.

## What it does

A `paste` handler on the editor, through the CodeMirror `domEventHandlers` E6
established. When the clipboard offers `text/html` whose content **is** a simple
table, it is replaced with a Markdown pipe table at the caret. Everything else
falls through to the ordinary paste.

## The refusals are the feature

`web/src/html-table-markdown.ts` refuses far more than it converts, and returns
a reason for each. A mangled table is worse than an HTML one, because the HTML
at least renders — the same rule E3's fix and P7's rewriter follow for a span
they cannot place exactly.

| Reason | Refused because |
|---|---|
| `merged-cells` | `colspan`/`rowspan` have no pipe-table representation |
| `ragged-rows` | padding a short row invents cells the author never wrote |
| `nested-block` | a nested table, list, `pre`, `blockquote`, or heading in a cell |
| `multiline-cell` | a `<br>` or a second `<p>`: a pipe-table row is one line |
| `content-outside-table` | the paste merely *contains* a table; rewriting part of it would drop the rest |
| `multiple-tables` | two tables side by side |
| `no-table`, `empty-table` | nothing to convert |

Because every refusal falls through, **nothing a user pastes can be lost here**.

## Decisions worth recording

**Wrapper elements are transparent, block structure is not.** Real pastes are
full of `<p>`, `<div>`, `<span>`, and `<font>` — Google Docs puts a paragraph in
every cell — so treating those as structure would refuse nearly every genuine
paste. A *second* paragraph, or a `<br>`, is different: that is multi-line
content, and it is refused rather than flattened into a sentence the author did
not write.

**A link is emitted only when its target survives the syntax.** A space ends an
unquoted URL and an unescaped `)` closes the link early — the same Markdown
rules E1a and E3 each met in a fixture. When the href will not survive, the text
is kept and the link dropped: losing a link is recoverable, writing a broken one
is less obviously so. `javascript:` targets are dropped outright.

**The first row is promoted when there is no `<th>`.** Markdown has no
headerless table, and that is what every renderer does with the result anyway,
so it is the honest rendering rather than an invention.

**A nested table reports `nested-block`, not `multiple-tables`.** The first
implementation counted every `<table>` in the document, so a nested one was
refused with the wrong diagnosis. Only outermost tables are counted now. The
refusal was correct either way; the reason code is what a user would read.

**Nothing is executed and no HTML is re-emitted.** Parsing uses `DOMParser`,
which builds an inert document — scripts do not run and resources are not
fetched — and only text plus a whitelisted set of attributes is ever read. A
fixture asserts a `<script>` and an `onerror` in a pasted cell leave no trace and
set nothing on `window`.

## Verification

95 web tests, including the conversion cases, every refusal, the escaping, and
the handler's fall-through contract.

Unit tests cannot cover the wiring — the converter is pure and the handler was
tested against a fake view — so the paste path was additionally driven
end-to-end in real headless Chrome, dispatching a genuine `ClipboardEvent`
carrying `text/html` at the live editor:

```json
{
  "simple_table_converted": true,
  "simple_text_sample": "| Item | Cost || --- | --- || Pan | 12 |start",
  "merged_fell_through_to_plain": true,
  "pwned": "undefined"
}
```

A simple table became a Markdown table at the caret; a merged-cell table fell
through to the plain-text clipboard flavour; nothing executed.

No scale profile: this is a per-paste client transform bounded by the size of
one clipboard payload, with no library-size dimension to measure.

## What is out of scope

A server-side HTML-to-Markdown converter for importers. That is a different
feature with different inputs, and the plan said so.

## Validation

`go vet ./...`, `go test ./...`, required files, plan-loop, scaffold validation,
OpenAPI parse, migration-copy equality, web typecheck/tests (95)/build,
`make gui`, docs-site build, Help reseed, REST/MCP smoke, performance smoke, the
E6a offline-assets check, and the end-to-end paste verification above.
