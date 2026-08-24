import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SyncCenter } from '../components/SyncCenter';
import type { SyncUIStatus } from '../api';

const baseStatus: SyncUIStatus = {
  status: 'attention',
  active_profile: { name: 'Work', profile_id: 'profile_work', database_id: 'db_1234567890abcdef', replica_id: 'rep_1234567890abcdef' },
  profiles: [
    { name: 'Personal', profile_id: 'profile_personal', database_id: 'db_personal', public_base_url: 'http://127.0.0.1:18192', sync_target: 'directory', active: false },
    { name: 'Work', profile_id: 'profile_work', database_id: 'db_1234567890abcdef', public_base_url: 'http://127.0.0.1:18191', sync_target: 'rest', active: true },
  ],
  configuration: { target: 'rest', rest_base_url: 'http://127.0.0.1:18192', rest_inbound_enabled: true, restart_required: false },
  journal_enabled: true,
  secret_store: { available: true, configured: true, name: 'locked-file-development', warning: 'Owner-only 0600 development file; not an OS keychain.' },
  peers: [{ replica_id: 'rep_peer_abcdefghijkl', status: 'behind', behind_operations: 3, snapshot_permitted: false }],
  jobs: [{ id: 'job_old', kind: 'sync_incremental', state: 'failed', phase: 'pull', processed: 2, total: 5, error: 'offline', created_at: '2026-08-24T16:00:00Z' }],
  conflicts: [{ id: 'conflict_1', document_id: 'doc_shared', revision_a: 'rev_a', revision_b: 'rev_b', base_revision_id: 'rev_base', kind: 'same_token', created_at: '2026-08-24T16:10:00Z' }],
  resources: [{ id: 'res_pdf', filename: 'paper.pdf', mime_type: 'application/pdf', size_bytes: 4096, availability: 'unavailable', pinned: false, requested: false }],
  repairs: [{ id: 'repair_1', kind: 'notebook.cycle', subject_id: 'nb_work', details: { effective: 'nb_recovered' } }],
};

function json(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
}

function mockFetch(extra?: (path: string, init?: RequestInit) => Response | undefined) {
  const calls: Array<{ path: string; init?: RequestInit }> = [];
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    calls.push({ path, init });
    const override = extra?.(path, init);
    if (override) return override;
    if (path === '/api/v1/sync-ui') return json(baseStatus);
    if (path === '/api/v1/jobs/sync/start') return json({ job: { id: 'job_new' } }, 202);
    if (path.endsWith('/retry')) return json({ job: { id: 'job_old', state: 'queued' } });
    return json({});
  });
  vi.stubGlobal('fetch', fetch);
  return { fetch, calls };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('SyncCenter', () => {
  it('keeps active profile identity visible and exposes keyboard-labelled responsive sections', async () => {
    mockFetch();
    const onClose = vi.fn();
    render(<SyncCenter onClose={onClose} />);
    expect(await screen.findByRole('dialog', { name: 'Sync center' })).toBeInTheDocument();
    expect(screen.getByText('Work', { selector: 'strong' })).toBeInTheDocument();
    expect(screen.getByText(/Library db_1234567/)).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Synchronization sections' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Conflicts 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Attachments 1' })).toBeInTheDocument();
    await userEvent.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('starts and retries only explicit durable jobs', async () => {
    const { calls } = mockFetch();
    render(<SyncCenter onClose={() => undefined} />);
    await screen.findByText('Needs attention');
    await userEvent.click(screen.getByRole('button', { name: 'Sync now' }));
    await screen.findByText('Synchronization was queued.');
    const start = calls.find((call) => call.path === '/api/v1/jobs/sync/start');
    expect(JSON.parse(String(start?.init?.body))).toEqual({ kind: 'incremental' });
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(calls.some((call) => call.path === '/api/v1/jobs/job_old/retry')).toBe(true);
  });

  it('uses the native directory chooser only when the local desktop bridge is present', async () => {
    const choose = vi.fn(async () => '/tmp/notrios-carrier');
    Object.defineProperty(window, 'go', { configurable: true, value: { main: { NativeUIBridge: { ChooseSyncDirectory: choose } } } });
    mockFetch();
    render(<SyncCenter onClose={() => undefined} />);
    await screen.findByText('Needs attention');
    await userEvent.click(screen.getByRole('button', { name: 'Setup' }));
    await userEvent.click(screen.getByRole('radio', { name: 'Shared directory' }));
    await userEvent.click(screen.getByRole('button', { name: 'Choose folder…' }));
    expect(choose).toHaveBeenCalledOnce();
    expect(await screen.findByDisplayValue('/tmp/notrios-carrier')).toBeInTheDocument();
    Reflect.deleteProperty(window, 'go');
  });

  it('compares base/current/other and submits the chosen two-parent resolution', async () => {
    const detail = {
      ...baseStatus.conflicts[0], title: 'Shared note', body_mime_type: 'text/markdown', region_count: 1,
      base: { id: 'rev_base', title: 'Shared note', body: 'base body' },
      local: { id: 'rev_a', title: 'Shared note', body: 'current body' },
      remote: { id: 'rev_b', title: 'Shared note', body: 'other body' },
    };
    const { calls } = mockFetch((path, init) => {
      if (path === '/api/v1/sync-ui/conflicts/conflict_1' && !init?.method) return json(detail);
      if (path.endsWith('/resolve')) return json({ id: 'doc_shared', title: 'Shared note', body: 'other body' });
      return undefined;
    });
    render(<SyncCenter onClose={() => undefined} />);
    await screen.findByText('Needs attention');
    await userEvent.click(screen.getByRole('button', { name: 'Conflicts 1' }));
    await userEvent.click(await screen.findByRole('button', { name: /doc_shared/i }));
    expect(await screen.findByDisplayValue('base body')).toBeInTheDocument();
    expect(screen.getByLabelText('Current on this profile')).toHaveValue('current body');
    expect(screen.getByLabelText('Other revision')).toHaveValue('other body');
    await userEvent.click(screen.getByRole('button', { name: 'Use other' }));
    await userEvent.click(screen.getByRole('button', { name: 'Save resolution' }));
    await screen.findByText(/two-parent revision/i);
    const resolved = calls.find((call) => call.path.endsWith('/resolve'));
    expect(JSON.parse(String(resolved?.init?.body))).toEqual({ title: 'Shared note', body: 'other body' });
  });

  it('copies only the single-use pairing code and never stores a password in clipboard', async () => {
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    mockFetch((path) => path === '/api/v1/sync-ui/invitations'
      ? json({ code: 'ABCD-EFGH-IJKL', expires_at: '2026-08-24T18:00:00Z', single_use: true }, 201)
      : undefined);
    render(<SyncCenter onClose={() => undefined} />);
    await screen.findByText('Needs attention');
    await userEvent.click(screen.getByRole('button', { name: 'Peers' }));
    await userEvent.click(screen.getByRole('button', { name: 'Create 15-minute code' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Copy code' }));
    expect(writeText).toHaveBeenCalledWith('ABCD-EFGH-IJKL');
    expect(writeText).toHaveBeenCalledTimes(1);
  });

  it('clears a wrong backup password while keeping retry and cancel explicit', async () => {
    mockFetch((path) => path === '/api/v1/sync-ui/backups/inspect'
      ? json({ error: { code: 'wrong_backup_password', message: 'That password did not open this backup. Nothing was changed; retry or cancel.' } }, 401)
      : undefined);
    render(<SyncCenter onClose={() => undefined} />);
    await screen.findByText('Needs attention');
    await userEvent.click(screen.getByRole('button', { name: 'Backup & recovery' }));
    const review = screen.getByRole('heading', { name: 'Review a backup for restore' }).closest('article');
    expect(review).not.toBeNull();
    const controls = within(review!);
    const file = new File(['NPB1-not-a-real-fixture'], 'backup.npb', { type: 'application/vnd.notrios.password-backup' });
    await userEvent.upload(controls.getByLabelText('Backup file'), file);
    const password = controls.getByLabelText('Password');
    await userEvent.type(password, 'wrong password');
    await userEvent.click(controls.getByRole('button', { name: 'Show' }));
    expect(password).toHaveAttribute('type', 'text');
    await userEvent.click(controls.getByRole('button', { name: 'Verify for review' }));
    expect(await screen.findByText(/Nothing was changed/)).toBeInTheDocument();
    await waitFor(() => expect(password).toHaveValue(''));
    expect(controls.getByRole('button', { name: 'Cancel' })).toBeEnabled();
  });
});
