// The panel that replaces the editor when several notes are picked out.
//
// Selecting more than one note is the primitive this is built on, and it is
// what the interface was missing: `SearchPane` tracked one note and rendered
// each hit as a button that opens it, so a person could organise a library one
// note at a time while REST and the command line could act on five hundred.
//
// It replaces the editor rather than opening beside it, for the reason Joplin's
// does: with several notes chosen there is no one note to read, and showing the
// last one clicked would be an answer to a question nobody asked.
import { useState } from 'react';
import type { BatchResult } from '../api';
import type { NotebookOption } from '../sidebar';

export interface SelectionPanelProps {
  /** The notes chosen, in the order the results list shows them. */
  selected: { id: string; title: string }[];
  notebooks: NotebookOption[];
  busy: boolean;
  /** The report from the last run, kept on screen until something changes it. */
  report: BatchResult | null;
  error: string;
  onClear: () => void;
  onMove: (notebookID: string) => void;
  onAddTag: (tag: string) => void;
  onRemoveTag: (tag: string) => void;
  onDuplicate: () => void;
  onTrash: () => void;
  onRestore: () => void;
}

export function SelectionPanel({
  selected, notebooks, busy, report, error,
  onClear, onMove, onAddTag, onRemoveTag, onDuplicate, onTrash, onRestore,
}: SelectionPanelProps) {
  const [notebookID, setNotebookID] = useState('');
  const [tag, setTag] = useState('');
  const trimmedTag = tag.trim();

  return (
    <section className="pane selection-pane" aria-label="Selected notes" data-testid="selection-panel">
      <div className="selection-header">
        <strong data-testid="selection-count">
          {selected.length} {selected.length === 1 ? 'note' : 'notes'} selected
        </strong>
        {/* Leaving the selection returns to the note that was open. Selecting
            several results and then changing your mind should not also move you
            somewhere else. */}
        <button type="button" data-testid="selection-clear" onClick={onClear}>Back to the note</button>
      </div>

      <ul className="selection-list" data-testid="selection-list">
        {selected.map((note) => <li key={note.id}>{note.title || note.id}</li>)}
      </ul>

      <div className="selection-actions">
        <label>Move to notebook
          <select value={notebookID} data-testid="selection-notebook"
            onChange={(event) => setNotebookID(event.target.value)}>
            <option value="">Choose a notebook…</option>
            {notebooks.map((notebook) => (
              <option key={notebook.id} value={notebook.id}>{'\u00a0'.repeat(notebook.depth * 2) + notebook.name}</option>
            ))}
          </select>
        </label>
        <button type="button" className="primary-button" data-testid="selection-move"
          disabled={busy || notebookID === ''} onClick={() => onMove(notebookID)}>
          Move
        </button>

        <label>Tag
          <input value={tag} data-testid="selection-tag" placeholder="field/dusk"
            onChange={(event) => setTag(event.target.value)} />
        </label>
        <div className="row-actions">
          <button type="button" data-testid="selection-add-tag"
            disabled={busy || trimmedTag === ''} onClick={() => onAddTag(trimmedTag)}>Add tag</button>
          <button type="button" data-testid="selection-remove-tag"
            disabled={busy || trimmedTag === ''} onClick={() => onRemoveTag(trimmedTag)}>Remove tag</button>
        </div>

        <div className="row-actions">
          <button type="button" data-testid="selection-duplicate"
            disabled={busy} onClick={onDuplicate}>Duplicate</button>
          <button type="button" data-testid="selection-restore"
            disabled={busy} onClick={onRestore}>Restore from Trash</button>
          {/* Destructive, and it goes to the Trash rather than through it: the
              same reversible delete every other surface performs. */}
          <button type="button" className="danger-button" data-testid="selection-trash"
            disabled={busy} onClick={onTrash}>Move to Trash</button>
        </div>
      </div>

      {error ? <p className="muted selection-error" role="status" data-testid="selection-error">{error}</p> : null}

      {report ? (
        // Every item, in both modes. A run that reports only a total cannot
        // tell you which half happened, and a note that was skipped -- a tag it
        // already had, a notebook it was already in -- is not a note that
        // failed.
        <div className="selection-report" data-testid="selection-report" role="status">
          <strong>
            {report.applied} applied · {report.skipped} skipped · {report.failed} failed
            {report.rolled_back > 0 ? ` · ${report.rolled_back} rolled back` : ''}
          </strong>
          <ul>
            {report.items.map((item) => (
              <li key={`${item.document_id}-${item.status}`}>
                <code>{item.document_id}</code> · {item.status}
                {item.reason ? ` · ${item.reason}` : ''}
                {item.error ? ` · ${item.error}` : ''}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}
