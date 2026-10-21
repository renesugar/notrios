// The preview marks from the editor's own buffer check (v1.0 J40-B), so the
// editor hands each check up as it lands, and a read-only note, which the
// editor does not check, gives the preview nothing to mark from.
import { cleanup, render, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EditorPane, type EditorPaneProps } from '../components/EditorPane';
import type { DocumentRecord } from '../api';

vi.mock('md-editor-rt', () => ({
  MdEditor: ({ value }: { value: string }) => <textarea data-testid="editor-stub" value={value} onChange={() => {}} />,
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
  config: () => {},
  DropdownToolbar: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

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
    body: 'See [[index]].',
    onBodyChange: vi.fn(),
    selectedDocument: doc(),
    editable: true,
    busy: false,
    unsaved: false,
    draftKept: true,
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
    notebookOptions: [{ id: 'nb_notes', name: 'Notes', depth: 0 }],
    notebookID: 'nb_notes',
    notebookLabel: 'Notes',
    onSelectNotebook: vi.fn(),
    onDelete: vi.fn(),
    onRestore: vi.fn(),
    onPurge: vi.fn(),
    ...overrides,
  };
  return render(<EditorPane {...props} />);
}

const link = { raw_target: 'index', status: 'ambiguous', start_byte: 4, end_byte: 13, line: 1, column: 5 };

function stubCheck() {
  const fetchMock = vi.fn(async (url: string) => {
    if (url === '/api/v1/links/check') {
      return new Response(JSON.stringify({ links: [link], total: 1, unresolved: 1 }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }
    // The inspector's local graph also asks; an empty graph keeps it quiet.
    return new Response(JSON.stringify({ nodes: [], edges: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } });
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('J40-B: the editor hands its check to the preview', () => {
  it('passes up the body it checked and the links it found', async () => {
    stubCheck();
    const onLinksChecked = vi.fn();
    renderPane({ onLinksChecked });
    await waitFor(() => expect(onLinksChecked).toHaveBeenCalledWith({ body: 'See [[index]].', links: [link] }), { timeout: 3000 });
  });

  it('passes up nothing for a read-only note, and asks the service nothing', async () => {
    const fetchMock = stubCheck();
    const onLinksChecked = vi.fn();
    renderPane({ onLinksChecked, editable: false, selectedDocument: doc({ editable: false }) });
    await new Promise((resolve) => setTimeout(resolve, 900));
    expect(onLinksChecked).toHaveBeenCalled();
    expect(onLinksChecked.mock.calls.every(([check]) => check === null)).toBe(true);
    expect(fetchMock.mock.calls.some(([url]) => url === '/api/v1/links/check')).toBe(false);
  });
});
