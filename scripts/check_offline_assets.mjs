// v0.5 E6a — asserts the built-in UI loads nothing from a third party.
//
// Before E6a, `md-editor-rt` fetched KaTeX, highlight.js, echarts, cropperjs,
// and prettier from `unpkg.com` at runtime, so a local-first application loaded
// remote executable JavaScript on every launch and math silently stopped
// rendering without a network. Those libraries are bundled now.
//
// This is the check that keeps them bundled. It drives a real headless Chrome
// with the browser cache disabled and every known CDN blocked, then fails if
// the page issued a cross-origin request, injected a remote script or
// stylesheet, reported a CSP violation, or failed to render the formula.
//
// Usage: node scripts/check_offline_assets.mjs <base-url> <document-id>

const [, , baseURL, documentID] = process.argv;
if (!baseURL || !documentID) {
  console.error('usage: node scripts/check_offline_assets.mjs <base-url> <document-id>');
  process.exit(2);
}

class CDP {
  constructor(socket) {
    this.socket = socket; this.nextID = 1; this.pending = new Map(); this.events = [];
    socket.addEventListener('message', (event) => {
      const message = JSON.parse(event.data);
      if (message.id && this.pending.has(message.id)) {
        const { resolve, reject } = this.pending.get(message.id);
        this.pending.delete(message.id);
        message.error ? reject(new Error(JSON.stringify(message.error))) : resolve(message.result);
      } else if (message.method) this.events.push(message);
    });
  }
  static async connect(url) {
    const socket = new WebSocket(url);
    await new Promise((resolve, reject) => {
      socket.addEventListener('open', resolve, { once: true });
      socket.addEventListener('error', reject, { once: true });
    });
    return new CDP(socket);
  }
  send(method, params = {}) {
    const id = this.nextID++;
    this.socket.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => this.pending.set(id, { resolve, reject }));
  }
  async evaluate(expression) {
    const result = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description ?? 'evaluate failed');
    return result.result.value;
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const port = process.env.NOTRIOS_CDP_PORT || '9222';

let targets = null;
for (let attempt = 0; attempt < 60; attempt++) {
  try {
    targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
    if (targets.some((t) => t.type === 'page')) break;
  } catch { /* Chrome still starting */ }
  await sleep(250);
}
const page = targets?.find((t) => t.type === 'page');
if (!page) { console.error('no Chrome page target found'); process.exit(1); }

const cdp = await CDP.connect(page.webSocketDebuggerUrl);
await cdp.send('Page.enable');
await cdp.send('Runtime.enable');
await cdp.send('Network.enable');
await cdp.send('Log.enable');
// A warm cache would hide exactly the regression this guards against.
await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
await cdp.send('Network.clearBrowserCache');
await cdp.send('Network.setBlockedURLs', { urls: ['*unpkg.com*', '*jsdelivr*', '*cdnjs*', '*googleapis*'] });

await cdp.send('Page.navigate', { url: `${baseURL}/#document=${documentID}` });
for (let attempt = 0; attempt < 160; attempt++) {
  if (await cdp.evaluate(`document.querySelector('.title-input')?.value?.length > 0`)) break;
  await sleep(250);
}
// Give any runtime injection the time it would need to happen.
await sleep(5000);

const origin = new URL(baseURL).origin;
const external = [...new Set(cdp.events
  .filter((e) => e.method === 'Network.requestWillBeSent')
  .map((e) => e.params.request.url)
  .filter((url) => !url.startsWith(origin) && !url.startsWith('data:') && !url.startsWith('blob:')))];

const dom = JSON.parse(await cdp.evaluate(`
  (() => JSON.stringify({
    katex: document.querySelectorAll('.katex').length,
    katexErrors: document.querySelectorAll('.katex-error').length,
    remoteScripts: [...document.querySelectorAll('script[src]')].map(s => s.src).filter(s => !s.startsWith(location.origin)),
    remoteLinks: [...document.querySelectorAll('link[href]')].map(l => l.href).filter(h => !h.startsWith(location.origin) && !h.startsWith('data:')),
  }))()
`));

const cspViolations = cdp.events
  .filter((e) => e.method === 'Log.entryAdded')
  .map((e) => e.params.entry.text)
  .filter((text) => /Content Security Policy/i.test(text));

const failures = [];
if (external.length) failures.push(`third-party requests: ${external.join(', ')}`);
if (dom.remoteScripts.length) failures.push(`remote scripts injected: ${dom.remoteScripts.join(', ')}`);
if (dom.remoteLinks.length) failures.push(`remote stylesheets injected: ${dom.remoteLinks.join(', ')}`);
if (cspViolations.length) failures.push(`CSP violations: ${cspViolations.length} (first: ${cspViolations[0].slice(0, 160)})`);
// The positive half: the note's math must actually have rendered offline,
// otherwise "no external requests" would also pass on a broken page.
if (dom.katex < 1) failures.push('no KaTeX output — math did not render offline');
if (dom.katexErrors > 0) failures.push(`KaTeX reported ${dom.katexErrors} render errors`);

console.log(JSON.stringify({ external_requests: external, dom, csp_violations: cspViolations.length }, null, 2));
if (failures.length) {
  console.error('\nFAIL:\n  ' + failures.join('\n  '));
  process.exit(1);
}
console.log('\nOK: no third-party requests, no CSP violations, math rendered offline.');
process.exit(0);
