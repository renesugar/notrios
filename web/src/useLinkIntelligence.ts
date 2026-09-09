// Editor link intelligence, client side.
//
// Two hooks, both debounced, both cancelling the request they superseded, and
// both failing to *nothing*. An assist that throws an error banner while
// someone is typing is worse than an assist that is quietly absent, so an
// offline or erroring service leaves the editor with no suggestions and no
// markers rather than with a problem to dismiss.
//
// What this file deliberately does not do is parse Markdown. Deciding what is a
// link belongs to the canonical extractor in the service; a second
// implementation here would drift and start disagreeing with what a save
// actually records.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  checkBufferLinks,
  suggestLinkTargets,
  type CheckedLink,
  type DocumentSuggestion,
} from './api';

/** The shortest query the service will answer. */
export const MIN_SUGGEST_QUERY = 2;

/** Debounce windows, in milliseconds. */
export const SUGGEST_DEBOUNCE_MS = 150;
export const CHECK_DEBOUNCE_MS = 600;

/** A link status that means the link would not open anything. */
export function isBrokenStatus(status: string): boolean {
  return status !== 'resolved' && status !== 'external';
}

/** A short, human explanation of why a link is marked. */
export function describeLinkStatus(status: string): string {
  switch (status) {
    case 'unresolved':
      return 'no note or resource with this target';
    case 'ambiguous':
      return 'more than one note or resource matches';
    case 'stale_anchor':
      return 'the note exists but the section or block does not';
    case 'invalid':
      return 'not a link this application can read';
    case 'external':
      return 'external link, not checked';
    default:
      return status;
  }
}

/**
 * Bounded suggestions for a partial link target. Returns nothing at all until
 * the query is long enough — a one-character query matches so much of a large
 * library that answering it is neither cheap nor useful.
 */
export function useLinkSuggestions(query: string, excludeDocumentID?: string) {
  const [suggestions, setSuggestions] = useState<DocumentSuggestion[]>([]);
  const [truncated, setTruncated] = useState(false);

  useEffect(() => {
    const trimmed = query.trim();
    if (trimmed.length < MIN_SUGGEST_QUERY) {
      setSuggestions([]);
      setTruncated(false);
      return;
    }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      suggestLinkTargets(trimmed, { excludeDocumentID, signal: controller.signal })
        .then((response) => {
          setSuggestions(response.suggestions ?? []);
          setTruncated(Boolean(response.truncated));
        })
        .catch(() => {
          // Degrade to no suggestions. See the file comment.
          setSuggestions([]);
          setTruncated(false);
        });
    }, SUGGEST_DEBOUNCE_MS);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [query, excludeDocumentID]);

  return { suggestions, truncated };
}

export interface BufferLinkState {
  /** Every link the buffer contains, in document order. */
  links: CheckedLink[];
  /** The subset that would not open anything. */
  broken: CheckedLink[];
  /** True once at least one check has returned for the current buffer. */
  checked: boolean;
  /**
   * The exact text the service classified. Every offset in `links` describes
   * this string, so anything placing a marker must compare against it rather
   * than assume the buffer has not moved on.
   */
  checkedBody: string;
}

/**
 * Classifies the links in the current buffer without saving it. The debounce is
 * longer than the suggestion one on purpose: a check reads the whole buffer, and
 * it is useful a moment after typing stops rather than during it.
 */
export function useBufferLinks(body: string, documentID: string | undefined, enabled: boolean): BufferLinkState {
  const [links, setLinks] = useState<CheckedLink[]>([]);
  const [checked, setChecked] = useState(false);
  const [checkedBody, setCheckedBody] = useState('');
  // Identifies the buffer a response belongs to, so a slow reply for older text
  // cannot overwrite a newer result.
  const latest = useRef(0);

  useEffect(() => {
    if (!enabled) {
      setLinks([]);
      setChecked(false);
      setCheckedBody('');
      return;
    }
    const generation = ++latest.current;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      checkBufferLinks(body, { documentID, signal: controller.signal })
        .then((response) => {
          if (generation !== latest.current) return;
          setLinks(response.links ?? []);
          setCheckedBody(body);
          setChecked(true);
        })
        .catch(() => {
          if (generation !== latest.current) return;
          setLinks([]);
          setCheckedBody('');
          setChecked(false);
        });
    }, CHECK_DEBOUNCE_MS);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [body, documentID, enabled]);

  const broken = useMemo(() => links.filter((link) => isBrokenStatus(link.status)), [links]);
  return { links, broken, checked, checkedBody };
}

/**
 * Builds the Markdown a chosen suggestion inserts. The canonical URI is used
 * rather than the title: a title-resolved link breaks the moment the note is
 * renamed, which is exactly what `notriosctl fix` exists to repair afterwards.
 */
export function linkMarkdown(suggestion: DocumentSuggestion): string {
  return `[${suggestion.title}](${suggestion.uri})`;
}

/** Inserts text at a caret position, returning the new value and caret. */
export function insertAt(value: string, caret: number, insertion: string): { value: string; caret: number } {
  const at = Math.max(0, Math.min(caret, value.length));
  return {
    value: value.slice(0, at) + insertion + value.slice(at),
    caret: at + insertion.length,
  };
}

/** A stable callback that never changes identity, for effect dependencies. */
export function useStableCallback<T extends (...args: never[]) => unknown>(fn: T): T {
  const ref = useRef(fn);
  ref.current = fn;
  return useCallback(((...args: never[]) => ref.current(...args)) as T, []);
}
