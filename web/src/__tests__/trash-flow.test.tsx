// The delete/restore flow end to end in the app shell (v0.5 E8).
//
// Two properties matter here and neither is visible from the components alone:
// a cancelled confirmation must reach the service not at all, and a deletion
// must carry the revision the note was opened at, so a note edited elsewhere
// fails the precondition instead of being deleted out from under the other
// writer.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../App';

vi.mock('md-editor-rt', () => ({
  // The real editor renders `defToolbars` into its toolbar; a stub that drops
  // them would hide the notebook picker from every test that uses it.
  MdEditor: ({ value, readOnly, defToolbars }: { value: string; readOnly?: boolean; defToolbars?: React.ReactNode }) => (
    <>
      <div data-testid="editor-toolbar-stub">{defToolbars}</div>
      <textarea data-testid="editor-stub" readOnly={readOnly} value={value} onChange={() => {}} />
    </>
  ),
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
  config: () => {},
  // The editor's toolbar takes custom items; the picker is one, so the stub has
  // to render it rather than swallow it.
  DropdownToolbar: ({ children, overlay, disabled }: { children?: React.ReactNode; overlay?: React.ReactNode; disabled?: boolean }) => (
    <div data-testid="dropdown-toolbar" data-disabled={disabled ? 'true' : 'false'}>
      {children}
      {overlay}
    </div>
  ),
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

// `vi.hoisted` because vi.mock factories are hoisted above ordinary consts.
const { liveNote, trashedNote } = vi.hoisted(() => {
  const live = {
    id: 'doc_live',
    uri: 'document://default/documents/doc_live',
    collection_id: 'default',
    title: 'Live note',
    body_mime_type: 'text/markdown',
    body: 'hello',
    current_revision_id: 'rev_7',
    editable: true,
  };
  return { liveNote: live, trashedNote: { ...live, editable: false, deleted_at: '2026-08-06T10:00:00Z' } };
});

vi.mock('../api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api')>();
  return {
    ...original,
    getStatus: vi.fn().mockResolvedValue({ service: 'notrios', version: 'test', status: 'running', search_sidecar: { configured: false, available: false, active: false, state: 'off', backlog: 0, failed_jobs: 0 } }),
    getNotebookTree: vi.fn().mockResolvedValue([
      { id: 'nb_notes', name: 'Notes', builtin: false, position: 0 },
      { id: 'nb_work', name: 'Work', builtin: false, position: 1 },
    ]),
    listSearchNotebooks: vi.fn().mockResolvedValue([
      { id: 'snb_all_notes', name: 'All notes', query: '', builtin: true, sort_anchor: 'first' },
      { id: 'snb_trash', name: 'Trash', query: 'is:trashed', builtin: true, sort_anchor: 'last' },
    ]),
    listTags: vi.fn().mockResolvedValue([]),
    search: vi.fn().mockResolvedValue({ hits: [{ id: 'doc_live', uri: liveNote.uri, source: 'managed-notes', title: 'Live note' }], next_cursor: '' }),
    getDocument: vi.fn().mockResolvedValue(liveNote),
    listDocumentResources: vi.fn().mockResolvedValue({ resources: [] }),
    listDocumentLinks: vi.fn().mockResolvedValue({ outgoing: [], incoming: [] }),
    scanRemoteMedia: vi.fn().mockResolvedValue({ media: [], counts: {} }),
    deleteDocument: vi.fn().mockResolvedValue(undefined),
    restoreTrashedDocument: vi.fn().mockResolvedValue({ ...liveNote, title: 'Live note' }),
    purgeDocument: vi.fn().mockResolvedValue(undefined),
    previewNotebookDeletion: vi.fn().mockResolvedValue({
      notebook_id: 'nb_work',
      name: 'Work',
      notebooks: 2,
      descendant_names: ['Reports'],
      notes: 4,
      trashed_notes: 0,
      rehome_notebook_id: 'nb_notes',
      deletable: true,
    }),
    deleteNotebook: vi.fn().mockResolvedValue(undefined),
  };
});

import * as api from '../api';

async function openTheNote() {
  render(<App />);
  fireEvent.click(await screen.findByText('Live note'));
  return screen.findByTestId('delete-button');
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe('moving a note to the Trash', () => {
  it('asks first, and a refusal reaches the service not at all', async () => {
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(false));
    fireEvent.click(await openTheNote());
    expect(window.confirm).toHaveBeenCalled();
    expect(api.deleteDocument).not.toHaveBeenCalled();
    // The note is still open and still editable.
    expect(screen.getByTestId('delete-button')).toBeInTheDocument();
  });

  it('deletes against the opened revision, clears the editor, and drops the row', async () => {
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true));
    fireEvent.click(await openTheNote());

    await waitFor(() => expect(api.deleteDocument).toHaveBeenCalledWith('doc_live', 'rev_7'));
    expect(await screen.findByText(/Moved “Live note” to the Trash/)).toBeInTheDocument();
    // The result row went with it: the note no longer matches the query that
    // produced it, and re-running the search would lose the scroll position.
    await waitFor(() => expect(screen.queryByText('Live note')).toBeNull());
  });

  it('reports a failed precondition instead of pretending the note is gone', async () => {
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true));
    vi.mocked(api.deleteDocument).mockRejectedValueOnce(new Error('revision precondition failed'));
    fireEvent.click(await openTheNote());

    expect(await screen.findByText('revision precondition failed')).toBeInTheDocument();
    expect(screen.getByText('Live note')).toBeInTheDocument();
  });
});

describe('a note that is already in the Trash', () => {
  it('offers restore, and restoring reopens it as an editable note', async () => {
    vi.mocked(api.getDocument).mockResolvedValue(trashedNote);
    render(<App />);
    fireEvent.click(await screen.findByText('Live note'));

    const restore = await screen.findByTestId('restore-button');
    expect(screen.getByTestId('trashed-badge')).toBeInTheDocument();
    expect(screen.queryByTestId('delete-button')).toBeNull();

    fireEvent.click(restore);
    await waitFor(() => expect(api.restoreTrashedDocument).toHaveBeenCalledWith('doc_live'));
    expect(await screen.findByText(/Restored “Live note”/)).toBeInTheDocument();
    expect(await screen.findByTestId('delete-button')).toBeInTheDocument();
  });

  it('purges only after a confirmation that says it cannot be undone', async () => {
    vi.mocked(api.getDocument).mockResolvedValue(trashedNote);
    const confirmMock = vi.fn().mockReturnValue(true);
    vi.stubGlobal('confirm', confirmMock);
    render(<App />);
    fireEvent.click(await screen.findByText('Live note'));

    fireEvent.click(await screen.findByTestId('purge-button'));
    expect(confirmMock.mock.calls[0][0]).toContain('cannot be undone');
    await waitFor(() => expect(api.purgeDocument).toHaveBeenCalledWith('doc_live'));
    expect(await screen.findByText(/Permanently deleted “Live note”/)).toBeInTheDocument();
  });
});

describe('deleting a notebook', () => {
  it('asks the service what would happen and confirms with that answer', async () => {
    const confirmMock = vi.fn().mockReturnValue(true);
    vi.stubGlobal('confirm', confirmMock);
    render(<App />);

    fireEvent.click(await screen.findByTestId('sidebar-delete-nb_work'));
    await waitFor(() => expect(api.previewNotebookDeletion).toHaveBeenCalledWith('nb_work'));
    // The prompt carries the service's counts and the re-homing rule, not a
    // generic "are you sure".
    const prompt = confirmMock.mock.calls[0][0] as string;
    expect(prompt).toContain('2 notebooks are removed, including Reports.');
    expect(prompt).toContain('4 note(s) are not deleted: they move to the Trash');
    expect(prompt).toContain('Those notes move to “Notes”');
    await waitFor(() => expect(api.deleteNotebook).toHaveBeenCalledWith('nb_work'));
    expect(await screen.findByText(/4 note\(s\) moved to the Trash/)).toBeInTheDocument();
  });

  it('previews without deleting when the confirmation is refused', async () => {
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(false));
    render(<App />);

    fireEvent.click(await screen.findByTestId('sidebar-delete-nb_work'));
    await waitFor(() => expect(api.previewNotebookDeletion).toHaveBeenCalled());
    expect(api.deleteNotebook).not.toHaveBeenCalled();
  });

  it('refuses a protected notebook with the service reason rather than a failed request', async () => {
    vi.mocked(api.previewNotebookDeletion).mockResolvedValueOnce({
      notebook_id: 'nb_work',
      name: 'Work',
      notebooks: 1,
      descendant_names: [],
      notes: 0,
      trashed_notes: 0,
      rehome_notebook_id: 'nb_notes',
      deletable: false,
      reason: 'builtin notebooks cannot be deleted',
    });
    const confirmMock = vi.fn().mockReturnValue(true);
    vi.stubGlobal('confirm', confirmMock);
    render(<App />);

    fireEvent.click(await screen.findByTestId('sidebar-delete-nb_work'));
    expect(await screen.findByText(/builtin notebooks cannot be deleted/)).toBeInTheDocument();
    expect(confirmMock).not.toHaveBeenCalled();
    expect(api.deleteNotebook).not.toHaveBeenCalled();
  });
});
