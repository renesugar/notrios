# v0.5 E1b — Scheme-scoped anchor decoding

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## Why

E1a shipped a defect: `stablelink.Parse` accepted
`notrios://…/doc_b#Kitchen%20Plan` and returned an anchor that could never
resolve. `validateID` refuses percent-escapes in identifiers, but
`validateAnchor` did not, so an escaped anchor passed validation, slugified to
`kitchen20plan`, and missed the heading. Accepting input guaranteed to fail is
worse than rejecting it.

The question that followed was whether to support the encoded form at all, and
if so how to know when `%20` means a space rather than three characters an
author typed.

## The decision

**The scheme is the declaration.** Percent-encoding is defined for URIs, so an
anchor's escapes are read as escapes exactly when the link carries one of
Notrios' URI schemes:

| Written as | Decoded | Why |
|---|---|---|
| `notrios://…/doc_x#Kitchen%20Plan` | yes | declares itself a URI; RFC 3986 defines `%20` |
| `document://…/doc_x#Kitchen%20Plan` | yes | same |
| `[x](#Kitchen%20Plan)` | no | not a URI; these are bytes the author typed |
| `[x](Some%20Note)` | no | a target is an identifier; P5's rule stands |

This is the "unique prefix" idea applied to the prefix Notrios already has. A
new marker was considered and rejected for two reasons: it would not help the
case that motivated decoding, since a pasted Obsidian URI carries no Notrios
prefix; and it would add a second link spelling to the 18 non-test files that
interpret a target or an anchor. Joplin needs `:/` because a bare 32-hex string
is ambiguous with a title — Notrios answered that question in the MVP with
`document://`, and already converts Joplin's prefix at the import boundary
rather than adopting it.

## Why anchors and not targets

The failure modes are not symmetric. A wrong decode in a target opens the
**wrong note**, silently. A wrong decode in an anchor lands in the **right
note** at the top, and `notriosctl lint` reports the anchor as unresolved. The
first deserves an explicit marker; the second does not justify one.

## Details worth recording

**Invalid escapes stay literal.** `%zz`, a trailing `%`, `50%off` are returned
unchanged. A heading with a real percent sign is likelier than a typo in an
escape, and silently deleting bytes would break a link in a way nobody could
see. `%%20` decodes to `% ` — the first `%` is not a valid escape and stays, the
second decodes — which is what every lenient decoder does, `urllib` included.

**`+` is not a space.** That is form encoding, not fragment encoding; treating
it as a space would break every heading containing a plus.

**`Parse` keeps the anchor as written**, so a link round-trips byte for byte and
the reply echoes what was asked. Decoding is a resolution-time reading, applied
in three places: stable-link resolution, the lint heading check, and backlink
counting, so both spellings of an anchor count against one heading.

**Block anchors were left in SQL.** The lint block-anchor check compares markers
and block IDs inside the single link scan's SQL, and it is not decoded: Obsidian
block IDs and Notrios markers are Latin letters, digits, and dashes, so nothing
in that charset ever needs escaping. Moving that comparison into Go for a
pathological input was not worth the cost, and it is recorded here rather than
left as a silent asymmetry.

## A test corrected me

My first test asserted `%%20` should be returned unchanged. The implementation
decoded it to `% `, and the implementation was right — `urllib.parse.unquote`
agrees. The test now documents the per-sequence rule instead of asserting a
tidier one that no decoder follows.

## Validation

- `internal/stablelink`: scheme recognition including case folding and rejection
  of `https://`/`mailto:`/bare targets; decoding of spaces, `%25`, UTF-8, and
  lowercase hex; invalid escapes left literal; `+` preserved; identifiers still
  refusing escapes while anchors accept them; byte-for-byte round trip;
- `internal/store`: an escaped anchor resolving through a `notrios://` link
  while the same bytes in a bare Markdown anchor do not, with lint reporting
  exactly the bare one; a decoded URI anchor counting as a backlink; a heading
  containing a literal percent sign resolving by text and by `%25`;
- `go vet ./...`, `go test ./...`, `npm test`, required-file, scaffold, docs
  site, `make gui`, smoke, and performance smoke.

## Next task

E4, graph traversal and paths, requires user approval.
