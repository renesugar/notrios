import { chromium } from 'playwright';
import fs from 'node:fs';
const BASE = 'http://127.0.0.1:8099';
const browser = await chromium.launch();
const context = await browser.newContext();
const page = await context.newPage();
let bytes = 0; const seen = [];
page.on('response', async r => {
  try { const b = await r.body(); bytes += b.length; seen.push([r.url().replace(BASE,''), b.length]); } catch {}
});
await page.goto(`${BASE}/index.html`, { waitUntil: 'networkidle' });
await page.waitForFunction('window.__ready === true', { timeout: 30000 });
const afterLoad = bytes;
const src = fs.readFileSync('fixtures/flowchart_basic.mmd', 'utf8');
await page.evaluate(s => window.__renderDiagram('m1', s), src);
await page.waitForTimeout(500);
console.log(`  bytes to load the page/module graph: ${(afterLoad/1024).toFixed(0)} KiB`);
console.log(`  bytes after rendering one flowchart:  ${(bytes/1024).toFixed(0)} KiB`);
console.log(`  chunks fetched: ${seen.length}`);
console.log('  largest:');
seen.sort((a,b)=>b[1]-a[1]).slice(0,5).forEach(([u,n]) => console.log(`    ${(n/1024).toFixed(0).padStart(5)} KiB  ${u}`));
await browser.close();
