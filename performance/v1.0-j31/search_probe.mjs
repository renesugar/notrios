// J31: the refreshed theme still searches. Loads the built site's search page
// under a subpath, runs a query, and reports results, console problems and
// requests that left the origin.
//
//   node search_probe.mjs <base-url> <query>
//
// PLAYWRIGHT_MODULE names a Playwright module, as G18g's browser smoke does.
const moduleName = process.env.PLAYWRIGHT_MODULE || 'playwright';
const { chromium } = await import(moduleName);
const [base, query] = process.argv.slice(2);
const launch = process.env.CHROME_PATH
  ? { executablePath: process.env.CHROME_PATH, headless: true, args: ['--no-sandbox'] }
  : { headless: true };
const browser = await chromium.launch(launch);
try {
  const page = await browser.newPage();
  const problems = [];
  const offOrigin = new Set();
  const origin = new URL(base).origin;
  page.on('console', (m) => { if (['error', 'warning'].includes(m.type())) problems.push(`${m.type()}: ${m.text()}`); });
  page.on('pageerror', (e) => problems.push(`pageerror: ${e.message}`));
  page.on('requestfailed', (r) => problems.push(`failed: ${r.url()}`));
  page.on('request', (r) => { if (!r.url().startsWith(origin) && !r.url().startsWith('data:')) offOrigin.add(r.url()); });
  await page.addInitScript(() => {
    window.__j31Csp = [];
    document.addEventListener('securitypolicyviolation', (e) => window.__j31Csp.push(`${e.violatedDirective} ${e.blockedURI}`));
  });
  await page.goto(`${base}search/?q=${encodeURIComponent(query)}`, { waitUntil: 'networkidle' });
  const results = page.locator('[data-ledger-search-results] a');
  await results.first().waitFor({ timeout: 20000 });
  const hrefs = await results.evaluateAll((nodes) => nodes.map((node) => node.getAttribute('href')));
  const status = (await page.locator('[data-ledger-search-results]').first().textContent() || '').slice(0, 120);
  const report = {
    query,
    results: hrefs.length,
    first_hrefs: hrefs.slice(0, 3),
    hrefs_under_base: hrefs.every((href) => href && href.startsWith(new URL(base).pathname)),
    status_text: status.replace(/\s+/g, ' ').trim(),
    console_problems: problems,
    off_origin_requests: [...offOrigin],
    csp_violations: await page.evaluate(() => window.__j31Csp || []),
  };
  console.log(JSON.stringify(report, null, 2));
  if (!report.results || !report.hrefs_under_base || problems.length || offOrigin.size || report.csp_violations.length) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
