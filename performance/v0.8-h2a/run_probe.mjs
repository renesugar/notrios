import { chromium } from 'playwright';
import fs from 'node:fs';

const BASE = 'http://127.0.0.1:8099';
const DEADLINE_MS = 2000;   // G18 contract bound
const fixtures = fs.readdirSync('fixtures').filter(f => f.endsWith('.mmd'));

const browser = await chromium.launch();
const context = await browser.newContext();
const page = await context.newPage();

const cspViolations = [];
const consoleErrors = [];
const pageErrors = [];
const requests = [];

page.on('console', m => {
  const text = m.text();
  if (/Content Security Policy|Refused to/i.test(text)) cspViolations.push(text);
  else if (m.type() === 'error') consoleErrors.push(text);
});
page.on('pageerror', e => pageErrors.push(String(e && e.message || e)));
page.on('request', r => requests.push(r.url()));

await page.goto(`${BASE}/index.html`, { waitUntil: 'networkidle' });
await page.waitForFunction('window.__ready === true', { timeout: 30000 });

// Bytes actually transferred to reach a first render.
const beforeFirst = requests.length;
const results = {};
for (const file of fixtures) {
  const id = file.replace(/\.mmd$/, '');
  const source = fs.readFileSync(`fixtures/${file}`, 'utf8');
  const outcome = await page.evaluate(async ([id, src, deadline]) => {
    const started = performance.now();
    const race = await Promise.race([
      window.__renderDiagram('d_' + id.replace(/[^a-z0-9]/gi, ''), src),
      new Promise(r => setTimeout(() => r({ ok: false, timedOut: true }), deadline)),
    ]);
    return { ...race, wall: performance.now() - started };
  }, [id, source, DEADLINE_MS]);

  // Inspect any produced SVG for dangerous constructs.
  let audit = null;
  if (outcome.ok && outcome.svg) {
    audit = await page.evaluate((svg) => {
      const holder = document.createElement('div');
      holder.innerHTML = svg;
      const q = s => holder.querySelectorAll(s).length;
      const attrs = [];
      holder.querySelectorAll('*').forEach(el => {
        for (const a of el.attributes) if (/^on/i.test(a.name)) attrs.push(a.name);
      });
      const hrefs = [];
      holder.querySelectorAll('[href], [xlink\\:href], a').forEach(el => {
        const v = el.getAttribute('href') || el.getAttribute('xlink:href');
        if (v) hrefs.push(v);
      });
      return {
        scripts: q('script'), iframes: q('iframe'), foreignObjects: q('foreignObject'),
        eventHandlerAttrs: attrs, hrefs,
        remoteImages: [...holder.querySelectorAll('image, img')]
          .map(el => el.getAttribute('href') || el.getAttribute('xlink:href') || el.getAttribute('src'))
          .filter(v => v && /^https?:/i.test(v)),
      };
    }, outcome.svg);
  }
  results[id] = {
    rendered: !!outcome.ok, timed_out: !!outcome.timedOut,
    ms: Math.round(outcome.wall), svg_bytes: outcome.bytes || 0,
    error: outcome.error ? String(outcome.error).slice(0, 200) : null,
    audit,
  };
  console.log(`  ${id.padEnd(28)} rendered=${!!outcome.ok} ms=${Math.round(outcome.wall)} ${outcome.timedOut ? 'TIMED_OUT' : ''}`);
}

const pwned = await page.evaluate(() => ({ a: window.__pwned }));
const crossOrigin = requests.filter(u => !u.startsWith(BASE) && !u.startsWith('data:') && !u.startsWith('blob:'));

const report = {
  csp_violations: cspViolations,
  console_errors: consoleErrors,
  page_errors: pageErrors,
  total_requests: requests.length,
  cross_origin_requests: crossOrigin,
  script_execution_detected: pwned.a !== undefined,
  fixtures: results,
};
fs.writeFileSync('probe-results.json', JSON.stringify(report, null, 2));
console.log('\n  CSP violations:', cspViolations.length);
console.log('  cross-origin requests:', crossOrigin.length);
console.log('  injected script executed:', pwned.a !== undefined);
await browser.close();
