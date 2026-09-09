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
    // Spy on the object the module actually calls. `saveDraft` uses the bare
    // `localStorage` global, and `window.localStorage` is only guaranteed to be
    // the same object in some environments: under Node 22 in CI it was not, so
    // this patched something the module never touched, the real write succeeded,
    // and the test asserted a refusal that had not happened. It passed locally
    // on Node 26 for four milestones and failed the first time CI ran it.
    vi.spyOn(localStorage, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError');
    });
    expect(saveDraft({ documentID: null, title: 'T', body: 'small' })).toBe(false);
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
