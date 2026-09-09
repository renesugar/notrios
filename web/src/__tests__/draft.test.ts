// The storage half of draft protection. The app-level behaviour is in
// draft-protection.test.tsx; these are the cases that only appear when the
// stored slot is damaged, oversized, or unavailable — where the wrong answer
// is a crash or a silent promise to keep work that is not being kept.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DRAFT_KEY, MAX_DRAFT_BYTES, clearDraft, discardPrompt, draftDiffers, loadDraft, sanitizeDraft, saveDraft } from '../draft';

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

describe('the stored draft', () => {
  it('round-trips through storage with the note it belongs to', () => {
    expect(saveDraft({ documentID: 'doc_1', title: 'T', body: 'B' }, new Date('2026-09-04T12:00:00Z'))).toBe(true);
    expect(loadDraft()).toEqual({ documentID: 'doc_1', title: 'T', body: 'B', savedAt: '2026-09-04T12:00:00.000Z' });
    clearDraft();
    expect(loadDraft()).toBeNull();
  });

  it('reads nothing rather than throwing when the slot is damaged', () => {
    localStorage.setItem(DRAFT_KEY, 'not json');
    expect(loadDraft()).toBeNull();
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ title: 'only a title' }));
    expect(loadDraft()).toBeNull();
    expect(sanitizeDraft({ documentID: 7, title: 'T', body: 'B' })).toEqual({ documentID: null, title: 'T', body: 'B', savedAt: '' });
  });

  it('reports failure instead of pretending, when the draft is too big or storage refuses', () => {
    expect(saveDraft({ documentID: null, title: 'T', body: 'x'.repeat(MAX_DRAFT_BYTES) })).toBe(false);
    // Replace the global binding rather than spying on the storage object.
    //
    // Spying does not survive the difference between environments, and the way
    // it fails is silent. Under Node 22 `localStorage` is jsdom's `Storage`, a
    // Proxy whose defineProperty trap *stores items*: assigning `setItem` on it
    // writes an entry called "setItem" and leaves the real method in place, so
    // the spy was never called and the write succeeded. Under Node 26 it is
    // Node's own built-in `MemoryStorage`, which is not a jsdom `Storage` at
    // all -- `localStorage instanceof Storage` is false -- so spying on
    // `Storage.prototype` patches a prototype nothing here inherits from.
    //
    // An instance spy passes on 26 and silently no-ops on 22; a prototype spy
    // does the reverse. Both leave a test that asserts a refusal which never
    // happened. `saveDraft` reads the global at call time, so swapping the
    // binding works wherever the test runs and cannot half-apply.
    const original = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: {
        setItem() {
          throw new Error('QuotaExceededError');
        },
      },
    });
    try {
      expect(saveDraft({ documentID: null, title: 'T', body: 'small' })).toBe(false);
    } finally {
      if (original) Object.defineProperty(globalThis, 'localStorage', original);
    }
  });
});

describe('what counts as unsaved', () => {
  it('is any difference from the note as it was loaded', () => {
    expect(draftDiffers({ title: 'T', body: 'B' }, { title: 'T', body: 'B' })).toBe(false);
    expect(draftDiffers({ title: 'T', body: 'B' }, { title: 'T', body: 'B ' })).toBe(true);
    expect(draftDiffers({ title: 'T', body: 'B' }, { title: 't', body: 'B' })).toBe(true);
  });

  it('is asked about by name, and says what is lost', () => {
    expect(discardPrompt('Shopping', 'open the other note')).toContain('unsaved changes to “Shopping”');
    expect(discardPrompt('Shopping', 'open the other note')).toContain('Discard them and open the other note?');
    expect(discardPrompt('   ', 'start a new note')).toContain('changes to this note');
  });
});
