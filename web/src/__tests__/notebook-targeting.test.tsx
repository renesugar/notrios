// The editor toolbar's arrangement, and where a note gets filed (v0.5 E10 + v0.6 F0).
//
// These assert arrangement and behaviour, never CSS. The stacking itself is a
// container query and belongs to the browser sweep; what is checkable here is
// that the title and the actions are separate rows, that the toolbar offers
// only actions on the open note, and that a note's notebook comes from the note
// rather than from wherever the sidebar happens to point.
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EditorPane, type EditorPaneProps } from '../components/EditorPane';
import { SearchPane, type SearchPaneProps } from '../components/SearchPane';
import { creationTargetFor, notebookOptions, composeSidebar } from '../sidebar';
import type { DocumentRecord, NotebookTreeNode, SearchNotebook } from '../api';

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
  DropdownToolbar: ({ children, overlay, disabled }: { children?: React.ReactNode; overlay?: React.ReactNode; disabled?: boolean }) => (
    <div data-testid="dropdown-toolbar" data-disabled={disabled ? 'true' : 'false'}>
      {children}
      {overlay}
    </div>
  ),
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

afterEach(cleanup);

const choices = [
  { id: 'nb_notes', name: 'Notes', depth: 0 },
  { id: 'nb_work', name: 'Work', depth: 0 },
  { id: 'nb_reports', name: 'Reports', depth: 1 },
];

function doc(overrides: Partial<DocumentRecord> = {}): DocumentRecord {
  return {
    id: 'doc_x',
    uri: 'document://default/documents/doc_x',
    collection_id: 'default',
    notebook_id: 'nb_work',
    title: 'Title',
    body_mime_type: 'text/markdown',
    current_revision_id: 'rev_1',
    editable: true,
    ...overrides,
  };
}

function renderPane(overrides: Partial<EditorPaneProps> = {}) {
  const props: EditorPaneProps = {
    title: 'Title',
    onTitleChange: vi.fn(),
    body: '# body',
    onBodyChange: vi.fn(),
    selectedDocument: doc(),
    editable: true,
    busy: false,
    themeBase: 'light',
    onSave: vi.fn(),
    onUploadAndAttach: vi.fn(),
    onEditorUploadImages: vi.fn(),
    links: [],
    backlinks: [],
    resources: [],
    remoteMedia: [],
    onLocalizeRemoteMedia: vi.fn(),
    onOpenDocument: vi.fn(),
    trashed: false,
    notebookOptions: choices,
    notebookID: 'nb_work',
    defaultNotebookName: 'Notes',
    onSelectNotebook: vi.fn(),
    onDelete: vi.fn(),
    onRestore: vi.fn(),
    onPurge: vi.fn(),
    ...overrides,
  };
  return { props, ...render(<EditorPane {...props} />) };
}

describe('the editor toolbar is a title row and an action row', () => {
  // The chip used to share the title's row at every width. Separate rows is
  // the structural fix; the stacking is a container query the browser sweep
  // covers.
  it('keeps the title out of the row that holds the chip and the buttons', () => {
    renderPane({ trashed: true, editable: false, selectedDocument: doc({ editable: false, deleted_at: '2026-08-06T10:00:00Z' }) });
    const actions = screen.getByTestId('editor-toolbar-actions');
    expect(within(actions).getByTestId('trashed-badge')).toBeInTheDocument();
    expect(within(actions).getByTestId('restore-button')).toBeInTheDocument();
    expect(within(actions).getByTestId('purge-button')).toBeInTheDocument();
    // The title input is a sibling of the action row, not a member of it.
    expect(within(actions).queryByLabelText('Note title')).toBeNull();
    expect(screen.getByLabelText('Note title')).toBeInTheDocument();
  });

  it('marks the trashed action row so it can stack sooner than the editable one', () => {
    renderPane({ trashed: true, editable: false, selectedDocument: doc({ editable: false, deleted_at: 'x' }) });
    expect(screen.getByTestId('editor-toolbar-actions').className).toContain('trashed-actions');
    cleanup();
    renderPane({});
    expect(screen.getByTestId('editor-toolbar-actions').className).not.toContain('trashed-actions');
  });
});

describe('"New note" is not an action on the open note', () => {
  // It used to render for every open note, including read-only Help notes and
  // trashed ones, because the condition tested only that a note was open.
  it('is absent from the editor toolbar in every state', () => {
    for (const state of [
      {},
      { editable: false, selectedDocument: doc({ editable: false }) },
      { trashed: true, editable: false, selectedDocument: doc({ editable: false, deleted_at: 'x' }) },
      { selectedDocument: null },
    ] as Partial<EditorPaneProps>[]) {
      renderPane(state);
      expect(screen.queryByText('New note')).toBeNull();
      cleanup();
    }
  });

  // Hiding it without relocating it would strand a reader: it is the only path
  // back to a blank draft, so on a library holding only read-only Help notes
  // there would be no way to create one.
  it('is offered by the search pane whatever is open, and names its destination', () => {
    const onNewNote = vi.fn();
    const props: SearchPaneProps = {
      query: '',
      onQueryChange: vi.fn(),
      onSubmit: vi.fn(),
      paged: { hits: [], loading: false, exhausted: true, started: true, error: null, start: vi.fn(), loadMore: vi.fn(), patchHit: vi.fn(), prependHit: vi.fn(), removeHit: vi.fn() },
      onOpenHit: vi.fn(),
      selectedDocumentID: null,
      busy: false,
      onNewNote,
      newNoteNotebookName: 'Work',
    };
    render(<SearchPane {...props} />);
    const button = screen.getByTestId('new-note-button');
    expect(button.textContent).toContain('Work');
    fireEvent.click(button);
    expect(onNewNote).toHaveBeenCalled();
  });
});

describe('the notebook control in the editor toolbar', () => {
  it('shows the open note’s notebook and files it elsewhere on selection', () => {
    const { props } = renderPane({});
    expect(screen.getByTestId('notebook-picker-trigger').textContent).toBe('Work');
    fireEvent.click(screen.getByTestId('notebook-option-nb_reports'));
    expect(props.onSelectNotebook).toHaveBeenCalledWith('nb_reports');
  });

  it('does not re-file a note into the notebook it is already in', () => {
    const { props } = renderPane({});
    fireEvent.click(screen.getByTestId('notebook-option-nb_work'));
    expect(props.onSelectNotebook).not.toHaveBeenCalled();
  });

  // Disabled rather than hidden: a note's notebook stays visible even where it
  // cannot be changed, which is the point of the control being a second cue.
  it('is disabled but present for a read-only note and a trashed one', () => {
    renderPane({ editable: false, selectedDocument: doc({ editable: false }) });
    expect(screen.getByTestId('dropdown-toolbar').getAttribute('data-disabled')).toBe('true');
    expect(screen.getByTestId('notebook-picker-trigger')).toBeInTheDocument();
    cleanup();
    renderPane({ trashed: true, editable: false, selectedDocument: doc({ editable: false, deleted_at: 'x' }) });
    expect(screen.getByTestId('dropdown-toolbar').getAttribute('data-disabled')).toBe('true');
  });

  it('falls back to the default notebook label when nothing matches', () => {
    renderPane({ notebookID: null });
    expect(screen.getByTestId('notebook-picker-trigger').textContent).toBe('Notes');
  });
});

describe('choosing where a new note is filed', () => {
  const tree: NotebookTreeNode[] = [
    { id: 'nb_notes', name: 'Notes', builtin: false, position: 0 },
    { id: 'nb_work', name: 'Work', builtin: false, position: 1, children: [{ id: 'nb_reports', name: 'Reports', builtin: false, position: 0 }] },
    { id: 'nb_help', name: 'Help', builtin: true, position: 2 },
  ];
  const searches: SearchNotebook[] = [
    { id: 'snb_all_notes', name: 'All notes', query: '', builtin: true, sort_anchor: 'first' },
    { id: 'snb_trash', name: 'Trash', query: 'is:trashed', builtin: true, sort_anchor: 'last' },
  ];
  const rows = composeSidebar(tree, searches);
  const rowFor = (id: string) => rows.find((row) => row.id === id)!;

  it('targets the selected notebook, including a nested one', () => {
    expect(creationTargetFor(rowFor('nb_work'))).toBe('nb_work');
    expect(creationTargetFor(rowFor('nb_reports'))).toBe('nb_reports');
  });

  // "All notes", a saved search, and Help name a view rather than a place, so
  // the service's default is the right answer rather than a consolation.
  it('defers to the default notebook for views that are not places', () => {
    expect(creationTargetFor(rowFor('snb_all_notes'))).toBeNull();
    expect(creationTargetFor(rowFor('snb_trash'))).toBeNull();
    expect(creationTargetFor(rowFor('nb_help'))).toBeNull();
    expect(creationTargetFor(null)).toBeNull();
  });

  // Help is refused by the service, so offering it would offer a guaranteed
  // 403; the picker mirrors the rule rather than discovering it.
  it('never offers a builtin notebook as a destination, and keeps tree order', () => {
    expect(notebookOptions(tree)).toEqual([
      { id: 'nb_notes', name: 'Notes', depth: 0 },
      { id: 'nb_work', name: 'Work', depth: 0 },
      { id: 'nb_reports', name: 'Reports', depth: 1 },
    ]);
  });
});
