// Search pane: query box plus cursor-paged results in their own scroll
// container. An IntersectionObserver sentinel near the bottom loads the next
// page; a keyboard-accessible "Load more" button remains as a fallback.
import { useEffect, useRef } from 'react';
import type { SearchHit } from '../api';
import type { PagedSearch } from '../usePagedSearch';
import { Snippet } from '../preview-utils';

export interface SearchPaneProps {
  query: string;
  onQueryChange: (value: string) => void;
  onSubmit: () => void;
  paged: PagedSearch;
  onOpenHit: (hit: SearchHit) => void;
  selectedDocumentID: string | null;
  busy: boolean;
}

export function SearchPane({ query, onQueryChange, onSubmit, paged, onOpenHit, selectedDocumentID, busy }: SearchPaneProps) {
  const sentinelRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const hasMore = !paged.exhausted && paged.started;

  useEffect(() => {
    const sentinel = sentinelRef.current;
    const root = listRef.current;
    if (!sentinel || !root || !hasMore) return;
    if (typeof IntersectionObserver === 'undefined') return; // manual fallback only
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          void paged.loadMore(); // the hook ignores overlapping/duplicate calls
        }
      },
      { root, rootMargin: '200px 0px' },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, paged.loadMore, paged]);

  return (
    <section className="pane search-pane" aria-label="Search" data-testid="pane-search">
      <form
        className="search-row"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit();
        }}
      >
        <input
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Search notes…"
          aria-label="Search query"
        />
        <button type="submit" disabled={busy}>
          Search
        </button>
      </form>
      <div className="pane-scroll result-list" ref={listRef} aria-live="polite" data-testid="search-results">
        {paged.hits.map((hit) => (
          <button
            className={selectedDocumentID === hit.id ? 'result-card active' : 'result-card'}
            key={hit.id}
            onClick={() => onOpenHit(hit)}
            aria-current={selectedDocumentID === hit.id ? 'true' : undefined}
          >
            <strong>
              {hit.editable === false ? '🔒 ' : ''}
              {hit.title ?? hit.uri}
            </strong>
            {hit.sources && hit.sources.length > 0 && (
              <span className="search-sources" aria-label={`Search sources: ${hit.sources.join(', ')}`}>
                {hit.sources.map((source) => (
                  <span className="search-source" key={source}>{source}</span>
                ))}
              </span>
            )}
            {hit.snippet && <Snippet htmlSnippet={hit.snippet} />}
          </button>
        ))}
        {paged.error && <p className="muted result-status">Search failed: {paged.error}</p>}
        {paged.loading && <p className="muted result-status" data-testid="search-loading">Loading…</p>}
        {!paged.loading && paged.started && paged.hits.length === 0 && !paged.error && (
          <p className="muted result-status">No matching notes.</p>
        )}
        {!paged.loading && paged.exhausted && paged.hits.length > 0 && (
          <p className="muted result-status" data-testid="search-end">End of results</p>
        )}
        {hasMore && (
          <>
            <div ref={sentinelRef} className="scroll-sentinel" aria-hidden="true" data-testid="search-sentinel" />
            <button type="button" className="load-more" onClick={() => void paged.loadMore()} disabled={paged.loading}>
              Load more
            </button>
          </>
        )}
      </div>
    </section>
  );
}
