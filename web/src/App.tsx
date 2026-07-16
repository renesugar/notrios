import { useEffect, useMemo, useState, type MouseEvent } from 'react';
import { MdEditor, type PreviewRendererProps, type UploadImgCallBack } from 'md-editor-rt';
import 'md-editor-rt/lib/style.css';
import {
  attachResource,
  createDocument,
  getDocument,
  getNotebookTree,
  getStatus,
  listDocumentLinks,
  listDocumentResources,
  listSearchNotebooks,
  listTags,
  resourceContentURL,
  search,
  updateDocument,
  uploadResource,
  type DocumentLink,
  type DocumentRecord,
  type NotebookTreeNode,
  type ResourceReference,
  type SearchHit,
  type SearchNotebook,
  type StatusResponse,
  type TagRecord,
} from './api';

const defaultBody = `# New note\n\nThis Markdown note will be saved through the REST API and indexed by SQLite FTS5.\n\nTry linking another note with:\n\n[Related note](document://default/documents/<document-id>)\n`;

export function App() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [selectedDocument, setSelectedDocument] = useState<DocumentRecord | null>(null);
  const [title, setTitle] = useState('New note');
  const [body, setBody] = useState(defaultBody);
  const [resources, setResources] = useState<ResourceReference[]>([]);
  const [links, setLinks] = useState<DocumentLink[]>([]);
  const [backlinks, setBacklinks] = useState<DocumentLink[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [notebooks, setNotebooks] = useState<NotebookTreeNode[]>([]);
  const [searchNotebooks, setSearchNotebooks] = useState<SearchNotebook[]>([]);
  const [tags, setTags] = useState<TagRecord[]>([]);
  const [nextCursor, setNextCursor] = useState('');
  const [activeQuery, setActiveQuery] = useState('');

  async function refreshSidebar() {
    try {
      const [tree, savedSearches, tagList] = await Promise.all([getNotebookTree(), listSearchNotebooks(), listTags()]);
      setNotebooks(tree);
      setSearchNotebooks(savedSearches);
      setTags(tagList);
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  useEffect(() => {
    getStatus()
      .then(setStatus)
      .catch((err: unknown) => setError(errorMessage(err)));
    void refreshSidebar();
    // Startup view: the "All notes" search notebook (empty query) with
    // incremental cursor loading.
    void onSearch('');
    const openHelp = () => void onSearch('notebook:help');
    window.addEventListener('notrios:open-help', openHelp);
    return () => window.removeEventListener('notrios:open-help', openHelp);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const statusText = useMemo(() => {
    if (!status) return 'loading...';
    const schema = status.database_info?.schema_version ? ` schema ${status.database_info.schema_version}` : '';
    return `${status.service} ${status.version} · ${status.status} · ${status.database ?? 'unknown database'}${schema}`;
  }, [status]);

  async function refreshDocumentSidebars(documentID: string) {
    const [resourcePage, linkPage] = await Promise.all([
      listDocumentResources(documentID),
      listDocumentLinks(documentID, 'both'),
    ]);
    setResources(resourcePage.resources);
    setLinks(linkPage.outgoing ?? []);
    setBacklinks(linkPage.incoming ?? []);
  }

  async function onSearch(nextQuery = query) {
    setBusy(true);
    setError(null);
    setMessage(null);
    setQuery(nextQuery);
    setActiveQuery(nextQuery);
    try {
      const result = await search(nextQuery, 25);
      setHits(result.hits);
      setNextCursor(result.next_cursor ?? '');
      if (result.hits.length === 0) {
        setMessage(nextQuery.trim() ? 'No matching notes found.' : 'No notes found yet.');
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onLoadMore() {
    if (!nextCursor) return;
    setBusy(true);
    try {
      const result = await search(activeQuery, 25, nextCursor);
      setHits((previous) => [...previous, ...result.hits]);
      setNextCursor(result.next_cursor ?? '');
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onSaveDocument() {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const saved = selectedDocument
        ? await updateDocument(selectedDocument.id, {
            title,
            body,
            body_mime_type: selectedDocument.body_mime_type,
            base_revision_id: selectedDocument.current_revision_id,
          })
        : await createDocument({ title, body });
      setSelectedDocument(saved);
      setTitle(saved.title);
      setBody(saved.body ?? '');
      await refreshDocumentSidebars(saved.id);
      setQuery(title);
      setMessage(`Saved “${saved.title}”.`);
      const result = await search(title);
      setHits(result.hits);
      setNextCursor(result.next_cursor ?? '');
      void refreshSidebar();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function openDocumentByID(documentID: string) {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const documentRecord = await getDocument(documentID);
      setSelectedDocument(documentRecord);
      setTitle(documentRecord.title);
      setBody(documentRecord.body ?? '');
      await refreshDocumentSidebars(documentRecord.id);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onOpenHit(hit: SearchHit) {
    await openDocumentByID(hit.id);
  }

  async function onUploadAndAttachResource(file: File | null) {
    if (!selectedDocument || !file) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const resource = await uploadResource(file, selectedDocument.collection_id);
      await attachResource(selectedDocument.id, resource.id, file.type.startsWith('image/') ? 'embedded' : 'attachment');
      const page = await listDocumentResources(selectedDocument.id);
      setResources(page.resources);
      const markdownLink = file.type.startsWith('image/')
        ? `\n![${file.name}](${resource.uri})\n`
        : `\n[${file.name}](${resource.uri})\n`;
      setBody((previous) => `${previous.trimEnd()}${markdownLink}`);
      setMessage(`Uploaded and attached “${file.name}”. Save the note to persist the inserted Markdown link.`);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onEditorUploadImages(files: Array<File>, callback: UploadImgCallBack) {
    if (!selectedDocument) {
      setError('Save or open a note before using the editor image upload command.');
      callback([]);
      return;
    }
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const uploaded = [];
      for (const file of files) {
        const resource = await uploadResource(file, selectedDocument.collection_id);
        await attachResource(selectedDocument.id, resource.id, 'embedded');
        uploaded.push({ url: resource.uri, alt: file.name, title: file.name });
      }
      const page = await listDocumentResources(selectedDocument.id);
      setResources(page.resources);
      callback(uploaded);
      setMessage('Image uploaded as a local resource. Save the note to persist the inserted Markdown link.');
    } catch (err) {
      setError(errorMessage(err));
      callback([]);
    } finally {
      setBusy(false);
    }
  }

  async function onPreviewClick(event: MouseEvent<HTMLDivElement>) {
    const target = event.target;
    if (!(target instanceof Element)) return;
    const anchor = target.closest('a');
    if (!anchor) return;
    const appURI = anchor.getAttribute('data-app-uri') || anchor.getAttribute('href') || '';
    if (appURI.startsWith('document://')) {
      event.preventDefault();
      const documentID = parseDocumentIDFromURI(appURI);
      if (!documentID) {
        setError(`Could not parse document link: ${appURI}`);
        return;
      }
      await openDocumentByID(documentID);
      return;
    }
    if (appURI.startsWith('resource://')) {
      event.preventDefault();
      const resourceID = parseResourceIDFromURI(appURI);
      if (!resourceID) {
        setError(`Could not parse resource link: ${appURI}`);
        return;
      }
      window.location.assign(resourceContentURL(resourceID, true));
    }
  }

  function PreviewRenderer({ html, id, className }: PreviewRendererProps) {
    return (
      <div
        id={id}
        className={className}
        onClick={(event) => {
          void onPreviewClick(event);
        }}
        dangerouslySetInnerHTML={{ __html: normalizePreviewHTML(html) }}
      />
    );
  }

  function resetEditor() {
    setSelectedDocument(null);
    setTitle('New note');
    setBody(defaultBody);
    setResources([]);
    setLinks([]);
    setBacklinks([]);
    setMessage(null);
    setError(null);
  }

  return (
    <main className="app-shell">
      <header className="app-header">
        <h1>Notrios</h1>
        <p className="status">{statusText}</p>
      </header>

      <section className="workspace">
        <aside className="sidebar">
          <h3>Notebooks</h3>
          <ul className="sidebar-list">
            {searchNotebooks
              .filter((sn) => sn.sort_anchor === 'first')
              .map((sn) => (
                <SearchNotebookRow key={sn.id} notebook={sn} active={activeQuery === sn.query} onSelect={() => void onSearch(sn.query)} />
              ))}
            <NotebookTree nodes={notebooks} activeQuery={activeQuery} onSelect={(name) => void onSearch(`notebook:"${name}"`)} />
            {searchNotebooks
              .filter((sn) => sn.sort_anchor === 'normal')
              .map((sn) => (
                <SearchNotebookRow key={sn.id} notebook={sn} active={activeQuery === sn.query} onSelect={() => void onSearch(sn.query)} />
              ))}
            {searchNotebooks
              .filter((sn) => sn.sort_anchor === 'last')
              .map((sn) => (
                <SearchNotebookRow key={sn.id} notebook={sn} active={activeQuery === sn.query} onSelect={() => void onSearch(sn.query)} />
              ))}
          </ul>
          {tags.length > 0 && (
            <>
              <h3>Tags</h3>
              <ul className="sidebar-list">
                {tags.map((tag) => (
                  <li key={tag.id}>
                    <button
                      type="button"
                      className={activeQuery === `tag:"${tag.name}"` ? 'sidebar-item active' : 'sidebar-item'}
                      onClick={() => void onSearch(`tag:"${tag.name}"`)}
                    >
                      <span className="sidebar-label">{tag.name}</span>
                      <span className="tag-count">{tag.note_count}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </>
          )}
        </aside>

        <section className="grid">
        <article className="panel editor-panel">
          <div className="panel-heading">
            <h2>{selectedDocument ? 'Edit note' : 'Create note'}</h2>
            <span className="muted">md-editor-rt preview + app link routing</span>
          </div>
          <label>
            Title
            <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Note title" />
          </label>
          <div className="body-field">
            <span className="field-label">Markdown body</span>
            <MdEditor
              id="notrios-editor"
              value={body}
              onChange={setBody}
              onUploadImg={(files, callback) => {
                void onEditorUploadImages(files, callback);
              }}
              previewComponent={PreviewRenderer}
              sanitize={normalizePreviewHTML}
              language="en-US"
              previewTheme="github"
              theme="light"
              noMermaid
              style={{ height: '520px' }}
            />
          </div>
          <div className="button-row">
            <button onClick={onSaveDocument} disabled={busy || title.trim() === ''}>
              {busy ? 'Working...' : selectedDocument ? 'Save revision' : 'Create note'}
            </button>
            {selectedDocument && (
              <button type="button" onClick={resetEditor}>
                New note
              </button>
            )}
          </div>
          {selectedDocument && (
            <label className="resource-upload">
              Upload image/PDF/resource
              <input
                type="file"
                disabled={busy}
                onChange={(event) => {
                  const file = event.target.files?.[0] ?? null;
                  event.currentTarget.value = '';
                  void onUploadAndAttachResource(file);
                }}
              />
            </label>
          )}
        </article>

        <article className="panel search-panel">
          <div className="panel-heading">
            <h2>Search</h2>
            <span className="muted">POST /api/v1/search</span>
          </div>
          <div className="search-row">
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search notes" />
            <button onClick={() => void onSearch()} disabled={busy}>Search</button>
          </div>
          <div className="result-list" aria-live="polite">
            {hits.map((hit) => (
              <button className="result-card" key={hit.id} onClick={() => void onOpenHit(hit)}>
                <strong>{hit.title ?? hit.uri}</strong>
                {hit.snippet && <Snippet htmlSnippet={hit.snippet} />}
                <small>{hit.uri}</small>
              </button>
            ))}
            {nextCursor && (
              <button type="button" className="load-more" onClick={() => void onLoadMore()} disabled={busy}>
                Load more
              </button>
            )}
          </div>
        </article>
      </section>
      </section>

      {(error || message) && (
        <section className={error ? 'notice error' : 'notice'}>
          {error ?? message}
        </section>
      )}

      <section className="panel document-panel">
        <div className="panel-heading">
          <h2>Opened note context</h2>
          <span className="muted">links, backlinks, and local resources</span>
        </div>
        {selectedDocument ? (
          <div>
            <h3>{selectedDocument.title}</h3>
            <dl className="document-meta">
              <div><dt>ID</dt><dd>{selectedDocument.id}</dd></div>
              <div><dt>URI</dt><dd>{selectedDocument.uri}</dd></div>
              <div><dt>Revision</dt><dd>{selectedDocument.current_revision_id}</dd></div>
            </dl>
            {(links.length > 0 || backlinks.length > 0) && (
              <div className="link-list">
                <h4>Links</h4>
                {links.length > 0 && (
                  <>
                    <strong>Outgoing</strong>
                    <ul>
                      {links.map((link) => (
                        <li key={`out-${link.id}`}>
                          <LinkLabel link={link} onOpenDocument={openDocumentByID} />
                          <span className="muted"> · {link.resolution_status} · {link.relation_type}</span>
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
                          <button className="text-button" type="button" onClick={() => void openDocumentByID(link.source_document_id)}>
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
                <h4>Attached resources</h4>
                <ul>
                  {resources.map((ref) => (
                    <li key={`${ref.resource_id}-${ref.relation_type}-${ref.ordinal ?? 0}`}>
                      <a href={resourceContentURL(ref.resource_id)}>{ref.resource?.filename || ref.resource_id}</a>
                      <span className="muted"> · {ref.relation_type} · {ref.resource?.mime_type}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        ) : (
          <p className="muted">Save or select a note to open it here.</p>
        )}
      </section>
    </main>
  );
}

function SearchNotebookRow({ notebook, active, onSelect }: { notebook: SearchNotebook; active: boolean; onSelect: () => void }) {
  return (
    <li>
      <button type="button" className={active ? 'sidebar-item active' : 'sidebar-item'} onClick={onSelect}>
        <span className="sidebar-label">
          {notebook.icon_emoji ? `${notebook.icon_emoji} ` : ''}
          {notebook.name}
        </span>
      </button>
    </li>
  );
}

function NotebookTree({ nodes, activeQuery, onSelect, depth = 0 }: { nodes: NotebookTreeNode[]; activeQuery: string; onSelect: (name: string) => void; depth?: number }) {
  return (
    <>
      {nodes.map((node) => (
        <li key={node.id}>
          <button
            type="button"
            className={activeQuery === `notebook:"${node.name}"` ? 'sidebar-item active' : 'sidebar-item'}
            style={{ paddingLeft: `${12 + depth * 16}px` }}
            onClick={() => onSelect(node.name)}
          >
            <span className="sidebar-label">
              {node.icon_emoji ? `${node.icon_emoji} ` : ''}
              {node.name}
            </span>
          </button>
          {node.children && node.children.length > 0 && (
            <ul className="sidebar-list nested">
              <NotebookTree nodes={node.children} activeQuery={activeQuery} onSelect={onSelect} depth={depth + 1} />
            </ul>
          )}
        </li>
      ))}
    </>
  );
}

function LinkLabel({ link, onOpenDocument }: { link: DocumentLink; onOpenDocument: (documentID: string) => Promise<void> }) {
  const label = link.display_text || link.raw_target || link.target_uri || '(untitled link)';
  if (link.target_document_id && link.resolution_status === 'resolved') {
    return (
      <button className="text-button" type="button" onClick={() => void onOpenDocument(link.target_document_id ?? '')}>
        {label}
      </button>
    );
  }
  if (link.target_resource_id && link.resolution_status === 'resolved') {
    return <a href={resourceContentURL(link.target_resource_id)}>{label}</a>;
  }
  if (link.target_uri?.startsWith('http://') || link.target_uri?.startsWith('https://')) {
    return <a href={link.target_uri} target="_blank" rel="noreferrer">{label}</a>;
  }
  return <span>{label}</span>;
}

function Snippet({ htmlSnippet }: { htmlSnippet: string }) {
  const parts = htmlSnippet.split(/(<\/?mark>)/g);
  let marked = false;
  return (
    <span>
      {parts.map((part, index) => {
        if (part === '<mark>') {
          marked = true;
          return null;
        }
        if (part === '</mark>') {
          marked = false;
          return null;
        }
        return marked ? <mark key={index}>{part}</mark> : <span key={index}>{part}</span>;
      })}
    </span>
  );
}

function normalizePreviewHTML(html: string): string {
  if (typeof DOMParser === 'undefined') return html;
  const parser = new DOMParser();
  const doc = parser.parseFromString(html, 'text/html');

  doc.querySelectorAll('script, style, iframe, object, embed, form, input, button, meta, link').forEach((node) => node.remove());

  doc.body.querySelectorAll('*').forEach((element) => {
    for (const attr of Array.from(element.attributes)) {
      const name = attr.name.toLowerCase();
      if (name.startsWith('on')) element.removeAttribute(attr.name);
      if (name === 'style') element.removeAttribute(attr.name);
    }
  });

  doc.querySelectorAll<HTMLAnchorElement>('a[href]').forEach((anchor) => {
    const href = anchor.getAttribute('href') || '';
    if (href.startsWith('document://')) {
      anchor.setAttribute('data-app-uri', href);
      anchor.setAttribute('href', '#');
      return;
    }
    if (href.startsWith('resource://')) {
      anchor.setAttribute('data-app-uri', href);
      const resourceID = parseResourceIDFromURI(href);
      if (resourceID) anchor.setAttribute('href', resourceContentURL(resourceID, true));
      return;
    }
    if (href.startsWith('http://') || href.startsWith('https://') || href.startsWith('mailto:') || href.startsWith('#')) {
      anchor.setAttribute('rel', 'noreferrer');
      if (href.startsWith('http')) anchor.setAttribute('target', '_blank');
      return;
    }
    anchor.removeAttribute('href');
  });

  doc.querySelectorAll<HTMLImageElement>('img[src]').forEach((image) => {
    const src = image.getAttribute('src') || '';
    if (src.startsWith('resource://')) {
      image.setAttribute('data-app-uri', src);
      const resourceID = parseResourceIDFromURI(src);
      if (resourceID) image.setAttribute('src', resourceContentURL(resourceID, false));
      return;
    }
    if (src.startsWith('http://') || src.startsWith('https://')) return;
    if (src.startsWith('data:image/')) return;
    image.removeAttribute('src');
  });

  return doc.body.innerHTML;
}

function parseDocumentIDFromURI(uri: string): string | null {
  try {
    const parsed = new URL(uri);
    const parts = parsed.pathname.split('/').filter(Boolean);
    const index = parts.indexOf('documents');
    if (index >= 0 && parts[index + 1]) return decodeURIComponent(parts[index + 1]);
  } catch {
    // Fall through to regex fallback.
  }
  const match = uri.match(/documents\/([^/#?]+)/);
  return match ? decodeURIComponent(match[1]) : null;
}

function parseResourceIDFromURI(uri: string): string | null {
  try {
    const parsed = new URL(uri);
    const parts = parsed.pathname.split('/').filter(Boolean);
    const index = parts.indexOf('resources');
    if (index >= 0 && parts[index + 1]) return decodeURIComponent(parts[index + 1]);
  } catch {
    // Fall through to regex fallback.
  }
  const match = uri.match(/resources\/([^/#?]+)/);
  return match ? decodeURIComponent(match[1]) : null;
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
