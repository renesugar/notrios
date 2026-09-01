import { chromium } from 'playwright';
import fs from 'node:fs';
const BASE = 'http://127.0.0.1:8099';
const out = {};
const browser = await chromium.launch();

// Test the recommended containment (strict + htmlLabels:false) and, for
// comparison, the 'loose' level the references claim is needed for navigation.
for (const level of ['strict', 'loose']) {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const requests = [];
  page.on('request', r => requests.push(r.url()));
  await page.goto(`${BASE}/index.html`, { waitUntil: 'networkidle' });
  await page.waitForFunction('window.__ready === true', { timeout: 30000 });

  out[level] = {};
  for (const name of ['click_schemes', 'click_blank_target', 'html_anchor_in_label']) {
    const src = fs.readFileSync(`fixtures/${name}.mmd`, 'utf8');
    const r = await page.evaluate(async ([lvl, id, s]) => {
      try {
        const res = await window.__renderLevel(lvl, id, s);
        const holder = document.createElement('div');
        holder.innerHTML = res.svg;
        const links = [...holder.querySelectorAll('a')].map(a => ({
          href: a.getAttribute('href') || a.getAttribute('xlink:href'),
          target: a.getAttribute('target'),
          rel: a.getAttribute('rel'),
        }));
        // Mermaid also attaches click handlers to nodes for `click` directives.
        const clickableNodes = holder.querySelectorAll('.clickable, [class*=clickable]').length;
        return { ok: true, links, clickableNodes,
                 hasForeignObject: holder.querySelectorAll('foreignObject').length,
                 svgSample: res.svg.slice(0, 0) };
      } catch (e) { return { ok: false, error: String(e && e.message || e).slice(0, 160) }; }
    }, [level, `${level}_${name}`, src]);
    out[level][name] = r;
    const hrefs = r.ok ? r.links.map(l => l.href).join(' | ') : `ERROR ${r.error}`;
    console.log(`  [${level}] ${name.padEnd(22)} anchors=${r.ok ? r.links.length : '-'} clickable=${r.ok ? r.clickableNodes : '-'}`);
    if (r.ok && r.links.length) console.log(`      hrefs: ${hrefs}`);
  }
  out[level].cross_origin = requests.filter(u => !u.startsWith(BASE) && !u.startsWith('data:') && !u.startsWith('blob:'));
  out[level].pwned = await page.evaluate(() => window.__pwned);
  await ctx.close();
}
fs.writeFileSync('click-probe-results.json', JSON.stringify(out, null, 2));
await browser.close();
