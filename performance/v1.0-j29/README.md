# v1.0 J29: the HTML reference forms the remote-media scanner covers

J8 found that `ScanBody` found Markdown references and quoted `<img src="…">`,
but not an unquoted `<img src=…>` or a `<video src="…">` (finding J8-F3). A
reference the scan never returns is never evaluated by any later stage.

## Owner decisions (2026-09-17)

- `<video>` and the other media elements are in scope.
- Inline images are not remote. The owner named the form:
  `!\[.*?\]\(data:image\/(?:png|jpeg|jpg|gif|webp|svg\+xml);base64,[A-Za-z0-9+/=]+\)`.
  Inline SVG is fine.

## What changed

`internal/media/htmlrefs.go` replaces the single `img` pattern.
- **Finding tags.** A pattern finds where each covered element's start tag
  begins, and requires the name to end there, so `<imgx>` is not `<img>`.
- **Reading attributes.** A small reader follows HTML's attribute rules for
  that one tag: double-quoted, single-quoted and unquoted values, values across
  lines, a `>` inside quotes, and the first value when an attribute repeats.
  Values are entity-decoded with the standard `html` package.
- **Covered elements and attributes:**
  - `img`: `src`, `srcset`;
  - `source`: `src`, `srcset`;
  - `video`: `src`, `poster`;
  - `audio`: `src`;
  - `track`: `src`;
  - `embed`: `src`;
  - `object`: `data`;
  - SVG `image`: `href`, `xlink:href`.

  Each `srcset` candidate is reported separately.
- **Why not an HTML parser.** `golang.org/x/net/html` is only an indirect
  dependency. Its tokenizer's raw-text states (`<textarea>`, `<plaintext>`)
  would swallow the rest of a Markdown note in which such a tag appears as
  prose, so it could miss references a renderer loads.
- **Inline images.** `ScanBody` no longer reports a URI matching the owner's
  form (`data:image/(png|jpeg|jpg|gif|webp|svg+xml);base64,<base64>`), in
  Markdown or HTML. Any other `data:` URI, and `file:`, is still reported and
  blocked. `EvaluateURLs` still refuses `data:`.

`SECURITY_AND_MEDIA_POLICY.md` gains "What the scan covers": the table, the
inline-image rule, and what the scan does not promise. That covers CSS,
documents, `link`, `meta`, `input type=image`, script-built sources, HTML a
renderer reads differently, code spans that are reported although they do not
render, and entity-encoded URLs that are reported but not rewritten.

## Proof

`internal/media/j29_scan_forms_test.go`:

| test | covers |
|---|---|
| `TestJ29EveryCoveredElementAndAttributeIsFound` | 17 bodies: unquoted, single-quoted, upper case, `>` inside a quoted value, across lines, entity-decoded, and every element and attribute in the table |
| `TestJ29SrcsetCandidatesAreEachFound` | width and density descriptors, with and without a space after the comma |
| `TestJ29ReferencesAreEvaluatedAtTheirTagsLine` | multi-line tag, reported at the tag's line, with a blocked `poster` evaluated like any reference |
| `TestJ29UncoveredAndNonMatchingFormsAreNotReported` | `iframe`, CSS `url()`, `link`, `a`, `<imgx>`, `data-src`, a relative path |
| `TestJ29InlineDataURIsAreNotRemote` | base64 PNG, JPEG, JPG, GIF, WebP and SVG images are not reported in Markdown, `img`, SVG `image` or `poster`; `data:text/html`, `data:image/bmp`, non-base64 SVG and a malformed payload are reported blocked; `file:` is reported blocked; `EvaluateURLs` still refuses `data:` |
| `TestJ29MalformedTagsNeitherPanicNorInvent` | unterminated tags and quotes, a stray `=`, a repeated attribute, `<img/src=…>`, an empty `srcset`, a valueless attribute |
| `FuzzJ29HTMLMediaReferences` | offsets always inside the body; a 30-second run made 164,595 executions with no failure |

**On the code before this change:**
- **Covered forms:** 14 of the 17 bodies in the first test were not found. The
  three already found were the quoted `img` forms (single-quoted, upper case,
  across lines).
- **`srcset` and line tests:** both failed.
- **Uncovered forms:** the only failures were two false positives of the old
  pattern, which reported `<imgx src=…>` and `<img data-src=…>`.
- **Inline images:** the test failed, because a Markdown `data:` embed was
  reported blocked.

The existing `TestScanBodyExtraction` passes unchanged.

### J8's evidence

`TestJ8ScanBodyFindsWhatAPreviewWouldFetch` now finds all nine of its
references and logs nothing as unscanned; before, the unquoted `img` and
`video` were J8-F3. It also required `data:image/png;base64,AAAA` to be
reported. That requirement contradicts the owner's direction, so it was removed
with a comment saying why. That is the only edit to J8's files.

## Found and deferred: J34

Designing the scan, a probe of the GUI preview's `normalizePreviewHTML` showed
it keeps remote `video` and `audio` `src`, `video` `poster`, `track` `src` and
SVG `image` `href`. The served CSP blocks remote audio and video through
`default-src 'self'`. `img-src` allows remote images, though, and `poster` and
SVG `image` are image loads. This is recorded as **J34** and not fixed here,
because the sanitizer is not the scanner. Following the owner, inline
`data:image/…` sources and inline SVG must keep rendering there.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry. `validate-scaffold.sh` and
`make g18g-validate` pass.
