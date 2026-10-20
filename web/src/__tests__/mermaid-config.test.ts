import { describe, expect, it, vi } from 'vitest';

// mermaid cannot run under jsdom, so the module is replaced and what it is
// configured with is read back: the configuration is the containment H2a and
// J45 measured in a browser, and this pins that it is the one shipped.
const initialize = vi.fn();
vi.mock('mermaid', () => ({
  default: {
    initialize,
    render: vi.fn(async () => ({ svg: '<svg xmlns="http://www.w3.org/2000/svg"></svg>' })),
  },
}));

import { renderDiagram, resetMermaidForTests } from '../mermaid-render';

describe('mermaid configuration', () => {
  it('keeps the measured containment and lays out without ELK (v1.0 J45)', async () => {
    resetMermaidForTests();
    await renderDiagram('flowchart TD\n  A --> B', 'diagram-config');
    expect(initialize).toHaveBeenCalledTimes(1);
    expect(initialize.mock.calls[0][0]).toMatchObject({
      startOnLoad: false,
      securityLevel: 'strict',
      htmlLabels: false,
      flowchart: { htmlLabels: false },
      class: { htmlLabels: false },
      suppressErrorRendering: true,
      // mermaid 12's default is ELK, which is not shipped (src/elk-refused.ts).
      layout: 'dagre',
      state: { layout: 'dagre' },
    });
  });
});
