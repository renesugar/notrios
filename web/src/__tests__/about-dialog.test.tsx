// About Notrios: the build report, and the copy that carries it.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AboutDialog, describeBuild } from '../components/AboutDialog';
import type { StatusResponse } from '../api';

const status = {
  service: 'notrios', version: '0.7.0', status: 'running',
  database_info: { schema_version: 27 },
  search_sidecar: { configured: false, available: false, active: false, state: 'off', backlog: 0, failed_jobs: 0 },
} as StatusResponse;

const build = {
  version: '0.7.0', revision: 'cc4f5deb9781e7744a7622b0e13cb631142fcff5',
  built_at: '2026-09-04T12:26:57Z', modified: true,
  go_version: 'go1.27.0', platform: 'linux/amd64', tags: 'gui,desktop,production,webkit2_41',
};

function bindBridge(about = vi.fn(async () => build), actions: string[] = []) {
  Object.defineProperty(window, 'go', {
    configurable: true,
    value: { main: { NativeUIBridge: { About: about, RecentActions: vi.fn(async () => actions) } } },
  });
  return about;
}

afterEach(() => Reflect.deleteProperty(window, 'go'));

describe('about', () => {
  it('identifies the build, not just the release', async () => {
    bindBridge();
    render(<AboutDialog status={status} onClose={() => undefined} />);
    const report = await screen.findByTestId('about-report');
    // The release number alone is what makes a bug report unanswerable between
    // releases, so the revision is the field this dialog exists for.
    expect(report).toHaveTextContent('cc4f5deb9781');
    expect(report).toHaveTextContent('(modified)');
    expect(report).toHaveTextContent('Schema: 27');
  });

  it('says so rather than guessing when there is no desktop bridge', () => {
    render(<AboutDialog status={status} onClose={() => undefined} />);
    const report = screen.getByTestId('about-report');
    expect(report).toHaveTextContent('Mode: browser');
    expect(report).not.toHaveTextContent('Revision:');
  });

  it('puts the whole report on the clipboard', async () => {
    bindBridge();
    const writeText = vi.fn(async (_text: string) => undefined);
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    render(<AboutDialog status={status} onClose={() => undefined} />);
    await screen.findByText(/cc4f5deb9781/);
    await userEvent.click(screen.getByTestId('about-copy'));
    expect(writeText).toHaveBeenCalledOnce();
    expect(writeText.mock.calls[0][0]).toContain('cc4f5deb9781');
    expect(await screen.findByTestId('about-copied')).toHaveTextContent('Copied');
    Reflect.deleteProperty(navigator, 'clipboard');
  });

  // A copy that silently does nothing is worse than one that says it failed:
  // somebody pastes stale contents into a bug report and nobody can tell.
  it('reports a refused copy instead of pretending', async () => {
    bindBridge();
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true, value: { writeText: vi.fn(async () => { throw new Error('denied'); }) },
    });
    const execCommand = vi.fn(() => false);
    Object.defineProperty(document, 'execCommand', { configurable: true, value: execCommand });
    render(<AboutDialog status={status} onClose={() => undefined} />);
    await userEvent.click(screen.getByTestId('about-copy'));
    expect(await screen.findByTestId('about-copied')).toHaveTextContent('would not allow');
    Reflect.deleteProperty(navigator, 'clipboard');
  });

  it('focuses Copy so the report is one key away', async () => {
    bindBridge();
    render(<AboutDialog status={status} onClose={() => undefined} />);
    expect(screen.getByTestId('about-copy')).toHaveFocus();
  });

  // What is copied has to be what is shown. A Copy that carries more than the
  // dialog displays is how private things reach a bug report unnoticed.
  it('shows the activity it copies', async () => {
    bindBridge(vi.fn(async () => build), ['2026-09-04T12:00:00Z transfer: joplin-apply requested for /tmp/x']);
    render(<AboutDialog status={status} onClose={() => undefined} />);
    const report = await screen.findByTestId('about-report');
    expect(report).toHaveTextContent('Recent activity');
    expect(report).toHaveTextContent('joplin-apply requested');
  });

  // A shell that binds About but not RecentActions must still get a build
  // report. Calling an unbound method inside an effect throws synchronously and
  // takes the dialog down, which is how this failed against the real
  // application: a blank render, not a dialog.
  it('survives a shell that does not offer the transcript', async () => {
    Object.defineProperty(window, 'go', {
      configurable: true, value: { main: { NativeUIBridge: { About: vi.fn(async () => build) } } },
    });
    render(<AboutDialog status={status} onClose={() => undefined} />);
    expect(await screen.findByTestId('about-report')).toHaveTextContent('cc4f5deb9781');
  });

  // A Go nil slice crosses to JavaScript as null, not as an empty array, and
  // reading .length off it throws during render -- which unmounts everything
  // and leaves a blank window with no message. It happens only when the
  // transcript is empty, which is every fresh start.
  it('survives a null transcript', async () => {
    Object.defineProperty(window, 'go', {
      configurable: true,
      value: { main: { NativeUIBridge: {
        About: vi.fn(async () => build),
        RecentActions: vi.fn(async () => null as unknown as string[]),
      } } },
    });
    render(<AboutDialog status={status} onClose={() => undefined} />);
    expect(await screen.findByTestId('about-report')).toHaveTextContent('cc4f5deb9781');
  });

  it('closes on Escape', async () => {
    bindBridge();
    const onClose = vi.fn();
    render(<AboutDialog status={status} onClose={onClose} />);
    await userEvent.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('describes an unknown schema without inventing one', () => {
    expect(describeBuild(null, null)).toContain('Schema: unknown');
  });
});
