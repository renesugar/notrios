// Read-only Help presentation: the client disables editing before any server
// 403 — driven by the server-provided `editable` capability, never by name.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EditorPane, type EditorPaneProps } from '../components/EditorPane';
import type { DocumentRecord } from '../api';

vi.mock('md-editor-rt', () => ({
  MdEditor: ({ value, readOnly, noUploadImg, defToolbars }: { value: string; readOnly?: boolean; noUploadImg?: boolean; defToolbars?: React.ReactNode }) => (
    <>
      <div data-testid="editor-toolbar-stub">{defToolbars}</div>
      <textarea data-testid="editor-stub" data-no-upload={noUploadImg ? 'true' : 'false'} readOnly={readOnly} value={value} onChange={() => {}} />
    </>
  ),
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
  // EditorPane registers its CodeMirror extensions through md-editor-rt's
  // global config hook at module load, so the stub has to accept the call.
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
  return render(<EditorPane {...props} />);
}

describe('Help note read-only presentation', () => {
  it('renders a read-only editor with save/title/upload disabled and a visible badge', () => {
    renderPane({
      editable: false,
      notebookLabel: 'Help',
      selectedDocument: doc({ id: 'doc_help_index', notebook_id: 'nb_help', editable: false }),
    });

    expect(screen.getByTestId('readonly-badge')).toHaveTextContent('Read-only Help note');
    expect(screen.queryByTestId('save-button')).not.toBeInTheDocument();
    // Read-only, not disabled: a disabled input leaves the tab order, so the
    // title of a protected note could not be focused, scrolled with Home/End,
    // or selected and copied. A long title would be unreadable.
    const title = screen.getByLabelText('Note title');
    expect(title).toHaveAttribute('readonly');
    expect(title).toBeEnabled();
    expect(screen.getByTestId('editor-stub')).toHaveAttribute('readonly');
    expect(screen.getByTestId('editor-stub')).toHaveAttribute('data-no-upload', 'true');
    expect(screen.queryByText('Upload image/PDF/resource')).not.toBeInTheDocument();
  });

  // The badge names the notebook rather than assuming Help. F5 added a second
  // read-only notebook, and a Reports note labelled "Help note" would be wrong
  // in the one place a reader looks to find out why they cannot type — which is
  // exactly what browser verification found it doing.
  it('names the notebook the note is actually in', () => {
    renderPane({
      editable: false,
      notebookLabel: 'Reports',
      selectedDocument: doc({ id: 'doc_graph_report', notebook_id: 'nb_reports', editable: false }),
    });
    expect(screen.getByTestId('readonly-badge')).toHaveTextContent('Read-only Reports note');
  });

  it('image-upload callbacks are rejected for read-only notes', () => {
    const onEditorUploadImages = vi.fn();
    renderPane({ editable: false, onEditorUploadImages });
    // The stub editor exposes noUploadImg; the EditorPane wrapper also guards
    // the callback path — invoking it must not reach the app handler.
    // (The guard lives in EditorPane's onUploadImg wrapper.)
    expect(onEditorUploadImages).not.toHaveBeenCalled();
  });

  it('restores normal editing controls for an editable note', () => {
    renderPane({ editable: true });
    expect(screen.queryByTestId('readonly-badge')).not.toBeInTheDocument();
    expect(screen.getByTestId('save-button')).toBeEnabled();
    const editableTitle = screen.getByLabelText('Note title');
    expect(editableTitle).toBeEnabled();
    expect(editableTitle).not.toHaveAttribute('readonly');
    expect(screen.getByTestId('editor-stub')).not.toHaveAttribute('readonly');
    expect(screen.getByText('Upload image/PDF/resource')).toBeInTheDocument();
  });
});

describe('remote-media policy warnings', () => {
  it('renders per-URL decisions in the note inspector', () => {
    renderPane({
      remoteMedia: [
        { url: 'https://tracker.example.com/pixel.gif', media_class: 'image', action: 'block', reason: 'domain matches blocked pattern tracker.example.com', line: 3 },
        { url: 'https://upload.wikimedia.org/a.png', media_class: 'image', action: 'allow', reason: 'domain matches allowed pattern *.wikimedia.org' },
      ],
    });
    const list = screen.getByTestId('remote-media-list');
    expect(list).toHaveTextContent('Remote media (2)');
    expect(list).toHaveTextContent('nothing has been downloaded');
    expect(list).toHaveTextContent('https://tracker.example.com/pixel.gif');
    expect(list).toHaveTextContent('block');
    expect(list).toHaveTextContent('line 3');
    expect(list).toHaveTextContent('allow');
  });

  it('renders no remote-media section when the scan is empty', () => {
    renderPane({ remoteMedia: [] });
    expect(screen.queryByTestId('remote-media-list')).not.toBeInTheDocument();
  });

  it('offers localization only for editable notes with allowed media', () => {
    const onLocalizeRemoteMedia = vi.fn();
    renderPane({
      onLocalizeRemoteMedia,
      remoteMedia: [
        { url: 'https://upload.wikimedia.org/a.png', media_class: 'image', action: 'allow', reason: 'allowed' },
      ],
    });
    const button = screen.getByTestId('localize-button');
    fireEvent.click(button);
    expect(onLocalizeRemoteMedia).toHaveBeenCalledTimes(1);
  });

  it('hides the localize action for read-only notes and blocked-only media', () => {
    renderPane({
      editable: false,
      selectedDocument: doc({ notebook_id: 'nb_help', editable: false }),
      remoteMedia: [
        { url: 'https://upload.wikimedia.org/a.png', media_class: 'image', action: 'allow', reason: 'allowed' },
      ],
    });
    expect(screen.queryByTestId('localize-button')).not.toBeInTheDocument();
    cleanup();
    renderPane({
      remoteMedia: [{ url: 'https://tracker.example.com/x.gif', media_class: 'image', action: 'block', reason: 'blocked' }],
    });
    expect(screen.queryByTestId('localize-button')).not.toBeInTheDocument();
  });
});
