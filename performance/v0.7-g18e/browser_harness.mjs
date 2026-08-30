import fs from 'node:fs/promises';

export function assert(value, message) {
  if (!value) throw new Error(message);
}

export function metric() {
  return {
    clicks: 0,
    keypresses: 0,
    typed_fields: [],
    branches: 0,
    maximum_modal_depth: 0,
    recovery_steps: 0,
  };
}

export function recorder(page, metricState) {
  const typed = new Set(metricState.typed_fields);
  return {
    async click(locator) {
      await locator.click();
      metricState.clicks++;
    },
    async fill(locator, value, field) {
      await locator.fill(value);
      typed.add(field);
      metricState.typed_fields = [...typed].sort();
    },
    async press(locator, key) {
      await locator.press(key);
      metricState.keypresses++;
    },
    async select(locator, value) {
      await locator.selectOption(String(value));
      metricState.clicks++;
    },
    async check(locator) {
      await locator.check();
      metricState.clicks++;
    },
    async setInputFiles(locator, files, field = 'file') {
      await locator.setInputFiles(files);
      typed.add(field);
      metricState.typed_fields = [...typed].sort();
    },
    async pasteHTML(locator, html, field = 'note-body') {
      await page.evaluate(async (value) => {
        const item = new ClipboardItem({
          'text/html': new Blob([value], { type: 'text/html' }),
          'text/plain': new Blob(['Name\tValue\nalpha\t1'], { type: 'text/plain' }),
        });
        await navigator.clipboard.write([item]);
      }, html);
      await locator.press('ControlOrMeta+V');
      metricState.keypresses++;
      typed.add(field);
      metricState.typed_fields = [...typed].sort();
    },
    async confirm(trigger, expected, accept) {
      const dialogPromise = page.waitForEvent('dialog');
      const clickPromise = trigger.click();
      metricState.clicks++;
      const dialog = await dialogPromise;
      assert(expected.test(dialog.message()), `confirmation text ${JSON.stringify(dialog.message())} did not match ${expected}`);
      metricState.branches++;
      metricState.maximum_modal_depth = Math.max(metricState.maximum_modal_depth, 1);
      if (accept) await dialog.accept(); else await dialog.dismiss();
      await clickPromise;
      metricState.keypresses++;
    },
    recovery() {
      metricState.recovery_steps++;
    },
  };
}

export async function api(request, base, method, endpoint, body, expected = 200, headers = {}) {
  const response = await request.fetch(`${base}${endpoint}`, {
    method,
    data: body,
    headers,
    failOnStatusCode: false,
  });
  const contentType = response.headers()['content-type'] || '';
  const payload = contentType.includes('json') ? await response.json() : await response.body();
  assert(response.status() === expected, `${method} ${endpoint}: HTTP ${response.status()}, want ${expected}: ${Buffer.isBuffer(payload) ? payload.toString('utf8') : JSON.stringify(payload)}`);
  return payload;
}

export async function rawStatus(request, base, method, endpoint, body, headers = {}) {
  const response = await request.fetch(`${base}${endpoint}`, { method, data: body, headers, failOnStatusCode: false });
  return { status: response.status(), body: await response.body() };
}

export async function createNotebook(request, base, name) {
  return api(request, base, 'POST', '/api/v1/notebooks', { name }, 201);
}

export async function createDocument(request, base, title, body, notebookID = '') {
  const input = { title, body };
  if (notebookID) input.notebook_id = notebookID;
  return api(request, base, 'POST', '/api/v1/documents', input, 201);
}

export async function getDocument(request, base, id, expected = 200) {
  return api(request, base, 'GET', `/api/v1/documents/${encodeURIComponent(id)}`, undefined, expected);
}

export async function searchAPI(request, base, query, limit = 100, cursor = '') {
  return api(request, base, 'POST', '/api/v1/search', { query, limit, cursor }, 200);
}

export async function waitJob(request, base, id) {
  const deadline = Date.now() + 45_000;
  while (Date.now() < deadline) {
    const job = await api(request, base, 'GET', `/api/v1/jobs/${encodeURIComponent(id)}`, undefined, 200);
    if (job.state === 'succeeded') return job;
    assert(!['failed', 'cancelled'].includes(job.state), `job ${id} settled as ${job.state}: ${JSON.stringify(job)}`);
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
  throw new Error(`job ${id} did not settle`);
}

export async function queueSync(request, base) {
  const started = await api(request, base, 'POST', '/api/v1/jobs/sync/start', { kind: 'incremental' }, 202);
  return waitJob(request, base, started.job.id);
}

export async function openByTitle(page, title, actions) {
  const search = page.getByLabel('Search query');
  await actions.fill(search, `"${title}"`, 'search-query');
  await actions.press(search, 'Enter');
  const result = page.locator('.result-card').filter({ hasText: title }).first();
  await result.waitFor();
  await actions.click(result);
  await page.getByLabel('Note title').waitFor();
  await page.getByLabel('Note title').evaluate((element, expected) => {
    if (element.value !== expected) throw new Error(`opened title ${element.value}, want ${expected}`);
  }, title);
}

export async function setEditorBody(page, body, actions, field = 'note-body') {
  const editor = page.locator('.cm-content').first();
  await editor.waitFor();
  await actions.click(editor);
  await actions.press(editor, 'ControlOrMeta+A');
  await actions.fill(editor, body, field);
}

export async function saveEditor(page, actions) {
  await actions.click(page.getByTestId('save-button'));
  await page.getByRole('status').filter({ hasText: /Saved|Created/ }).waitFor();
}

export async function healthTracker(page, allowedOrigins) {
  const consoleErrors = [];
  const consoleWarnings = [];
  const pageErrors = [];
  const cspViolations = [];
  const externalRequests = [];
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text());
    if (message.type() === 'warning') consoleWarnings.push(message.text());
  });
  page.on('pageerror', (error) => pageErrors.push(String(error)));
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (!allowedOrigins.includes(url.origin) && !['data:', 'blob:'].includes(url.protocol)) externalRequests.push(request.url());
  });
  await page.exposeFunction('g18eRecordCSPViolation', (violation) => {
    cspViolations.push(JSON.stringify(violation));
  });
  await page.addInitScript(() => {
    document.addEventListener('securitypolicyviolation', (event) => {
      window.g18eRecordCSPViolation({
        blocked_uri: event.blockedURI,
        disposition: event.disposition,
        effective_directive: event.effectiveDirective,
        original_policy: event.originalPolicy,
      });
    });
  });
  return { console_errors: consoleErrors, console_warnings: consoleWarnings, page_errors: pageErrors, csp_violations: cspViolations, external_requests: externalRequests };
}

export async function writeJSON(path, value) {
  await fs.writeFile(path, `${JSON.stringify(value, null, 2)}\n`, 'utf8');
}

export async function execute(id, viewport, page, fn) {
  const actions = metric();
  const record = recorder(page, actions);
  const started = Date.now();
  try {
    const assertions = await fn(record, actions);
    assert(actions.clicks + actions.keypresses + actions.typed_fields.length > 0, `${id}: no user action was recorded`);
    assert(assertions.visible > 0, `${id}: no visible postcondition`);
    assert(assertions.canonical > 0, `${id}: no canonical/API postcondition`);
    return { id, viewport, state: 'passed', actions, assertions, duration_ms: Date.now() - started };
  } catch (error) {
    return { id, viewport, state: 'failed', actions, error: String(error), duration_ms: Date.now() - started };
  }
}
