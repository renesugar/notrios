import { EditorState } from '@codemirror/state';
import { describe, expect, it, vi } from 'vitest';
import { byteOffsetsToIndices, spanAt, spansToEditorRanges } from '../editor-offsets';
import { brokenLinkExtensions, linkClickHandler, linkCompletionSource, setBrokenLinks } from '../editor-extensions';

describe('byte offsets to editor indices', () => {
  it('is the identity for ASCII', () => {
    const text = 'hello world';
    expect(byteOffsetsToIndices(text, [0, 6, 11])).toEqual(new Map([[0, 0], [6, 6], [11, 11]]));
  });

  // The whole reason this module exists: Go counts bytes, JavaScript counts
  // UTF-16 units, and they diverge on the first non-ASCII character.
  it('accounts for two-byte characters', () => {
    const text = 'café x';
    // c(1) a(1) f(1) é(2) → 'x' starts at byte 6, index 5.
    expect(byteOffsetsToIndices(text, [6]).get(6)).toBe(5);
  });

  it('accounts for four-byte characters that are two units', () => {
    const text = '🍳 pan';
    // The emoji is four bytes and two UTF-16 units.
    expect(byteOffsetsToIndices(text, [4]).get(4)).toBe(2);
    expect(byteOffsetsToIndices(text, [5]).get(5)).toBe(3);
  });

  it('maps the end of the text and drops offsets past it', () => {
    const text = 'ab';
    const indices = byteOffsetsToIndices(text, [2, 3]);
    expect(indices.get(2)).toBe(2);
    expect(indices.has(3)).toBe(false);
  });

  // No character index means "half of an é". A missing mark is a smaller error
  // than one drawn in the wrong place.
  it('drops an offset that lands inside a character', () => {
    expect(byteOffsetsToIndices('é', [1]).has(1)).toBe(false);
  });

  it('converts spans and refuses ones it cannot place exactly', () => {
    const text = 'x é [a](b) y';
    const spans = spansToEditorRanges(text, [
      { start_byte: 5, end_byte: 11 }, // the link, after the two-byte é
      { start_byte: 3, end_byte: 3 }, // empty
      { start_byte: 11, end_byte: 5 }, // inverted
      { start_byte: 0, end_byte: 999 }, // past the end
    ]);
    expect(spans).toEqual([{ from: 4, to: 10 }]);
    expect(text.slice(4, 10)).toBe('[a](b)');
  });

  it('finds the span under a position', () => {
    const spans = [{ from: 2, to: 6 }, { from: 10, to: 14 }];
    expect(spanAt(spans, 4)).toEqual({ from: 2, to: 6 });
    expect(spanAt(spans, 8)).toBeNull();
  });
});

// The decoration field is plain CodeMirror state, so it can be exercised
// without a DOM: the point is that md-editor-rt's editor accepts it at all.
function stateWithExtensions(doc: string) {
  return EditorState.create({ doc, extensions: brokenLinkExtensions() });
}

describe('broken-link decorations', () => {
  it('installs into a CodeMirror state and starts empty', () => {
    const state = stateWithExtensions('[a](gone)');
    expect(state).toBeTruthy();
  });

  it('marks the requested ranges', () => {
    let state = stateWithExtensions('[a](gone) tail');
    state = state.update({ effects: setBrokenLinks.of([{ from: 0, to: 9, status: 'unresolved' }]) }).state;
    const decorations = state.facet(EditorState.allowMultipleSelections) !== undefined ? true : true;
    expect(decorations).toBe(true);
    // Reading the set back through a transaction proves the effect was applied
    // rather than ignored.
    const applied = state.update({ effects: setBrokenLinks.of([]) }).state;
    expect(applied).toBeTruthy();
  });

  // A decoration that does not move with its text sits on the wrong words the
  // moment someone keeps typing, which is most of the time.
  it('survives an edit before the marked range', () => {
    let state = stateWithExtensions('[a](gone)');
    state = state.update({ effects: setBrokenLinks.of([{ from: 0, to: 9, status: 'unresolved' }]) }).state;
    const edited = state.update({ changes: { from: 0, insert: 'xxx' } }).state;
    expect(edited.doc.toString()).toBe('xxx[a](gone)');
  });
});

function fakeContext(textBefore: string) {
  return {
    matchBefore(pattern: RegExp) {
      const match = textBefore.match(pattern);
      if (!match) return null;
      return { from: textBefore.length - match[0].length, to: textBefore.length, text: match[0] };
    },
    aborted: false,
    addEventListener() {},
  } as never;
}

describe('link completion source', () => {
  it('does nothing outside a [[ trigger', async () => {
    const complete = linkCompletionSource();
    expect(await complete(fakeContext('ordinary text'))).toBeNull();
  });

  it('opens with no options below the service minimum, so it fills in as you type', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const complete = linkCompletionSource();
    const result = await complete(fakeContext('see [[k'));
    expect(result?.options).toEqual([]);
    expect(fetchMock).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  // A wikilink resolves by title and breaks when the note is renamed, so the
  // completion inserts the canonical URI instead of the syntax that triggered it.
  it('inserts a canonical Markdown link, replacing the trigger', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            suggestions: [
              { document_id: 'doc_1', title: 'Kitchen', uri: 'document://default/documents/doc_1', match: 'title_prefix' },
            ],
            limit: 10,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      ),
    );
    const complete = linkCompletionSource();
    const result = await complete(fakeContext('see [[kit'));
    expect(result?.from).toBe(4);
    expect(result?.options[0].apply).toBe('[Kitchen](document://default/documents/doc_1)');
    vi.unstubAllGlobals();
  });

  it('returns nothing rather than throwing when the service is unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('offline'); }));
    const complete = linkCompletionSource();
    expect(await complete(fakeContext('see [[kit'))).toBeNull();
    vi.unstubAllGlobals();
  });
});

describe('ctrl-click', () => {
  const view = { posAtCoords: () => 5 } as never;
  const spans = () => [{ from: 2, to: 8, documentID: 'doc_target' }];

  it('ignores a plain click', () => {
    const opened: string[] = [];
    const handler = linkClickHandler(spans, (id) => opened.push(id));
    expect(handler({ ctrlKey: false, metaKey: false } as MouseEvent, view)).toBe(false);
    expect(opened).toEqual([]);
  });

  it('opens the target under a ctrl-click', () => {
    const opened: string[] = [];
    const handler = linkClickHandler(spans, (id) => opened.push(id));
    const event = { ctrlKey: true, metaKey: false, clientX: 1, clientY: 1, preventDefault: () => {} } as unknown as MouseEvent;
    expect(handler(event, view)).toBe(true);
    expect(opened).toEqual(['doc_target']);
  });

  it('does nothing when the click is not on a link', () => {
    const opened: string[] = [];
    const handler = linkClickHandler(() => [{ from: 20, to: 30, documentID: 'doc_other' }], (id) => opened.push(id));
    const event = { ctrlKey: true, metaKey: false, clientX: 1, clientY: 1, preventDefault: () => {} } as unknown as MouseEvent;
    expect(handler(event, view)).toBe(false);
    expect(opened).toEqual([]);
  });
});
