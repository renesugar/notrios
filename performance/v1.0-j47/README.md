# v1.0 J47: render a working wiki embed in the preview

The plan is in `PLAN.md` under J47, with the details settled before the work on
2026-10-21.

## J47-A: a wiki embed of an attachment is its Markdown embed

`web/src/preview-links.ts`'s wiki rule, for an embed whose target is a
`resource://` URI, now pushes the token markdown-it's own image rule pushes:
`image`, with `src`, an empty `alt` attribute, the text after the pipe parsed as
its children, and the source text the marks are keyed by. Everything after it,
the renderer and `normalizePreviewHTML`'s media rule, is the Markdown embed's
path, by construction. Any other embed stays the text it was.

`web/src/__tests__/j47-wiki-embeds.test.tsx` holds it to that. It renders each
wiki form beside its Markdown form in the real preview pane and compares every
attribute of the two images:

| wiki embed | Markdown embed |
|---|---|
| `![[resource://default/resources/res_a]]` | `![](resource://default/resources/res_a)` |
| `![[resource://default/resources/res_a\|A heron]]` | `![A heron](resource://default/resources/res_a)` |
| `![[resource://default/resources/res_a\|300]]` | `![300](resource://default/resources/res_a)` |
| `![[resource://not-a-resource]]` | `![](resource://not-a-resource)` |

**Other tests.**
- An alt text holding `<img onerror>` makes no element and no handler.
- A note embed, a stable-link embed, a remote URL embed and a name embed each
  stay text and load nothing.
- A broken attachment embed is marked like a link.
- J40-B's test that pinned a working embed as text now pins a note embed as
  text, since an attachment embed is this item's.

**Mutation.** Switching the branch off fails seven tests. Adding a `srcset` to
the token changes nothing, because the shared media rule removes it, which is
the point of sharing it.

## Found while measuring: no attachment image showed in the preview

`run_embed_probe.sh` is J34's probe, adapted. It uploads a real PNG attachment
and embeds it twice, as Markdown and as a wiki embed, beside a remote image, a
remote wiki embed and a note embed. It records what each became and whether
each image loaded. In Chromium, on the code with J47-A:

| embed | became | loaded |
|---|---|---|
| `![markdown attachment](resource://…)` | `img`, `data-app-uri` kept, **no `src`** | **no** |
| `![[resource://…\|wiki attachment]]` | the same | **no** |
| `![markdown remote](http://…)` | `img`, `data-remote-src`, no `src` | no, as J34 intends |
| `![[http://…]]`, `![[document://…]]` | text | — |

No request reached the stand-in remote server, and there was no CSP violation.

**The cause.** `PreviewPane` runs `normalizePreviewHTML` twice: once as
md-editor-rt's `sanitize` prop, and again in its own renderer. The first pass
turns `resource://` into `/api/v1/resources/<id>/content`. The second pass sees
a relative URL, which is neither `resource://`, `data:image/` nor remote, and
removes it. The double pass dates from `c630411` (2026-07-16). The probe shows
the result today. Whether images were also lost before J34 changed the media
rule was not checked.

So J47's rule is correct, and its working state cannot be met: the Markdown
embed it copies shows nothing either. Under J8 the fix is its own item, J48,
decided by the owner on 2026-10-21. J47 closes on top of it, as J39 did on J46.

**Two smaller findings.**
- J34's probe PNG is malformed: its IDAT checksum is wrong and the file is
  truncated. J34 only counted requests, so it never mattered. J47 measures
  whether images load, so its probe generates a valid PNG.
- WebKit no longer launches on this machine. Playwright reports the host is
  missing WebKit's system libraries, and installing them needs root. By the
  owner's decision, the probe runs in Chromium only, and this is why.

## J47-B: the preview's posture is unchanged

J47's probe is J34's adapted, run in Chromium on the UI built with J47-A and
J48. Its output is in `performance/v1.0-j48/AFTER-chromium.json`, and
`BEFORE-chromium.json` beside it shows the same probe before J48.

| embed | became | loaded |
|---|---|---|
| `![markdown attachment](resource://…)` | `img`, `src` the content URL | yes |
| `![[resource://…\|wiki attachment]]` | the same `img`, with the same `src` and `data-app-uri` | yes |
| `![markdown remote](http://…)` | `img` with `data-remote-src`, no `src` | no, as J34 intends |
| `![[http://…]]` | text | — |
| `![[document://…]]` | text | — |

- **Nothing remote.** The stand-in remote server received no request, and there
  was no CSP violation.
- **Nothing beyond Markdown.** A wiki embed loads only what the Markdown embed of
  the same attachment loads, through the same element and the same rule.
- **No new route to a remote image.** A wiki embed of a remote URL is not media
  at all.
- **J34's probe** matches J34's recorded `AFTER.json` field for field
  (`performance/v1.0-j48/J34-PROBE-chromium.json`).
- **The CSP** in `internal/httpapi/server.go` is unchanged. J47 changed no Go
  file.

The unit tests hold the same line without a browser. Each wiki form's image
equals its Markdown form's, and every other embed stays text and makes no
media element (`j47-wiki-embeds.test.tsx`).

WebKit could not be launched on this machine, as recorded above. This was
measured in Chromium only.

**Recorded under J8, not done here:**
- **A note embed stays text.** `![[document://…]]` shows another note inside
  this one in Obsidian, and the preview has no such feature.
- **An attachment that is not an image** shows as a broken image, as its
  Markdown embed does.
- **A name embed stays text even when the editor's check resolves it.** For
  example `![[pic.png]]`, written in Notrios rather than imported. An import
  rewrites every embed it resolves to `resource://`. Rendering a name embed from
  the check would turn text into media after the page is sanitised, which is the
  kind of change J34's rule exists to prevent.

## Archive

`notrios-v1.0-j47-4eaf46f.zip`, built from commit `4eaf46f`, the commit that
completed this item's record, with the ledger not yet flipped. 26,742,413 bytes,
3,024 entries, accepted by `check_release_zip.py`. No usage override was needed:
both the seven-day and five-hour buckets had 83% remaining. The archive also
carries J48, which this item's working state depended on.
