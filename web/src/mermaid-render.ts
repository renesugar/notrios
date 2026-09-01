// Rendering ```mermaid blocks in the preview.
//
// md-editor-rt keeps `noMermaid: true`, so the editor never renders a diagram
// itself. Diagrams are rendered here instead, in a post-pass over the already-
// rendered preview, for the same reason `note-query.ts` works that way: this
// file then owns the limits, the sanitisation, the cancellation, and the
// fallback, rather than inheriting whatever a library chose.
//
// The rules come from v0.8 H2a, which measured them:
//
// **The source is what is already on the page.** A fenced block renders as a
// code block first; a diagram replaces it only on success. "Preserve the source
// on failure" is therefore the default state rather than an error path someone
// has to remember to write, and every failure — malformed, oversized, timed
// out, sanitised to nothing, or an internal crash in the renderer — leaves the
// reader with exactly what they typed.
//
// **Mermaid is not trusted to be safe.** It is configured strictly *and* its
// output is sanitised afterwards. H2a found that the default configuration
// emits `foreignObject` and will fetch a remote image named in a label, and
// that a `click` directive's remote href survives. Configuration closes the
// first two; this file closes the third.
//
// **Local note links are the exception that is kept.** A diagram may link to
// one of the user's own notes, because that navigates inside the application
// where the reader can then decide about anything remote. Every other scheme
// loses its href.
import { isStableLink, parseStableLink } from './stable-links';

/**
 * Limits, all enforced before the renderer sees the source.
 *
 * Mermaid enforces its own `maxEdges` (500) and `maxTextSize` (50000) and
 * refuses over-limit input with a clear message, so these lower the ceiling
 * rather than inventing one. They cannot be raised from inside a diagram.
 */
export const MERMAID_LIMITS = {
  /** Diagrams per note. Beyond this the rest stay as source. */
  maxDiagramsPerNote: 20,
  /** Source bytes for one diagram. */
  maxSourceBytes: 65536,
  /** Edges mermaid will lay out before refusing. */
  maxEdges: 500,
  /** Characters mermaid will accept. */
  maxTextSize: 50000,
  /** Wall-clock budget for one diagram before the source is kept instead. */
  renderTimeoutMs: 2000,
} as const;

/** Elements never allowed in a rendered diagram, whatever produced them. */
const FORBIDDEN_ELEMENTS = [
  'script',
  'iframe',
  'object',
  'embed',
  'foreignObject',
  'foreignobject',
  'link',
  'base',
  'meta',
  'animate',
  'set',
  'handler',
];

let mermaidModule: Promise<typeof import('mermaid')> | null = null;

/**
 * Loads mermaid on first use.
 *
 * The import is dynamic so a library that roughly doubles the bundle is not
 * downloaded by a reader who never opens a note containing a diagram.
 */
function loadMermaid(): Promise<typeof import('mermaid')> {
  if (!mermaidModule) {
    mermaidModule = import('mermaid').then((module) => {
      module.default.initialize({
        startOnLoad: false,
        // Strict is the sanitising level. It is not sufficient on its own —
        // see sanitizeDiagramSVG — but it is the right floor.
        securityLevel: 'strict',
        // HTML labels are the vector H2a measured: they emit foreignObject and
        // a label's <img src="https://…"> is actually fetched. SVG text labels
        // cannot do either.
        htmlLabels: false,
        flowchart: { htmlLabels: false },
        class: { htmlLabels: false },
        // A diagram that fails must leave the source visible, not paint
        // mermaid's own error graphic over the block.
        suppressErrorRendering: true,
        maxEdges: MERMAID_LIMITS.maxEdges,
        maxTextSize: MERMAID_LIMITS.maxTextSize,
        theme: 'default',
      });
      return module;
    });
  }
  return mermaidModule;
}

/** Resets the cached module. Tests only. */
export function resetMermaidForTests(): void {
  mermaidModule = null;
}

/**
 * Returns true when a link may keep its href inside a diagram.
 *
 * Only the application's own note links survive. A remote URL is deliberately
 * de-linked rather than followed: the reader reaches it from the note that
 * contains it, where they can see it in context first. `javascript:` and
 * `data:` are refused here as well as by mermaid's own URL sanitiser and by
 * DOMPurify, which is three independent layers for the one that matters most.
 */
function isPermittedDiagramLink(href: string): boolean {
  const value = href.trim();
  if (value === '') return false;
  if (value.startsWith('#')) return true;
  return isStableLink(value);
}

/**
 * Strips everything a diagram may not contain and returns the SVG, or null if
 * nothing usable survives.
 *
 * Returning null rather than an empty string matters: an empty diagram is
 * indistinguishable from a blank area, and the caller has to know to keep the
 * source instead.
 */
export function sanitizeDiagramSVG(
  svg: string,
  options: { diagramID?: string; noteLinks?: Map<string, string> } = {},
): string | null {
  const holder = document.createElement('div');
  holder.innerHTML = svg;

  const root = holder.querySelector('svg');
  if (!root) return null;

  for (const selector of FORBIDDEN_ELEMENTS) {
    root.querySelectorAll(selector).forEach((node) => node.remove());
  }

  // Before any attribute is touched. A media element is judged by where it
  // points, so stripping the href first would leave nothing to judge and the
  // element would survive as an empty shell.
  root.querySelectorAll('image, img, use').forEach((node) => {
    const reference = node.getAttribute('href') || node.getAttribute('xlink:href') || node.getAttribute('src');
    if (!reference) {
      node.remove();
      return;
    }
    if (!reference.startsWith('#') && !isPermittedDiagramLink(reference)) {
      node.remove();
    }
  });

  root.querySelectorAll('*').forEach((element) => {
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase();
      // Event handlers, in any casing or namespace spelling.
      if (name.startsWith('on')) {
        element.removeAttribute(attribute.name);
        continue;
      }
      if (name === 'href' || name === 'xlink:href' || name === 'src') {
        if (!isPermittedDiagramLink(attribute.value)) {
          element.removeAttribute(attribute.name);
          // A link with no destination should not still look like one.
          if (element.tagName.toLowerCase() === 'a') {
            element.removeAttribute('target');
            element.removeAttribute('rel');
          }
        }
        continue;
      }
      // A style attribute can reference a remote URL; the diagram has no need
      // for one and a note is untrusted content.
      if (name === 'style' && /url\s*\(/i.test(attribute.value)) {
        element.removeAttribute(attribute.name);
      }
    }
  });

  // Mermaid emits a <style> element for theming. It is kept because removing it
  // makes every diagram unreadable, but only when it cannot reach the network:
  // a url() or @import inside note-derived CSS would be an exfiltration channel
  // that the img-src allowance in the page CSP would not stop.
  root.querySelectorAll('style').forEach((node) => {
    const css = node.textContent ?? '';
    if (/url\s*\(|@import/i.test(css)) node.remove();
  });

  // After every removal, so a reattached link cannot be undone by the pass that
  // strips destinations, and only ever carries a validated stable link.
  if (options.diagramID && options.noteLinks) {
    attachNoteLinks(root, options.diagramID, options.noteLinks);
  }

  root.setAttribute('role', 'img');
  if (!root.getAttribute('aria-label')) {
    root.setAttribute('aria-label', 'Mermaid diagram');
  }
  // The rendered width is mermaid's; letting it exceed the pane would push the
  // note sideways on a narrow screen.
  root.setAttribute('width', '100%');
  root.removeAttribute('height');

  return root.outerHTML;
}

/**
 * Extracts the note links a diagram declares.
 *
 * Mermaid drops a `notrios://` href before we ever see the SVG: `formatUrl`
 * passes the URL through, but the renderer's own DOM pass keeps only the
 * schemes it recognises, so a remote `https` link survives and the user's own
 * note link does not. That is backwards for this product, and it is why the
 * links are reattached here from the source instead.
 *
 * Only `click <node> "<uri>"` directives are read, and only when the URI is a
 * stable link. Nothing else in the source is interpreted, so there is no way to
 * turn a diagram into an arbitrary link.
 */
export function parseNoteLinks(source: string): Map<string, string> {
  const links = new Map<string, string>();
  const pattern = /^\s*click\s+([A-Za-z0-9_-]+)\s+"([^"]*)"/gm;
  for (const match of source.matchAll(pattern)) {
    const [, node, uri] = match;
    const value = uri.trim();
    // parseStableLink, not isStableLink: the latter only recognises the
    // scheme, and attaching a malformed note link would give the reader a
    // link that fails when they click it.
    if (isStableLink(value) && parseStableLink(value)) links.set(node, value);
  }
  return links;
}

/**
 * Reattaches note links to the nodes that declared them.
 *
 * The link becomes `data-app-uri` with `href="#"`, which is what the preview's
 * click handler already routes for every other in-app link, and what keeps the
 * node reachable by keyboard. It deliberately does not become a real `href`:
 * the application resolves a stable link through the service, because it may
 * name another database.
 */
function attachNoteLinks(root: SVGElement, diagramID: string, links: Map<string, string>): void {
  if (links.size === 0) return;
  root.querySelectorAll('a').forEach((anchor) => {
    const labelled = anchor.querySelector('[id]');
    const id = labelled?.getAttribute('id') ?? '';
    for (const [node, uri] of links) {
      // Anchored on the generated diagram id and the node name, so one node
      // name cannot match another that merely contains it.
      const pattern = new RegExp(`^${diagramID}-[A-Za-z0-9]+-${node}-\\d+$`);
      if (!pattern.test(id)) continue;
      anchor.setAttribute('data-app-uri', uri);
      anchor.setAttribute('href', '#');
      return;
    }
  });
}

/** One diagram's outcome, exported so the caller can report it. */
export interface DiagramOutcome {
  rendered: boolean;
  reason?: 'too-many' | 'too-large' | 'timeout' | 'invalid' | 'empty';
}

/**
 * Produces raw SVG for a diagram source, or throws.
 *
 * Injectable because mermaid cannot run under jsdom — it needs layout APIs no
 * DOM shim provides — so the block-handling logic here would otherwise be
 * testable only through a renderer that fails on every input, which would make
 * every fallback test pass for the wrong reason. Real rendering is verified in
 * a browser instead; see performance/v0.8-h2/.
 */
export type DiagramRenderer = (id: string, source: string) => Promise<string>;

const mermaidRenderer: DiagramRenderer = async (id, source) => {
  const mermaid = await loadMermaid();
  const { svg } = await mermaid.default.render(id, source);
  return svg;
};

/** Renders one source string, or explains why it did not. */
export async function renderDiagram(
  source: string,
  id: string,
  timeoutMs: number = MERMAID_LIMITS.renderTimeoutMs,
  renderer: DiagramRenderer = mermaidRenderer,
): Promise<{ svg: string | null; outcome: DiagramOutcome }> {
  if (source.length > MERMAID_LIMITS.maxSourceBytes) {
    return { svg: null, outcome: { rendered: false, reason: 'too-large' } };
  }

  let timer: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<'timeout'>((resolve) => {
    timer = setTimeout(() => resolve('timeout'), timeoutMs);
  });

  try {
    // Every throw is caught, not only the ones mermaid documents: H2a saw it
    // fail with an internal TypeError on a dense graph, and a renderer crash
    // must not take the note's preview with it.
    const attempt = renderer(id, source).catch(() => null);

    const outcome = await Promise.race([attempt, deadline]);
    if (outcome === 'timeout') {
      return { svg: null, outcome: { rendered: false, reason: 'timeout' } };
    }
    if (!outcome) {
      return { svg: null, outcome: { rendered: false, reason: 'invalid' } };
    }
    const safe = sanitizeDiagramSVG(outcome, {
      diagramID: id,
      noteLinks: parseNoteLinks(source),
    });
    if (!safe) {
      return { svg: null, outcome: { rendered: false, reason: 'empty' } };
    }
    return { svg: safe, outcome: { rendered: true } };
  } catch {
    return { svg: null, outcome: { rendered: false, reason: 'invalid' } };
  } finally {
    if (timer !== undefined) clearTimeout(timer);
  }
}

/**
 * Finds the fenced Mermaid blocks in a rendered preview.
 *
 * md-editor-rt marks a fenced block's language on the `<code>` element. The
 * `<pre>` is what gets replaced, so the whole block goes at once.
 */
function findDiagramBlocks(container: HTMLElement): HTMLPreElement[] {
  const blocks: HTMLPreElement[] = [];
  container.querySelectorAll('code').forEach((code) => {
    const language = (code.getAttribute('class') || '').toLowerCase();
    if (!/(^|\s)language-mermaid(\s|$)/.test(language)) return;
    const pre = code.closest('pre');
    if (pre instanceof HTMLPreElement && !pre.hasAttribute('data-mermaid-state')) {
      blocks.push(pre);
    }
  });
  return blocks;
}

/** Wraps a rendered diagram, keeping the source reachable beneath it. */
function diagramFigure(svg: string, source: string): HTMLElement {
  const figure = document.createElement('figure');
  figure.className = 'mermaid-diagram';
  figure.setAttribute('data-mermaid-state', 'rendered');

  const holder = document.createElement('div');
  holder.className = 'mermaid-diagram-svg';
  // Sanitised above: every forbidden element and attribute is already gone.
  holder.innerHTML = svg;
  figure.appendChild(holder);

  // The source stays available to anyone who wants it, including a screen
  // reader user for whom the picture is not the content. <details> gives that
  // without a click target that looks like part of the diagram.
  const details = document.createElement('details');
  details.className = 'mermaid-diagram-source';
  const summary = document.createElement('summary');
  summary.textContent = 'Diagram source';
  const pre = document.createElement('pre');
  const code = document.createElement('code');
  // textContent, never innerHTML: this is note content.
  code.textContent = source;
  pre.appendChild(code);
  details.append(summary, pre);
  figure.appendChild(details);
  return figure;
}

/** Marks a block that stayed as source, and says why. */
function markUnrendered(pre: HTMLPreElement, outcome: DiagramOutcome): void {
  pre.setAttribute('data-mermaid-state', outcome.reason ?? 'invalid');
  const existing = pre.parentElement?.querySelector('.mermaid-diagram-note');
  if (existing) return;
  const note = document.createElement('p');
  note.className = 'mermaid-diagram-note';
  note.setAttribute('role', 'status');
  note.textContent = explain(outcome);
  pre.parentElement?.insertBefore(note, pre);
}

function explain(outcome: DiagramOutcome): string {
  switch (outcome.reason) {
    case 'too-many':
      return `Only the first ${MERMAID_LIMITS.maxDiagramsPerNote} diagrams in a note are drawn. The source is shown below.`;
    case 'too-large':
      return 'This diagram is too large to draw. The source is shown below.';
    case 'timeout':
      return 'This diagram took too long to draw. The source is shown below.';
    case 'empty':
      return 'This diagram produced nothing that could be shown safely. The source is shown below.';
    default:
      return 'This diagram could not be drawn. The source is shown below.';
  }
}

/**
 * Renders every Mermaid block in a preview. Returns a cleanup function.
 *
 * Cancellation matters because a reader can move to another note while a
 * diagram is still rendering; a late result must not be written into a preview
 * that has moved on.
 */
export function renderMermaidBlocks(
  container: HTMLElement,
  renderer: DiagramRenderer = mermaidRenderer,
): () => void {
  const blocks = findDiagramBlocks(container);
  let cancelled = false;
  if (blocks.length === 0) return () => { cancelled = true; };

  blocks.forEach((pre, index) => {
    const source = pre.textContent ?? '';
    if (index >= MERMAID_LIMITS.maxDiagramsPerNote) {
      markUnrendered(pre, { rendered: false, reason: 'too-many' });
      return;
    }
    // Claim the block immediately so a re-entrant pass cannot start it twice.
    pre.setAttribute('data-mermaid-state', 'rendering');
    const id = `mermaid-${Date.now().toString(36)}-${index}`;
    renderDiagram(source, id, MERMAID_LIMITS.renderTimeoutMs, renderer)
      .then(({ svg, outcome }) => {
        if (cancelled || !pre.isConnected) return;
        if (!svg) {
          markUnrendered(pre, outcome);
          return;
        }
        pre.replaceWith(diagramFigure(svg, source));
      })
      .catch(() => {
        if (cancelled || !pre.isConnected) return;
        markUnrendered(pre, { rendered: false, reason: 'invalid' });
      });
  });

  return () => {
    cancelled = true;
  };
}
