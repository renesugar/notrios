// Selecting more than one note, and acting on the set (v0.8 H29).
//
// The interface could organise a library one note at a time while REST and the
// command line acted on five hundred. What was missing was not the operations —
// `POST /api/v1/batch` has had all six since v0.6 — but a way to say which
// notes, so these tests are about the selection and what it replaces.
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
  DropdownToolbar: ({ children, overlay }: { children?: React.ReactNode; overlay?: React.ReactNode }) => (
    <div data-testid="dropdown-toolbar">{children}{overlay}</div>
  ),
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

const { notes } = vi.hoisted(() => ({
  notes: ['one', 'two', 'three'].map((name, index) => ({
    id: `doc_${name}`,
    uri: `document://default/documents/doc_${name}`,
    collection_id: 'default',
    title: `Note ${name}`,
    body_mime_type: 'text/markdown',
    body: `body ${name}`,
    current_revision_id: `rev_${index}`,
    editable: true,
  })),
}));

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
    ]),
    listTags: vi.fn().mockResolvedValue([]),
    search: vi.fn().mockResolvedValue({
      hits: notes.map((note) => ({ id: note.id, uri: note.uri, source: 'managed-notes', title: note.title })),
      next_cursor: '',
    }),
    getDocument: vi.fn(async (id: string) => notes.find((note) => note.id === id)),
    listDocumentResources: vi.fn().mockResolvedValue({ resources: [] }),
    listDocumentLinks: vi.fn().mockResolvedValue({ outgoing: [], incoming: [] }),
    listDocumentTags: vi.fn().mockResolvedValue([]),
    scanRemoteMedia: vi.fn().mockResolvedValue({ media: [], counts: {} }),
    runBatch: vi.fn().mockResolvedValue({
      operation: 'add_tags', mode: 'best_effort', applied: 2, skipped: 0, failed: 0, rolled_back: 0,
      replayed: false, items: [
        { document_id: 'doc_one', status: 'applied' },
        { document_id: 'doc_two', status: 'applied' },
      ],
    }),
  };
});

import * as api from '../api';

async function checkboxes() {
  return await screen.findAllByTestId('result-check');
}

describe('selecting more than one note', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
  });
  afterEach(cleanup);

  it('replaces the editor with the operations that apply to a set', async () => {
    render(<App />);
    const boxes = await checkboxes();
    expect(screen.queryByTestId('selection-panel')).toBeNull();

    fireEvent.click(boxes[0]);
    fireEvent.click(boxes[1]);

    const panel = await screen.findByTestId('selection-panel');
    expect(panel).toBeTruthy();
    expect(screen.getByTestId('selection-count').textContent).toContain('2 notes selected');
    // The editor and the preview are both about one note, and with several
    // chosen there is no one note for either to show.
    expect(screen.queryByTestId('pane-editor')).toBeNull();
    expect(screen.queryByTestId('pane-preview')).toBeNull();
  });

  it('sends every checked note, and a request key so a second click is not a second run', async () => {
    render(<App />);
    const boxes = await checkboxes();
    fireEvent.click(boxes[0]);
    fireEvent.click(boxes[1]);

    fireEvent.change(await screen.findByTestId('selection-tag'), { target: { value: 'kitchen' } });
    fireEvent.click(screen.getByTestId('selection-add-tag'));

    await waitFor(() => expect(api.runBatch).toHaveBeenCalled());
    const request = vi.mocked(api.runBatch).mock.calls[0][0];
    expect(request.operation).toBe('add_tags');
    expect(request.tags).toEqual(['kitchen']);
    expect(request.items.map((item) => item.document_id)).toEqual(['doc_one', 'doc_two']);
    expect(request.request_key).toBeTruthy();
  });

  it('reports every item rather than a total', async () => {
    render(<App />);
    const boxes = await checkboxes();
    fireEvent.click(boxes[0]);
    fireEvent.click(boxes[1]);
    fireEvent.click(await screen.findByTestId('selection-duplicate'));

    const report = await screen.findByTestId('selection-report');
    expect(report.textContent).toContain('2 applied');
    expect(report.textContent).toContain('doc_one');
    expect(report.textContent).toContain('doc_two');
  });

  it('reads each note’s revision before trashing it, because the store requires one', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<App />);
    const boxes = await checkboxes();
    fireEvent.click(boxes[0]);
    fireEvent.click(boxes[1]);
    fireEvent.click(await screen.findByTestId('selection-trash'));

    await waitFor(() => expect(api.runBatch).toHaveBeenCalled());
    const request = vi.mocked(api.runBatch).mock.calls[0][0];
    expect(request.operation).toBe('trash');
    // Without this the store refuses every item: trash is preconditioned on a
    // revision and a search hit does not carry one.
    expect(request.items.every((item) => Boolean(item.base_revision_id))).toBe(true);
  });

  it('extends the selection over the results with shift, not over the click order', async () => {
    render(<App />);
    const boxes = await checkboxes();
    fireEvent.click(boxes[0]);
    // The third box, shift-held: everything between the anchor and it.
    fireEvent.click(boxes[2], { shiftKey: true });
    expect((await screen.findByTestId('selection-count')).textContent).toContain('3 notes selected');
  });

  it('leaves the selection without discarding anything, and comes back to the note', async () => {
    render(<App />);
    fireEvent.click(await screen.findByText('Note one'));
    await screen.findByTestId('pane-editor');

    const boxes = await checkboxes();
    fireEvent.click(boxes[1]);
    fireEvent.click(boxes[2]);
    await screen.findByTestId('selection-panel');

    fireEvent.click(screen.getByTestId('selection-clear'));

    // The editor returns showing the note that was open. Entering a selection
    // discards nothing, so nothing asked to.
    const editor = await screen.findByTestId('pane-editor');
    expect(editor).toBeTruthy();
    await waitFor(() => expect(screen.getByDisplayValue('Note one')).toBeTruthy());
  });
});
