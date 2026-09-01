// Minimal realistic integration: dynamic import (how a bundle should load a
// renderer this large), strict security level, no startOnLoad.
import mermaid from 'mermaid';

mermaid.initialize({
  startOnLoad: false,
  securityLevel: 'strict',
  // Containment candidate: SVG <text> labels instead of HTML in <foreignObject>.
  htmlLabels: false,
  flowchart: { htmlLabels: false },
  class: { htmlLabels: false },
  theme: 'default',
  suppressErrorRendering: true,
});

window.__renderDiagram = async (id, source) => {
  const started = performance.now();
  try {
    const { svg } = await mermaid.render(id, source);
    return { ok: true, ms: performance.now() - started, bytes: svg.length, svg };
  } catch (error) {
    return { ok: false, ms: performance.now() - started, error: String(error && error.message || error) };
  }
};
window.__ready = true;

window.__renderThemed = async (theme, id, source) => {
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', htmlLabels: false,
                       flowchart: { htmlLabels: false }, theme, suppressErrorRendering: true });
  return mermaid.render(id, source);
};
