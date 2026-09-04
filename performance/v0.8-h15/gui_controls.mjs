// Enumerate what the Wails GUI offers, by looking rather than by asking.
//
// Every other adapter onto Notrios can be counted from source: 66 command-line
// usage forms, 109 REST operations, 46 MCP tools, each with a pinned count that
// fails when it moves. The GUI cannot, so its column in the capability table
// has been filled in from whichever journeys happened to be written -- which
// understates it, and no gate can notice, because there is nothing to compare
// against.
//
// This produces that missing thing: a list of the controls the interface
// actually presents, so the GUI becomes a measured adapter like the others.
//
// It enumerates and does not activate. Delete, purge, retire and empty-trash
// are all reachable from this interface, and a crawler that clicked what it
// found would eventually find one of them. Navigation that only changes which
// view is showing is performed, because most controls do not exist until you
// are somewhere; anything else is read from its label and left alone.
import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');

const root = path.resolve(new URL('.', import.meta.url).pathname, '../..');
const baseURL = process.env.NOTRIOS_GUI_URL;
if (!baseURL) throw new Error('NOTRIOS_GUI_URL is required');
const outPath = process.env.NOTRIOS_GUI_CONTROLS
  || path.join(root, 'performance/v0.8-h15/GUI_CONTROLS.json');
const chromePath = process.env.CHROME_PATH || '/usr/bin/google-chrome';

const VIEWS = [
  { id: 'start', open: null },
  { id: 'all-notes', open: { testid: 'sidebar-row-snb_all_notes' } },
  { id: 'help', open: { testid: 'sidebar-row-nb_help' } },
  { id: 'trash', open: { testid: 'sidebar-row-snb_trash' } },
  { id: 'new-note', open: { testid: 'new-note-button' } },
  { id: 'sync-center', open: { role: 'button', name: 'Sync$' } },
];

function resolve(page, locator) {
  if (locator.testid) return page.getByTestId(locator.testid);
  return page.getByRole(locator.role, { name: new RegExp(locator.name, 'i') });
}

// A real function, not a string. Passing a string to page.evaluate evaluates it
// as an expression: the arrow function is constructed and never called, so this
// returned a function object rather than a list of controls. That is the same
// mistake the journey capture made with its marker, already found, already
// fixed and already commented -- and repeated here within the day. Written this
// way so the next person copying this file cannot inherit it.
function collectControls() {
  const seen = [];
  const selector = 'button, a[href], summary, [role="button"], [role="link"], [role="tab"],'
    + ' [role="menuitem"], [role="checkbox"], [role="switch"], input, select, textarea';
  for (const el of document.querySelectorAll(selector)) {
    const rect = el.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) continue;
    const label = (el.getAttribute('aria-label')
      || el.getAttribute('title')
      || el.getAttribute('placeholder')
      || (el.innerText || '').trim().split('\n')[0]
      || el.getAttribute('name')
      || '').trim().slice(0, 80);
    // The shape is recorded alongside the label, because identity has to come
    // from structure rather than from text. A list of search results renders
    // one control per note, and keying on the label would make the inventory
    // grow with the library instead of describing the interface.
    seen.push({
      tag: el.tagName.toLowerCase(),
      role: el.getAttribute('role') || '',
      testid: el.getAttribute('data-testid') || '',
      shape: [el.tagName.toLowerCase(), el.getAttribute('role') || '',
              (el.getAttribute('class') || '').trim().split(/\s+/).sort().join('.'),
              el.parentElement ? (el.parentElement.getAttribute('class') || '').trim().split(/\s+/).sort().join('.') : ''
      ].join('|'),
      label,
      disabled: el.hasAttribute('disabled') || el.getAttribute('aria-disabled') === 'true',
    });
  }
  return seen;
}

// A control is identified by its test id where it has one, and otherwise by
// what it is and what it says. Position is deliberately not part of identity:
// a control that moves is the same control.
function identify(control) {
  if (control.testid) return `testid:${control.testid}`;
  // Structure, not text. Two buttons with the same tag, role, classes and
  // parent are one control repeated over data -- one note per row -- and
  // counting them separately would measure the seeded library rather than the
  // interface.
  return `shape:${control.shape}`;
}

const controls = new Map();
const views = [];
const browser = await chromium.launch({ headless: true, executablePath: chromePath });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  await page.goto(baseURL, { waitUntil: 'domcontentloaded' });
  await page.locator('main.app-shell').waitFor();

  for (const view of VIEWS) {
    if (view.open) {
      const target = resolve(page, view.open).first();
      try {
        await target.waitFor({ state: 'visible', timeout: 5000 });
        await target.click();
        await page.waitForTimeout(400);
      } catch (error) {
        views.push({ id: view.id, reached: false, error: String(error).slice(0, 120) });
        continue;
      }
    }
    const found = await page.evaluate(collectControls);
    for (const control of found) {
      const id = identify(control);
      if (!controls.has(id)) {
        controls.set(id, { id, tag: control.tag, role: control.role, testid: control.testid,
          examples: [], instances: 0, views: [] });
      }
      const record = controls.get(id);
      record.instances++;
      // A few labels are kept as examples of what this control says, which is
      // what makes an entry legible without turning content into identity.
      if (control.label && record.examples.length < 3 && !record.examples.includes(control.label)) {
        record.examples.push(control.label);
      }
      if (!record.views.includes(view.id)) record.views.push(view.id);
    }
    views.push({ id: view.id, reached: true, controls: found.length });
  }
  await context.close();
} finally {
  await browser.close();
}

const inventory = [...controls.values()].sort((a, b) => a.id.localeCompare(b.id));
await fs.writeFile(outPath, JSON.stringify({
  schema: 'notrios.h15.gui-controls.v1',
  views,
  controls: inventory,
}, null, 2) + '\n');
console.log(JSON.stringify({ views, controls: inventory.length }, null, 2));
