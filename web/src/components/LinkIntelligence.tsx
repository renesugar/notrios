// Editor link intelligence, rendered.
//
// Two pieces sit inside the editor pane:
//
//   * a link picker that suggests notes by title and inserts a canonical
//     Markdown link at the caret;
//   * a list of the links in the current buffer that would not open anything,
//     located by line and column.
//
// E5 claimed this list existed *because* the editor could not underline a
// broken link in place. That was wrong: `md-editor-rt` is CodeMirror 6 and
// exposes it, so E6 added the in-editor underline (`editor-extensions.ts`).
//
// The list stayed anyway, because it turned out to be the more useful half. An
// underline says "something here is wrong" only where you happen to be looking;
// the list says how many links are broken, where each one is, and why — for a
// note longer than the screen, which is most of them.
import { useState } from 'react';
import type { CheckedLink } from '../api';
import { describeLinkStatus, linkMarkdown, useLinkSuggestions, MIN_SUGGEST_QUERY } from '../useLinkIntelligence';

export interface LinkPickerProps {
  /** The note being edited, excluded from its own suggestions. */
  documentID?: string;
  /** Inserts Markdown at the caret. */
  onInsert: (markdown: string) => void;
  disabled?: boolean;
}

export function LinkPicker({ documentID, onInsert, disabled }: LinkPickerProps) {
  const [query, setQuery] = useState('');
  const { suggestions, truncated } = useLinkSuggestions(query, documentID);

  return (
    <div className="link-picker" data-testid="link-picker">
      <label className="link-picker-field">
        Insert link to note
        <input
          type="search"
          value={query}
          disabled={disabled}
          placeholder={`Title (${MIN_SUGGEST_QUERY}+ characters)`}
          aria-label="Search notes to link"
          data-testid="link-picker-input"
          onChange={(event) => setQuery(event.target.value)}
        />
      </label>
      {suggestions.length > 0 && (
        <ul className="link-picker-results" data-testid="link-suggestions">
          {suggestions.map((suggestion) => (
            <li key={suggestion.document_id}>
              <button
                type="button"
                disabled={disabled}
                data-testid="link-suggestion"
                onClick={() => {
                  onInsert(linkMarkdown(suggestion));
                  setQuery('');
                }}
              >
                {suggestion.title}
              </button>
              {suggestion.match === 'word_prefix' && <span className="muted"> · word match</span>}
            </li>
          ))}
        </ul>
      )}
      {truncated && (
        <p className="muted" data-testid="link-suggestions-truncated">
          More notes match. Type more of the title.
        </p>
      )}
    </div>
  );
}

export interface BrokenLinkListProps {
  broken: CheckedLink[];
  checked: boolean;
  /** Total links in the buffer, so "all fine" can be said rather than implied. */
  total: number;
}

export function BrokenLinkList({ broken, checked, total }: BrokenLinkListProps) {
  if (!checked) {
    // Not "no problems" — nothing is known yet, or the service could not be
    // reached. Saying nothing is the honest rendering of that.
    return null;
  }
  if (broken.length === 0) {
    return (
      <p className="muted" data-testid="link-check-clean">
        {total === 0 ? 'No links in this note.' : `All ${total} links resolve.`}
      </p>
    );
  }
  return (
    <div className="link-list" data-testid="link-check-broken">
      <strong>
        {broken.length} of {total} links will not open
      </strong>
      <ul>
        {broken.map((link) => (
          <li key={`${link.start_byte}-${link.end_byte}`}>
            <code>{link.raw_target || `#${link.anchor_value ?? ''}`}</code>
            <span className="muted">
              {' '}
              · line {link.line}, column {link.column} · {describeLinkStatus(link.status)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
