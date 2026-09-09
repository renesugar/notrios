// Search pane: query box plus cursor-paged results in their own scroll
// container. An IntersectionObserver sentinel near the bottom loads the next
// page; a keyboard-accessible "Load more" button remains as a fallback.
import { useEffect, useRef, useState } from 'react';
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
  /**
   * Start a new note. It lives here rather than in the editor toolbar because
   * it is not an action on the open note — it discards the editor and starts a
   * draft. Among per-note controls it read, on a Help or trashed note, as an
   * offer to create something there.
   */
  onNewNote: () => void;
  /** Where a new note would be filed, for the label. */
  newNoteNotebookName: string;
  /**
   * The query that produced the results on screen, which is not necessarily
   * what is in the box: somebody types, searches, then keeps typing while
   * reading. Keeping a search saves the one they are looking at.
   */
  ranQuery: string;
  /** Saves the query that ran as a notebook under the given name. */
  onKeepSearch: (name: string) => Promise<void>;
  /**
   * The notes picked out for an operation on a set, which is a different thing
   * from the one note the editor is showing: `selectedDocumentID` is where you
   * are, and this is what you are about to act on.
   */
  checked: ReadonlySet<string>;
  /**
   * Toggles one note in or out of that set. `extend` asks for everything
   * between the last one touched and this one, which is what shift-click means
   * everywhere else a list of things can be selected.
   */
  onToggleChecked: (documentID: string, extend: boolean) => void;
}

export function SearchPane({ query, onQueryChange, onSubmit, paged, onOpenHit, selectedDocumentID, busy, onNewNote, newNoteNotebookName, ranQuery, onKeepSearch, checked, onToggleChecked }: SearchPaneProps) {
  const sentinelRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [keeping, setKeeping] = useState(false);
  const [keepName, setKeepName] = useState('');
  const [keepError, setKeepError] = useState('');
  const keepInputRef = useRef<HTMLInputElement>(null);

  // Offered only once a search has actually run and returned something. A
  // notebook made from a query nobody has seen work is a view that may be
  // empty, misspelled or wrong, and the point of putting this here rather than
  // in a form is that the results are on screen while the decision is made.
  const canKeep = paged.started && !paged.loading && ranQuery.trim() !== '' && paged.hits.length > 0;

  useEffect(() => {
    if (keeping) keepInputRef.current?.focus();
  }, [keeping]);

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
          placeholder="Search: (alpha OR beta) -tag:private"
          aria-label="Search query"
          aria-describedby="search-query-hint"
          data-testid="search-input"
          maxLength={4096}
        />
        <button type="submit" disabled={busy} data-testid="search-submit">
          Search
        </button>
      </form>
      <p className="search-query-hint" id="search-query-hint">
        Uppercase OR · implicit AND · -exclude · (group) · category: or notebook:
      </p>
      {/* The destination is named on the button, so where a note will be filed
          is answered before it is written rather than after it is saved. */}
      <button
        type="button"
        className="new-note-button"
        data-testid="new-note-button"
        onClick={onNewNote}
        title={`Start a new note in “${newNoteNotebookName}”`}
      >
        <span aria-hidden="true">＋</span> New note in <strong>{newNoteNotebookName}</strong>
      </button>
      {canKeep && !keeping && (
        <button type="button" className="keep-search-button" data-testid="keep-search"
          onClick={() => { setKeepName(ranQuery); setKeepError(''); setKeeping(true); }}
          title="Save this search as a notebook in the sidebar">
          <span aria-hidden="true">☆</span> Keep this search
        </button>
      )}
      {keeping && (
        <form className="keep-search-form" data-testid="keep-search-form"
          onSubmit={(event) => {
            event.preventDefault();
            setKeepError('');
            void onKeepSearch(keepName.trim())
              .then(() => { setKeeping(false); setKeepName(''); })
              .catch((error: unknown) => setKeepError(error instanceof Error ? error.message : String(error)));
          }}>
          <label>Name this search
            <input ref={keepInputRef} value={keepName} data-testid="keep-search-name"
              onChange={(event) => setKeepName(event.target.value)}
              maxLength={200} autoComplete="off" />
          </label>
          {/* The query is shown, not editable. Editing it here would make this a
              query form after all, and the whole reason to put the control on a
              search that ran is that the query is one somebody has watched
              work. */}
          <p className="muted keep-search-query" data-testid="keep-search-query">
            Keeps: <code>{ranQuery}</code>
          </p>
          <div className="row-actions">
            <button type="submit" className="primary-button" data-testid="keep-search-save"
              disabled={busy || keepName.trim() === ''}>Keep it</button>
            <button type="button" data-testid="keep-search-cancel"
              onClick={() => { setKeeping(false); setKeepError(''); }}>Cancel</button>
          </div>
          {keepError && <p className="sync-inline-notice error" role="status"
            data-testid="keep-search-error">{keepError}</p>}
        </form>
      )}
      <div className="pane-scroll result-list" ref={listRef} aria-live="polite" data-testid="search-results">
        {paged.hits.map((hit) => (
          // A row rather than a bare card, because the checkbox cannot live
          // inside the button: one interactive control nested in another is
          // unreachable by keyboard and ambiguous to a screen reader. The card
          // still opens the note; the checkbox picks it out for an operation on
          // several. Shown always rather than revealed by a mode, because a
          // selection you cannot see how to start is one nobody starts.
          <div className="result-row" key={hit.id}>
          <input
            type="checkbox"
            className="result-check"
            data-testid="result-check"
            checked={checked.has(hit.id)}
            aria-label={`Select ${hit.title ?? hit.uri}`}
            onChange={(event) => onToggleChecked(hit.id, (event.nativeEvent as MouseEvent).shiftKey === true)}
            onClick={(event) => event.stopPropagation()}
          />
          <button
            className={selectedDocumentID === hit.id ? 'result-card active' : 'result-card'}
            data-testid="result-card"
            onClick={(event) => {
              // Ctrl or Command adds to the set and shift extends it, which is
              // what a list of selectable things does everywhere else and what
              // somebody arriving from Joplin will try first. The checkbox is
              // the discoverable path; this is the fast one.
              if (event.ctrlKey || event.metaKey || event.shiftKey) {
                onToggleChecked(hit.id, event.shiftKey);
                return;
              }
              onOpenHit(hit);
            }}
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
          </div>
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
            <button type="button" className="load-more" data-testid="search-load-more" onClick={() => void paged.loadMore()} disabled={paged.loading}>
              Load more
            </button>
          </>
        )}
      </div>
    </section>
  );
}
