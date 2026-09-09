# v0.5 E6 — editor measurement, for the CodeMirror decision

Date: 2026-08-06. Generated data only; the profile note is synthetic and no
private corpus, note content, or local path appears here.

Reference machine: Intel Core i5-9300H, Linux/amd64, Chrome 151.0.7922.75
(headless, `--disable-gpu`), Node 26.3.0.

Harness: `scripts/run_editor_profile.sh` builds the web UI, starts a throwaway
`notriosd`, seeds one 206,549-character note, and drives real headless Chrome
through `scripts/measure_editor.mjs` over the DevTools Protocol. The protocol
client is written against Node's built-in `WebSocket`, so measuring the editor
added no dependency.

Keystroke latency is measured the way a person experiences it: the page
timestamps a real `keydown` and timestamps the `MutationObserver` callback for
the change that key caused. That covers the whole path — CodeMirror's
transaction, the decoration field, and the DOM write — rather than the part that
is convenient to time.

## What was being decided

Whether to migrate from `md-editor-rt` to `CodeMirror 6 + unified/remark/rehype`.

The premise was false. `md-editor-rt` 6.5.3 **is** CodeMirror 6 — it depends on
`@codemirror/{view,state,autocomplete,commands,language,search}` 6.x and exposes
them. So the measurement is not "editor A versus editor B"; it is "what do the
three features cost on the editor already shipping", which is the number the
decision actually turns on.

## Keystroke latency and paint

Three runs per arm. "before" is commit `e15aa74` (E5, no in-editor extensions);
"after" adds `[[` autocomplete, broken-link decorations, and Ctrl-click.

| Arm | first-paint | FCP | DOMContentLoaded | keystroke p50 | keystroke p95 |
|---|---:|---:|---:|---:|---:|
| before | 176 ms | 1276 ms | 856 ms | 30.9 ms | 71.4 ms |
| after | 204 ms | 1036 ms | 614 ms | 30.0 ms | 67.4 ms |

Medians of three. The individual runs are in the committed JSON.

**Typing is indistinguishable between the arms**, and the honest reason to say
so rather than to claim an improvement is the spread *within* each arm: first
paint alone ranged 160–528 ms across three identical "before" runs. The
between-arm differences are smaller than that. The correct reading is that the
extensions cost nothing measurable — not that they made anything faster.

The paint numbers are reported because the task asked for them, with the same
caveat: on a loaded laptop running a headless browser they measure the machine
at least as much as the application.

## Bundle

| | eager index chunk | gzip |
|---|---:|---:|
| before | 749.61 kB | 259.64 kB |
| after | 752.97 kB | 260.94 kB |

**+3.36 kB raw, +1.30 kB gzipped** for in-editor autocomplete, decorations, and
Ctrl-click — because CodeMirror was already in the bundle.

Declaring `@codemirror/{autocomplete,state,view}` as direct dependencies, pinned
to the versions already installed transitively, produced a **byte-identical
build** (same content hash) and a single copy of each package on disk. That
check matters: two copies of `@codemirror/state` break CodeMirror at runtime, so
"did declaring it duplicate anything" is a correctness question and not only a
size one. A duplicate would have added roughly 350 kB; the measured delta was
3.36 kB.

## What migrating away would actually buy

The whole `dist` tree:

| | files | raw | gzip |
|---|---:|---:|---:|
| eager index chunk | 1 | 752,976 | 254,495 |
| css | 1 | 78,291 | 14,490 |
| lazy chunks | 113 | 1,321,701 | 480,340 |
| total | 116 | 2,153,435 | 749,614 |

Those 113 lazy chunks are CodeMirror language modes, pulled in by
`@codemirror/language-data` for code-block syntax highlighting: `clojure`,
`fortran`, `vhdl`, `mumps`, and a hundred more. They are **61% of the
distribution by size and 97% of it by file count**, and a note editor rarely
meets them.

They are lazily loaded, so they cost distribution size rather than first paint.
That is the entire remaining case for owning the editor outright, and it is not
enough against re-implementing the preview renderer, the sanitizer, the toolbar,
image upload, and theming — all of which work today. If distribution size ever
becomes a real constraint, trimming `language-data` is the thing to try first,
not replacing the editor.

## What was not measured

**Behaviour inside the Wails webview.** The task asked for it and it was not
measured. The GUI runs WebKitGTK, which does not speak the DevTools Protocol
this harness uses, and no WebKit remote-inspection tooling is installed here.
What is verified is that `make gui` builds with the extensions and that the
webview loads the identical frontend bundle through the service handler
(`UI_DESIGN.md`), so the code path is the same one measured in Chrome — but the
engine is not, and no number here describes it.

**A second Markdown pipeline.** `unified`/`remark`/`rehype` were not
prototyped. They belong to the *preview*, not to the editor, and the preview was
never the thing E5 could not do. Worth recording separately: `remark-gfm`,
`remark-math`, `rehype-katex`, and `rehype-sanitize` are declared in
`web/package.json` and imported nowhere — dead dependencies, presumably added in
anticipation of exactly this migration.

## Reproducing

```bash
bash scripts/run_editor_profile.sh /tmp/editor.json 200
```

Committed JSON: `editor-before{,-2,-3}.json`, `editor-after{,-2,-3}.json`.
