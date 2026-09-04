// The unsaved draft: what is in the editor but not yet in the store.
//
// Notrios saves on purpose rather than continuously, because every save writes
// a `document_revisions` row that is replicated and never pruned — autosaving
// would turn a morning's typing into a hundred synced revisions. The cost of
// that choice is that unsaved work exists at all, so the interface has to keep
// it: this module is the browser-side half of that, and the confirmations in
// `App.tsx` are the other half.
//
// One slot, not one per note. Leaving a dirty editor always goes through a
// confirmation, so at most one note can have unsaved changes at a time, and a
// per-note map would only add ways for the stored copy to disagree with the
// editor.
//
// The draft is the user's own note text held in their own browser. It is never
// sent anywhere, and it is removed as soon as it is saved or discarded.

export const DRAFT_KEY = 'notrios.draft.v1';

/**
 * Above this the draft is not stored. localStorage quotas are a few megabytes
 * and are shared with everything else the app keeps; a note large enough to
 * threaten that would evict the pane widths and themes to protect one draft,
 * and the write would likely throw anyway.
 */
export const MAX_DRAFT_BYTES = 1_000_000;

export interface Draft {
  /** The note being edited, or null for a note that has never been saved. */
  documentID: string | null;
  title: string;
  body: string;
  /** When the draft was last written, for the message shown on restore. */
  savedAt: string;
}

/** sanitizeDraft accepts unknown parsed JSON and returns a draft or null. */
export function sanitizeDraft(raw: unknown): Draft | null {
  if (typeof raw !== 'object' || raw === null) return null;
  const candidate = raw as Record<string, unknown>;
  if (typeof candidate.title !== 'string' || typeof candidate.body !== 'string') return null;
  const documentID = typeof candidate.documentID === 'string' && candidate.documentID !== '' ? candidate.documentID : null;
  const savedAt = typeof candidate.savedAt === 'string' ? candidate.savedAt : '';
  return { documentID, title: candidate.title, body: candidate.body, savedAt };
}

export function loadDraft(): Draft | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (!raw) return null;
    return sanitizeDraft(JSON.parse(raw));
  } catch {
    return null;
  }
}

/**
 * saveDraft stores the draft, and reports whether it is actually stored. The
 * caller shows a different tooltip when it is not: telling somebody their work
 * is being kept when the write failed is worse than not keeping it.
 */
export function saveDraft(draft: Omit<Draft, 'savedAt'>, at: Date = new Date()): boolean {
  const payload = JSON.stringify({ ...draft, savedAt: at.toISOString() });
  if (payload.length > MAX_DRAFT_BYTES) return false;
  try {
    localStorage.setItem(DRAFT_KEY, payload);
    return true;
  } catch {
    return false;
  }
}

export function clearDraft() {
  try {
    localStorage.removeItem(DRAFT_KEY);
  } catch {
    // Nothing to do: the slot is either already gone or unreachable.
  }
}

/**
 * draftDiffers reports whether the editor holds anything the store does not.
 * The comparison is against the note as it was loaded (or the new-note
 * template), which is why a note reopened unchanged is not dirty.
 */
export function draftDiffers(baseline: { title: string; body: string }, current: { title: string; body: string }): boolean {
  return baseline.title !== current.title || baseline.body !== current.body;
}

/**
 * discardPrompt is the question asked before unsaved work is thrown away. It
 * names the note and says what is lost, rather than "are you sure": the same
 * rule the Trash confirmations follow.
 */
export function discardPrompt(noteTitle: string, action: string): string {
  const named = noteTitle.trim() === '' ? 'this note' : `“${noteTitle.trim()}”`;
  return `You have unsaved changes to ${named}.\n\nDiscard them and ${action}?`;
}
