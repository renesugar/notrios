// Where a note came from, in the note inspector.
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { EditorPane } from '../components/EditorPane';
import type { DocumentRecord } from '../api';

vi.mock('md-editor-rt', () => ({
  MdEditor: ({ defToolbars }: { defToolbars?: React.ReactNode }) => <div>{defToolbars}</div>,
  MdPreview: () => <div />,
  config: () => {},
  DropdownToolbar: ({ children, overlay }: { children?: React.ReactNode; overlay?: React.ReactNode }) => (
    <div>{children}{overlay}</div>
  ),
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

import type { EditorPaneProps } from '../components/EditorPane';

/** Everything the pane needs, so the test can say one thing about one field. */
function paneProps(selectedDocument: DocumentRecord): EditorPaneProps {
  return {
    title: selectedDocument.title, onTitleChange: vi.fn(),
    body: selectedDocument.body ?? '', onBodyChange: vi.fn(),
    selectedDocument, editable: true, busy: false, themeBase: 'light',
    onSave: vi.fn(), onUploadAndAttach: vi.fn(), onEditorUploadImages: vi.fn(),
    links: [], backlinks: [], resources: [], remoteMedia: [],
    onLocalizeRemoteMedia: vi.fn(), onOpenDocument: vi.fn(), trashed: false,
    notebookOptions: [], tags: [], onAddTag: vi.fn(), onRemoveTag: vi.fn(),
    notebookID: 'nb_notes', notebookLabel: 'Notes', onSelectNotebook: vi.fn(),
    onDelete: vi.fn(), onRestore: vi.fn(), onPurge: vi.fn(),
  };
}

function note(collectionID: string): DocumentRecord {
  return {
    id: 'doc_1', uri: 'document://default/documents/doc_1', collection_id: collectionID,
    notebook_id: 'nb_notes', title: 'A note', body: 'text', body_mime_type: 'text/markdown',
    current_revision_id: 'rev_1', created_at: '', updated_at: '', editable: true,
  } as DocumentRecord;
}

describe('a note says where it came from', () => {
  it('names the collection when the note was imported', () => {
    render(<EditorPane {...paneProps(note('joplin-raw-2026-07'))} />);
    expect(screen.getByTestId('note-collection')).toHaveTextContent('joplin-raw-2026-07');
  });

  // Every note written here is in `default`. Printing it on all of them would
  // be a row that says nothing, and would make provenance look like a property
  // of notes rather than of imports.
  it('says nothing for a note written here', () => {
    render(<EditorPane {...paneProps(note('default'))} />);
    expect(screen.queryByTestId('note-collection')).toBeNull();
  });
});
