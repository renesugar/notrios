// v0.5 E6 — editor measurement harness.
//
// Drives a real headless Chrome against a real `notriosd` and records first
// paint and end-to-end keystroke latency on a large note. It uses the Chrome
// DevTools Protocol directly over Node's built-in WebSocket, so it adds no
// dependency to a repository that deliberately keeps few.
//
// Keystroke latency is measured the way a person experiences it: the page
// timestamps a real `keydown` and timestamps the MutationObserver callback for
// the change that key caused. That measures the whole path — CodeMirror's
// transaction, our decoration field, and the DOM write — rather than the part
// that is convenient to time.
//
// Usage: node scripts/measure_editor.mjs <base-url> <document-id> [out.json]

const [, , baseURL, documentID, outputPath] = process.argv;
if (!baseURL || !documentID) {
  console.error('usage: node scripts/measure_editor.mjs <base-url> <document-id> [out.json]');
  process.exit(2);
}

const KEYSTROKES = 40;
const NOTE_CHARACTERS = Number(process.env.NOTRIOS_NOTE_CHARACTERS || 0);

class CDP {
  constructor(socket) {
    this.socket = socket;
    this.nextID = 1;
    this.pending = new Map();
    socket.addEventListener('message', (event) => {
      const message = JSON.parse(event.data);
      if (message.id && this.pending.has(message.id)) {
        const { resolve, reject } = this.pending.get(message.id);
        this.pending.delete(message.id);
        if (message.error) reject(new Error(JSON.stringify(message.error)));
        else resolve(message.result);
      }
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
    const result = await this.send('Runtime.evaluate', {
      expression,
      awaitPromise: true,
      returnByValue: true,
    });
    if (result.exceptionDetails) {
      throw new Error(result.exceptionDetails.exception?.description ?? 'evaluate failed');
    }
    return result.result.value;
  }

  close() {
    this.socket.close();
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitFor(cdp, expression, timeoutMS = 30000) {
  const deadline = Date.now() + timeoutMS;
  for (;;) {
    if (await cdp.evaluate(expression)) return;
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${expression}`);
    await sleep(100);
  }
}

function percentile(values, p) {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.ceil((sorted.length * p) / 100) - 1);
  return sorted[index];
}

async function main() {
  const health = await fetch(`${baseURL.replace(/\/$/, '')}/healthz`).catch(() => null);
  if (!health || !health.ok) throw new Error(`service not reachable at ${baseURL}`);

  const debuggerPort = process.env.NOTRIOS_CDP_PORT || '9222';
  let targets = null;
  for (let attempt = 0; attempt < 60; attempt++) {
    try {
      targets = await (await fetch(`http://127.0.0.1:${debuggerPort}/json/list`)).json();
      if (targets.some((t) => t.type === 'page')) break;
    } catch {
      // Chrome is still starting.
    }
    await sleep(250);
  }
  const page = targets?.find((t) => t.type === 'page');
  if (!page) throw new Error('no Chrome page target found');

  const cdp = await CDP.connect(page.webSocketDebuggerUrl);
  await cdp.send('Page.enable');
  await cdp.send('Runtime.enable');

  const url = `${baseURL.replace(/\/$/, '')}/#document=${documentID}`;
  await cdp.send('Page.navigate', { url });
  await waitFor(cdp, `!!document.querySelector('.cm-content')`);
  // The note arrives over REST after first paint. Waiting on the editor's own
  // text would not work: CodeMirror virtualizes, so only the visible lines are
  // ever in the DOM — which is exactly the property that keeps a 200 KB note
  // cheap to render, and worth knowing before judging the editor.
  await waitFor(cdp, `document.querySelector('.title-input')?.value?.length > 0`);
  await waitFor(cdp, `document.querySelectorAll('.cm-line').length > 5`);

  const paint = await cdp.evaluate(`
    JSON.stringify(performance.getEntriesByType('paint').map((e) => ({ name: e.name, startTime: e.startTime })))
  `);
  const navigation = await cdp.evaluate(`
    (() => {
      const nav = performance.getEntriesByType('navigation')[0];
      return JSON.stringify({ domContentLoaded: nav.domContentLoadedEventEnd, loadEvent: nav.loadEventEnd });
    })()
  `);

  const editorFacts = await cdp.evaluate(`
    (() => {
      const content = document.querySelector('.cm-content');
      return JSON.stringify({
        // Rendered, not total: CodeMirror keeps only the visible lines in the
        // DOM. The note's real size is recorded by the shell script.
        rendered_characters: content.textContent.length,
        rendered_lines: content.querySelectorAll('.cm-line').length,
        title: document.querySelector('.title-input')?.value ?? '',
      });
    })()
  `);

  // Instrument: timestamp each keydown, and the mutation it causes.
  await cdp.evaluate(`
    (() => {
      window.__notriosSamples = [];
      window.__notriosPending = null;
      const content = document.querySelector('.cm-content');
      document.addEventListener('keydown', () => { window.__notriosPending = performance.now(); }, true);
      const observer = new MutationObserver(() => {
        if (window.__notriosPending === null) return;
        window.__notriosSamples.push(performance.now() - window.__notriosPending);
        window.__notriosPending = null;
      });
      observer.observe(content, { childList: true, subtree: true, characterData: true });
      content.focus();
      return true;
    })()
  `);

  // Put the caret in the middle of the note, where a real edit happens, rather
  // than at the start where CodeMirror has the least to re-render.
  await cdp.evaluate(`
    (() => {
      const content = document.querySelector('.cm-content');
      const lines = content.querySelectorAll('.cm-line');
      const target = lines[Math.floor(lines.length / 2)];
      const range = document.createRange();
      range.selectNodeContents(target);
      range.collapse(false);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      content.focus();
      return true;
    })()
  `);

  for (let i = 0; i < KEYSTROKES; i++) {
    const char = String.fromCharCode(97 + (i % 26));
    await cdp.send('Input.dispatchKeyEvent', {
      type: 'keyDown', text: char, key: char, code: `Key${char.toUpperCase()}`, windowsVirtualKeyCode: char.toUpperCase().charCodeAt(0),
    });
    await cdp.send('Input.dispatchKeyEvent', {
      type: 'keyUp', key: char, code: `Key${char.toUpperCase()}`, windowsVirtualKeyCode: char.toUpperCase().charCodeAt(0),
    });
    await sleep(30);
  }

  const samples = JSON.parse(await cdp.evaluate(`JSON.stringify(window.__notriosSamples)`));

  const report = {
    generated_at: new Date().toISOString(),
    url,
    editor: { ...JSON.parse(editorFacts), note_characters: NOTE_CHARACTERS },
    paint: JSON.parse(paint),
    navigation: JSON.parse(navigation),
    keystrokes: {
      dispatched: KEYSTROKES,
      measured: samples.length,
      p50_ms: Number(percentile(samples, 50).toFixed(3)),
      p95_ms: Number(percentile(samples, 95).toFixed(3)),
      max_ms: Number(Math.max(0, ...samples).toFixed(3)),
    },
  };

  cdp.close();
  const json = `${JSON.stringify(report, null, 2)}\n`;
  if (outputPath) {
    const { writeFileSync } = await import('node:fs');
    writeFileSync(outputPath, json);
  }
  process.stdout.write(json);
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
