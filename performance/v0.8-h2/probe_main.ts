// Exercises the production mermaid-render module in a real browser, which is
// the only place mermaid can run: jsdom lacks the layout APIs it needs.
import { renderDiagram, renderMermaidBlocks, MERMAID_LIMITS } from './mermaid-render';

(window as unknown as Record<string, unknown>).__probe = {
  renderDiagram,
  renderMermaidBlocks,
  MERMAID_LIMITS,
};
(window as unknown as Record<string, unknown>).__ready = true;
