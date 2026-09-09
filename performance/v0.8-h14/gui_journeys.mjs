// GUI journey capture: drive the interface, and photograph each step with the
// click marked where the click actually goes.
//
// The marker is derived from the bounding box of the element about to be
// clicked, never from coordinates written down by hand. Three things follow,
// and they are the reason:
//
//   a moved button moves the circle, so a picture cannot drift from the
//   interface while still looking authoritative;
//
//   a locator that stops matching fails the journey rather than producing a
//   confident image of the wrong place -- the failure is loud, which is the
//   property a screenshot in documentation otherwise lacks entirely;
//
//   drawing in the page keeps the marker in the element's own coordinate space
//   at the page's device pixel ratio, so no second imaging toolchain is
//   involved and no scaling arithmetic can be wrong.
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import process from 'node:process';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');

const root = path.resolve(new URL('.', import.meta.url).pathname, '../..');
const catalogue = JSON.parse(
  await fs.readFile(path.join(root, 'docs/docjourneys/GUI_JOURNEYS.json'), 'utf8'));
const baseURL = process.env.NOTRIOS_GUI_URL;
if (!baseURL) throw new Error('NOTRIOS_GUI_URL is required');
const imageDir = process.env.NOTRIOS_GUI_IMAGE_DIR || path.join(root, 'docs/images/journeys');
const manifestPath = process.env.NOTRIOS_GUI_MANIFEST || path.join(root, 'docs/images/journeys/MANIFEST.json');
const chromePath = process.env.CHROME_PATH || '/usr/bin/google-chrome';

// The sandbox paths a step may type. A journey that says "the shared folder"
// must not carry one machine's temporary directory in the documentation, and a
// capture has to type a real one, so the catalogue names {carrier} and the
// runner is told what it is. Same arrangement the command-line journeys use for
// {db} and {sandbox}.
const values = { carrier: process.env.NOTRIOS_GUI_CARRIER ?? '' };
function substitute(text) {
  return Object.entries(values).reduce(
    (result, [name, value]) => result.replaceAll(`{${name}}`, value), text ?? '');
}

// resolve turns a catalogue locator into a Playwright one. Only these three
// kinds are allowed: a role and an accessible name, a test id, or a CSS
// selector. Anything looser -- an nth-child path, a coordinate -- would be a
// description of today's markup rather than of the thing being pointed at.
function resolve(page, locator) {
  if (locator.role) return page.getByRole(locator.role, { name: new RegExp(locator.name, 'i') });
  if (locator.testid) return page.getByTestId(locator.testid);
  if (locator.css) return page.locator(locator.css);
  throw new Error(`step locator names no role, testid or css: ${JSON.stringify(locator)}`);
}

// silenceSpellcheck turns the browser's own red underlines off before a shot.
//
// They are not part of the interface: Chromium paints them asynchronously, so
// the same note photographed twice differs by whichever squiggles had rendered
// yet. That put twenty-six of fifty-eight images in every capture diff while
// showing nothing about Notrios, and a picture diff that cannot mean anything
// defeats the manifest that exists to make a changed picture a signal.
//
// Turned off here rather than in the interface. A person writing notes wants
// spellcheck; only the camera does not.
function silenceSpellcheck() {
  for (const element of document.querySelectorAll('textarea, input, [contenteditable]')) {
    element.setAttribute('spellcheck', 'false');
    element.spellcheck = false;
  }
}

// drawMarker is a real function rather than a string. Passing a string here
// evaluates it as an expression: the arrow function is constructed, the
// argument is ignored, and nothing is ever called. That produced five
// screenshots with no marker on them and no error anywhere, which is the exact
// failure this whole design exists to prevent -- a confident picture of the
// wrong place. It was caught by two steps pointing at different elements coming
// out byte-identical, and that is now a check rather than a coincidence.
function drawMarker({ box, pointing }) {
  for (const id of ['notrios-journey-marker', 'notrios-journey-outline']) {
    document.getElementById(id)?.remove();
  }
  // Two marks, and they say different things.
  //
  // The outline is the element: how much of the screen is clickable. The circle
  // is the point Playwright actually clicks, which is the element's centre, at
  // a fixed size so it stays a pointer rather than a highlight. Sizing the
  // circle to the element instead produced a 244px ring over a full-width
  // sidebar row -- it swallowed five rows and pointed at nothing, which is a
  // picture that looks authoritative and tells the reader less than no picture
  // would.
  const outline = document.createElement('div');
  outline.id = 'notrios-journey-outline';
  Object.assign(outline.style, {
    position: 'fixed',
    left: (box.x - 3) + 'px',
    top: (box.y - 3) + 'px',
    width: (box.width + 6) + 'px',
    height: (box.height + 6) + 'px',
    border: '2px solid rgba(216,27,40,0.55)',
    borderRadius: '6px',
    pointerEvents: 'none',
    zIndex: '2147483646',
  });
  document.body.appendChild(outline);

  // Only where there is something to press. A screenshot_only step is "look at
  // this", and a click marker on one points at a place nobody should press --
  // it appeared in the middle of a read-only report, over blank space, looking
  // like an instruction.
  if (!pointing) return;

  const diameter = 40;
  const marker = document.createElement('div');
  marker.id = 'notrios-journey-marker';
  Object.assign(marker.style, {
    position: 'fixed',
    left: (box.x + box.width / 2 - diameter / 2) + 'px',
    top: (box.y + box.height / 2 - diameter / 2) + 'px',
    width: diameter + 'px',
    height: diameter + 'px',
    border: '4px solid #d81b28',
    borderRadius: '50%',
    boxShadow: '0 0 0 3px rgba(255,255,255,0.95), 0 0 12px rgba(216,27,40,0.6)',
    pointerEvents: 'none',
    zIndex: '2147483647',
  });
  document.body.appendChild(marker);
}

const results = [];
const manifest = [];
// Rendering flags, not behaviour flags. Subpixel text antialiasing is not
// deterministic between runs: two captures of an unchanged interface differed
// by ten pixels along the edges of two glyphs, which is enough to rewrite an
// image and put it in a diff that then says nothing. Grayscale antialiasing and
// a fixed colour profile make the same page produce the same bytes.
const browser = await chromium.launch({
  headless: true,
  executablePath: chromePath,
  args: [
    '--font-render-hinting=none',
    '--disable-lcd-text',
    '--disable-font-subpixel-positioning',
    '--force-color-profile=srgb',
    // Chromium's spellchecker paints its red underlines asynchronously, and
    // whether they had appeared before the shot was the single largest source
    // of pixel churn: 876 pixels under one line of a note body, present in one
    // run and absent in the next. Setting spellcheck=false on the document at
    // init did not settle it -- the markers are the service's, not the
    // element's -- so the service is turned off at the browser.
    '--disable-spell-checking',
    '--disable-features=SpellCheckService,Translate',
  ],
});
try {
  for (const journey of catalogue.journeys) {
    // A desktop journey is not this runner's to perform, and skipping it is
    // not the same as ignoring it: importing, exporting, snapshots and
    // publishing name a folder on the machine running the library, so their
    // controls are correctly disabled here. Driving them needs the real
    // application, which cmd/notriosctl TestDesktopJourneyCapture does under
    // Xvfb. Its images stay in the manifest below rather than being dropped.
    if (journey.driver === 'desktop') {
      results.push({ id: journey.id, state: 'desktop', error: 'driven by the desktop harness' });
      continue;
    }
    const context = await browser.newContext({
      viewport: { width: catalogue.viewport.width, height: catalogue.viewport.height },
      deviceScaleFactor: 1,
    });
    // Before anything renders, rather than after. Turning spellcheck off on
    // elements that already exist leaves Chromium's markers painted until
    // something else forces a relayout, so the same page photographed twice
    // differed by whichever squiggles had been drawn yet -- 876 pixels along
    // one line of a note body, in a diff that otherwise said nothing. Set on
    // the root at init, every element inherits it at creation and no marker is
    // ever computed.
    await context.addInitScript(() => {
      document.documentElement.spellcheck = false;
    });
    const page = await context.newPage();
    await page.goto(baseURL, { waitUntil: 'domcontentloaded' });
    await page.locator('main.app-shell').waitFor();

    try {
      for (const step of journey.steps) {
        const target = resolve(page, step.locator).first();
        // Waiting here is what makes a missing element a failure rather than a
        // screenshot of whatever happened to be on screen.
        await target.waitFor({ state: 'visible', timeout: 10000 });
        // Brought into view before it is measured. A screenshot is of the
        // viewport, so a control below the fold was photographed as whatever
        // happened to be on screen with the marker drawn outside it -- two
        // journeys produced pictures byte-identical to other steps, which the
        // duplicate check caught and which is what it is for. Visible to
        // Playwright means attached and not hidden; it does not mean a reader
        // could see it.
        await target.scrollIntoViewIfNeeded();
        const box = await target.boundingBox();
        if (!box) throw new Error(`${journey.id}/${step.id}: the element has no box to point at`);

        await page.evaluate(silenceSpellcheck);
        // Repainting without the underlines is not instant, and a screenshot
        // taken during it catches half of them -- which is the same flake in a
        // smaller window.
        await page.waitForTimeout(150);
        await page.evaluate(drawMarker, {
          box, pointing: step.action !== 'screenshot_only',
        });
        const file = `${journey.id}-${step.id}.png`;
        const absolute = path.join(imageDir, file);
        await page.screenshot({ path: absolute, fullPage: false });
        await page.evaluate(() => {
          for (const id of ['notrios-journey-marker', 'notrios-journey-outline']) {
            document.getElementById(id)?.remove();
          }
        });

        manifest.push({
          journey: journey.id,
          step: step.id,
          image: path.posix.join('images/journeys', file),
          locator: step.locator,
          sha256: crypto.createHash('sha256').update(await fs.readFile(absolute)).digest('hex'),
        });

        // Playwright dismisses dialogs unless told otherwise, so a step that
        // clicks a control guarded by window.confirm did nothing at all and
        // reported success: the delete journey clicked Move to Trash, the
        // confirmation was declined for it, and the note was still there. A
        // step must say that it answers a confirmation, and only that step's
        // dialog is accepted.
        if (step.confirm) page.once('dialog', (dialog) => void dialog.accept());
        if (step.action === 'click') await target.click();
        else if (step.action === 'fill') await target.fill(substitute(step.value));
        else if (step.action === 'choose') await target.selectOption(step.value ?? '');
        else if (step.action === 'attach') {
          // A path relative to the repository, resolved here rather than in the
          // catalogue: a journey says which file it attaches, and where that
          // file lives on the machine running the capture is the runner's
          // business. Refused outside the tree, because a catalogue entry is a
          // documentation file and should not be able to read /etc.
          const file = path.resolve(root, step.value ?? '');
          if (!file.startsWith(root + path.sep)) {
            throw new Error(`${journey.id}/${step.id}: attaches a file outside the repository`);
          }
          await target.setInputFiles(file);
        }
      }

      const post = journey.postcondition;
      const check = resolve(page, post.locator).first();
      await check.waitFor({ state: 'visible', timeout: 10000 });
      results.push({ id: journey.id, state: 'executed' });
    } catch (error) {
      results.push({ id: journey.id, state: 'failed', error: String(error) });
    }
    await context.close();
  }
} finally {
  await browser.close();
}

// Two steps pointing at different elements must not produce identical bytes.
// If the marker silently stops being drawn, every step in the same app state
// photographs the same way, and nothing else notices.
const byHash = new Map();
for (const image of manifest) {
  const same = byHash.get(image.sha256);
  if (same && JSON.stringify(same.locator) !== JSON.stringify(image.locator)) {
    results.push({
      id: `${image.journey}/${image.step}`,
      state: 'failed',
      error: `identical image to ${same.journey}/${same.step} despite a different locator; `
        + 'the click marker is probably not being drawn',
    });
  }
  if (!same) byHash.set(image.sha256, image);
}

// Keep what the desktop harness captured. Writing this file from one runner
// while the other owns half its rows would delete those rows on every run, and
// the deletion would look exactly like a journey that had never been captured.
const desktopJourneys = new Set(
  catalogue.journeys.filter((journey) => journey.driver === 'desktop').map((journey) => journey.id));
let existing = { images: [] };
try {
  existing = JSON.parse(await fs.readFile(manifestPath, 'utf8'));
} catch {
  // A first run has nothing to keep.
}
for (const image of existing.images ?? []) {
  if (desktopJourneys.has(image.journey)) manifest.push(image);
}

manifest.sort((left, right) =>
  left.journey.localeCompare(right.journey) || left.step.localeCompare(right.step));
await fs.writeFile(manifestPath, JSON.stringify({
  schema: 'notrios.docjourneys.gui-images.v1',
  viewport: catalogue.viewport,
  images: manifest,
}, null, 2) + '\n');

const failed = results.filter((item) => item.state !== 'executed' && item.state !== 'desktop');
console.log(JSON.stringify({ results, images: manifest.length }, null, 2));
if (failed.length) process.exit(1);
