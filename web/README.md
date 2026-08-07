# Notrios web interface

The React + Vite frontend that the desktop GUI renders and the service serves at
`/`. `bin/notrios` is a webview around this; there is no second implementation.

## Running it while you work

Two terminals. The service on one:

```sh
make serve          # from the repository root: runs the service on :8080
```

and Vite on the other:

```sh
cd web
npm install         # or `make deps` from the root, which uses the lockfile
npm run dev
```

Vite proxies `/api` and `/healthz` to `127.0.0.1:8080`, so the dev server gives
you hot reload against a real service and a real database.

For the production build — the one the service and the desktop binary actually
serve — use `make web` from the repository root, which writes `web/dist/`. Where
a binary looks for that directory is documented in
[the installation guide](../docs/installation.md#where-the-interface-files-have-to-be).

## Checks

```sh
npm run typecheck   # tsc --noEmit
npm test            # vitest + Testing Library, jsdom
npm run build       # typecheck then production build
```

`make validate` from the repository root runs the Go side; the frontend checks
above are separate and both run in CI.

## What lives where

| Path | Holds |
|---|---|
| `src/App.tsx` | the workspace shell: panes, search, selection, note operations |
| `src/components/` | the four panes, splitters, link intelligence, the notebook picker |
| `src/api.ts` | every REST call, and the only place `fetch` appears |
| `src/editor-*.ts` | CodeMirror extensions, byte↔index offset conversion, bundled editor assets |
| `src/sidebar.ts` | deterministic sidebar composition and notebook targeting |
| `src/panes.ts` | pane widths, splitter arithmetic, persistence |
| `src/themes.ts` | theme tokens and light/dark selection |
| `src/__tests__/` | vitest suites |

## Two rules worth knowing before changing things

**Note content is untrusted.** Anything derived from a note body or title is
written with `textContent` or as an element attribute, never concatenated into
HTML. Preview HTML goes through `normalizePreviewHTML` in `preview-utils.tsx`.

**Nothing is fetched at runtime.** KaTeX, highlight.js, and cropper are bundled
locally (`src/editor-assets.ts`) and the service serves a Content-Security-Policy
that refuses third-party script. `scripts/run_offline_assets_check.sh` fails the
build if a remote request reappears.
