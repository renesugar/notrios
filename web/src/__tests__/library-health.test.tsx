// Library health: two read-only reports, and the commands that act on them.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LibraryHealth } from '../components/LibraryHealth';
import * as api from '../api';
import type { GarbageCollectionReport, LintReport } from '../api';

function lint(overrides: Partial<LintReport> = {}): LintReport {
  return {
    version: 1, collection_id: 'default', detail_limit: 100,
    checks: [
      { check: 'broken_links', count: 3, findings: [], truncated: false },
      { check: 'missing_alt_text', count: 0, findings: [], truncated: false },
      { check: 'unreferenced_resources', count: 240, findings: new Array(100).fill(null)
        .map((_, i) => ({ check: 'unreferenced_resources', resource_id: `r${i}` })), truncated: true },
    ],
    total_findings: 243, report_sha256: 'abc', warnings: [],
    ...overrides,
  };
}

function garbage(overrides: Partial<GarbageCollectionReport> = {}): GarbageCollectionReport {
  return {
    dry_run: true, as_of: '2026-09-04T12:00:00Z',
    eligible: [{}, {}], retained: [{}], removed: [],
    referenced_resource_count: 41, blobs_removed: 0, bytes_removed: 2_400_000,
    warnings: [], ...overrides,
  };
}

function bindRepairs(overrides: Record<string, unknown> = {}) {
  const bridge = {
    PlanFixes: vi.fn(async () => ({
      plan: { documents: [{ document_id: 'doc_1', base_revision_id: 'rev_1', edits: [{ kind: 'title', line: 1 }] }],
              total_edits: 4, total_documents: 2, truncated: false, warnings: [] },
      applied: false,
    })),
    ApplyFixes: vi.fn(async () => ({
      plan: { documents: [], total_edits: 4, total_documents: 2, truncated: false, warnings: [] },
      applied: true,
      results: [
        { document_id: 'doc_1', applied: 3, skipped: 0 },
        { document_id: 'doc_2', applied: 0, skipped: 0, error: 'the note changed since the plan was made' },
      ],
    })),
    ...overrides,
  };
  Object.defineProperty(window, 'go', { configurable: true, value: { main: { NativeUIBridge: bridge } } });
  return bridge;
}

afterEach(() => {
  vi.restoreAllMocks();
  Reflect.deleteProperty(window, 'go');
});

describe('library health', () => {
  it('reports both halves, not just the one that answered', async () => {
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    expect(await screen.findByTestId('library-health-lint')).toBeInTheDocument();
    expect(screen.getByTestId('library-health-gc')).toBeInTheDocument();
  });

  it('lists only the checks that found something', async () => {
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    expect(await screen.findByTestId('library-health-check-broken_links')).toHaveTextContent('3 · broken links');
    expect(screen.queryByTestId('library-health-check-missing_alt_text')).toBeNull();
  });

  // The count is complete and only the examples are capped. A reader who takes
  // a truncated list for the whole problem under-counts by 140 here.
  it('says when the examples are capped but the count is not', async () => {
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    const row = await screen.findByTestId('library-health-check-unreferenced_resources');
    expect(row).toHaveTextContent('240 · unreferenced resources');
    expect(row).toHaveTextContent('showing the first 100');
  });

  it('says a clean library is clean rather than showing nothing', async () => {
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint({
      checks: [{ check: 'broken_links', count: 0, findings: [], truncated: false }], total_findings: 0,
    }));
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    expect(await screen.findByTestId('library-health-clean')).toHaveTextContent('Nothing found');
  });

  // Both endpoints are read-only by the service's decision. Naming the command
  // is the honest alternative to a button that cannot exist.
  // Collection stays a command because it is the irreversible one: a repair
  // writes a revision, deleting a blob does not.
  it('names the command for the operation it does not offer', async () => {
    bindRepairs();
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    expect(await screen.findByTestId('library-health-gc')).toHaveTextContent('notriosctl gc --apply');
    // And offers a button for the one it does.
    expect(screen.getByTestId('fix-apply')).toBeInTheDocument();
  });

  it('surfaces a failure instead of an empty clean bill of health', async () => {
    vi.spyOn(api, 'getLintReport').mockRejectedValue(new Error('the service is unreachable'));
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    expect(await screen.findByTestId('library-health-error')).toHaveTextContent('unreachable');
    expect(screen.queryByTestId('library-health-clean')).toBeNull();
  });

  // Repairs run in the process that owns the library. A browser is told why
  // rather than finding the section absent, which would read as "there is
  // nothing to repair".
  it('explains rather than hides repairs without the desktop bridge', async () => {
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    expect(await screen.findByTestId('fix-unavailable')).toHaveTextContent('notriosctl fix');
    expect(screen.queryByTestId('fix-apply')).toBeNull();
  });

  it('cannot repair before it has planned', async () => {
    bindRepairs();
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    expect(await screen.findByTestId('fix-apply')).toBeDisabled();
  });

  it('plans without repairing', async () => {
    const bridge = bindRepairs();
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    await userEvent.click(await screen.findByTestId('fix-plan'));
    expect(await screen.findByTestId('fix-plan-report')).toHaveTextContent('4 edits across 2 notes');
    expect(bridge.ApplyFixes).not.toHaveBeenCalled();
  });

  // A note edited between the plan and the apply fails its own precondition.
  // Reporting that per note is the difference between "some repairs refused"
  // and a silent partial result.
  it('reports which notes refused rather than only the successes', async () => {
    bindRepairs();
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    await userEvent.click(await screen.findByTestId('fix-plan'));
    await screen.findByTestId('fix-plan-report');
    await userEvent.click(screen.getByTestId('fix-apply'));
    const results = await screen.findByTestId('fix-results');
    expect(results).toHaveTextContent('3 repaired');
    expect(results).toHaveTextContent('refused: the note changed since the plan was made');
  });

  it('offers nothing to repair when there is nothing', async () => {
    bindRepairs({ PlanFixes: vi.fn(async () => ({
      plan: { documents: [], total_edits: 0, total_documents: 0, truncated: false, warnings: [] },
      applied: false,
    })) });
    vi.spyOn(api, 'getLintReport').mockResolvedValue(lint());
    vi.spyOn(api, 'getGarbageReport').mockResolvedValue(garbage());
    render(<LibraryHealth onClose={vi.fn()} />);
    await userEvent.click(await screen.findByTestId('fix-plan'));
    expect(await screen.findByTestId('fix-nothing')).toBeInTheDocument();
    expect(screen.getByTestId('fix-apply')).toBeDisabled();
  });
});
