// J47: open a note in the real UI and record what each embed rendered as,
// whether each image actually loaded, every request to a server standing in
// for "remote", and every CSP violation, in Chromium and in WebKit.
//
//   node embed_probe.mjs <app-url> <remote-origin> <document-id>
//
// PLAYWRIGHT_MODULE names a Playwright module, as performance/v1.0-j34's probe
// does; nothing here is a repository dependency.
const moduleName = process.env.PLAYWRIGHT_MODULE || 'playwright';
const playwright = await import(moduleName);
const [app, remote, documentID] = process.argv.slice(2);
const report = {};
for (const engine of (process.env.J47_ENGINES || 'chromium,webkit').split(',')) {
  const launchOptions = engine === 'chromium' && process.env.CHROME_PATH
    ? { executablePath: process.env.CHROME_PATH, headless: true, args: ['--no-sandbox'] }
    : { headless: true };
  const browser = await playwright[engine].launch(launchOptions);
  try {
    const page = await browser.newPage();
    const requests = [];
    page.on('request', (request) => { if (request.url().startsWith(remote)) requests.push(request.url()); });
    await page.addInitScript(() => {
      window.__j47Csp = [];
      document.addEventListener('securitypolicyviolation', (e) => window.__j47Csp.push(`${e.violatedDirective} ${e.blockedURI}`));
    });
    await page.goto(`${app}/#document=${documentID}`, { waitUntil: 'networkidle' });
    await page.locator('[data-testid="pane-preview"] img').first().waitFor({ timeout: 20000 });
    await page.waitForTimeout(3000);
    const preview = await page.locator('[data-testid="pane-preview"]').evaluate((pane) => {
      const images = [...pane.querySelectorAll('#notrios-preview img')].map((img) => ({
        alt: img.getAttribute('alt'),
        src: (img.getAttribute('src') || '').slice(0, 80) || null,
        data_app_uri: img.getAttribute('data-app-uri'),
        data_remote_src: img.getAttribute('data-remote-src'),
        loaded: img.complete && img.naturalWidth > 0,
      }));
      const text = pane.querySelector('#notrios-preview')?.textContent || '';
      return { images, embeds_left_as_text: (text.match(/!\[\[[^\]]*\]\]/g) || []) };
    });
    report[engine] = {
      remote_requests: [...new Set(requests)].sort(),
      csp_violations: await page.evaluate(() => window.__j47Csp),
      ...preview,
    };
  } finally {
    await browser.close();
  }
}
console.log(JSON.stringify(report, null, 2));
