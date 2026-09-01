import { chromium } from 'playwright';
import fs from 'node:fs';
const BASE = 'http://127.0.0.1:8099';
const out = {};
const browser = await chromium.launch();

// 1. Genuine slow render against the 2000 ms contract deadline.
{
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${BASE}/index.html`, { waitUntil: 'networkidle' });
  await page.waitForFunction('window.__ready === true', { timeout: 30000 });
  const src = fs.readFileSync('fixtures/slow_dense_graph.mmd', 'utf8');
  const r = await page.evaluate(async (s) => {
    const t = performance.now();
    try { const { svg } = await window.__renderDiagram('slow1', s); return { ok:true, ms: performance.now()-t, bytes: svg.length }; }
    catch (e) { return { ok:false, ms: performance.now()-t, error:String(e) }; }
  }, src);
  out.slow_dense_graph = { ...r, ms: Math.round(r.ms), exceeds_2000ms_deadline: r.ms > 2000 };
  console.log(`  slow_dense_graph: rendered=${r.ok} ms=${Math.round(r.ms)} exceeds2s=${r.ms>2000}`);
  await ctx.close();
}

// 2. Both themes.
for (const theme of ['default', 'dark']) {
  const ctx = await browser.newContext({ colorScheme: theme === 'dark' ? 'dark' : 'light' });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/index.html`, { waitUntil: 'networkidle' });
  await page.waitForFunction('window.__ready === true', { timeout: 30000 });
  const r = await page.evaluate(async ([t, s]) => {
    const m = (await import('/assets/' + [...document.querySelectorAll('script[type=module]')].length)).default;
    return null;
  }, [theme, '']).catch(() => null);
  const themed = await page.evaluate(async ([t, s]) => {
    // Re-initialize with the theme under test, then render.
    const started = performance.now();
    try {
      const res = await window.__renderThemed(t, 'th_' + t, s);
      return { ok: true, ms: performance.now() - started, bytes: res.svg.length, hasDarkFill: /#1f2020|#333|rgb\(/i.test(res.svg) };
    } catch (e) { return { ok: false, error: String(e) }; }
  }, [theme, fs.readFileSync('fixtures/flowchart_basic.mmd', 'utf8')]);
  out[`theme_${theme}`] = themed;
  console.log(`  theme ${theme}: ${JSON.stringify(themed).slice(0, 120)}`);
  await ctx.close();
}

// 3. Narrow layout.
{
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/index.html`, { waitUntil: 'networkidle' });
  await page.waitForFunction('window.__ready === true', { timeout: 30000 });
  const r = await page.evaluate(async (s) => {
    const { svg } = await window.__renderDiagram('narrow', s);
    const holder = document.createElement('div');
    holder.style.width = '390px';
    holder.innerHTML = svg;
    document.body.appendChild(holder);
    const el = holder.querySelector('svg');
    const box = el.getBoundingClientRect();
    return { svgWidthAttr: el.getAttribute('width'), style: el.getAttribute('style'),
             renderedWidth: Math.round(box.width), overflowsViewport: box.width > 390 };
  }, fs.readFileSync('fixtures/flowchart_basic.mmd', 'utf8'));
  out.narrow_390 = r;
  console.log(`  narrow 390px: renderedWidth=${r.renderedWidth} overflows=${r.overflowsViewport} widthAttr=${r.svgWidthAttr}`);
  await ctx.close();
}

fs.writeFileSync('probe2-results.json', JSON.stringify(out, null, 2));
await browser.close();
