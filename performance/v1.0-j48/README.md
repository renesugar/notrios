# v1.0 J48: show attachments in the preview

The plan is in `PLAN.md` under J48. J47's browser probe found the defect on
2026-10-21, and the owner made it its own item.

## The defect

`PreviewPane` runs `normalizePreviewHTML` twice: once as md-editor-rt's
`sanitize` prop, and again in its own renderer.
- **Attachment embeds.** The first pass turned `resource://` into the local
  content URL, `/api/v1/resources/<id>/content`. The second pass saw a relative
  URL, which was neither `resource://`, `data:image/` nor remote, and removed
  it. No attachment embedded in a note showed in the preview.
- **Attachment links.** A `resource://` link lost its download `href` the same
  way. Its click still worked through `data-app-uri`.
- **Note and stable links.** These were changed too: they gained a
  `rel="noreferrer"` on the second pass.

`BEFORE-chromium.json` is J47's probe on the code before this item. Both
attachment embeds were an `img` with no `src`, and neither loaded.

## J48-A: the sanitiser leaves its own output alone

Both passes stay, so no path into the page goes unsanitised.
`web/src/preview-utils.tsx` now recognises what it writes:
- **Media.** A media attribute records the `resource://` it was resolved from,
  one record per attribute: `data-app-src`, `data-app-poster`,
  `data-app-xlink-href`. One record is not enough, because a video's source and
  poster can name two attachments, and `data-app-uri` keeps only the last one
  written. A later pass keeps the value only when it is exactly the content URL
  that record resolves to.
- **Links.** A link is left as it is when its `href` is exactly what its
  `data-app-uri` resolves to: the download URL for an attachment, and `#` for a
  note or a stable link.

Anything else relative is removed, as before. A note could already write such a
pair in raw HTML, and keeping it loads nothing a `resource://` embed of the same
attachment could not.

`web/src/__tests__/j48-idempotent-sanitiser.test.tsx`, 29 tests:
- **Idempotent.** Sanitising twice gives exactly what sanitising once gives.
  This holds for each of 18 inputs and for all of them as one page. The inputs
  are J34's media corpus with `resource://` on every media attribute, every link
  form the preview routes or refuses, an event handler, a style and a query
  block.
- **Resolutions survive.** Every attachment the sanitiser resolved survives a
  second pass: image, video source and poster, track, audio, SVG image, and the
  download link.
- **Forged pairs are refused.** A pair is refused when its recorded attachment
  names another attachment, another path or another origin, when it uses the
  download form on an image, when it holds a remote URL, or when it is recorded
  on a different attribute. A `javascript:` link is refused whatever app URI it
  claims.
- **The pane itself.** In the real preview pane, a Markdown embed, a wiki embed
  and a link to one attachment carry its content and download URLs.

**12 of the 29 fail on the code before this item.** They are every corpus entry
the second pass changed, the whole-page and survival tests, the exact-pair test
and the pane test. The forged-pair tests pass before and after, as guards that
nothing became loadable. J47's test that an attachment embed carries its content
URL, held back until this item, now passes.

## J48-B: shown in the browser

Both probes ran in Chromium, against the rebuilt UI. WebKit cannot launch on
this machine (J47).
- **J47's probe** (`AFTER-chromium.json`): the Markdown and wiki attachment
  embeds each have `src` set to the attachment's content URL, and **both
  loaded**. The remote image is inert, the remote wiki embed and the note embed
  are text, the stand-in remote server received no request, and there was no
  CSP violation.
- **J34's probe** (`J34-PROBE-chromium.json`): its Chromium result is identical
  to J34's recorded `AFTER.json`: remote requests, CSP violations, elements and
  inline images, field for field.

J34's six unit tests pass unchanged. The CSP (`internal/httpapi/server.go`) is
unchanged, and no Go file changed. The whole web suite passes: 39 files, 362
tests.

`UI_DESIGN.md` says the sanitiser runs twice and leaves its own output alone.
