import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BrokenLinkList, LinkPicker } from '../components/LinkIntelligence';
import { describeLinkStatus, insertAt, isBrokenStatus, linkMarkdown } from '../useLinkIntelligence';
import type { CheckedLink } from '../api';

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('link status helpers', () => {
  it('treats external links as fine and everything unresolvable as broken', () => {
    expect(isBrokenStatus('resolved')).toBe(false);
    expect(isBrokenStatus('external')).toBe(false);
    for (const status of ['unresolved', 'ambiguous', 'stale_anchor', 'invalid']) {
      expect(isBrokenStatus(status)).toBe(true);
    }
  });

  it('explains a stale anchor as the note being present and the section not', () => {
    expect(describeLinkStatus('stale_anchor')).toContain('section');
  });

  it('inserts the canonical URI rather than the title', () => {
    const markdown = linkMarkdown({
      document_id: 'doc_1',
      title: 'Kitchen Plan',
      uri: 'document://default/documents/doc_1',
      match: 'title_prefix',
    });
    expect(markdown).toBe('[Kitchen Plan](document://default/documents/doc_1)');
  });

  it('inserts at the caret and reports where the caret lands', () => {
    expect(insertAt('ab', 1, 'XY')).toEqual({ value: 'aXYb', caret: 3 });
    expect(insertAt('ab', 99, 'X')).toEqual({ value: 'abX', caret: 3 });
    expect(insertAt('ab', -5, 'X')).toEqual({ value: 'Xab', caret: 1 });
  });
});

describe('LinkPicker', () => {
  beforeEach(() => {
    vi.useRealTimers();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('asks for nothing until the query is long enough', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const user = userEvent.setup();
    render(<LinkPicker onInsert={() => {}} />);

    await user.type(screen.getByLabelText('Search notes to link'), 'k');
    await new Promise((resolve) => setTimeout(resolve, 250));
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('suggests notes and inserts the canonical link when one is chosen', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({
          suggestions: [
            { document_id: 'doc_1', title: 'Kitchen', uri: 'document://default/documents/doc_1', match: 'title_prefix' },
            { document_id: 'doc_2', title: 'Annual Kitchen Review', uri: 'document://default/documents/doc_2', match: 'word_prefix' },
          ],
          limit: 10,
        }),
      ),
    );
    const inserted: string[] = [];
    const user = userEvent.setup();
    render(<LinkPicker documentID="doc_here" onInsert={(markdown) => inserted.push(markdown)} />);

    await user.type(screen.getByLabelText('Search notes to link'), 'kit');
    await waitFor(() => expect(screen.getByTestId('link-suggestions')).toBeTruthy());
    expect(screen.getByText('Annual Kitchen Review')).toBeTruthy();

    await user.click(screen.getByText('Kitchen'));
    expect(inserted).toEqual(['[Kitchen](document://default/documents/doc_1)']);
  });

  it('shows nothing rather than an error when the service cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('offline'); }));
    const user = userEvent.setup();
    render(<LinkPicker onInsert={() => {}} />);

    await user.type(screen.getByLabelText('Search notes to link'), 'kit');
    await new Promise((resolve) => setTimeout(resolve, 250));
    expect(screen.queryByTestId('link-suggestions')).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('BrokenLinkList', () => {
  const broken: CheckedLink[] = [
    {
      raw_target: 'document://default/documents/gone',
      status: 'unresolved',
      start_byte: 10,
      end_byte: 40,
      line: 3,
      column: 1,
    },
  ];

  it('renders nothing before a check has returned', () => {
    const { container } = render(<BrokenLinkList broken={[]} checked={false} total={0} />);
    expect(container.textContent).toBe('');
  });

  it('says all links resolve rather than implying it with silence', () => {
    render(<BrokenLinkList broken={[]} checked total={4} />);
    expect(screen.getByTestId('link-check-clean').textContent).toContain('All 4 links resolve');
  });

  it('locates each broken link and says why it is marked', () => {
    render(<BrokenLinkList broken={broken} checked total={5} />);
    const list = screen.getByTestId('link-check-broken');
    expect(list.textContent).toContain('1 of 5 links will not open');
    expect(list.textContent).toContain('line 3, column 1');
    expect(list.textContent).toContain('no note or resource with this target');
  });
});
