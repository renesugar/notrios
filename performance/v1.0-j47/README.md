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
