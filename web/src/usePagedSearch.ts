// Cursor-paged search state: first-page loads, sentinel-driven next pages,
// stale-response protection when the query changes mid-flight, duplicate-row
// suppression, and in-place hit patching so saving a note never resets the
// results list or its scroll position.
import { useCallback, useRef, useState } from 'react';
import { search, type SearchHit } from './api';

export interface PagedSearch {
  hits: SearchHit[];
  loading: boolean;
  /** True once a page returned an empty next_cursor. */
  exhausted: boolean;
  /** True when at least one page has loaded for the current query. */
  started: boolean;
  error: string | null;
  start: (query: string) => Promise<void>;
  loadMore: () => Promise<void>;
  /** Update a hit in place (e.g. after saving a note) without re-searching. */
  patchHit: (id: string, patch: Partial<SearchHit>) => void;
  /** Insert a hit at the top if it is not already present. */
  prependHit: (hit: SearchHit) => void;
}

function dedupe(existing: SearchHit[], incoming: SearchHit[]): SearchHit[] {
  const seen = new Set(existing.map((hit) => hit.id));
  const appended = incoming.filter((hit) => {
    if (seen.has(hit.id)) return false;
    seen.add(hit.id);
    return true;
  });
  return [...existing, ...appended];
}

export function usePagedSearch(pageSize = 25): PagedSearch {
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [loading, setLoading] = useState(false);
  const [exhausted, setExhausted] = useState(false);
  const [started, setStarted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Refs guard against overlapping requests and stale responses: every
  // start() bumps the epoch; responses from older epochs are discarded.
  const epochRef = useRef(0);
  const queryRef = useRef('');
  const cursorRef = useRef('');
  const inFlightRef = useRef(false);

  const start = useCallback(
    async (query: string) => {
      const epoch = ++epochRef.current;
      queryRef.current = query;
      cursorRef.current = '';
      inFlightRef.current = true;
      setLoading(true);
      setError(null);
      setHits([]);
      setExhausted(false);
      setStarted(true);
      try {
        const result = await search(query, pageSize);
        if (epoch !== epochRef.current) return; // stale: a newer search started
        setHits(dedupe([], result.hits));
        cursorRef.current = result.next_cursor ?? '';
        setExhausted(!result.next_cursor);
      } catch (err) {
        if (epoch !== epochRef.current) return;
        setError(err instanceof Error ? err.message : String(err));
        setExhausted(true);
      } finally {
        if (epoch === epochRef.current) {
          setLoading(false);
          inFlightRef.current = false;
        }
      }
    },
    [pageSize],
  );

  const loadMore = useCallback(async () => {
    if (inFlightRef.current || !cursorRef.current) return;
    const epoch = epochRef.current;
    inFlightRef.current = true;
    setLoading(true);
    try {
      const result = await search(queryRef.current, pageSize, cursorRef.current);
      if (epoch !== epochRef.current) return; // stale: query changed mid-flight
      setHits((previous) => dedupe(previous, result.hits));
      cursorRef.current = result.next_cursor ?? '';
      setExhausted(!result.next_cursor);
    } catch (err) {
      if (epoch !== epochRef.current) return;
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      if (epoch === epochRef.current) {
        setLoading(false);
        inFlightRef.current = false;
      }
    }
  }, [pageSize]);

  const patchHit = useCallback((id: string, patch: Partial<SearchHit>) => {
    setHits((previous) => previous.map((hit) => (hit.id === id ? { ...hit, ...patch } : hit)));
  }, []);

  const prependHit = useCallback((hit: SearchHit) => {
    setHits((previous) => (previous.some((existing) => existing.id === hit.id) ? previous : [hit, ...previous]));
  }, []);

  return { hits, loading, exhausted, started, error, start, loadMore, patchHit, prependHit };
}
