// Library Health writes the reports a person reads in Reports (v1.0 J40-D,
// decision 6): the ambiguous-links report, and the graph report, which had no
// control at all before.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LibraryHealth } from '../components/LibraryHealth';
import * as api from '../api';

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function stubService() {
  vi.spyOn(api, 'getLintReport').mockResolvedValue({
    version: 1, collection_id: 'default', detail_limit: 100,
    checks: [{ check: 'ambiguous_link', count: 2, findings: [], truncated: false }],
    total_findings: 2, report_sha256: 'abc', warnings: [],
  });
  vi.spyOn(api, 'getGarbageReport').mockResolvedValue({
    dry_run: true, as_of: '2026-10-21T12:00:00Z', eligible: [], retained: [], removed: [],
    referenced_resource_count: 0, blobs_removed: 0, bytes_removed: 0, warnings: [],
  });
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    if (url === '/api/v1/admin/links/report/note' && init?.method === 'POST') {
      return json({ document_id: 'doc_link_report', uri: 'document://default/documents/doc_link_report',
        title: 'Ambiguous links report', report: { ambiguous_links: 2, notes_with_ambiguous_links: 1 } });
    }
    if (url === '/api/v1/graph/report/note' && init?.method === 'POST') {
      return json({ document: { id: 'doc_graph_report', title: 'Library graph report' },
        report: { document_count: 12, orphan_count: 3 } });
    }
    return json({ error: { code: 'not_found', message: url } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('J40-D: Library Health writes the reports', () => {
  it('writes the ambiguous-links report on request, says what it found, and opens it', async () => {
    const fetchMock = stubService();
    const onOpenDocument = vi.fn();
    const onClose = vi.fn();
    render(<LibraryHealth onClose={onClose} onOpenDocument={onOpenDocument} />);
    await screen.findByTestId('library-health-reports');
    // Nothing is written until asked: opening the screen reads, never writes.
    expect(fetchMock.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(false);
    expect(screen.queryByTestId('open-report-links')).toBeNull();

    await userEvent.click(screen.getByTestId('write-report-links'));
    expect(await screen.findByTestId('written-report-links')).toHaveTextContent('2 ambiguous links in 1 note.');
    await userEvent.click(screen.getByTestId('open-report-links'));
    expect(onOpenDocument).toHaveBeenCalledWith('doc_link_report');
    expect(onClose).toHaveBeenCalled();
  });

  it('writes the graph report, which had no control before', async () => {
    stubService();
    render(<LibraryHealth onClose={vi.fn()} onOpenDocument={vi.fn()} />);
    await userEvent.click(await screen.findByTestId('write-report-graph'));
    expect(await screen.findByTestId('written-report-graph')).toHaveTextContent('12 notes measured, 3 with nothing linking to them.');
    expect(screen.getByTestId('open-report-graph')).toHaveTextContent('Library graph report');
  });

  it('shows a failure instead of a success', async () => {
    stubService();
    vi.stubGlobal('fetch', vi.fn(async () => json({ error: { code: 'link_report_failed', message: 'the library is busy' } }, 500)));
    render(<LibraryHealth onClose={vi.fn()} onOpenDocument={vi.fn()} />);
    await userEvent.click(await screen.findByTestId('write-report-links'));
    expect(await screen.findByTestId('library-health-report-error')).toBeInTheDocument();
    expect(screen.queryByTestId('written-report-links')).toBeNull();
  });
});
