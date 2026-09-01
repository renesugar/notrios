import { chromium } from 'playwright';
import fs from 'node:fs';
const BASE = 'http://127.0.0.1:8098';
const browser = await chromium.launch();
const context = await browser.newContext();
const page = await context.newPage();
const csp = [], requests = [], pageErrors = [];
page.on('console', m => { if (/Content Security Policy|Refused to/i.test(m.text())) csp.push(m.text()); });
page.on('pageerror', e => pageErrors.push(String(e?.message || e)));
page.on('request', r => requests.push(r.url()));
await page.goto(`${BASE}/index.html`, { waitUntil: 'networkidle' });
await page.waitForFunction('window.__ready === true', { timeout: 30000 });

const results = {};
const cases = {
  flowchart: 'flowchart TD\n  A[Start] --> B{Choice}\n  B -->|yes| C[Done]\n',
  sequence: 'sequenceDiagram\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi\n',
  malformed: 'flowchart TD\n  A[[[[ --> ???\n',
  html_payload: 'flowchart TD\n  A["<img src=x onerror=window.__pwned=1>"] --> B["<script>window.__pwned=2<\\/script>"]\n',
  remote_image: 'flowchart TD\n  A["<img src=\'https://example.invalid/track.png\'>"]\n',
  note_link: 'flowchart LR\n  A[note] --> B[remote]\n  click A "notrios://databases/db_a/documents/doc_b"\n  click B "https://example.invalid/x"\n',
  over_limit: 'flowchart TD\n' + Array.from({length: 300}, (_, i) => `  N${i} --> N${i+1}\n  N${i} --> N${i+2}\n`).join(''),
};
for (const [name, source] of Object.entries(cases)) {
  results[name] = await page.evaluate(async ([n, s]) => {
    const { svg, outcome } = await window.__probe.renderDiagram(s, 'p_' + n);
    if (!svg) return { rendered: false, reason: outcome.reason };
    const holder = document.createElement('div');
    holder.innerHTML = svg;
    const hrefs = [...holder.querySelectorAll('[href],[xlink\\:href]')]
      .map(el => el.getAttribute('href') || el.getAttribute('xlink:href')).filter(Boolean);
    const appURIs = [...holder.querySelectorAll('[data-app-uri]')].map(el => el.getAttribute('data-app-uri'));
    return {
      rendered: true,
      foreignObjects: holder.querySelectorAll('foreignObject').length,
      scripts: holder.querySelectorAll('script').length,
      hrefs, appURIs,
      hasRole: holder.querySelector('svg')?.getAttribute('role'),
      width: holder.querySelector('svg')?.getAttribute('width'),
    };
  }, [name, source]);
  const r = results[name];
  console.log(`  ${name.padEnd(14)} rendered=${r.rendered}${r.rendered ? ` fo=${r.foreignObjects} scripts=${r.scripts} hrefs=${JSON.stringify(r.hrefs)} app=${JSON.stringify(r.appURIs)}` : ` reason=${r.reason}`}`);
}
const pwned = await page.evaluate(() => window.__pwned);
const crossOrigin = requests.filter(u => !u.startsWith(BASE) && !u.startsWith('data:') && !u.startsWith('blob:'));
const report = { csp_violations: csp, page_errors: pageErrors, cross_origin_requests: crossOrigin,
                 script_execution_detected: pwned !== undefined, cases: results };
fs.writeFileSync('browser-results.json', JSON.stringify(report, null, 2));
console.log(`\n  CSP violations: ${csp.length}   cross-origin: ${crossOrigin.length}   script executed: ${pwned !== undefined}`);
await browser.close();
