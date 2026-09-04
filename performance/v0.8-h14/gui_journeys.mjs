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
const browser = await chromium.launch({ headless: true, executablePath: chromePath });
try {
  for (const journey of catalogue.journeys) {
    const context = await browser.newContext({
      viewport: { width: catalogue.viewport.width, height: catalogue.viewport.height },
      deviceScaleFactor: 1,
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
        const box = await target.boundingBox();
        if (!box) throw new Error(`${journey.id}/${step.id}: the element has no box to point at`);

        await page.evaluate(drawMarker, { box, pointing: step.action === 'click' || step.action === 'fill' });
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
        else if (step.action === 'fill') await target.fill(step.value ?? '');
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

manifest.sort((left, right) =>
  left.journey.localeCompare(right.journey) || left.step.localeCompare(right.step));
await fs.writeFile(manifestPath, JSON.stringify({
  schema: 'notrios.docjourneys.gui-images.v1',
  viewport: catalogue.viewport,
  images: manifest,
}, null, 2) + '\n');

const failed = results.filter((item) => item.state !== 'executed');
console.log(JSON.stringify({ results, images: manifest.length }, null, 2));
if (failed.length) process.exit(1);
