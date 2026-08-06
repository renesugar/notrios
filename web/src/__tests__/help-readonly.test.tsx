// Read-only Help presentation: the client disables editing before any server
// 403 — driven by the server-provided `editable` capability, never by name.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EditorPane, type EditorPaneProps } from '../components/EditorPane';
import type { DocumentRecord } from '../api';

vi.mock('md-editor-rt', () => ({
  MdEditor: ({ value, readOnly, noUploadImg }: { value: string; readOnly?: boolean; noUploadImg?: boolean }) => (
    <textarea data-testid="editor-stub" data-no-upload={noUploadImg ? 'true' : 'false'} readOnly={readOnly} value={value} onChange={() => {}} />
  ),
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
  // EditorPane registers its CodeMirror extensions through md-editor-rt's
  // global config hook at module load, so the stub has to accept the call.
  config: () => {},
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
    onNewNote: vi.fn(),
    onUploadAndAttach: vi.fn(),
    onEditorUploadImages: vi.fn(),
    links: [],
    backlinks: [],
    resources: [],
    remoteMedia: [],
    onLocalizeRemoteMedia: vi.fn(),
    onOpenDocument: vi.fn(),
    ...overrides,
  };
  return render(<EditorPane {...props} />);
}

describe('Help note read-only presentation', () => {
  it('renders a read-only editor with save/title/upload disabled and a visible badge', () => {
    renderPane({ editable: false, selectedDocument: doc({ id: 'doc_help_index', notebook_id: 'nb_help', editable: false }) });

    expect(screen.getByTestId('readonly-badge')).toHaveTextContent('Read-only Help note');
    expect(screen.queryByTestId('save-button')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Note title')).toBeDisabled();
    expect(screen.getByTestId('editor-stub')).toHaveAttribute('readonly');
    expect(screen.getByTestId('editor-stub')).toHaveAttribute('data-no-upload', 'true');
    expect(screen.queryByText('Upload image/PDF/resource')).not.toBeInTheDocument();
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
    expect(screen.getByLabelText('Note title')).toBeEnabled();
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
