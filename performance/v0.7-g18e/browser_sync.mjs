import {
  api, assert, createDocument, execute, getDocument, queueSync, rawStatus, recorder,
  searchAPI, waitJob,
} from './browser_harness.mjs';

async function openSync(page, actions) {
  await actions.click(page.getByRole('button', { name: /Sync$/ }));
  const dialog = page.getByRole('dialog', { name: /Sync center/i });
  await dialog.waitFor();
  return dialog;
}

async function closeSync(page, actions) {
  const close = page.getByLabel('Close sync center');
  if (await close.isVisible().catch(() => false)) await actions.click(close);
}

async function tab(page, actions, name) {
  await actions.click(page.getByRole('button', { name, exact: typeof name === 'string' }));
}

async function refreshPage(page, base) {
  await page.goto(base, { waitUntil: 'domcontentloaded' });
  await page.locator('main.app-shell').waitFor();
}

export async function runSyncJourneys({ hostPage, joinerPage, request, hostBase, joinerBase, viewport }) {
  const results = [];
  const run = async (id, fn) => {
    const result = await execute(id, viewport, joinerPage, fn);
    results.push(result);
    if (result.state === 'failed') throw new Error(`${id}: ${result.error}`);
  };
  let backupPath = '';
  let pairedReplica = '';
  let resourceID = '';
  let conflictDocID = '';

  await run('open-sync-center', async (a) => {
    const dialog = await openSync(joinerPage, a);
    assert(await dialog.getByText(/Library .*…/).isVisible(), 'profile library identity missing');
    if (viewport === 'desktop') assert(await dialog.getByText(/Replica .*…/).isVisible(), 'profile replica identity missing');
    const close = joinerPage.getByLabel('Close sync center');
    const box = await close.boundingBox();
    assert(box && box.width >= 44 && box.height >= 44, `close target is ${box?.width}x${box?.height}, want at least 44x44`);
    await a.press(close, 'Escape');
    await dialog.waitFor({ state: 'hidden' });
    a.recovery();
    const status = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    assert(status.active_profile.database_id && status.active_profile.replica_id, 'sync status omitted active identity');
    return { visible: 4, canonical: 1 };
  });

  await run('sync-setup', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, 'Setup');
    await a.check(dialog.getByLabel('REST peer'));
    await a.fill(dialog.getByLabel('Peer address'), hostBase, 'peer-address');
    await a.click(dialog.getByRole('button', { name: 'Save transport' }));
    await dialog.getByRole('status').filter({ hasText: /Configuration saved/ }).waitFor();
    assert(await dialog.getByText(/Restart this profile/).isVisible(), 'restart guidance missing');
    const status = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    assert(status.configuration.target === 'rest' && status.configuration.rest_base_url === hostBase, 'transport was not persisted');
    await closeSync(joinerPage, a);
    return { visible: 2, canonical: 2 };
  });

  await run('sync-pairing', async (joinerActions, state) => {
    const hostActions = recorder(hostPage, state);
    const hostDialog = await openSync(hostPage, hostActions);
    await tab(hostPage, hostActions, 'Peers');
    await hostActions.fill(hostDialog.getByLabel('Private label'), `G18e ${viewport}`, 'private-label');
    await hostActions.click(hostDialog.getByRole('button', { name: 'Create 15-minute code' }));
    const codeNode = hostDialog.locator('.pairing-code code');
    await codeNode.waitFor();
    const code = (await codeNode.textContent()).trim();
    assert(code.length > 0, 'invitation code missing');

    const joinerDialog = await openSync(joinerPage, joinerActions);
    await tab(joinerPage, joinerActions, 'Peers');
    await joinerActions.fill(joinerDialog.getByLabel('Peer address'), hostBase, 'peer-address');
    await joinerActions.fill(joinerDialog.getByLabel('Single-use code'), code, 'single-use-code');
    await joinerActions.click(joinerDialog.getByRole('button', { name: 'Pair explicitly' }));
    await joinerDialog.getByRole('status').filter({ hasText: /peer is paired/i }).waitFor();
    const joinStatus = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    assert(joinStatus.peers.length === 1, `joiner peer count ${joinStatus.peers.length}, want 1`);
    const hostStatus = await api(request, hostBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    pairedReplica = joinStatus.active_profile.replica_id;
    assert(hostStatus.peers.some((peer) => peer.replica_id === pairedReplica), 'host did not enroll joiner replica');
    const replay = await rawStatus(request, joinerBase, 'POST', '/api/v1/sync-ui/pair', { base_url: hostBase, code });
    assert(replay.status >= 400, 'single-use invitation was accepted twice');
    await closeSync(joinerPage, joinerActions);
    await refreshPage(hostPage, hostBase);
    const freshHostDialog = await openSync(hostPage, hostActions);
    await tab(hostPage, hostActions, 'Peers');
    const allow = freshHostDialog.getByRole('button', { name: 'Allow catch-up snapshot' });
    if (await allow.isVisible().catch(() => false)) {
      await hostActions.click(allow);
      await freshHostDialog.getByRole('status').filter({ hasText: /may now request/ }).waitFor();
    }
    await closeSync(hostPage, hostActions);
    return { visible: 3, canonical: 3 };
  });

  await run('sync-jobs', async (a) => {
    const dialog = await openSync(joinerPage, a);
    const response = joinerPage.waitForResponse((item) => item.url().endsWith('/api/v1/jobs/sync/start') && item.request().method() === 'POST');
    await a.click(dialog.getByRole('button', { name: 'Sync now' }));
    const started = await (await response).json();
    await dialog.getByRole('status').filter({ hasText: /queued/ }).waitFor();
    const job = await waitJob(request, joinerBase, started.job.id);
    await refreshPage(joinerPage, joinerBase);
    const refreshed = await openSync(joinerPage, a);
    assert(await refreshed.getByText('succeeded', { exact: true }).first().isVisible(), 'terminal sync state not visible');
    const status = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    assert(job.state === 'succeeded' && status.jobs.some((item) => item.id === job.id && item.state === 'succeeded'), 'terminal job absent from sync status');
    await closeSync(joinerPage, a);
    return { visible: 2, canonical: 2 };
  });

  // Fixture mechanics are deliberately outside the measured action stream.
  const shared = (await searchAPI(request, hostBase, `"Shared fixture ${viewport === 'desktop' ? 'desktop' : 'narrow'}"`)).hits[0];
  assert(shared, 'launcher shared fixture missing');
  const common = await getDocument(request, hostBase, shared.id);
  await api(request, hostBase, 'PUT', `/api/v1/documents/${shared.id}`, { title: common.title, body: `host divergent ${viewport}`, base_revision_id: common.current_revision_id }, 200);
  await api(request, joinerBase, 'PUT', `/api/v1/documents/${shared.id}`, { title: common.title, body: `joiner divergent ${viewport}`, base_revision_id: common.current_revision_id }, 200);
  conflictDocID = shared.id;
  const attachmentDoc = await createDocument(request, hostBase, `G18e Lazy ${viewport}`, 'lazy attachment');
  const resource = await api(request, hostBase, 'POST', '/api/v1/resources?filename=g18e-lazy.txt', Buffer.from(`lazy bytes ${viewport}`), 201, { 'content-type': 'text/plain' });
  resourceID = resource.id;
  await api(request, hostBase, 'POST', `/api/v1/documents/${attachmentDoc.id}/resources/${resourceID}`, { relation_type: 'attachment', ordinal: 0 }, 200);
  await queueSync(request, joinerBase);
  await queueSync(request, hostBase);
  await queueSync(request, joinerBase);
  await refreshPage(joinerPage, joinerBase);

  await run('sync-lazy-resource', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, /Attachments/);
    const row = dialog.locator('.resource-row').filter({ hasText: 'g18e-lazy.txt' });
    await row.waitFor();
    const download = row.getByRole('button', { name: 'Download' });
    await download.waitFor();
    assert(/not on this device|pending/i.test(await row.innerText()), `lazy attachment state is not visibly unavailable: ${await row.innerText()}`);
    const response = joinerPage.waitForResponse((item) => item.url().includes(`/resources/${resourceID}/intent`) && item.request().method() === 'POST');
    await a.click(download);
    const intent = await (await response).json();
    await dialog.getByRole('status').filter({ hasText: /Requested/ }).waitFor();
    const status = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    const current = status.resources.find((item) => item.id === resourceID);
    assert(current?.requested === true && intent.job?.id, 'resource intent or fetch job missing');
    await closeSync(joinerPage, a);
    return { visible: 2, canonical: 2 };
  });

  await run('sync-conflict', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, /Conflicts/);
    const conflict = dialog.locator('.conflict-row').filter({ hasText: conflictDocID });
    await a.click(conflict);
    assert((await dialog.getByLabel('Common base').inputValue()).includes('G18e'), 'common base not visible');
    assert((await dialog.getByLabel('Current on this profile').inputValue()).includes('joiner divergent'), 'local head not visible');
    assert((await dialog.getByLabel('Other revision').inputValue()).includes('host divergent'), 'other head not visible');
    await a.fill(dialog.getByLabel('Your resolved note'), `explicit resolution ${viewport}`, 'conflict-resolution');
    await a.click(dialog.getByRole('button', { name: 'Save resolution' }));
    await dialog.getByRole('status').filter({ hasText: /Conflict resolved/ }).waitFor();
    const status = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    const document = await getDocument(request, joinerBase, conflictDocID);
    assert(!status.conflicts.some((item) => item.document_id === conflictDocID) && document.body === `explicit resolution ${viewport}`, 'canonical conflict was not resolved');
    await closeSync(joinerPage, a);
    return { visible: 4, canonical: 2 };
  });

  const password = `g18e backup ${viewport}`;
  await run('sync-backup-create', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, 'Backup & recovery');
    const create = dialog.locator('.sync-card').filter({ hasText: 'Create a password-protected backup' });
    const passwords = create.locator('input[type=password]');
    await a.fill(passwords.nth(0), password, 'backup-password');
    await a.fill(passwords.nth(1), password, 'backup-confirm-password');
    const downloadPromise = joinerPage.waitForEvent('download');
    const responsePromise = joinerPage.waitForResponse((item) => item.url().endsWith('/api/v1/sync-ui/backups') && item.request().method() === 'POST');
    await a.click(create.getByRole('button', { name: 'Create and download' }));
    const download = await downloadPromise;
    const response = await responsePromise;
    backupPath = await download.path();
    assert(backupPath, 'browser download has no path');
    await dialog.getByRole('status').filter({ hasText: /Verified encrypted backup created/ }).waitFor();
    assert(response.status() === 200 && response.headers()['content-type'].includes('application/vnd.notrios.password-backup'), 'backup response type/status wrong');
    const bytes = await import('node:fs/promises').then(({ stat }) => stat(backupPath));
    assert(bytes.size > 0, 'downloaded backup is empty');
    await closeSync(joinerPage, a);
    return { visible: 1, canonical: 2 };
  });

  await run('sync-backup-inspect', async (a) => {
    const before = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, 'Backup & recovery');
    const inspect = dialog.locator('.sync-card').filter({ hasText: 'Review a backup for restore' });
    await a.setInputFiles(inspect.locator('input[type=file]'), backupPath, 'backup-file');
    await a.fill(inspect.locator('input[type=password]'), 'wrong g18e password', 'backup-open-password');
    await a.click(inspect.getByRole('button', { name: 'Verify for review' }));
    await dialog.getByRole('status').filter({ hasText: /password|decrypt|unauthorized/i }).waitFor();
    a.recovery();
    await a.fill(inspect.locator('input[type=password]'), password, 'backup-open-password');
    await a.click(inspect.getByRole('button', { name: 'Verify for review' }));
    const review = dialog.getByRole('region', { name: 'Destructive restore review' });
    await review.waitFor();
    assert(await review.getByText(/Verified—destructive review required/).isVisible(), 'verified review missing');
    assert(await review.getByRole('button', { name: 'Apply requires profile shutdown' }).isDisabled(), 'web restore apply is not disabled');
    const after = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    assert(after.active_profile.database_id === before.active_profile.database_id && after.active_profile.replica_id === before.active_profile.replica_id, 'backup inspection mutated identity');
    await closeSync(joinerPage, a);
    return { visible: 3, canonical: 2 };
  });

  await run('sync-recovery-review', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, 'Backup & recovery');
    const button = dialog.getByRole('button', { name: 'Request reset review' });
    await a.confirm(button, /Nothing will be replaced.*separate destructive review/s, false);
    a.recovery();
    const before = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    const responsePromise = joinerPage.waitForResponse((item) => item.url().endsWith('/api/v1/sync-ui/recovery') && item.request().method() === 'POST');
    await a.confirm(button, /Request a reset snapshot/s, true);
    const response = await responsePromise;
    const payload = await response.json();
    await dialog.getByRole('status').filter({ hasText: /Reset preparation was queued/ }).waitFor();
    assert(response.status() === 202 && payload.destructive_review_required === true && payload.job?.id, 'reset response omitted destructive review/job');
    const after = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    assert(after.active_profile.database_id === before.active_profile.database_id, 'reset review changed canonical library');
    await closeSync(joinerPage, a);
    return { visible: 2, canonical: 2 };
  });

  await run('sync-retention', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, 'Retention');
    await dialog.getByRole('heading', { name: 'Safe retention horizon' }).waitFor();
    await dialog.getByText(/days of history/).waitFor();
    assert(await dialog.getByText(/Peer watermarks/).isVisible(), 'peer watermarks missing');
    const report = await api(request, joinerBase, 'GET', '/api/v1/sync-ui/retention', undefined, 200);
    assert(Number.isFinite(report.history_seconds) && Array.isArray(report.peers), 'retention report omitted floors/peers');
    await closeSync(joinerPage, a);
    return { visible: 3, canonical: 1 };
  });

  await run('sync-repairs', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, 'Repairs');
    await dialog.getByRole('heading', { name: 'Notebook repair reports' }).waitFor();
    const status = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    if (status.repairs.length === 0) assert(await dialog.getByText('No synchronization repairs have been recorded.').isVisible(), 'empty repair state missing');
    else assert((await dialog.locator('.repair-row').count()) === status.repairs.length, 'visible repair rows differ from status');
    await closeSync(joinerPage, a);
    return { visible: 2, canonical: 1 };
  });

  await run('sync-retirement', async (a) => {
    const dialog = await openSync(joinerPage, a);
    await tab(joinerPage, a, 'Peers');
    await a.click(dialog.getByRole('button', { name: 'Review retirement' }).first());
    const review = dialog.getByRole('region', { name: 'Peer retirement review' });
    await review.waitFor();
    assert(await review.getByText('That device must reset and pair as a new replica.').isVisible(), 'retirement impact missing');
    await a.fill(review.getByLabel('Owner-visible reason (optional)'), `G18e ${viewport} retirement`, 'retirement-reason');
    await a.check(review.getByLabel(/I understand this peer must reset/));
    await a.click(review.getByRole('button', { name: 'Retire peer' }));
    await dialog.getByRole('status').filter({ hasText: /Peer retired/ }).waitFor();
    const status = await api(request, joinerBase, 'GET', '/api/v1/sync-ui', undefined, 200);
    assert(status.peers.some((peer) => peer.status === 'retired'), 'canonical peer is not retired');
    await closeSync(joinerPage, a);
    return { visible: 2, canonical: 1 };
  });

  assert(pairedReplica && resourceID && conflictDocID, 'sync fixture state was incomplete');
  return results;
}
