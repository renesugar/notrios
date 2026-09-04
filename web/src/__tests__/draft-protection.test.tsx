// The unsaved draft, end to end in the app shell.
//
// Notrios saves on purpose: every save writes a revision that is replicated
// and never pruned, so the button stays and autosave does not arrive. That
// choice is only defensible if the interface protects what has not been saved,
// which is three properties, none of them visible from a component alone:
// nothing replaces the editor without asking, a refusal changes nothing at
// all, and a reload finds the work again.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../App';
import { DRAFT_KEY, loadDraft } from '../draft';

vi.mock('md-editor-rt', () => ({
  // Unlike the other app-level stubs this one is writable: the whole subject
  // here is what happens after somebody types.
  MdEditor: ({ value, readOnly, onChange, defToolbars }: { value: string; readOnly?: boolean; onChange?: (v: string) => void; defToolbars?: React.ReactNode }) => (
    <>
      <div data-testid="editor-toolbar-stub">{defToolbars}</div>
      <textarea data-testid="editor-stub" readOnly={readOnly} value={value} onChange={(event) => onChange?.(event.target.value)} />
    </>
  ),
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
  config: () => {},
  DropdownToolbar: ({ children, overlay }: { children?: React.ReactNode; overlay?: React.ReactNode }) => (
    <div>{children}{overlay}</div>
  ),
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

const { firstNote, secondNote } = vi.hoisted(() => {
  const first = {
    id: 'doc_first',
    uri: 'document://default/documents/doc_first',
    collection_id: 'default',
    title: 'First note',
    body_mime_type: 'text/markdown',
    body: 'first body',
    current_revision_id: 'rev_1',
    editable: true,
  };
  return { firstNote: first, secondNote: { ...first, id: 'doc_second', uri: 'document://default/documents/doc_second', title: 'Second note', body: 'second body' } };
});

vi.mock('../api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api')>();
  return {
    ...original,
    getStatus: vi.fn().mockResolvedValue({ service: 'notrios', version: 'test', status: 'running', search_sidecar: { configured: false, available: false, active: false, state: 'off', backlog: 0, failed_jobs: 0 } }),
    getNotebookTree: vi.fn().mockResolvedValue([{ id: 'nb_notes', name: 'Notes', builtin: false, position: 0 }]),
    listSearchNotebooks: vi.fn().mockResolvedValue([{ id: 'snb_all_notes', name: 'All notes', query: '', builtin: true, sort_anchor: 'first' }]),
    listTags: vi.fn().mockResolvedValue([]),
    listDocumentTags: vi.fn().mockResolvedValue([]),
    search: vi.fn().mockResolvedValue({
      hits: [
        { id: 'doc_first', uri: firstNote.uri, source: 'managed-notes', title: 'First note' },
        { id: 'doc_second', uri: secondNote.uri, source: 'managed-notes', title: 'Second note' },
      ],
      next_cursor: '',
    }),
    getDocument: vi.fn().mockImplementation((id: string) => Promise.resolve(id === 'doc_second' ? secondNote : firstNote)),
    listDocumentResources: vi.fn().mockResolvedValue({ resources: [] }),
    listDocumentLinks: vi.fn().mockResolvedValue({ outgoing: [], incoming: [] }),
    scanRemoteMedia: vi.fn().mockResolvedValue({ media: [], counts: {} }),
    updateDocument: vi.fn().mockResolvedValue({ ...firstNote, body: 'first body, edited', current_revision_id: 'rev_2' }),
    createDocument: vi.fn().mockResolvedValue({ ...firstNote, id: 'doc_new', title: 'Fresh', body: 'typed' }),
  };
});

import * as api from '../api';

/** Opens the first note and types into it, which is where every case starts. */
async function openAndType(text = 'first body, edited') {
  render(<App />);
  fireEvent.click(await screen.findByText('First note'));
  const editor = await screen.findByTestId('editor-stub');
  fireEvent.change(editor, { target: { value: text } });
  return screen.findByTestId('unsaved-badge');
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe('an editor holding unsaved work', () => {
  it('says so, and keeps the draft in this browser', async () => {
    await openAndType();
    await waitFor(() => expect(loadDraft()?.body).toBe('first body, edited'));
    expect(loadDraft()?.documentID).toBe('doc_first');
  });

  it('does not claim unsaved work when the note is only reopened', async () => {
    render(<App />);
    fireEvent.click(await screen.findByText('First note'));
    await screen.findByTestId('save-button');
    expect(screen.queryByTestId('unsaved-badge')).toBeNull();
    expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
  });

  it('asks before another note replaces it, and a refusal opens nothing', async () => {
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(false));
    await openAndType();
    vi.mocked(api.getDocument).mockClear();

    fireEvent.click(screen.getByText('Second note'));
    expect((window.confirm as unknown as ReturnType<typeof vi.fn>).mock.calls[0][0]).toContain('unsaved changes');
    expect(api.getDocument).not.toHaveBeenCalled();
    // Still the first note, still holding what was typed.
    expect(screen.getByTestId('editor-stub')).toHaveValue('first body, edited');
    expect(screen.getByTestId('unsaved-badge')).toBeInTheDocument();
  });

  it('opens the other note once the discard is confirmed, and forgets the draft', async () => {
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true));
    await openAndType();

    fireEvent.click(screen.getByText('Second note'));
    await waitFor(() => expect(screen.getByTestId('editor-stub')).toHaveValue('second body'));
    expect(screen.queryByTestId('unsaved-badge')).toBeNull();
    await waitFor(() => expect(localStorage.getItem(DRAFT_KEY)).toBeNull());
  });

  it('asks before a new note replaces it', async () => {
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(false));
    await openAndType();

    fireEvent.click(screen.getByRole('button', { name: /New note in/ }));
    expect(window.confirm).toHaveBeenCalled();
    expect(screen.getByTestId('editor-stub')).toHaveValue('first body, edited');
  });

  it('writes the last keystrokes when the window is going away', async () => {
    await openAndType();
    // The stored copy is written on a short delay; typing and closing straight
    // afterwards is exactly the case the delay could lose.
    fireEvent.change(screen.getByTestId('editor-stub'), { target: { value: 'typed right at the end' } });
    window.dispatchEvent(new Event('beforeunload', { cancelable: true }));
    expect(loadDraft()?.body).toBe('typed right at the end');
  });

  it('drops the badge and the stored draft once it is saved', async () => {
    await openAndType();
    fireEvent.click(screen.getByTestId('save-button'));

    await waitFor(() => expect(api.updateDocument).toHaveBeenCalled());
    await waitFor(() => expect(screen.queryByTestId('unsaved-badge')).toBeNull());
    expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
  });
});

describe('the desktop shell', () => {
  it('is told what the editor is holding, because closing the window is not the frontend\'s to intercept', async () => {
    const reported: boolean[] = [];
    (window as unknown as { go: unknown }).go = {
      main: { WindowState: { SetUnsavedChanges: (unsaved: boolean) => { reported.push(unsaved); return Promise.resolve(); } } },
    };
    try {
      await openAndType();
      await waitFor(() => expect(reported).toContain(true));
      fireEvent.click(screen.getByTestId('save-button'));
      await waitFor(() => expect(reported[reported.length - 1]).toBe(false));
    } finally {
      delete (window as unknown as { go?: unknown }).go;
    }
  });
});

describe('a window that is reloaded with unsaved work', () => {
  it('reopens the note and puts the typed text back, ready to save as a revision', async () => {
    await openAndType();
    await waitFor(() => expect(loadDraft()).not.toBeNull());
    cleanup(); // the reload

    render(<App />);
    await waitFor(() => expect(screen.getByTestId('editor-stub')).toHaveValue('first body, edited'));
    expect(await screen.findByText(/Restored unsaved changes/)).toBeInTheDocument();
    // Loaded as the note it belongs to, so saving writes a revision of it
    // rather than a second note with the same words in it.
    expect(api.getDocument).toHaveBeenCalledWith('doc_first');
    fireEvent.click(screen.getByTestId('save-button'));
    await waitFor(() => expect(api.updateDocument).toHaveBeenCalled());
    expect(api.createDocument).not.toHaveBeenCalled();
  });

  it('restores a note that was never saved without inventing a document', async () => {
    render(<App />);
    await screen.findByTestId('save-button');
    fireEvent.change(await screen.findByTestId('editor-stub'), { target: { value: 'typed' } });
    await waitFor(() => expect(loadDraft()?.documentID).toBeNull());
    cleanup();

    render(<App />);
    await waitFor(() => expect(screen.getByTestId('editor-stub')).toHaveValue('typed'));
    fireEvent.click(screen.getByTestId('save-button'));
    await waitFor(() => expect(api.createDocument).toHaveBeenCalled());
  });
});
