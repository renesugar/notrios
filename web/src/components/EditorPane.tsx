// Editor pane: title, save controls, the Markdown source editor (source-only
// mode — the rendered preview is the separate PreviewPane), and a collapsible
// "Note info" inspector for links, backlinks, and attached resources so the
// note context never becomes a fifth workspace column.
//
// Read-only notes (server-provided `editable: false`, e.g. the Help notebook)
// render with a disabled editor, hidden save/upload controls, and a visible
// badge — the client never waits for a server 403 to explain protection.
import { useEffect, useMemo, useRef } from 'react';
import { MdEditor, config, type ExposeParam, type UploadImgCallBack } from 'md-editor-rt';
import type { ThemeMode } from '../themes';
import { resourceContentURL, type DocumentLink, type DocumentRecord, type RemoteMediaDecision, type ResourceReference } from '../api';
import { useBufferLinks } from '../useLinkIntelligence';
import { BrokenLinkList, LinkPicker } from './LinkIntelligence';
import {
  applyBrokenLinks,
  brokenLinkExtensions,
  clearBrokenLinks,
  linkClickHandler,
  linkCompletionSource,
} from '../editor-extensions';
import { spansToEditorRanges } from '../editor-offsets';

// md-editor-rt's editor pane is CodeMirror 6, and it accepts CodeMirror
// extensions through this hook. Registering once at module load is what its
// global config is for; the extensions themselves hold no note state.
config({
  codeMirrorExtensions(extensions) {
    return [
      ...extensions,
      ...brokenLinkExtensions().map((extension, index) => ({
        type: `notrios-broken-links-${index}`,
        extension,
      })),
    ];
  },
});

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
  /** Server-side policy decisions for remote media found in this note. */
  remoteMedia: RemoteMediaDecision[];
  /** Localize policy-allowed remote media into local resources. */
  onLocalizeRemoteMedia: () => void;
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
    remoteMedia,
    onLocalizeRemoteMedia,
    onOpenDocument,
  } = props;

  const editorRef = useRef<ExposeParam>(null);
  const bufferLinks = useBufferLinks(body, selectedDocument?.id, editable);

  // Resolved link spans in editor coordinates, for Ctrl-click. They are derived
  // from the body the service checked, not the current one, which is why the
  // click handler re-reads them through a ref rather than closing over them.
  const clickableSpans = useMemo(() => {
    const resolved = bufferLinks.links.filter((link) => link.target_document_id);
    return spansToEditorRanges(bufferLinks.checkedBody, resolved).map((span, index) => ({
      ...span,
      documentID: resolved[index]?.target_document_id,
    }));
  }, [bufferLinks.links, bufferLinks.checkedBody]);
  const clickableRef = useRef(clickableSpans);
  clickableRef.current = clickableSpans;

  const completions = useMemo(
    () => [linkCompletionSource(() => selectedDocument?.id)],
    [selectedDocument?.id],
  );

  // Push the marks into CodeMirror whenever a check returns. `applyBrokenLinks`
  // refuses when the buffer has moved on, so a stale offset never underlines
  // the wrong text.
  useEffect(() => {
    const view = editorRef.current?.getEditorView();
    if (!view) return;
    if (!bufferLinks.checked) {
      clearBrokenLinks(view);
      return;
    }
    applyBrokenLinks(view, bufferLinks.checkedBody, bufferLinks.links);
  }, [bufferLinks.checked, bufferLinks.checkedBody, bufferLinks.links]);

  // Ctrl-click opens the target. `posAtCoords` is the source-position access the
  // editor turned out to expose after all.
  useEffect(() => {
    if (!editorRef.current) return;
    editorRef.current.domEventHandlers({
      mousedown: linkClickHandler(() => clickableRef.current, onOpenDocument),
    });
  }, [onOpenDocument]);

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
          ref={editorRef}
          completions={completions}
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
            {editable && (
              <div className="link-intelligence" data-testid="link-intelligence">
                <LinkPicker
                  documentID={selectedDocument.id}
                  disabled={busy}
                  onInsert={(markdown) => {
                    editorRef.current?.insert(() => ({
                      targetValue: markdown,
                      select: false,
                      deviationStart: 0,
                      deviationEnd: 0,
                    }));
                  }}
                />
                <BrokenLinkList
                  broken={bufferLinks.broken}
                  checked={bufferLinks.checked}
                  total={bufferLinks.links.length}
                />
              </div>
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
            {remoteMedia.length > 0 && (
              <div className="remote-media-list" data-testid="remote-media-list">
                <strong>Remote media ({remoteMedia.length})</strong>
                {editable && remoteMedia.some((decision) => decision.action === 'allow') && (
                  <button
                    type="button"
                    className="localize-button"
                    data-testid="localize-button"
                    disabled={busy}
                    onClick={onLocalizeRemoteMedia}
                    title="Download allowed remote media through the quarantine pipeline and rewrite this note to local resource:// links"
                  >
                    Localize allowed media
                  </button>
                )}
                <p className="muted remote-media-hint">
                  Detected by a server-side policy scan; nothing has been downloaded.
                </p>
                <ul>
                  {remoteMedia.map((decision) => (
                    <li key={decision.url}>
                      <span className={`media-action media-action-${decision.action}`}>{decision.action}</span>{' '}
                      <span className="remote-media-url">{decision.url}</span>
                      <span className="muted">
                        {' '}
                        · {decision.media_class} · {decision.reason}
                        {decision.line ? ` · line ${decision.line}` : ''}
                      </span>
                    </li>
                  ))}
                </ul>
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
