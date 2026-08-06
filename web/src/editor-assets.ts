// Every asset the editor would otherwise fetch from a CDN, supplied locally.
//
// `md-editor-rt` does not bundle its optional libraries. Its default config
// points at `https://unpkg.com/...` and injects `<script>`/`<link>` tags at
// runtime, so loading a single note issued thirteen requests to a third-party
// CDN — KaTeX and three fonts, highlight.js and a theme, echarts, cropperjs,
// and prettier twice.
//
// That is not a size question, it is a correctness and privacy one. Notrios
// binds to loopback, quarantines remote *images* behind a media policy, and
// refuses to fetch a byte during a static media scan — while the UI loaded
// remote *executable JavaScript* unconditionally, in the desktop app too. And
// with the CDN unreachable the failure was silent: `$E = mc^2$` rendered as its
// own LaTeX source, with no error and no placeholder.
//
// The rule this file applies is simple: **anything the editor would fetch is
// either bundled or turned off.** Supplying an `instance` is what stops the
// injection — md-editor-rt skips both the script and the stylesheet when one is
// configured, which is why the CSS is imported here explicitly.
import Cropper from 'cropperjs';
import hljs from 'highlight.js/lib/common';
import katex from 'katex';
import { config } from 'md-editor-rt';

import 'katex/dist/katex.min.css';
import 'highlight.js/styles/github.css';
import 'cropperjs/dist/cropper.css';

/**
 * The extensions with no local instance, disabled at the call site through
 * `noEcharts` / `noPrettier`. Exported so the editor and the preview cannot
 * drift apart on which ones are off.
 */
export const disabledEditorExtensions = {
  // Charts rendered from note content. Notrios has no such feature, and a
  // charting library that evaluates a code block's contents is not something to
  // carry for a feature nobody asked for.
  noEcharts: true,
  // Markdown reformatting. Notrios stores exactly what the author wrote; block
  // identity is content-derived, so a reformat would remint every anchor in the
  // note (`PROJECT_DECISIONS.md` 17).
  noPrettier: true,
  // Already off before this change.
  noMermaid: true,
} as const;

let installed = false;

/**
 * Points md-editor-rt at the bundled libraries. Safe to call more than once;
 * the editor and preview panes both call it so neither depends on import order.
 *
 * `highlight.js/lib/common` is the ~40-language subset rather than the full
 * ~190. A note app's fenced blocks are not an argument for carrying Nix and
 * Prolog grammars, and an unlisted language degrades to plain text rather than
 * to an error.
 */
export function installEditorAssets(): void {
  if (installed) return;
  installed = true;
  config({
    editorExtensions: {
      katex: { instance: katex },
      highlight: { instance: hljs },
      // Cropper backs the image-upload dialog's crop tab. It is bundled rather
      // than disabled because disabling it would need `noUploadImg`, which
      // takes image upload with it — trading one working feature for another
      // is not what "work offline" was supposed to mean.
      cropper: { instance: Cropper },
    },
  });
}
