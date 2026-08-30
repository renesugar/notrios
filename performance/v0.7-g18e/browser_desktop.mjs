import {
  api, assert, createDocument, createNotebook, execute, getDocument, openByTitle,
  rawStatus, saveEditor, searchAPI, setEditorBody,
} from './browser_harness.mjs';

const token = 'g18e-browser';
const unique = (name) => `G18e ${name}`;

async function reload(page, base) {
  await page.goto(base, { waitUntil: 'domcontentloaded' });
  await page.locator('main.app-shell').waitFor();
}

export async function runDesktopJourneys(page, request, base) {
  const results = [];
  const run = async (id, fn) => {
    const result = await execute(id, 'desktop', page, fn);
    results.push(result);
    if (result.state === 'failed') throw new Error(`${id}: ${result.error}`);
  };

  const opened = await createDocument(request, base, unique('Open result'), `${token} open body`);
  await run('search-open', async (a) => {
    await openByTitle(page, opened.title, a);
    assert(await page.getByTestId('pane-preview').getByText(`${token} open body`).isVisible(), 'selected body missing from preview');
    const searched = await searchAPI(request, base, `"${opened.title}"`);
    assert(searched.hits.some((hit) => hit.id === opened.id), 'search API omitted selected document');
    assert((await getDocument(request, base, opened.id)).body === `${token} open body`, 'canonical body differs');
    return { visible: 2, canonical: 2 };
  });

  for (let i = 0; i < 27; i++) await createDocument(request, base, `G18e Page ${String(i).padStart(2, '0')}`, `${token} pagination`);
  await run('search-page', async (a) => {
    const field = page.getByLabel('Search query');
    await a.fill(field, '"g18e" pagination', 'search-query');
    await a.press(field, 'Enter');
    await page.locator('.result-card').first().waitFor();
    await page.getByTestId('search-loading').waitFor({ state: 'hidden' }).catch(() => {});
    const firstIDs = await page.locator('.result-card').evaluateAll((rows) => rows.map((row) => row.textContent));
    if (firstIDs.length <= 20) {
      const load = page.getByRole('button', { name: 'Load more' });
      if (await load.isVisible().catch(() => false)) await a.click(load);
      await page.waitForFunction((count) => document.querySelectorAll('.result-card').length > count, firstIDs.length, { timeout: 10_000 });
    }
    const allIDs = await page.locator('.result-card').evaluateAll((rows) => rows.map((row) => row.textContent));
    assert(allIDs.length > 20 && new Set(allIDs).size === allIDs.length, 'next page did not extend results with unique hits');
    const first = await searchAPI(request, base, '"g18e" pagination', 20);
    assert(first.next_cursor, 'canonical first page has no cursor');
    const second = await searchAPI(request, base, '"g18e" pagination', 20, first.next_cursor);
    assert(second.hits.length > 0 && second.hits.every((hit) => !first.hits.some((old) => old.id === hit.id)), 'canonical pages overlap');
    return { visible: 1, canonical: 2 };
  });

  const createNB = await createNotebook(request, base, unique('Creation notebook'));
  await reload(page, base);
  await run('new-note-in-notebook', async (a) => {
    await a.click(page.getByTestId(`sidebar-row-${createNB.id}`));
    await a.click(page.getByTestId('new-note-button'));
    await a.fill(page.getByLabel('Note title'), unique('Created note'), 'note-title');
    await setEditorBody(page, `${token} created body`, a);
    await saveEditor(page, a);
    assert(await page.getByTestId('notebook-picker-trigger').getByText(createNB.name).isVisible(), 'created note notebook is not shown');
    const found = await searchAPI(request, base, `"${unique('Created note')}"`);
    assert(found.hits.length === 1, 'created note missing from canonical search');
    const doc = await getDocument(request, base, found.hits[0].id);
    assert(doc.notebook_id === createNB.id && doc.body === `${token} created body`, 'created note canonical notebook/body differs');
    return { visible: 2, canonical: 2 };
  });

  const edited = await createDocument(request, base, unique('Edit note'), `${token} before edit`);
  await run('edit-save', async (a) => {
    await openByTitle(page, edited.title, a);
    const before = await getDocument(request, base, edited.id);
    await setEditorBody(page, `${token} after edit`, a);
    await saveEditor(page, a);
    await reload(page, base);
    await openByTitle(page, edited.title, a);
    assert(await page.locator('.cm-content').getByText(`${token} after edit`).isVisible(), 'saved text missing after reload');
    const after = await getDocument(request, base, edited.id);
    assert(after.body === `${token} after edit` && after.current_revision_id !== before.current_revision_id, 'canonical revision did not change');
    return { visible: 1, canonical: 2 };
  });

  const sourceNB = await createNotebook(request, base, unique('Move source'));
  const destNB = await createNotebook(request, base, unique('Move destination'));
  const moved = await createDocument(request, base, unique('Move note'), `${token} move`, sourceNB.id);
  await reload(page, base);
  await run('move-note', async (a) => {
    await openByTitle(page, moved.title, a);
    await a.click(page.getByTestId('notebook-picker-trigger'));
    await a.click(page.getByTestId(`notebook-option-${destNB.id}`));
    await page.getByRole('status').filter({ hasText: /Filed/ }).waitFor();
    assert(await page.getByTestId('notebook-picker-trigger').getByText(destNB.name).isVisible(), 'destination notebook not visible');
    assert((await getDocument(request, base, moved.id)).notebook_id === destNB.id, 'canonical notebook did not move');
    return { visible: 2, canonical: 1 };
  });

  const trash = await createDocument(request, base, unique('Trash restore'), `${token} trash restore`);
  await run('trash-restore', async (a) => {
    await openByTitle(page, trash.title, a);
    await a.confirm(page.getByTestId('delete-button'), /Move .* to the Trash.*restore/s, true);
    await page.getByRole('status').filter({ hasText: /Moved .* to the Trash/ }).waitFor();
    const trashed = await getDocument(request, base, trash.id);
    assert(trashed.deleted_at, 'canonical note is not marked trashed');
    const field = page.getByLabel('Search query');
    await a.fill(field, `is:trashed "${trash.title}"`, 'trash-search');
    await a.press(field, 'Enter');
    await a.click(page.locator('.result-card').filter({ hasText: trash.title }));
    assert(await page.getByTestId('trashed-badge').isVisible(), 'trash badge missing');
    await a.click(page.getByTestId('restore-button'));
    await page.getByRole('status').filter({ hasText: /Restored/ }).waitFor();
    assert((await getDocument(request, base, trash.id)).deleted_at == null, 'restore did not clear deleted_at');
    return { visible: 3, canonical: 2 };
  });

  const purge = await createDocument(request, base, unique('Purge note'), `${token} purge`);
  await api(request, base, 'DELETE', `/api/v1/documents/${purge.id}?base_revision_id=${encodeURIComponent(purge.current_revision_id)}`, undefined, 204);
  await run('purge-note', async (a) => {
    const field = page.getByLabel('Search query');
    await a.fill(field, `is:trashed "${purge.title}"`, 'trash-search');
    await a.press(field, 'Enter');
    await a.click(page.locator('.result-card').filter({ hasText: purge.title }));
    await a.confirm(page.getByTestId('purge-button'), /Permanently delete.*cannot be undone/s, true);
    await page.getByRole('status').filter({ hasText: /Permanently deleted/ }).waitFor();
    assert((await page.getByLabel('Note title').inputValue()) === 'New note' && !(await page.getByTestId('purge-button').count()), 'purged note remains selected');
    const remaining = await api(request, base, 'GET', '/api/v1/trash?limit=100', undefined, 200);
    assert(!remaining.documents.some((document) => document.id === purge.id), 'purged document remains in Trash');
    return { visible: 2, canonical: 1 };
  });

  const deleteNB = await createNotebook(request, base, unique('Delete notebook'));
  const deleteDoc = await createDocument(request, base, unique('Notebook child'), `${token} notebook child`, deleteNB.id);
  await reload(page, base);
  await run('delete-notebook-review', async (a) => {
    const preview = await api(request, base, 'GET', `/api/v1/notebooks/${deleteNB.id}/deletion-preview`, undefined, 200);
    assert(preview.notes === 1 && preview.deletable, 'canonical notebook preview wrong');
    await a.confirm(page.getByTestId(`sidebar-delete-${deleteNB.id}`), /1 note.*Trash/s, true);
    await page.getByRole('status').filter({ hasText: /Deleted/ }).waitFor();
    await page.getByTestId(`sidebar-row-${deleteNB.id}`).waitFor({ state: 'detached' });
    assert(!(await page.getByTestId(`sidebar-row-${deleteNB.id}`).count()), 'deleted notebook remains in sidebar');
    assert((await rawStatus(request, base, 'GET', `/api/v1/notebooks/${deleteNB.id}`)).status === 404, 'deleted notebook still canonical');
    const trashSearch = await searchAPI(request, base, `is:trashed "${deleteDoc.title}"`);
    assert(trashSearch.hits.some((hit) => hit.id === deleteDoc.id), 'child note was not moved to Trash');
    return { visible: 2, canonical: 3 };
  });

  await run('protected-items', async (a) => {
    await a.click(page.getByTestId('sidebar-row-nb_help'));
    const result = page.locator('.result-card').first();
    await result.waitFor();
    await a.click(result);
    await page.getByTestId('readonly-badge').waitFor();
    assert(await page.getByTestId('readonly-badge').isVisible(), 'Help note is not marked read-only');
    assert(await page.getByLabel('Note title').getAttribute('readonly') !== null, 'Help title is editable');
    const help = await api(request, base, 'GET', `/api/v1/documents/${await page.getByTestId('note-inspector').locator('dd').first().textContent()}`, undefined, 200);
    const refused = await rawStatus(request, base, 'PUT', `/api/v1/documents/${help.id}`, { title: help.title, body: 'mutated', base_revision_id: help.current_revision_id });
    assert(refused.status === 403 && (await getDocument(request, base, help.id)).body === help.body, 'protected canonical body changed');
    return { visible: 2, canonical: 2 };
  });

  await run('themes', async (a) => {
    await a.click(page.getByLabel('Theme settings'));
    const panel = page.getByRole('region', { name: 'Theme settings' });
    await panel.waitFor();
    await a.select(panel.getByLabel('Light mode uses'), 'Dark');
    await a.click(page.getByLabel('Toggle light/dark theme'));
    assert(await panel.isVisible(), 'theme panel closed unexpectedly');
    const stored = await page.evaluate(() => localStorage.getItem('notrios.lightTheme'));
    assert(stored === 'Dark', `theme storage is ${stored}`);
    await reload(page, base);
    assert(await page.evaluate(() => localStorage.getItem('notrios.lightTheme')) === 'Dark', 'theme choice did not survive reload');
    return { visible: 1, canonical: 2 };
  });

  const linkTarget = await createDocument(request, base, unique('Link target'), '# Link target');
  const linkSource = await createDocument(request, base, unique('Link source'), `${token} link source`);
  await run('insert-and-check-link', async (a) => {
    await openByTitle(page, linkSource.title, a);
    await a.click(page.getByTestId('note-inspector').locator('summary'));
    await a.fill(page.getByLabel('Search notes to link'), 'Link target', 'link-picker-query');
    await a.click(page.getByTestId('link-suggestions').getByRole('button', { name: linkTarget.title }));
    await page.getByTestId('link-check-clean').getByText(/All 1 link/).waitFor();
    assert(await page.getByTestId('link-check-clean').getByText(/All 1 link/).isVisible(), 'link check did not report a resolved link');
    await saveEditor(page, a);
    const canonical = await getDocument(request, base, linkSource.id);
    assert(canonical.body.includes('document://') && canonical.body.includes(linkTarget.id), 'stable document link missing from body');
    return { visible: 2, canonical: 1 };
  });

  await run('preview-links', async (a) => {
    await openByTitle(page, linkSource.title, a);
    const previewLink = page.getByTestId('pane-preview').getByRole('link', { name: linkTarget.title });
    await a.click(previewLink);
    assert((await page.getByLabel('Note title').inputValue()) === linkTarget.title, 'preview link did not open target');
    assert((await getDocument(request, base, linkTarget.id)).title === linkTarget.title, 'preview target canonical lookup failed');
    return { visible: 1, canonical: 1 };
  });

  const resourceDoc = await createDocument(request, base, unique('Resource note'), `${token} resource`);
  await run('upload-resource', async (a) => {
    await openByTitle(page, resourceDoc.title, a);
    await a.click(page.getByTestId('note-inspector').locator('summary'));
    await a.setInputFiles(page.getByLabel('Upload image/PDF/resource'), { name: 'g18e.txt', mimeType: 'text/plain', buffer: Buffer.from('G18e resource bytes') }, 'resource-file');
    await page.getByRole('status').filter({ hasText: /Uploaded and attached/ }).waitFor();
    const inspector = page.getByTestId('note-inspector');
    if (!(await inspector.evaluate((element) => element.open))) await a.click(inspector.locator('summary'));
    const attached = inspector.locator('li').filter({ hasText: 'attachment' }).getByRole('link', { name: 'g18e.txt' });
    await attached.waitFor();
    assert(await attached.isVisible(), 'attached resource not visible');
    await saveEditor(page, a);
    const refs = await api(request, base, 'GET', `/api/v1/documents/${resourceDoc.id}/resources`, undefined, 200);
    assert(refs.resources.length === 1 && refs.resources[0].resource.filename === 'g18e.txt', 'canonical resource reference missing');
    return { visible: 2, canonical: 1 };
  });

  const remote = await createDocument(request, base, unique('Remote media'), '![blocked](http://127.0.0.1/private.png)');
  await run('localize-remote-media', async (a) => {
    await openByTitle(page, remote.title, a);
    const inspector = page.getByTestId('note-inspector');
    await a.click(inspector.locator('summary'));
    const media = page.getByTestId('remote-media-list');
    await media.waitFor({ state: 'attached' });
    if (!(await inspector.evaluate((element) => element.open))) await a.click(inspector.locator('summary'));
    await media.waitFor();
    assert(await media.getByText('block', { exact: true }).isVisible(), 'blocked SSRF media decision not visible');
    assert(!(await page.getByTestId('localize-button').count()), 'blocked media incorrectly offers localization');
    const before = await getDocument(request, base, remote.id);
    const scan = await api(request, base, 'POST', `/api/v1/documents/${remote.id}/remote-media/scan`, {}, 200);
    assert(scan.media.some((decision) => decision.action === 'block') && (await getDocument(request, base, remote.id)).current_revision_id === before.current_revision_id, 'policy scan mutated canonical note or omitted block');
    return { visible: 2, canonical: 2 };
  });

  const graphA = await createDocument(request, base, unique('Graph A'), `${token} graph`);
  const graphB = await createDocument(request, base, unique('Graph B'), `[Graph A](document://default/documents/${graphA.id})`);
  await run('inspect-local-graph', async (a) => {
    await openByTitle(page, graphB.title, a);
    const inspector = page.getByTestId('note-inspector');
    await a.click(inspector.locator('summary'));
    const graph = page.getByTestId('local-graph');
    await graph.waitFor({ state: 'attached' });
    if (!(await inspector.evaluate((element) => element.open))) await a.click(inspector.locator('summary'));
    await a.select(page.getByLabel('Hops from this note'), '2');
    await graph.getByText(graphA.title).waitFor();
    assert(await graph.getByTestId('local-graph-depth-1').isVisible(), 'one-hop group absent');
    const canonical = await api(request, base, 'POST', '/api/v1/graph', { roots: [graphB.id], depth: 2, direction: 'both' }, 200);
    assert(canonical.nodes.some((node) => node.id === graphA.id), 'graph API omitted target');
    return { visible: 2, canonical: 1 };
  });

  const table = await createDocument(request, base, unique('Table paste'), 'Before table\n\n');
  await run('paste-table', async (a) => {
    await openByTitle(page, table.title, a);
    const editor = page.locator('.cm-content').first();
    await a.click(editor);
    await a.press(editor, 'ControlOrMeta+End');
    await a.pasteHTML(editor, '<table><tr><th>Name</th><th>Value</th></tr><tr><td>alpha</td><td>1</td></tr></table>', 'pasted-table');
    await page.waitForFunction(() => document.querySelector('.cm-content')?.textContent?.includes('| Name | Value |'));
    await saveEditor(page, a);
    assert(await page.getByTestId('pane-preview').locator('table').isVisible(), 'pasted table is not rendered');
    assert((await getDocument(request, base, table.id)).body.includes('| Name | Value |'), 'canonical body lacks Markdown table');
    return { visible: 1, canonical: 1 };
  });

  const queryDoc = await createDocument(request, base, unique('Live query'), '```note-query\nquery: g18e pagination\nlimit: 20\n```');
  await run('live-query-block', async (a) => {
    await openByTitle(page, queryDoc.title, a);
    await a.click(page.getByTestId('pane-preview'));
    const block = page.getByTestId('pane-preview').locator('[data-note-query-state="ok"]');
    await block.waitFor();
    assert((await block.locator('li').count()) > 0, 'live query has no rendered results');
    assert((await getDocument(request, base, queryDoc.id)).body.includes('```note-query'), 'canonical query block changed');
    return { visible: 1, canonical: 1 };
  });

  const rich = await createDocument(request, base, unique('Math code'), '$$x^2$$\n\n```javascript\nconst answer = 42;\n```');
  await run('math-and-code', async (a) => {
    await openByTitle(page, rich.title, a);
    await a.click(page.getByTestId('pane-preview'));
    assert(await page.getByTestId('pane-preview').locator('.katex').isVisible(), 'KaTeX output missing');
    assert(await page.getByTestId('pane-preview').locator('code.language-javascript').isVisible(), 'highlighted code missing');
    assert((await getDocument(request, base, rich.id)).body.includes('const answer = 42'), 'canonical math/code body differs');
    return { visible: 2, canonical: 1 };
  });

  await run('resize-panes', async (a) => {
    const splitter = page.getByRole('separator', { name: 'Resize sidebar' });
    await splitter.focus();
    const before = Number(await splitter.getAttribute('aria-valuenow'));
    await a.press(splitter, 'ArrowRight');
    const changed = Number(await splitter.getAttribute('aria-valuenow'));
    assert(changed > before, 'ArrowRight did not resize pane');
    await a.press(splitter, 'Home');
    assert(Number(await splitter.getAttribute('aria-valuenow')) < changed, 'Home did not reach minimum');
    await a.press(splitter, 'ArrowRight');
    const stored = await page.evaluate(() => localStorage.getItem('notrios.paneWidths.v1'));
    assert(stored && JSON.parse(stored).sidebar > 0, 'pane width not persisted');
    return { visible: 2, canonical: 1 };
  });

  const cancelDoc = await createDocument(request, base, unique('Cancel destructive'), `${token} cancel`);
  await run('destructive-confirmations', async (a) => {
    await openByTitle(page, cancelDoc.title, a);
    const before = await getDocument(request, base, cancelDoc.id);
    await a.confirm(page.getByTestId('delete-button'), /Move .* to the Trash/s, false);
    a.recovery();
    assert((await page.getByLabel('Note title').inputValue()) === cancelDoc.title, 'cancelled delete changed visible selection');
    assert((await getDocument(request, base, cancelDoc.id)).current_revision_id === before.current_revision_id, 'cancelled delete mutated canonical note');
    return { visible: 1, canonical: 1 };
  });

  return results;
}
