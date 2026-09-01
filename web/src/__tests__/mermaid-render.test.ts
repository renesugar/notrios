import { beforeEach, describe, expect, it } from 'vitest';
import {
  MERMAID_LIMITS,
  renderDiagram,
  renderMermaidBlocks,
  parseNoteLinks,
  sanitizeDiagramSVG,
  type DiagramRenderer,
} from '../mermaid-render';

/** Builds a preview container holding fenced mermaid blocks. */
function preview(...sources: string[]): HTMLElement {
  const container = document.createElement('div');
  container.innerHTML = sources
    .map((source) => `<pre><code class="language-mermaid">${source
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')}</code></pre>`)
    .join('');
  document.body.appendChild(container);
  return container;
}

beforeEach(() => {
  document.body.replaceChildren();
});

describe('sanitizeDiagramSVG', () => {
  it('removes every element a diagram may not contain', () => {
    // foreignObject is on this list even though mermaid uses it legitimately
    // for HTML labels: H2a disabled those labels precisely so it never appears.
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg">
         <script>window.__pwned = 1</script>
         <iframe src="https://example.invalid"></iframe>
         <foreignObject><b>html</b></foreignObject>
         <object data="x"></object>
         <g><text>safe</text></g>
       </svg>`,
    );
    expect(svg).not.toBeNull();
    for (const forbidden of ['<script', '<iframe', '<foreignObject', '<foreignobject', '<object']) {
      expect(svg, forbidden).not.toContain(forbidden);
    }
    expect(svg).toContain('safe');
  });

  it('removes an embed element', () => {
    // Tested on its own: <embed> is an HTML void element, and inside SVG it
    // makes the parser swallow the rest of the subtree, which would test the
    // parser rather than the sanitizer if it shared a fixture.
    const svg = sanitizeDiagramSVG(`<svg xmlns="http://www.w3.org/2000/svg"><embed src="x"></svg>`);
    expect(svg).not.toContain('<embed');
  });

  it('strips event handlers in any casing', () => {
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><g onclick="x()" ONMOUSEOVER="y()" onLoad="z()"><text>t</text></g></svg>`,
    );
    expect(svg?.toLowerCase()).not.toContain('onclick');
    expect(svg?.toLowerCase()).not.toContain('onmouseover');
    expect(svg?.toLowerCase()).not.toContain('onload');
  });

  it('keeps a note link and drops every other destination', () => {
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg">
         <a href="notrios://databases/db_a/documents/doc_b"><text>note</text></a>
         <a href="https://example.invalid/remote"><text>remote</text></a>
         <a href="javascript:window.__pwned=2"><text>bad</text></a>
         <a href="#section"><text>anchor</text></a>
       </svg>`,
    );
    expect(svg).toContain('notrios://databases/db_a/documents/doc_b');
    expect(svg).toContain('#section');
    // A remote URL is de-linked rather than followed: the reader reaches it
    // from the note that contains it.
    expect(svg).not.toContain('https://example.invalid/remote');
    expect(svg).not.toContain('javascript:');
  });

  it('removes a remote image reference outright', () => {
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.invalid/track.png"/><text>t</text></svg>`,
    );
    expect(svg).not.toContain('example.invalid');
    expect(svg).not.toContain('<image');
  });

  it('drops a style element or attribute that can reach the network', () => {
    const withURL = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><style>.a{background:url(https://example.invalid/x)}</style><text>t</text></svg>`,
    );
    expect(withURL).not.toContain('example.invalid');
    expect(withURL).not.toContain('<style');

    const withImport = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><style>@import "https://example.invalid/x.css";</style><text>t</text></svg>`,
    );
    expect(withImport).not.toContain('@import');

    const attribute = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><g style="fill:url(https://example.invalid/x)"><text>t</text></g></svg>`,
    );
    expect(attribute).not.toContain('example.invalid');
  });

  it('keeps ordinary theming styles, which every diagram needs', () => {
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><style>.node rect{fill:#eee;stroke:#333}</style><text>t</text></svg>`,
    );
    expect(svg).toContain('<style');
    expect(svg).toContain('fill:#eee');
  });

  it('returns null when there is no svg to keep', () => {
    expect(sanitizeDiagramSVG('')).toBeNull();
    expect(sanitizeDiagramSVG('<p>not a diagram</p>')).toBeNull();
  });

  it('labels the diagram and makes it fit a narrow pane', () => {
    const svg = sanitizeDiagramSVG(`<svg xmlns="http://www.w3.org/2000/svg" width="900" height="400"><text>t</text></svg>`);
    expect(svg).toContain('role="img"');
    expect(svg).toContain('aria-label="Mermaid diagram"');
    expect(svg).toContain('width="100%"');
    expect(svg).not.toContain('height="400"');
  });
});

describe('renderDiagram', () => {
  // A stub stands in for mermaid, which cannot run under jsdom. These tests are
  // about this file's decisions — limits, timeouts, sanitisation, fallback —
  // not about mermaid's own rendering, which is verified in a browser.
  const working: DiagramRenderer = async (id) =>
    `<svg xmlns="http://www.w3.org/2000/svg" id="${id}"><g><text>drawn</text></g></svg>`;
  const failing: DiagramRenderer = async () => {
    throw new Error('parse error');
  };
  const crashing: DiagramRenderer = async () => {
    // H2a saw mermaid fail this way on a dense graph, not with a clean error.
    throw new TypeError("Cannot read properties of undefined (reading 'length')");
  };
  const slow: DiagramRenderer = () => new Promise(() => {});
  const emptyOutput: DiagramRenderer = async () => '<p>no svg here</p>';

  it('draws an ordinary diagram and sanitises the result', async () => {
    const { svg, outcome } = await renderDiagram('flowchart TD\n A --> B\n', 'ok1', 2000, working);
    expect(outcome.rendered).toBe(true);
    expect(svg).toContain('drawn');
    // Sanitisation ran: the accessibility and sizing attributes are present.
    expect(svg).toContain('role="img"');
    expect(svg).toContain('width="100%"');
  });

  it('keeps the source when the renderer rejects', async () => {
    const { svg, outcome } = await renderDiagram('bad', 'bad1', 2000, failing);
    expect(svg).toBeNull();
    expect(outcome.reason).toBe('invalid');
  });

  it('survives an internal crash in the renderer', async () => {
    const { svg, outcome } = await renderDiagram('dense', 'crash1', 2000, crashing);
    expect(svg).toBeNull();
    expect(outcome.reason).toBe('invalid');
  });

  it('gives up on a diagram that never finishes', async () => {
    const { svg, outcome } = await renderDiagram('slow', 'slow1', 20, slow);
    expect(svg).toBeNull();
    expect(outcome.reason).toBe('timeout');
  });

  it('keeps the source when nothing survives sanitisation', async () => {
    const { svg, outcome } = await renderDiagram('x', 'empty1', 2000, emptyOutput);
    expect(svg).toBeNull();
    expect(outcome.reason).toBe('empty');
  });

  it('refuses a source over the byte limit without calling the renderer', async () => {
    let called = false;
    const watcher: DiagramRenderer = async () => {
      called = true;
      return '<svg xmlns="http://www.w3.org/2000/svg"/>';
    };
    const huge = `flowchart TD\n  A[${'x'.repeat(MERMAID_LIMITS.maxSourceBytes)}]\n`;
    const { svg, outcome } = await renderDiagram(huge, 'big1', 2000, watcher);
    expect(svg).toBeNull();
    expect(outcome.reason).toBe('too-large');
    expect(called, 'an over-limit source reached the renderer').toBe(false);
  });

  it('strips a dangerous payload the renderer emitted', async () => {
    const hostile: DiagramRenderer = async () =>
      `<svg xmlns="http://www.w3.org/2000/svg"><a href="https://example.invalid/x" onclick="window.__pwned=1"><text>t</text></a></svg>`;
    const { svg, outcome } = await renderDiagram('x', 'hostile1', 2000, hostile);
    expect(outcome.rendered).toBe(true);
    expect(svg?.toLowerCase()).not.toContain('onclick');
    expect(svg).not.toContain('example.invalid');
  });
});

describe('renderMermaidBlocks', () => {
  const working: DiagramRenderer = async (id) =>
    `<svg xmlns="http://www.w3.org/2000/svg" id="${id}"><g><text>drawn</text></g></svg>`;
  const failing: DiagramRenderer = async () => {
    throw new Error('parse error');
  };

  async function settle(): Promise<void> {
    await new Promise((resolve) => setTimeout(resolve, 50));
  }

  it('replaces a rendered block and keeps its source reachable', async () => {
    const container = preview('flowchart TD\n  A[Start] --> B[End]\n');
    renderMermaidBlocks(container, working);
    await settle();

    const figure = container.querySelector('figure.mermaid-diagram');
    expect(figure, 'the diagram did not replace its block').not.toBeNull();
    expect(figure?.querySelector('svg')).not.toBeNull();
    // The source is still there, one disclosure away, and written as text.
    const source = figure?.querySelector('details.mermaid-diagram-source code');
    expect(source?.textContent).toContain('flowchart TD');
  });

  it('leaves a failed block exactly as it was, with an explanation', async () => {
    const container = preview('flowchart TD\n  A[[[[ --> ???\n');
    renderMermaidBlocks(container, failing);
    await settle();

    const pre = container.querySelector('pre');
    expect(pre, 'the source block was removed on failure').not.toBeNull();
    expect(pre?.textContent).toContain('flowchart TD');
    const note = container.querySelector('.mermaid-diagram-note');
    expect(note?.textContent).toContain('could not be drawn');
    expect(note?.getAttribute('role')).toBe('status');
  });

  it('draws only the first N diagrams and says so for the rest', async () => {
    const sources = Array.from({ length: MERMAID_LIMITS.maxDiagramsPerNote + 2 }, () => 'flowchart TD\n  A --> B\n');
    const container = preview(...sources);
    renderMermaidBlocks(container, working);
    await settle();

    const skipped = container.querySelectorAll('[data-mermaid-state="too-many"]');
    expect(skipped.length).toBe(2);
    expect(container.querySelector('.mermaid-diagram-note')?.textContent).toContain('first 20 diagrams');
  });

  it('ignores fenced blocks that are not mermaid', async () => {
    const container = document.createElement('div');
    container.innerHTML = '<pre><code class="language-go">func main() {}</code></pre>';
    document.body.appendChild(container);
    renderMermaidBlocks(container, working);
    await settle();
    expect(container.querySelector('figure.mermaid-diagram')).toBeNull();
    expect(container.querySelector('pre')?.textContent).toContain('func main');
  });

  it('does not write into a preview the reader has left', async () => {
    const container = preview('flowchart TD\n  A[Start] --> B[End]\n');
    const stop = renderMermaidBlocks(container, working);
    stop();
    await settle();
    expect(container.querySelector('figure.mermaid-diagram')).toBeNull();
    expect(container.querySelector('pre')).not.toBeNull();
  });

  it('does not render the same block twice', async () => {
    const container = preview('flowchart TD\n  A --> B\n');
    renderMermaidBlocks(container, working);
    renderMermaidBlocks(container, working);
    await settle();
    expect(container.querySelectorAll('figure.mermaid-diagram').length).toBe(1);
  });
});

describe('note links in diagrams', () => {
  it('reads only click directives naming a stable link', () => {
    const links = parseNoteLinks(
      [
        'flowchart LR',
        '  A --> B',
        '  click A "notrios://databases/db_a/documents/doc_b"',
        '  click B "https://example.invalid/x"',
        '  click C "javascript:alert(1)"',
        '  click D "notrios://malformed"',
      ].join('\n'),
    );
    // Only the well-formed note link survives. A remote URL is deliberately
    // not reattached: the reader reaches it from the note that contains it.
    expect([...links.keys()]).toEqual(['A']);
    expect(links.get('A')).toBe('notrios://databases/db_a/documents/doc_b');
    // "notrios://malformed" is refused: the scheme is right but the shape is
    // not, and a link that fails on click is worse than no link.
    expect(links.has('D')).toBe(false);
  });

  it('reattaches a note link as in-app routing, not a real href', () => {
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><a><g id="d1-flowchart-A-0"><text>note</text></g></a></svg>`,
      { diagramID: 'd1', noteLinks: new Map([['A', 'notrios://databases/db_a/documents/doc_b']]) },
    );
    // data-app-uri is what the preview's click handler routes on; href="#"
    // keeps the node keyboard-reachable without navigating anywhere.
    expect(svg).toContain('data-app-uri="notrios://databases/db_a/documents/doc_b"');
    expect(svg).toContain('href="#"');
  });

  it('does not attach a link to a node that did not declare one', () => {
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><a><g id="d1-flowchart-B-1"><text>other</text></g></a></svg>`,
      { diagramID: 'd1', noteLinks: new Map([['A', 'notrios://databases/db_a/documents/doc_b']]) },
    );
    expect(svg).not.toContain('data-app-uri');
  });

  it('does not let one node name match another that contains it', () => {
    const svg = sanitizeDiagramSVG(
      `<svg xmlns="http://www.w3.org/2000/svg"><a><g id="d1-flowchart-AB-0"><text>ab</text></g></a></svg>`,
      { diagramID: 'd1', noteLinks: new Map([['A', 'notrios://databases/db_a/documents/doc_b']]) },
    );
    expect(svg).not.toContain('data-app-uri');
  });

  it('cannot be used to attach a remote or dangerous destination', () => {
    // Even if a caller passed one, the map is built by parseNoteLinks which
    // only admits stable links; this pins that the two agree.
    for (const hostile of ['https://example.invalid/x', 'javascript:alert(1)', 'data:text/html,x']) {
      const links = parseNoteLinks(`flowchart LR\n  A --> B\n  click A "${hostile}"\n`);
      expect(links.size, hostile).toBe(0);
    }
  });
});
