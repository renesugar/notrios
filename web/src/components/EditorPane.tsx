// Editor pane: title, save controls, the Markdown source editor (source-only
// mode — the rendered preview is the separate PreviewPane), and a collapsible
// "Note info" inspector for links, backlinks, and attached resources so the
// note context never becomes a fifth workspace column.
//
// Read-only notes (server-provided `editable: false`, e.g. the Help notebook)
// render with a disabled editor, hidden save/upload controls, and a visible
// badge — the client never waits for a server 403 to explain protection.
import { MdEditor, type UploadImgCallBack } from 'md-editor-rt';
import type { ThemeMode } from '../themes';
import { resourceContentURL, type DocumentLink, type DocumentRecord, type ResourceReference } from '../api';

export interface EditorPaneProps {
  title: string;
  onTitleChange: (value: string) => void;
  body: string;
  onBodyChange: (value: string) => void;
  selectedDocument: DocumentRecord | null;
  editable: boolean;
  busy: boolean;
  themeBase: ThemeMode;
  onSave: () => void;
  onNewNote: () => void;
  onUploadAndAttach: (file: File | null) => void;
  onEditorUploadImages: (files: Array<File>, callback: UploadImgCallBack) => void;
  links: DocumentLink[];
  backlinks: DocumentLink[];
  resources: ResourceReference[];
  onOpenDocument: (documentID: string) => void;
}

export function EditorPane(props: EditorPaneProps) {
  const {
    title,
    onTitleChange,
    body,
    onBodyChange,
    selectedDocument,
    editable,
    busy,
    themeBase,
    onSave,
    onNewNote,
    onUploadAndAttach,
    onEditorUploadImages,
    links,
    backlinks,
    resources,
    onOpenDocument,
  } = props;

  return (
    <section className="pane editor-pane" aria-label="Markdown editor" data-testid="pane-editor">
      <div className="editor-toolbar">
        <input
          className="title-input"
          value={title}
          onChange={(event) => onTitleChange(event.target.value)}
          placeholder="Note title"
          aria-label="Note title"
          disabled={!editable}
        />
        {!editable && (
          <span className="readonly-badge" data-testid="readonly-badge" role="status">
            Read-only Help note
          </span>
        )}
        {editable && (
          <button onClick={onSave} disabled={busy || title.trim() === ''} data-testid="save-button">
            {busy ? 'Working…' : selectedDocument ? 'Save revision' : 'Create note'}
          </button>
        )}
        {selectedDocument && (
          <button type="button" onClick={onNewNote}>
            New note
          </button>
        )}
      </div>

      <div className="editor-host">
        <MdEditor
          id="notrios-editor"
          value={body}
          onChange={onBodyChange}
          preview={false}
          readOnly={!editable}
          noUploadImg={!editable}
          onUploadImg={(files, callback) => {
            if (!editable) {
              callback([]);
              return;
            }
            onEditorUploadImages(files, callback);
          }}
          toolbarsExclude={['preview', 'previewOnly', 'htmlPreview', 'catalog', 'github', 'fullscreen', 'pageFullscreen', 'save']}
          language="en-US"
          theme={themeBase}
          noMermaid
          style={{ height: '100%' }}
        />
      </div>

      {selectedDocument && (
        <details className="note-inspector" data-testid="note-inspector">
          <summary>Note info</summary>
          <div className="note-inspector-body">
            <dl className="document-meta">
              <div>
                <dt>ID</dt>
                <dd>{selectedDocument.id}</dd>
              </div>
              <div>
                <dt>Revision</dt>
                <dd>{selectedDocument.current_revision_id}</dd>
              </div>
            </dl>
            {editable && (
              <label className="resource-upload">
                Upload image/PDF/resource
                <input
                  type="file"
                  disabled={busy}
                  onChange={(event) => {
                    const file = event.target.files?.[0] ?? null;
                    event.currentTarget.value = '';
                    onUploadAndAttach(file);
                  }}
                />
              </label>
            )}
            {(links.length > 0 || backlinks.length > 0) && (
              <div className="link-list">
                {links.length > 0 && (
                  <>
                    <strong>Outgoing links</strong>
                    <ul>
                      {links.map((link) => (
                        <li key={`out-${link.id}`}>
                          <LinkLabel link={link} onOpenDocument={onOpenDocument} />
                          <span className="muted">
                            {' '}
                            · {link.resolution_status} · {link.relation_type}
                          </span>
                        </li>
                      ))}
                    </ul>
                  </>
                )}
                {backlinks.length > 0 && (
                  <>
                    <strong>Backlinks</strong>
                    <ul>
                      {backlinks.map((link) => (
                        <li key={`in-${link.id}`}>
                          <button className="text-button" type="button" onClick={() => onOpenDocument(link.source_document_id)}>
                            {link.source_document_id}
                          </button>
                          <span className="muted"> · {link.raw_target}</span>
                        </li>
                      ))}
                    </ul>
                  </>
                )}
              </div>
            )}
            {resources.length > 0 && (
              <div className="resource-list">
                <strong>Attached resources</strong>
                <ul>
                  {resources.map((ref) => (
                    <li key={`${ref.resource_id}-${ref.relation_type}-${ref.ordinal ?? 0}`}>
                      <a href={resourceContentURL(ref.resource_id)}>{ref.resource?.filename || ref.resource_id}</a>
                      <span className="muted">
                        {' '}
                        · {ref.relation_type} · {ref.resource?.mime_type}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </details>
      )}
    </section>
  );
}

function LinkLabel({ link, onOpenDocument }: { link: DocumentLink; onOpenDocument: (documentID: string) => void }) {
  const label = link.display_text || link.raw_target || link.target_uri || '(untitled link)';
  if (link.target_document_id && link.resolution_status === 'resolved') {
    return (
      <button className="text-button" type="button" onClick={() => onOpenDocument(link.target_document_id ?? '')}>
        {label}
      </button>
    );
  }
  if (link.target_resource_id && link.resolution_status === 'resolved') {
    return <a href={resourceContentURL(link.target_resource_id)}>{label}</a>;
  }
  if (link.target_uri?.startsWith('http://') || link.target_uri?.startsWith('https://')) {
    return (
      <a href={link.target_uri} target="_blank" rel="noreferrer">
        {label}
      </a>
    );
  }
  return <span>{label}</span>;
}
