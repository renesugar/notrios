// The notebook control that lives inside the Markdown editor's own toolbar.
//
// It is two things at once, and that is the point. It *shows* which notebook
// the note in front of you belongs to — the second visual cue after the
// sidebar's highlight — and changing it *files* the note there. Joplin's
// failure mode is writing several notes into the wrong notebook before
// noticing; a control that answers the question where the typing happens is
// what shortens that.
//
// It sits in `md-editor-rt`'s toolbar rather than in a row of its own for a
// measured reason: `.md-editor-toolbar-wrapper` is `overflow-x: auto`, so that
// toolbar scrolls at narrow widths instead of wrapping. A separate control
// above it would have reintroduced exactly the wrapping E10 removes.
import { useState } from 'react';
import { DropdownToolbar } from 'md-editor-rt';
import type { NotebookOption } from '../sidebar';

export interface NotebookPickerProps {
  options: NotebookOption[];
  /** The notebook currently shown. */
  selectedID: string | null;
  /**
   * The name to display. It is passed in rather than looked up in `options`
   * because `options` excludes builtin notebooks — they are not valid
   * destinations — and a note *in* one still has to show where it lives. A
   * Help note read "Notes" until this was separated.
   */
  label: string;
  /** Read-only and trashed notes show the control disabled, never hidden. */
  disabled: boolean;
  onSelect: (notebookID: string) => void;
}

export function NotebookPicker({ options, selectedID, label, disabled, onSelect }: NotebookPickerProps) {
  const [visible, setVisible] = useState(false);

  return (
    <DropdownToolbar
      title={disabled ? `Notebook: ${label}` : `Notebook: ${label} — click to file this note elsewhere`}
      visible={visible && !disabled}
      onChange={(next) => setVisible(disabled ? false : next)}
      disabled={disabled}
      overlay={
        <ul className="notebook-picker" role="listbox" aria-label="Notebook" data-testid="notebook-picker">
          {options.map((option) => (
            <li key={option.id}>
              <button
                type="button"
                role="option"
                aria-selected={option.id === selectedID}
                className={option.id === selectedID ? 'notebook-picker-option selected' : 'notebook-picker-option'}
                style={option.depth > 0 ? { paddingLeft: `${10 + option.depth * 14}px` } : undefined}
                data-testid={`notebook-option-${option.id}`}
                onClick={() => {
                  setVisible(false);
                  if (option.id !== selectedID) onSelect(option.id);
                }}
              >
                {option.name}
              </button>
            </li>
          ))}
        </ul>
      }
    >
      <span
        className="notebook-picker-trigger"
        data-testid="notebook-picker-trigger"
        aria-disabled={disabled ? 'true' : undefined}
      >
        <span className="notebook-picker-name">{label}</span>
      </span>
    </DropdownToolbar>
  );
}
