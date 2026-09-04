// The tag control that lives beside the notebook control in the editor toolbar.
//
// It exists because tagging a note was reachable over REST and MCP and from
// neither surface a person uses. v0.8 H14 found that by reading the
// documentation, then produced it again by comparing the two surfaces
// mechanically; the command line gained `tags add`, `tags remove` and `tags
// list`, and this is the other half.
//
// It sits in the toolbar for the reason the notebook picker gives: that toolbar
// is the one thing that stays with the note you are typing in, and a control in
// a panel somewhere else answers the question in a place you are not looking.
//
// Tags are shown as removable chips rather than as an editable comma list. A
// text field holding "a, b, c" makes removing one tag an exercise in careful
// deletion, and makes a stray comma into a new tag nobody asked for.
import { useEffect, useState } from 'react';
import { DropdownToolbar } from 'md-editor-rt';

export interface TagEditorProps {
  /** The note being edited, or null when nothing is open. */
  documentID: string | null;
  /** Tags currently on the note, in the order the server returned them. */
  tags: string[];
  /** Read-only and trashed notes show the control disabled, never hidden. */
  disabled: boolean;
  onAdd: (tag: string) => void;
  onRemove: (tag: string) => void;
}

export function TagEditor({ documentID, tags, disabled, onAdd, onRemove }: TagEditorProps) {
  const [visible, setVisible] = useState(false);
  const [draft, setDraft] = useState('');

  // Closing when the note changes, because a panel left open over a different
  // note invites tagging the wrong one.
  useEffect(() => {
    setVisible(false);
    setDraft('');
  }, [documentID]);

  const submit = () => {
    const value = draft.trim();
    if (value === '' || tags.includes(value)) {
      // Adding a tag twice is not an error and not a second tag; clearing the
      // field is the whole response.
      setDraft('');
      return;
    }
    onAdd(value);
    setDraft('');
  };

  const summary = tags.length === 0 ? 'Tags' : `Tags: ${tags.join(', ')}`;

  return (
    <DropdownToolbar
      title={disabled ? summary : `${summary} — click to add or remove`}
      visible={visible && !disabled}
      onChange={(next) => setVisible(next && !disabled)}
      // A span with a role rather than a bare span. The control crawl looks for
      // things that announce themselves as controls and did not see this one --
      // which is the same reason a screen reader would not have. The crawler
      // found an accessibility defect by being unable to find a button, which
      // is a better outcome than teaching it to look for spans.
      trigger={
        <span
          className="tag-editor-trigger"
          data-testid="tag-editor-trigger"
          role="button"
          tabIndex={disabled ? -1 : 0}
          aria-disabled={disabled}
          aria-label={summary}
        >
          {tags.length === 0 ? '🏷' : `🏷 ${tags.length}`}
        </span>
      }
      overlay={
        <div className="tag-editor" data-testid="tag-editor">
          <ul className="tag-editor-list" data-testid="tag-editor-list">
            {tags.map((tag) => (
              <li key={tag} className="tag-editor-chip">
                <span className="tag-editor-name">{tag}</span>
                <button
                  type="button"
                  className="tag-editor-remove"
                  data-testid={`tag-remove-${tag}`}
                  aria-label={`Remove tag ${tag}`}
                  disabled={disabled}
                  onClick={() => onRemove(tag)}
                >
                  ×
                </button>
              </li>
            ))}
            {tags.length === 0 && <li className="tag-editor-empty">No tags yet</li>}
          </ul>
          <div className="tag-editor-add">
            <input
              type="text"
              className="tag-editor-input"
              data-testid="tag-editor-input"
              placeholder="Add a tag"
              aria-label="Add a tag"
              value={draft}
              disabled={disabled}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={(event) => {
                // Enter adds, because typing a tag and pressing enter is what
                // people do. The key is stopped so md-editor-rt does not also
                // treat it as editor input.
                if (event.key === 'Enter') {
                  event.preventDefault();
                  event.stopPropagation();
                  submit();
                }
              }}
            />
            <button
              type="button"
              className="tag-editor-submit"
              data-testid="tag-editor-add"
              disabled={disabled || draft.trim() === ''}
              onClick={submit}
            >
              Add
            </button>
          </div>
        </div>
      }
    />
  );
}
