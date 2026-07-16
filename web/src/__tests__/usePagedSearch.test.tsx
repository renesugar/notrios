// Cursor-paging behavior: single first-page request, single next-page request,
// in-order appends without duplicates, stale-response rejection when the query
// changes, and termination on an empty cursor.
import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { usePagedSearch } from '../usePagedSearch';
import * as api from '../api';

vi.mock('../api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api')>();
  return { ...original, search: vi.fn() };
});

const searchMock = vi.mocked(api.search);

function hit(id: string): api.SearchHit {
  return { id, uri: `document://default/documents/${id}`, source: 'managed-notes', title: id };
}

beforeEach(() => {
  searchMock.mockReset();
});

describe('usePagedSearch', () => {
  it('requests the first page once and appends the next page once, in order', async () => {
    searchMock.mockResolvedValueOnce({ hits: [hit('a'), hit('b')], next_cursor: 'c1' });
    const { result } = renderHook(() => usePagedSearch(2));

    await act(() => result.current.start('query'));
    expect(searchMock).toHaveBeenCalledTimes(1);
    expect(searchMock).toHaveBeenCalledWith('query', 2);
    expect(result.current.hits.map((h) => h.id)).toEqual(['a', 'b']);
    expect(result.current.exhausted).toBe(false);

    searchMock.mockResolvedValueOnce({ hits: [hit('b'), hit('c')], next_cursor: '' });
    await act(() => result.current.loadMore());
    expect(searchMock).toHaveBeenCalledTimes(2);
    expect(searchMock).toHaveBeenLastCalledWith('query', 2, 'c1');
    // Overlapping page: 'b' is not duplicated.
    expect(result.current.hits.map((h) => h.id)).toEqual(['a', 'b', 'c']);
    expect(result.current.exhausted).toBe(true);
  });

  it('stops paging when next_cursor is empty', async () => {
    searchMock.mockResolvedValueOnce({ hits: [hit('a')], next_cursor: '' });
    const { result } = renderHook(() => usePagedSearch());
    await act(() => result.current.start('q'));
    expect(result.current.exhausted).toBe(true);

    await act(() => result.current.loadMore());
    expect(searchMock).toHaveBeenCalledTimes(1); // no extra request
  });

  it('does not issue duplicate concurrent cursor requests', async () => {
    searchMock.mockResolvedValueOnce({ hits: [hit('a')], next_cursor: 'c1' });
    const { result } = renderHook(() => usePagedSearch());
    await act(() => result.current.start('q'));

    let release: (value: api.SearchResponse) => void = () => {};
    searchMock.mockImplementationOnce(() => new Promise((resolve) => (release = resolve)));
    let first: Promise<void>;
    act(() => {
      first = result.current.loadMore();
      void result.current.loadMore(); // second call while the first is in flight
    });
    expect(searchMock).toHaveBeenCalledTimes(2); // start + one loadMore
    await act(async () => {
      release({ hits: [hit('b')], next_cursor: '' });
      await first;
    });
    expect(result.current.hits.map((h) => h.id)).toEqual(['a', 'b']);
  });

  it('ignores stale responses after the query changes', async () => {
    let releaseOld: (value: api.SearchResponse) => void = () => {};
    searchMock.mockImplementationOnce(() => new Promise((resolve) => (releaseOld = resolve)));
    const { result } = renderHook(() => usePagedSearch());

    let oldStart: Promise<void>;
    act(() => {
      oldStart = result.current.start('old-query');
    });

    searchMock.mockResolvedValueOnce({ hits: [hit('new')], next_cursor: '' });
    await act(() => result.current.start('new-query'));
    expect(result.current.hits.map((h) => h.id)).toEqual(['new']);

    await act(async () => {
      releaseOld({ hits: [hit('stale')], next_cursor: 'old-cursor' });
      await oldStart;
    });
    // The stale first page must not replace the new query's results.
    await waitFor(() => expect(result.current.hits.map((h) => h.id)).toEqual(['new']));
  });

  it('patches and prepends hits in place for save flows', async () => {
    searchMock.mockResolvedValueOnce({ hits: [hit('a')], next_cursor: '' });
    const { result } = renderHook(() => usePagedSearch());
    await act(() => result.current.start('q'));

    act(() => result.current.patchHit('a', { title: 'renamed' }));
    expect(result.current.hits[0].title).toBe('renamed');

    act(() => result.current.prependHit(hit('new')));
    expect(result.current.hits.map((h) => h.id)).toEqual(['new', 'a']);
    act(() => result.current.prependHit(hit('new')));
    expect(result.current.hits.map((h) => h.id)).toEqual(['new', 'a']); // no duplicate
  });
});
