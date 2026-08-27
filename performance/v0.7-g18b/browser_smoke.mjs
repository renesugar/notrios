const moduleName = process.env.PLAYWRIGHT_MODULE || 'playwright';
const { chromium } = await import(moduleName);

const base = process.env.G18B_BASE_URL || 'http://127.0.0.1:18618/notrios/';
const origin = new URL(base).origin;
const routes = [
  'index.html', 'installation.html', 'service.html', 'cli.html',
  'query-language.html', 'selection-planning.html', 'archive-v2.html',
  'stable-links.html', 'publishing.html', 'gui.html', 'import-export.html',
  'operations.html', 'api/rest.html', 'api/mcp.html', 'troubleshooting.html',
];

const browser = await chromium.launch({
  executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome',
  headless: true,
  args: ['--no-sandbox'],
});
const problems = [];
const requests = new Set();

function watch(page, label) {
  page.on('request', request => requests.add(request.url()));
  page.on('console', message => {
    if (['error', 'warning'].includes(message.type())) {
      problems.push(`${label}: ${message.type()}: ${message.text()}`);
    }
  });
  page.on('pageerror', error => problems.push(`${label}: pageerror: ${error.message}`));
}

async function contrast(page, selector) {
  return page.locator(selector).first().evaluate(element => {
    function rgba(value) {
      const canvas = document.createElement('canvas');
      canvas.width = canvas.height = 1;
      const context = canvas.getContext('2d', { willReadFrequently: true });
      context.clearRect(0, 0, 1, 1);
      context.fillStyle = value;
      context.fillRect(0, 0, 1, 1);
      return [...context.getImageData(0, 0, 1, 1).data];
    }
    function luminance(rgb) {
      const values = rgb.slice(0, 3).map(channel => {
        const value = channel / 255;
        return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      });
      return 0.2126 * values[0] + 0.7152 * values[1] + 0.0722 * values[2];
    }
    const foreground = rgba(getComputedStyle(element).color);
    let node = element;
    let background = [0, 0, 0, 0];
    while (node && background[3] === 0) {
      background = rgba(getComputedStyle(node).backgroundColor);
      node = node.parentElement;
    }
    const l1 = luminance(foreground);
    const l2 = luminance(background);
    return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
  });
}

try {
  const desktop = await browser.newPage({ viewport: { width: 1440, height: 960 } });
  watch(desktop, 'desktop');
  await desktop.goto(`${base}index.html`, { waitUntil: 'networkidle' });
  if ((await desktop.locator('.ledger-sidebar a.ledger-term').count()) !== 15) {
    throw new Error('sidebar does not enumerate all 15 documentation pages');
  }
  for (const route of routes) {
    const response = await desktop.request.get(`${base}${route}`);
    if (!response.ok()) throw new Error(`${route} returned ${response.status()}`);
  }
  await desktop.goto(`${base}service.html#configuration`, { waitUntil: 'networkidle' });
  if (!(await desktop.locator('#configuration').count())) throw new Error('configuration alias missing');
  await desktop.goto(`${base}stable-links.html#linking-to-a-block-not-just-a-note`, { waitUntil: 'networkidle' });
  if (!(await desktop.locator('#linking-to-a-block-not-just-a-note').count())) throw new Error('stable-link alias missing');

  const themeRatios = {};
  await desktop.goto(`${base}index.html`, { waitUntil: 'networkidle' });
  for (const theme of ['light', 'dark', 'contrast']) {
    await desktop.locator('[data-ledger-theme-button]').click();
    await desktop.locator(`[data-theme-value="${theme}"]`).click();
    const ratio = await contrast(desktop, '.ledger-prose p');
    if (ratio < 4.5) throw new Error(`${theme} prose contrast ${ratio.toFixed(2)} is below 4.5`);
    themeRatios[theme] = Number(ratio.toFixed(2));
  }

  await desktop.goto(`${base}search/`, { waitUntil: 'networkidle' });
  const input = desktop.locator('[data-ledger-search-input]');
  await input.fill('Argon2id');
  await desktop.locator('[data-ledger-search-form]').evaluate(form => form.requestSubmit());
  await desktop.locator('[data-ledger-result-count]').filter({ hasText: /result/ }).waitFor();
  const resultHref = await desktop.locator('[data-ledger-search-results] a').first().getAttribute('href');
  if (!resultHref?.startsWith('/notrios/')) throw new Error(`search result escaped base path: ${resultHref}`);

  const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } });
  watch(mobile, 'mobile');
  await mobile.goto(`${base}index.html`, { waitUntil: 'networkidle' });
  const hamburger = mobile.locator('[data-ledger-drawer-toggle]');
  const box = await hamburger.boundingBox();
  if (!box || box.width < 44 || box.height < 44) throw new Error(`mobile menu target too small: ${JSON.stringify(box)}`);
  await hamburger.click();
  await mobile.keyboard.press('Escape');
  if (await hamburger.getAttribute('aria-expanded') !== 'false') throw new Error('Escape did not close drawer');
  const overflow = await mobile.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  if (overflow > 1) throw new Error(`mobile page overflows by ${overflow}px`);

  const external = [...requests].filter(url => !url.startsWith(origin) && !url.startsWith('data:'));
  if (external.length) throw new Error(`external runtime requests: ${external.join(', ')}`);
  if (problems.length) throw new Error(`browser problems: ${problems.join(' | ')}`);
  console.log(JSON.stringify({ routes: routes.length, known_query: 'Argon2id', theme_contrast_ratios: themeRatios, mobile_target: box, mobile_overflow_px: overflow, external_requests: external, console_problems: problems.length }, null, 2));
} finally {
  await browser.close();
}
