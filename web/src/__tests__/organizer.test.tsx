// Trash-first deletion in the GUI (v0.5 E8).
//
// The point of these tests is not that buttons exist. It is that the client
// never invents the rules: what a trashed note looks like comes from the
// server's `deleted_at`, and what deleting a notebook does to the notes inside
// it comes from the service's own preview.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EditorPane, type EditorPaneProps } from '../components/EditorPane';
import { SidebarPane } from '../components/SidebarPane';
import { notebookDeletionPrompt, notebookName } from '../organizer';
import { composeSidebar } from '../sidebar';
import type { DocumentRecord, NotebookDeletionPreview, NotebookTreeNode, SearchNotebook } from '../api';

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

afterEach(cleanup);

function doc(overrides: Partial<DocumentRecord> = {}): DocumentRecord {
  return {
    id: 'doc_x',
    uri: 'document://default/documents/doc_x',
    collection_id: 'default',
    title: 'Title',
    body_mime_type: 'text/markdown',
    current_revision_id: 'rev_1',
    editable: true,
    ...overrides,
  };
}

function renderPane(overrides: Partial<EditorPaneProps>) {
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
    tags: [],
    onAddTag: vi.fn(),
    onRemoveTag: vi.fn(),
    notebookOptions: [
      { id: 'nb_notes', name: 'Notes', depth: 0 },
      { id: 'nb_work', name: 'Work', depth: 0 },
    ],
    notebookID: 'nb_notes',
    notebookLabel: 'Work',
    onSelectNotebook: vi.fn(),
    onDelete: vi.fn(),
    onRestore: vi.fn(),
    onPurge: vi.fn(),
    ...overrides,
  };
  return { props, ...render(<EditorPane {...props} />) };
}

describe('trash-first deletion in the editor pane', () => {
  it('offers Move to Trash on an editable note and nothing else', () => {
    const { props } = renderPane({});
    fireEvent.click(screen.getByTestId('delete-button'));
    expect(props.onDelete).toHaveBeenCalled();
    expect(screen.queryByTestId('restore-button')).toBeNull();
    expect(screen.queryByTestId('purge-button')).toBeNull();
  });

  // A trashed note and a Help note are both uneditable, but only one of them
  // can be brought back, and calling a trashed note "read-only" would hide
  // exactly that.
  it('distinguishes a trashed note from a read-only Help note', () => {
    const { props } = renderPane({
      trashed: true,
      editable: false,
      selectedDocument: doc({ editable: false, deleted_at: '2026-08-06T10:00:00Z' }),
    });
    expect(screen.getByTestId('trashed-badge').textContent).toContain('In the Trash');
    expect(screen.queryByTestId('readonly-badge')).toBeNull();
    // Nothing offers to edit or delete it again; the two offers are recovery
    // and permanent removal.
    expect(screen.queryByTestId('save-button')).toBeNull();
    expect(screen.queryByTestId('delete-button')).toBeNull();

    fireEvent.click(screen.getByTestId('restore-button'));
    expect(props.onRestore).toHaveBeenCalled();
    fireEvent.click(screen.getByTestId('purge-button'));
    expect(props.onPurge).toHaveBeenCalled();
  });

  it('keeps the Help badge and offers no deletion for a protected note', () => {
    renderPane({ editable: false, selectedDocument: doc({ editable: false }) });
    expect(screen.getByTestId('readonly-badge').textContent).toContain('Read-only');
    expect(screen.queryByTestId('trashed-badge')).toBeNull();
    expect(screen.queryByTestId('delete-button')).toBeNull();
    expect(screen.queryByTestId('restore-button')).toBeNull();
  });
});

describe('notebook deletion in the sidebar', () => {
  const tree: NotebookTreeNode[] = [
    { id: 'nb_notes', name: 'Notes', builtin: false, position: 0 },
    { id: 'nb_work', name: 'Work', builtin: false, position: 1 },
    { id: 'nb_help', name: 'Help', builtin: true, position: 2 },
  ];
  const searches: SearchNotebook[] = [
    { id: 'snb_all_notes', name: 'All notes', query: '', builtin: true, sort_anchor: 'first' },
    { id: 'snb_trash', name: 'Trash', query: 'is:trashed', builtin: true, sort_anchor: 'last' },
  ];

  // The service refuses to delete builtins and the default notebook. Offering
  // a button that always fails would be a lie about what the app can do.
  it('offers deletion only where the service would allow it', () => {
    const onDeleteNotebook = vi.fn();
    render(
      <SidebarPane
        rows={composeSidebar(tree, searches)}
        tags={[]}
        activeQuery=""
        selectedRowID={null}
        onSelectRow={vi.fn()}
        onSelectQuery={vi.fn()}
        onDeleteNotebook={onDeleteNotebook}
        onRenameTag={vi.fn()}
      />,
    );
    expect(screen.queryByTestId('sidebar-delete-nb_notes')).toBeNull();
    expect(screen.queryByTestId('sidebar-delete-nb_help')).toBeNull();
    expect(screen.queryByTestId('sidebar-delete-snb_trash')).toBeNull();
    expect(screen.queryByTestId('sidebar-delete-snb_all_notes')).toBeNull();

    fireEvent.click(screen.getByTestId('sidebar-delete-nb_work'));
    expect(onDeleteNotebook).toHaveBeenCalledWith(expect.objectContaining({ id: 'nb_work' }));
  });
});

describe('the notebook deletion confirmation', () => {
  function preview(overrides: Partial<NotebookDeletionPreview> = {}): NotebookDeletionPreview {
    return {
      notebook_id: 'nb_work',
      name: 'Work',
      notebooks: 1,
      descendant_names: [],
      notes: 0,
      trashed_notes: 0,
      rehome_notebook_id: 'nb_notes',
      deletable: true,
      ...overrides,
    };
  }

  // The re-homing rule is the part nobody expects, and it is the reason a
  // restore later has somewhere to land. If the prompt does not say it, the
  // GUI has hidden a store rule rather than surfaced it.
  it('says the notes survive, and where they land', () => {
    const text = notebookDeletionPrompt(preview({ notebooks: 3, descendant_names: ['Reports', 'Drafts'], notes: 12 }), 'Notes');
    expect(text).toContain('Delete the notebook “Work”?');
    expect(text).toContain('3 notebooks are removed, including Reports, Drafts.');
    expect(text).toContain('12 note(s) are not deleted: they move to the Trash');
    expect(text).toContain('Those notes move to “Notes”');
  });

  it('does not promise a rescue for an empty notebook', () => {
    const text = notebookDeletionPrompt(preview(), 'Notes');
    expect(text).toContain('It holds no notes.');
    expect(text).not.toContain('move to the Trash');
  });

  it('counts notes already in the Trash separately', () => {
    const text = notebookDeletionPrompt(preview({ notes: 2, trashed_notes: 5 }), 'Notes');
    expect(text).toContain('5 note(s) already in the Trash are also affected.');
  });

  it('marks a truncated descendant list rather than implying it is complete', () => {
    const text = notebookDeletionPrompt(preview({ notebooks: 90, descendant_names: ['A', 'B'], truncated: true }), 'Notes');
    expect(text).toContain('A, B, …');
  });

  it('resolves the re-home notebook by ID through the tree, including children', () => {
    const tree: NotebookTreeNode[] = [
      { id: 'nb_a', name: 'A', builtin: false, position: 0, children: [{ id: 'nb_notes', name: 'Notes', builtin: false, position: 0 }] },
    ];
    expect(notebookName(tree, 'nb_notes')).toBe('Notes');
    expect(notebookName(tree, 'nb_missing')).toBe('nb_missing');
  });
});
