// Renaming a tag hierarchy: see what it would do, then agree to it.
//
// A rename reaches every note carrying the tag and, with children included,
// every note under it. At a terminal that is a number printed afterwards; here
// the whole point is that the number comes first.
//
// The report shown is not a prediction. The service performs the rename inside
// a transaction and rolls it back for a dry run, so what this displays and what
// applying would do cannot disagree — which is a stronger promise than the
// importers' dry runs make, and worth saying out loud in the interface rather
// than only in the API description.
import { useCallback, useEffect, useRef, useState } from 'react';
import { renameTag, type TagRenameResult } from '../api';

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function TagRename({ tag, onClose, onRenamed }: {
  tag: string;
  onClose: () => void;
  onRenamed: (result: TagRenameResult) => void;
}) {
  const [to, setTo] = useState(tag);
  const [includeChildren, setIncludeChildren] = useState(true);
  const [preview, setPreview] = useState<TagRenameResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  // Any edit invalidates the report. A preview of a different rename sitting
  // above an Apply button is how somebody agrees to something they did not
  // read.
  const changeTo = useCallback((value: string) => {
    setTo(value);
    setPreview(null);
    setError('');
  }, []);
  const changeChildren = useCallback((value: boolean) => {
    setIncludeChildren(value);
    setPreview(null);
    setError('');
  }, []);

  const run = useCallback(async (dryRun: boolean) => {
    setBusy(true);
    setError('');
    try {
      const result = await renameTag(tag, to.trim(), includeChildren, dryRun);
      if (dryRun) setPreview(result);
      else onRenamed(result);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setBusy(false);
    }
  }, [tag, to, includeChildren, onRenamed]);

  const unchanged = to.trim() === '' || to.trim() === tag;

  return (
    <div className="sync-center-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="about-dialog" role="dialog" aria-modal="true" aria-labelledby="tag-rename-title"
        data-testid="tag-rename">
        <h2 id="tag-rename-title">Rename “{tag}”</h2>

        <label>New name
          <input ref={inputRef} value={to} data-testid="tag-rename-to"
            onChange={(event) => changeTo(event.target.value)} maxLength={200} autoComplete="off" />
        </label>
        <label className="checkbox-row">
          <input type="checkbox" checked={includeChildren} data-testid="tag-rename-children"
            onChange={(event) => changeChildren(event.target.checked)} />
          Rename everything under “{tag}/” as well
        </label>

        <div className="row-actions">
          <button type="button" data-testid="tag-rename-preview" disabled={busy || unchanged}
            onClick={() => void run(true)}>Show me what changes</button>
          {/* Apply is reachable only from a report of this exact rename. */}
          <button type="button" className="primary-button" data-testid="tag-rename-apply"
            disabled={busy || preview === null}
            onClick={() => void run(false)}>Rename</button>
          <button type="button" data-testid="tag-rename-cancel" onClick={onClose}>Cancel</button>
        </div>

        {preview && (
          <div className="tag-rename-report" data-testid="tag-rename-report">
            <p>
              <strong>{preview.changes.length}</strong> {preview.changes.length === 1 ? 'tag' : 'tags'} affected,
              touching <strong>{preview.notes}</strong> {preview.notes === 1 ? 'note' : 'notes'}.
            </p>
            <ul>
              {preview.changes.map((change) => (
                <li key={change.tag_id}>
                  <code>{change.from}</code> → <code>{change.to}</code>
                  {change.action === 'merge' ? (
                    // A merge is not a rename: the destination already exists,
                    // and afterwards the two are one tag. Saying "merges into"
                    // is the difference between a reader expecting a rename and
                    // getting one.
                    <span className="tag-rename-merge"> merges into an existing tag; {change.notes_gained} of
                      its {change.notes} {change.notes === 1 ? 'note' : 'notes'} gain it</span>
                  ) : (
                    <span className="muted"> · {change.notes} {change.notes === 1 ? 'note' : 'notes'}</span>
                  )}
                </li>
              ))}
            </ul>
            {preview.warnings.length > 0 && (
              <ul className="tag-rename-warnings" data-testid="tag-rename-warnings">
                {preview.warnings.map((warning) => <li key={warning}>{warning}</li>)}
              </ul>
            )}
          </div>
        )}

        {error && <p className="sync-inline-notice error" role="status" data-testid="tag-rename-error">{error}</p>}
      </section>
    </div>
  );
}
