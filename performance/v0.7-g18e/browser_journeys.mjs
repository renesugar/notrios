import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { assert, healthTracker, writeJSON } from './browser_harness.mjs';
import { runDesktopJourneys } from './browser_desktop.mjs';
import { runSyncJourneys } from './browser_sync.mjs';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const root = path.resolve(new URL('.', import.meta.url).pathname, '../..');
const manifest = JSON.parse(await fs.readFile(path.join(root, 'performance/v0.7-g18e/JOURNEYS.json'), 'utf8'));
const reportPath = process.env.G18E_REPORT_PATH || '/tmp/notrios-g18e-report.json';
const chromePath = process.env.CHROME_PATH || process.env.G18E_CHROME_PATH || '/usr/bin/google-chrome';
const pairs = {
  desktop: { host: process.env.G18E_DESKTOP_HOST_URL, joiner: process.env.G18E_DESKTOP_JOINER_URL },
  'narrow-sync': { host: process.env.G18E_NARROW_HOST_URL, joiner: process.env.G18E_NARROW_JOINER_URL },
};
for (const [viewport, pair] of Object.entries(pairs)) assert(pair.host && pair.joiner, `missing ${viewport} host/joiner URL`);

const results = [];
const health = [];
const screenshots = [];
const browser = await chromium.launch({ headless: true, executablePath: chromePath });
try {
  for (const viewport of ['desktop', 'narrow-sync']) {
    const dimensions = manifest.viewports.find((item) => item.id === viewport);
    const context = await browser.newContext({
      viewport: { width: dimensions.width, height: dimensions.height },
      acceptDownloads: true,
      permissions: ['clipboard-read', 'clipboard-write'],
    });
    const allowed = Object.values(pairs[viewport]).map((value) => new URL(value).origin);
    const hostPage = await context.newPage();
    const joinerPage = await context.newPage();
    const hostHealth = await healthTracker(hostPage, allowed);
    const joinerHealth = await healthTracker(joinerPage, allowed);
    await Promise.all([
      hostPage.goto(pairs[viewport].host, { waitUntil: 'domcontentloaded' }),
      joinerPage.goto(pairs[viewport].joiner, { waitUntil: 'domcontentloaded' }),
    ]);
    await Promise.all([hostPage.locator('main.app-shell').waitFor(), joinerPage.locator('main.app-shell').waitFor()]);

    if (viewport === 'desktop') results.push(...await runDesktopJourneys(joinerPage, context.request, pairs[viewport].joiner));
    results.push(...await runSyncJourneys({
      hostPage, joinerPage, request: context.request,
      hostBase: pairs[viewport].host, joinerBase: pairs[viewport].joiner, viewport,
    }));

    if (viewport === 'narrow-sync') {
      await joinerPage.getByRole('button', { name: /Sync$/ }).click();
      await joinerPage.getByRole('dialog', { name: /Sync center/i }).waitFor();
    }
    const screenshot = viewport === 'desktop' ? '/tmp/notrios-g18e-desktop.png' : '/tmp/notrios-g18e-narrow-sync.png';
    await joinerPage.screenshot({ path: screenshot, fullPage: false });
    screenshots.push(screenshot);
    health.push({ viewport, replica: 'host', ...hostHealth }, { viewport, replica: 'joiner', ...joinerHealth });
    await context.close();
  }
} catch (error) {
  results.push({ id: 'runner', state: 'failed', error: String(error) });
} finally {
  await browser.close();
}

for (const journey of manifest.journeys.filter((item) => item.state === 'unverified')) {
  results.push({ id: journey.id, state: 'unrun', reason: journey.unrun_reason });
}

const executed = manifest.journeys.filter((item) => item.state === 'executed');
if (!results.some((result) => result.id === 'runner' && result.state === 'failed')) {
  for (const journey of executed) {
    for (const viewport of journey.viewports) {
      assert(results.some((result) => result.id === journey.id && result.viewport === viewport), `missing result ${journey.id}/${viewport}`);
    }
  }
}
const failedHealth = health.flatMap((entry) => [
  ...entry.console_errors.filter((value) => value !== 'Failed to load resource: the server responded with a status of 401 (Unauthorized)').map((value) => `${entry.viewport}/${entry.replica} console: ${value}`),
  ...entry.page_errors.map((value) => `${entry.viewport}/${entry.replica} pageerror: ${value}`),
  ...entry.csp_violations.map((value) => `${entry.viewport}/${entry.replica} CSP: ${value}`),
  ...entry.external_requests.map((value) => `${entry.viewport}/${entry.replica} external request: ${value}`),
]);
const report = {
  schema: 'notrios.docjourney.report.v1',
  manifest_counts: { total: manifest.journeys.length, executed: executed.length, unverified: manifest.journeys.length - executed.length },
  runtime: { excluded_from_metrics: true, sleep_based_success: false },
  browser_policy: manifest.browser_policy,
  viewports: manifest.viewports,
  results,
  screenshots,
  health,
  health_failures: failedHealth,
};
await writeJSON(reportPath, report);

const failures = results.filter((result) => result.state === 'failed');
console.log(JSON.stringify({ report: reportPath, results: results.length, failures: failures.length, health_failures: failedHealth.length }));
if (failures.length || failedHealth.length) throw new Error(`G18e evidence failed: ${failures.length} journey failures, ${failedHealth.length} browser health failures`);
