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
//
// The first version of this file crawled six *views* and concluded that the GUI
// had no attachment, remote-media or job controls. That was wrong, and wrong in
// the way a measurement is worst: it produced a number that looked like an
// answer. Those controls are not in other places, they are in other *states* --
// the sync centre has eight tabs and the crawl only ever saw the one it opens
// on; the localize button needs a note that has remote media in it; restore and
// purge need a note that is in the Trash. So this crawls states, each one a
// short sequence of steps from a freshly loaded page, and the seeding that
// makes those states reachable is part of the harness rather than an accident
// of whatever the library happened to contain.
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import process from 'node:process';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');

const root = path.resolve(new URL('.', import.meta.url).pathname, '../..');
const baseURL = process.env.NOTRIOS_GUI_URL;
if (!baseURL) throw new Error('NOTRIOS_GUI_URL is required');
const outPath = process.env.NOTRIOS_GUI_CONTROLS
  || path.join(root, 'performance/v0.8-h15/GUI_CONTROLS.json');
const chromePath = process.env.CHROME_PATH || '/usr/bin/google-chrome';

// A state is a name and the steps that reach it. Steps are deliberately small
// and declarative -- a test id, an accessible role and name, or a CSS selector
// with an index -- so that a state is readable as a claim about the interface
// ("open the sync centre, then the Attachments tab") rather than as a script.
const SIDEBAR_ALL = { testid: 'sidebar-row-snb_all_notes' };
const SIDEBAR_TRASH = { testid: 'sidebar-row-snb_trash' };
// Located by name now that it has one. It used to be located by class,
// because its descriptive string is a `title` attribute and its glyph span is
// aria-hidden, so the accessible name was the bare word "Sync" -- and a
// role+name step written from the label the earlier inventory displayed
// silently failed all eight sync states. v0.8 H28 gave it and the eight tabs
// test ids, which is what a step should name: a class is a styling decision
// and a test id is a promise.
const OPEN_SYNC = { testid: 'sync-header-button' };
const syncTab = (id) => ({ testid: `sync-tab-${id}` });

const STATES = [
  { id: 'start', steps: [] },
  { id: 'all-notes', steps: [SIDEBAR_ALL] },
  { id: 'help', steps: [{ testid: 'sidebar-row-nb_help' }] },
  { id: 'trash', steps: [SIDEBAR_TRASH] },
  { id: 'new-note', steps: [{ testid: 'new-note-button' }] },

  // A note has to be *open* before the controls that act on one exist. The
  // six-view crawl clicked into a notebook and stopped there, which is why it
  // recorded no delete button in an interface that has had one throughout.
  // Named fixtures rather than "whatever is first". The first card in All
  // notes is a Help note, which is read only, so an index-based click would
  // have gone on missing the controls that only a writable note has.
  { id: 'note-open', steps: [SIDEBAR_ALL, { css: '.result-card', match: 'Crawl fixture note' }] },
  { id: 'note-trashed', steps: [SIDEBAR_TRASH, { css: '.result-card', match: 'Trashed fixture note' }] },

  // Seeded with a note whose body carries an image on an allowed domain. The
  // scan that produces this list is server-side policy only; nothing is
  // fetched, here or by the crawl.
  { id: 'note-remote-media', steps: [SIDEBAR_ALL, { css: '.result-card', match: 'Remote media fixture' }] },

  // Eight tabs, of which the earlier crawl saw one. Attachments and Backup are
  // where two of the three "missing" capabilities actually live.
  { id: 'sync-overview', steps: [OPEN_SYNC] },
  { id: 'sync-setup', steps: [OPEN_SYNC, syncTab('setup')] },
  { id: 'sync-peers', steps: [OPEN_SYNC, syncTab('peers')] },
  { id: 'sync-retention', steps: [OPEN_SYNC, syncTab('retention')] },
  { id: 'sync-attachments', steps: [OPEN_SYNC, syncTab('resources')] },
  { id: 'sync-conflicts', steps: [OPEN_SYNC, syncTab('conflicts')] },
  { id: 'sync-backup', steps: [OPEN_SYNC, syncTab('backup')] },
  { id: 'sync-repairs', steps: [OPEN_SYNC, syncTab('repairs')] },
];

function resolve(page, step) {
  if (step.testid) return page.getByTestId(step.testid);
  if (step.css) {
    const all = step.match ? page.locator(step.css).filter({ hasText: step.match }) : page.locator(step.css);
    return all.nth(step.nth || 0);
  }
  return page.getByRole(step.role, { name: new RegExp(step.name, 'i') });
}

function describe(step) {
  if (step.testid) return `testid=${step.testid}`;
  if (step.css) return `css=${step.css}${step.match ? ` text~${step.match}` : ''}#${step.nth || 0}`;
  return `${step.role}=${step.name}`;
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
const states = [];
const browser = await chromium.launch({ headless: true, executablePath: chromePath });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();

  for (const state of STATES) {
    // Each state starts from a fresh load. The earlier crawl accumulated clicks
    // across its whole run, which happened to work when the only modal was
    // opened last; with sixteen states one left-open dialog would silently
    // change everything measured after it.
    await page.goto(baseURL, { waitUntil: 'domcontentloaded' });
    await page.locator('main.app-shell').waitFor();

    let failed = null;
    for (const step of state.steps) {
      const target = resolve(page, step);
      try {
        await target.waitFor({ state: 'visible', timeout: 5000 });
        await target.click();
        await page.waitForTimeout(400);
      } catch (error) {
        failed = `${describe(step)}: ${String(error).slice(0, 100)}`;
        break;
      }
    }
    if (failed) {
      states.push({ id: state.id, reached: false, error: failed });
      continue;
    }

    const found = await page.evaluate(collectControls);
    for (const control of found) {
      const id = identify(control);
      if (!controls.has(id)) {
        controls.set(id, { id, tag: control.tag, role: control.role, testid: control.testid,
          examples: [], instances: 0, enabled_instances: 0, states: [] });
      }
      const record = controls.get(id);
      record.instances++;
      // Counted, not just observed. A control the crawl only ever finds
      // disabled is a real fact about this mode -- the crawl is a browser, so
      // anything gated on the native bridge must be disabled every time it is
      // seen, and a control that came back enabled would mean that gate had
      // broken open.
      if (!control.disabled) record.enabled_instances++;
      // A few labels are kept as examples of what this control says, which is
      // what makes an entry legible without turning content into identity.
      if (control.label && record.examples.length < 3 && !record.examples.includes(control.label)) {
        record.examples.push(control.label);
      }
      if (!record.states.includes(state.id)) record.states.push(state.id);
    }
    states.push({ id: state.id, reached: true, controls: found.length, steps: state.steps.map(describe) });
  }
  await context.close();
} finally {
  await browser.close();
}


// The signature of everything that decides what this crawl would find: test
// ids, interactive elements, and the roles that make a non-element behave as
// one. Recorded here, at the moment of measurement, and checked by
// validate_evidence.py without a browser -- which is what stops a control being
// added while the committed inventory keeps passing. Prose, styling and
// comments do not move it, so an ordinary edit does not force a re-crawl.
//
// Kept identical to interface_signature.py by hand. That duplication is the
// price of not putting a browser in `make validate`, and its failure mode is
// safe: an implementation that drifts produces a mismatch, which asks for a
// crawl rather than passing quietly.
const SIGNATURE_TOKENS = /data-testid\s*=\s*(?:"[^"]*"|'[^']*'|\{[^}]*\})|<button|<input|<select|<textarea|<a\s|role="button"|role="tab"/g;

async function interfaceFiles(directory) {
  const found = [];
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === '__tests__') continue;
      found.push(...await interfaceFiles(full));
    } else if (entry.name.endsWith('.ts') || entry.name.endsWith('.tsx')) {
      found.push(full);
    }
  }
  return found;
}

async function interfaceSignature(webSrc) {
  const files = (await interfaceFiles(webSrc)).sort();
  const lines = [];
  for (const file of files) {
    const text = await fs.readFile(file, 'utf8');
    const relative = path.relative(root, file).split(path.sep).join('/');
    lines.push(relative + '|' + (text.match(SIGNATURE_TOKENS) ?? []).join('|'));
  }
  return crypto.createHash('sha256').update(lines.join('\n')).digest('hex');
}

const inventory = [...controls.values()]
  .map((control) => ({ ...control, always_disabled: control.enabled_instances === 0 }))
  .sort((a, b) => a.id.localeCompare(b.id));

// How much of the interface can be pointed at by name, recorded as a number so
// it stops being an impression. A journey against an unnamed control has to
// find it by shape -- the nth button inside the third div -- which breaks on a
// layout change that broke nothing, so this is the measure of whether the
// interface can be documented at all. v0.8 H15 measured 22 of 59; H28 named the
// controls the twelve unjourneyed features are reached through.
const addressable = inventory.filter((control) => control.testid !== '').length;
const summary = {
  controls: inventory.length,
  addressable,
  unaddressable: inventory.length - addressable,
};
await fs.writeFile(outPath, JSON.stringify({
  schema: 'notrios.h15.gui-controls.v3',
  interface_signature: await interfaceSignature(path.join(root, 'web/src')),
  summary,
  states,
  controls: inventory,
}, null, 2) + '\n');
console.log(JSON.stringify({ states, summary }, null, 2));

// An unreached state is a failure, not a note in the output. The first run of
// the sixteen-state crawl reached eight of them -- one locator was written from
// a `title` attribute that is not the accessible name -- and the Go test around
// it passed, because the crawl exited 0 and reported a smaller inventory. A
// measurement that quietly shrinks is worse than one that stops, so it stops.
const unreached = states.filter((state) => !state.reached);
if (unreached.length > 0) {
  console.error(`${unreached.length} of ${states.length} states were never reached:`);
  for (const state of unreached) console.error(`  ${state.id}: ${state.error}`);
  process.exitCode = 1;
}
