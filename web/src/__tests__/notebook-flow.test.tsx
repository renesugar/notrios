// Filing notes, end to end in the app shell (v0.6 F0).
//
// Before this, the GUI created every note in "Notes" and offered no way to move
// one, so a note filed wrongly could not be corrected from the built-in client
// at all. These assert the two halves: a new note follows the sidebar, and a
// note already written can be re-filed.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../App';

vi.mock('md-editor-rt', () => ({
  MdEditor: ({ value, readOnly, defToolbars }: { value: string; readOnly?: boolean; defToolbars?: React.ReactNode }) => (
    <>
      <div data-testid="editor-toolbar-stub">{defToolbars}</div>
      <textarea data-testid="editor-stub" readOnly={readOnly} value={value} onChange={() => {}} />
    </>
  ),
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
  config: () => {},
  DropdownToolbar: ({ children, overlay, disabled }: { children?: React.ReactNode; overlay?: React.ReactNode; disabled?: boolean }) => (
    <div data-testid="dropdown-toolbar" data-disabled={disabled ? 'true' : 'false'}>
      {children}
      {overlay}
    </div>
  ),
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

const { note } = vi.hoisted(() => ({
  note: {
    id: 'doc_a',
    uri: 'document://default/documents/doc_a',
    collection_id: 'default',
    notebook_id: 'nb_notes',
    title: 'A note',
    body_mime_type: 'text/markdown',
    body: 'hello',
    current_revision_id: 'rev_1',
    editable: true,
  },
}));

vi.mock('../api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api')>();
  return {
    ...original,
    getStatus: vi.fn().mockResolvedValue({ service: 'notrios', version: 'test', status: 'running', search_sidecar: { configured: false, available: false, active: false, state: 'off', backlog: 0, failed_jobs: 0 } }),
    getNotebookTree: vi.fn().mockResolvedValue([
      { id: 'nb_notes', name: 'Notes', builtin: false, position: 0 },
      { id: 'nb_work', name: 'Work', builtin: false, position: 1 },
      { id: 'nb_help', name: 'Help', builtin: true, position: 2 },
    ]),
    listSearchNotebooks: vi.fn().mockResolvedValue([
      { id: 'snb_all_notes', name: 'All notes', query: '', builtin: true, sort_anchor: 'first' },
      { id: 'snb_trash', name: 'Trash', query: 'is:trashed', builtin: true, sort_anchor: 'last' },
    ]),
    listTags: vi.fn().mockResolvedValue([{ id: 'tag_1', name: 'todo', note_count: 1 }]),
    search: vi.fn().mockResolvedValue({ hits: [{ id: 'doc_a', uri: note.uri, source: 'managed-notes', title: 'A note' }], next_cursor: '' }),
    getDocument: vi.fn().mockResolvedValue(note),
    listDocumentResources: vi.fn().mockResolvedValue({ resources: [] }),
    listDocumentLinks: vi.fn().mockResolvedValue({ outgoing: [], incoming: [] }),
    scanRemoteMedia: vi.fn().mockResolvedValue({ media: [], counts: {} }),
    createDocument: vi.fn().mockResolvedValue({ ...note, id: 'doc_new', title: 'Fresh' }),
    moveDocumentToNotebook: vi.fn().mockResolvedValue({ ...note, notebook_id: 'nb_work' }),
  };
});

import * as api from '../api';

beforeEach(() => localStorage.clear());
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('a new note follows the sidebar selection', () => {
  it('starts in the default notebook and says so', async () => {
    render(<App />);
    expect((await screen.findByTestId('new-note-button')).textContent).toContain('Notes');
  });

  it('retargets when a notebook is selected, and creates there', async () => {
    render(<App />);
    fireEvent.click(await screen.findByTestId('sidebar-row-nb_work'));
    await waitFor(() => expect(screen.getByTestId('new-note-button').textContent).toContain('Work'));

    fireEvent.click(screen.getByTestId('new-note-button'));
    fireEvent.change(screen.getByLabelText('Note title'), { target: { value: 'Fresh' } });
    fireEvent.click(screen.getByTestId('save-button'));
    await waitFor(() =>
      expect(api.createDocument).toHaveBeenCalledWith(expect.objectContaining({ notebook_id: 'nb_work' })),
    );
  });

  // "All notes", the Trash, Help, and a tag name a view rather than a place.
  // Falling back to the default is the right answer there, not a consolation.
  it('falls back to the default for views that are not places', async () => {
    render(<App />);
    fireEvent.click(await screen.findByTestId('sidebar-row-nb_work'));
    await waitFor(() => expect(screen.getByTestId('new-note-button').textContent).toContain('Work'));

    for (const rowID of ['snb_all_notes', 'snb_trash', 'nb_help']) {
      fireEvent.click(await screen.findByTestId(`sidebar-row-nb_work`));
      fireEvent.click(await screen.findByTestId(`sidebar-row-${rowID}`));
      await waitFor(() => expect(screen.getByTestId('new-note-button').textContent).toContain('Notes'));
    }

    // A tag is not a notebook either, and selecting one clears the target.
    fireEvent.click(await screen.findByTestId('sidebar-row-nb_work'));
    await waitFor(() => expect(screen.getByTestId('new-note-button').textContent).toContain('Work'));
    fireEvent.click(screen.getByText('todo'));
    await waitFor(() => expect(screen.getByTestId('new-note-button').textContent).toContain('Notes'));
  });

  // The sidebar highlight is the primary cue, and it has to survive the
  // name-collision trap: selection is tracked by ID, not by the row's
  // `notebook:"<name>"` query.
  it('highlights the selected row by identity', async () => {
    render(<App />);
    const work = await screen.findByTestId('sidebar-row-nb_work');
    fireEvent.click(work);
    await waitFor(() => expect(work.getAttribute('aria-current')).toBe('true'));
    expect(screen.getByTestId('sidebar-row-nb_notes').getAttribute('aria-current')).toBeNull();
  });
});

describe('re-filing a note that is already written', () => {
  it('moves the open note and says where it landed', async () => {
    render(<App />);
    fireEvent.click(await screen.findByText('A note'));
    await screen.findByTestId('notebook-picker-trigger');
    // The control shows the note's own notebook, not the sidebar's selection.
    expect(screen.getByTestId('notebook-picker-trigger').textContent).toBe('Notes');

    fireEvent.click(screen.getByTestId('notebook-option-nb_work'));
    await waitFor(() => expect(api.moveDocumentToNotebook).toHaveBeenCalledWith('doc_a', 'nb_work'));
    expect(await screen.findByText(/Filed “A note” in “Work”/)).toBeInTheDocument();
  });

  it('reports a refused move instead of showing the note as moved', async () => {
    vi.mocked(api.moveDocumentToNotebook).mockRejectedValueOnce(new Error('Help notes cannot be moved'));
    render(<App />);
    fireEvent.click(await screen.findByText('A note'));
    await screen.findByTestId('notebook-picker-trigger');

    fireEvent.click(screen.getByTestId('notebook-option-nb_work'));
    expect(await screen.findByText('Help notes cannot be moved')).toBeInTheDocument();
    expect(screen.getByTestId('notebook-picker-trigger').textContent).toBe('Notes');
  });
});
