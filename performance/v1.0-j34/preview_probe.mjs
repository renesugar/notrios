// J34: open a note in the real UI and record every request the preview makes
// to a server standing in for "remote", in Chromium and in WebKit.
//
//   node preview_probe.mjs <app-url> <remote-origin> <document-id>
//
// PLAYWRIGHT_MODULE names a Playwright module, as performance/v0.7-g18g's
// browser smoke does; nothing here is a repository dependency.
const moduleName = process.env.PLAYWRIGHT_MODULE || 'playwright';
const playwright = await import(moduleName);
const [app, remote, documentID] = process.argv.slice(2);
const report = {};
for (const engine of (process.env.J34_ENGINES || 'chromium,webkit').split(',')) {
  const launchOptions = engine === 'chromium' && process.env.CHROME_PATH
    ? { executablePath: process.env.CHROME_PATH, headless: true, args: ['--no-sandbox'] }
    : { headless: true };
  const browser = await playwright[engine].launch(launchOptions);
  try {
    const page = await browser.newPage();
    const requests = [];
    page.on('request', (request) => { if (request.url().startsWith(remote)) requests.push(request.url()); });
    await page.addInitScript(() => {
      window.__j34Csp = [];
      document.addEventListener('securitypolicyviolation', (e) => window.__j34Csp.push(`${e.violatedDirective} ${e.blockedURI}`));
    });
    await page.goto(`${app}/#document=${documentID}`, { waitUntil: 'networkidle' });
    await page.locator('[data-testid="pane-preview"] img, [data-testid="pane-preview"] svg').first().waitFor({ timeout: 20000 });
    await page.waitForTimeout(3000);
    const preview = await page.locator('[data-testid="pane-preview"]').evaluate((pane) => {
      const out = [];
      pane.querySelectorAll('img, video, audio, source, track, embed, object, image, svg').forEach((el) => {
        const attrs = {};
        for (const a of el.attributes) if (/^(src|srcset|poster|data|href|xlink:href|data-remote-[a-z-]+)$/.test(a.name)) attrs[a.name] = a.value.slice(0, 80);
        out.push({ tag: el.tagName.toLowerCase(), ...attrs });
      });
      const inlineImages = [...pane.querySelectorAll('img')].filter((img) => (img.getAttribute('src') || '').startsWith('data:image/'));
      return { elements: out, inline_images_loaded: inlineImages.map((img) => img.complete && img.naturalWidth > 0) };
    });
    report[engine] = {
      remote_requests: [...new Set(requests)].sort(),
      csp_violations: await page.evaluate(() => window.__j34Csp),
      ...preview,
    };
  } finally {
    await browser.close();
  }
}
console.log(JSON.stringify(report, null, 2));
