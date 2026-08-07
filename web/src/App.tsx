// Notrios desktop workspace. Below the native menu (a Wails responsibility)
// the layout is four panes in fixed DOM and visual order per UI_DESIGN.md —
// sidebar | search/results | Markdown editor | Markdown preview — separated
// by three draggable, keyboard-accessible splitters. Each pane scrolls
// independently; the document body never scrolls at desktop sizes.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { UploadImgCallBack } from 'md-editor-rt';
import 'md-editor-rt/lib/style.css';
import {
  attachResource,
  createDocument,
  deleteDocument,
  deleteNotebook,
  getDocument,
  getNotebookTree,
  getStatus,
  listDocumentLinks,
  listDocumentResources,
  listSearchNotebooks,
  listTags,
  localizeRemoteMedia,
  moveDocumentToNotebook,
  previewNotebookDeletion,
  purgeDocument,
  resolveStableLink,
  restoreTrashedDocument,
  scanRemoteMedia,
  updateDocument,
  uploadResource,
  type DocumentLink,
  type DocumentRecord,
  type NotebookTreeNode,
  type RemoteMediaDecision,
  type ResourceReference,
  type SearchHit,
  type SearchNotebook,
  type StatusResponse,
  type TagRecord,
} from './api';
import { parseDeepLinkHash, stableLinkStatusMessage } from './stable-links';
import {
  allThemes,
  applyTheme,
  loadCustomThemes,
  loadChoice,
  loadMode,
  resolveTheme,
  saveChoice,
  saveCustomThemes,
  saveMode,
  type Theme,
  type ThemeMode,
  type ThemeTokens,
} from './themes';
import {
  composeSidebar,
  creationTargetFor,
  DEFAULT_NOTEBOOK_ID,
  notebookOptions as flattenNotebookOptions,
  type SidebarRow,
} from './sidebar';
import { notebookDeletionPrompt, notebookName } from './organizer';
import {
  clampWidths,
  DEFAULT_WIDTHS,
  equalizeEditorPreview,
  loadPaneWidths,
  MIN_WIDTHS,
  MIN_WORKSPACE_WIDTH,
  previewWidth,
  resizeAt,
  savePaneWidths,
  type PaneWidths,
  type SplitterIndex,
} from './panes';
import { usePagedSearch } from './usePagedSearch';
import { errorMessage } from './preview-utils';
import { PaneSplitter } from './components/PaneSplitter';
import { SidebarPane } from './components/SidebarPane';
import { SearchPane } from './components/SearchPane';
import { EditorPane } from './components/EditorPane';
import { PreviewPane } from './components/PreviewPane';

const defaultBody = `# New note\n\nWrite Markdown here. Link other notes with:\n\n[Related note](document://default/documents/<document-id>)\n`;

declare global {
  interface Window {
    __notriosOpenHelp?: number;
  }
}

export function App() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [query, setQuery] = useState('');
  const [activeQuery, setActiveQuery] = useState('');
  const [selectedDocument, setSelectedDocument] = useState<DocumentRecord | null>(null);
  const [title, setTitle] = useState('New note');
  const [body, setBody] = useState(defaultBody);
  const [resources, setResources] = useState<ResourceReference[]>([]);
  const [links, setLinks] = useState<DocumentLink[]>([]);
  const [backlinks, setBacklinks] = useState<DocumentLink[]>([]);
  const [remoteMedia, setRemoteMedia] = useState<RemoteMediaDecision[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [notebooks, setNotebooks] = useState<NotebookTreeNode[]>([]);
  const [searchNotebooks, setSearchNotebooks] = useState<SearchNotebook[]>([]);
  const [tags, setTags] = useState<TagRecord[]>([]);
  // The selected sidebar row, kept whole rather than as its query. A notebook
  // row's query is `notebook:"<name>"` and names are unique only among
  // siblings, so `Contacts/Work` and `Personal/Work` share a query — deriving
  // the creation target from `activeQuery` would file into the wrong one.
  const [selectedRow, setSelectedRow] = useState<SidebarRow | null>(null);

  const paged = usePagedSearch(25);

  // ----- Themes -----
  const [mode, setMode] = useState<ThemeMode>(() => loadMode());
  const [customThemes, setCustomThemes] = useState<Theme[]>(() => loadCustomThemes());
  const [showThemes, setShowThemes] = useState(false);
  const [newThemeName, setNewThemeName] = useState('');
  const activeTheme = useMemo(() => resolveTheme(mode, customThemes), [mode, customThemes, showThemes]);
  useEffect(() => {
    applyTheme(activeTheme);
  }, [activeTheme]);

  // ----- Pane widths -----
  const workspaceRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState<number>(() => Math.max(window.innerWidth, MIN_WORKSPACE_WIDTH));
  const [widths, setWidths] = useState<PaneWidths>(() => loadPaneWidths());
  const dragSnapshotRef = useRef<PaneWidths | null>(null);
  const lastMeasuredWidthRef = useRef<number | null>(null);

  useEffect(() => {
    const element = workspaceRef.current;
    if (!element) return;
    const measure = () => {
      const width = element.clientWidth;
      setContainerWidth(width);
      const previous = lastMeasuredWidthRef.current;
      lastMeasuredWidthRef.current = width;
      // A window resize (any container-width change after the initial
      // measurement) re-splits the post-sidebar/search space equally between
      // the editor and the preview; splitter drags can then adjust them.
      if (previous !== null && previous !== width) {
        setWidths((current) => {
          const next = equalizeEditorPreview(current, width);
          savePaneWidths(next);
          return next;
        });
      }
    };
    measure();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const clamped = useMemo(() => clampWidths(widths, containerWidth), [widths, containerWidth]);
  const persist = useCallback((next: PaneWidths) => {
    setWidths(next);
    savePaneWidths(next);
  }, []);

  const splitterHandlers = (index: SplitterIndex) => ({
    onDragStart: () => {
      dragSnapshotRef.current = clamped;
    },
    onDrag: (totalDelta: number) => {
      const base = dragSnapshotRef.current ?? clamped;
      persist(resizeAt(base, index, totalDelta, containerWidth));
    },
    onDragEnd: () => {
      dragSnapshotRef.current = null;
    },
    onStep: (delta: number) => persist(resizeAt(clamped, index, delta, containerWidth)),
    onReset: () => persist(equalizeEditorPreview({ ...DEFAULT_WIDTHS }, containerWidth)),
  });

  // ----- Search -----
  const runSearch = useCallback(
    (nextQuery: string) => {
      setQuery(nextQuery);
      setActiveQuery(nextQuery);
      setError(null);
      setMessage(null);
      void paged.start(nextQuery);
    },
    [paged],
  );

  // ----- Startup: status, sidebar, "All notes", Help-menu readiness -----
  const refreshSidebar = useCallback(async () => {
    try {
      const [tree, savedSearches, tagList] = await Promise.all([getNotebookTree(), listSearchNotebooks(), listTags()]);
      setNotebooks(tree);
      setSearchNotebooks(savedSearches);
      setTags(tagList);
    } catch (err) {
      setError(errorMessage(err));
    }
  }, []);

  useEffect(() => {
    const refreshStatus = () => {
      getStatus()
        .then(setStatus)
        .catch((err: unknown) => setError(errorMessage(err)));
    };
    refreshStatus();
    const statusInterval = window.setInterval(refreshStatus, 30_000);
    void refreshSidebar();
    const openHelp = () => runSearch('notebook:"Help"');
    // The native Help menu sets a flag before dispatching, so a click that
    // happens before React mounts is not lost.
    if (window.__notriosOpenHelp) {
      delete window.__notriosOpenHelp;
      openHelp();
    } else {
      runSearch(''); // startup view: the "All notes" search notebook
    }
    // The desktop protocol handler resolves a notrios:// link to a document ID
    // and opens the local UI at `#document=<id>`, so honour that on startup.
    const deepLink = parseDeepLinkHash(window.location.hash);
    if (deepLink) void openDocumentByID(deepLink.documentID);
    window.addEventListener('notrios:open-help', openHelp);
    return () => {
      window.clearInterval(statusInterval);
      window.removeEventListener('notrios:open-help', openHelp);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const sidebarRows = useMemo(() => composeSidebar(notebooks, searchNotebooks), [notebooks, searchNotebooks]);
  const notebookChoices = useMemo(() => flattenNotebookOptions(notebooks), [notebooks]);

  // Where a new note is filed. `null` means the service's default notebook,
  // which is the right answer for "All notes", a saved search, and Help — those
  // name a view rather than a place.
  const creationNotebookID = useMemo(() => creationTargetFor(selectedRow), [selectedRow]);
  const defaultNotebookName = useMemo(
    () => notebookChoices.find((option) => option.id === DEFAULT_NOTEBOOK_ID)?.name ?? 'Notes',
    [notebookChoices],
  );
  const creationNotebookName = useMemo(
    () => notebookChoices.find((option) => option.id === creationNotebookID)?.name ?? defaultNotebookName,
    [notebookChoices, creationNotebookID, defaultNotebookName],
  );
  // The toolbar control shows the *open note's* notebook once one is open, and
  // the pending creation target otherwise. Showing the sidebar's selection for
  // an open note would claim a note reached from "All notes" lives wherever the
  // sidebar happens to point.
  const toolbarNotebookID = selectedDocument ? selectedDocument.notebook_id ?? null : creationNotebookID;

  const statusText = useMemo(() => {
    if (!status) return 'loading…';
    const schema = status.database_info?.schema_version ? ` · schema ${status.database_info.schema_version}` : '';
    const sidecar = status.search_sidecar;
    if (!sidecar?.configured) return `${status.service} ${status.version}${schema} · Recoll off`;
    const sync = sidecar.last_sync_at ? new Date(sidecar.last_sync_at).toLocaleTimeString() : 'never';
    return `${status.service} ${status.version}${schema} · Recoll ${sidecar.state} · ${sidecar.backlog} pending · synced ${sync}`;
  }, [status]);

  const editable = selectedDocument ? selectedDocument.editable !== false : true;
  // Trashed is not the same as uneditable: a Help note is permanently
  // read-only, a trashed note is one click from being editable again.
  const trashed = Boolean(selectedDocument?.deleted_at);

  // ----- Document operations -----
  async function refreshDocumentSidebars(documentID: string, isTrashed = false) {
    const [resourcePage, linkPage] = await Promise.all([listDocumentResources(documentID), listDocumentLinks(documentID, 'both')]);
    setResources(resourcePage.resources);
    setLinks(linkPage.outgoing ?? []);
    setBacklinks(linkPage.incoming ?? []);
    // Remote-media policy scan (server-side, static — nothing downloaded);
    // best-effort: a scan failure never blocks opening the note.
    //
    // Skipped for a trashed note: localizing media writes a revision, which a
    // trashed note cannot take, so the scan would only ever produce an offer
    // that must be refused — and the service declines to scan one anyway.
    if (isTrashed) {
      setRemoteMedia([]);
      return;
    }
    try {
      const scan = await scanRemoteMedia(documentID);
      setRemoteMedia(scan.media ?? []);
    } catch {
      setRemoteMedia([]);
    }
  }

  const openDocumentByID = useCallback(async (documentID: string) => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const documentRecord = await getDocument(documentID);
      setSelectedDocument(documentRecord);
      setTitle(documentRecord.title);
      setBody(documentRecord.body ?? '');
      await refreshDocumentSidebars(documentRecord.id, Boolean(documentRecord.deleted_at));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }, []);

  // A stable link is resolved by the service, which answers only for the
  // database it has open. A link belonging to another library is reported, not
  // opened: silently matching its document ID here would show the wrong note.
  const openStableLink = useCallback(async (uri: string) => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const resolution = await resolveStableLink(uri);
      if (resolution.status === 'resolved' || resolution.status === 'trashed') {
        await openDocumentByID(resolution.document_id);
        if (resolution.status === 'trashed') setMessage('This note is in the Trash.');
        return;
      }
      setError(stableLinkStatusMessage(resolution.status, resolution.uri));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }, [openDocumentByID]);

  async function onSaveDocument() {
    if (!editable) return;
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
        : await createDocument({ title, body, notebook_id: creationNotebookID ?? undefined });
      const isNew = !selectedDocument;
      setSelectedDocument(saved);
      setTitle(saved.title);
      setBody(saved.body ?? '');
      await refreshDocumentSidebars(saved.id);
      setMessage(`Saved “${saved.title}”.`);
      // Update the results in place instead of re-searching, so the search
      // pane keeps its contents and scroll position.
      if (isNew) {
        paged.prependHit({ id: saved.id, uri: saved.uri, source: 'managed-notes', sources: ['sqlite'], title: saved.title, editable: true });
      } else {
        paged.patchHit(saved.id, { title: saved.title });
      }
      void refreshSidebar();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onUploadAndAttachResource(file: File | null) {
    if (!selectedDocument || !file || !editable) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const resource = await uploadResource(file, selectedDocument.collection_id);
      await attachResource(selectedDocument.id, resource.id, file.type.startsWith('image/') ? 'embedded' : 'attachment');
      const page = await listDocumentResources(selectedDocument.id);
      setResources(page.resources);
      const markdownLink = file.type.startsWith('image/') ? `\n![${file.name}](${resource.uri})\n` : `\n[${file.name}](${resource.uri})\n`;
      setBody((previous) => `${previous.trimEnd()}${markdownLink}`);
      setMessage(`Uploaded and attached “${file.name}”. Save the note to persist the inserted Markdown link.`);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onEditorUploadImages(files: Array<File>, callback: UploadImgCallBack) {
    if (!selectedDocument || !editable) {
      setError('Save or open an editable note before using the editor image upload command.');
      callback([]);
      return;
    }
    setBusy(true);
    setError(null);
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

  async function onLocalizeRemoteMedia() {
    if (!selectedDocument || !editable) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const result = await localizeRemoteMedia(selectedDocument.id, selectedDocument.current_revision_id);
      const parts = [`${result.localized.length} localized`];
      if (result.blocked.length > 0) parts.push(`${result.blocked.length} blocked`);
      if (result.review.length > 0) parts.push(`${result.review.length} needing review`);
      if (result.failed.length > 0) parts.push(`${result.failed.length} failed`);
      setMessage(`Remote media: ${parts.join(', ')}.`);
      // The note body (and revision) changed; reload it, which also
      // refreshes the remote-media scan.
      await openDocumentByID(selectedDocument.id);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  // ----- Trash-first deletion and restore -----
  //
  // Deleting a note moves it to the Trash; that is the store's rule, and these
  // handlers are what makes it reachable without the CLI. Every irreversible
  // step asks first, and the question says what actually happens rather than
  // "are you sure": the difference between "moves to the Trash" and "cannot be
  // undone" is the only thing worth confirming.

  async function onDeleteDocument() {
    if (!selectedDocument || !editable) return;
    if (!window.confirm(`Move “${selectedDocument.title}” to the Trash?\n\nIt stays in the Trash until you restore it or delete it permanently.`)) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    const { id, title: deletedTitle } = selectedDocument;
    try {
      await deleteDocument(id, selectedDocument.current_revision_id);
      paged.removeHit(id);
      resetEditor();
      setMessage(`Moved “${deletedTitle}” to the Trash.`);
      void refreshSidebar();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onRestoreDocument() {
    if (!selectedDocument) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const restored = await restoreTrashedDocument(selectedDocument.id);
      setSelectedDocument(restored);
      setTitle(restored.title);
      setBody(restored.body ?? '');
      await refreshDocumentSidebars(restored.id);
      // It was listed because it was trashed; it no longer is. The list is not
      // re-run, so the note the user is now editing stays on screen.
      paged.removeHit(restored.id);
      setMessage(`Restored “${restored.title}”.`);
      void refreshSidebar();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onPurgeDocument() {
    if (!selectedDocument) return;
    if (!window.confirm(`Permanently delete “${selectedDocument.title}”?\n\nThe note and every revision of it are removed. This cannot be undone.`)) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    const { id, title: purgedTitle } = selectedDocument;
    try {
      await purgeDocument(id);
      paged.removeHit(id);
      resetEditor();
      setMessage(`Permanently deleted “${purgedTitle}”.`);
      void refreshSidebar();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  // Deleting a notebook is trash-first too, and what it does to the notes
  // inside it is not guessable from a dialog that only says "delete?". The
  // service is asked what would happen and the answer is what gets confirmed.
  async function onDeleteNotebookRow(row: SidebarRow) {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const preview = await previewNotebookDeletion(row.id);
      if (!preview.deletable) {
        setError(`“${preview.name}” cannot be deleted: ${preview.reason ?? 'it is protected'}.`);
        return;
      }
      if (!window.confirm(notebookDeletionPrompt(preview, notebookName(notebooks, preview.rehome_notebook_id)))) return;
      await deleteNotebook(row.id);
      setMessage(
        preview.notes > 0
          ? `Deleted “${preview.name}”. ${preview.notes} note(s) moved to the Trash.`
          : `Deleted “${preview.name}”.`,
      );
      await refreshSidebar();
      // The results and the open note may both have come from a notebook that
      // no longer exists, so re-run the search and re-read the note rather than
      // leaving rows that point at nothing.
      if (activeQuery === row.query) {
        runSearch('');
      } else if (activeQuery !== '') {
        void paged.start(activeQuery);
      }
      if (selectedDocument) {
        const current = await getDocument(selectedDocument.id).catch(() => null);
        if (current) setSelectedDocument(current);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  // Filing a note. A move is not revision-scoped — it changes where a note
  // lives, not what it says — so it takes effect immediately and the message
  // names where the note landed rather than leaving the toolbar to imply it.
  async function onSelectNotebook(notebookID: string) {
    if (!selectedDocument) {
      // No note yet: the choice retargets the pending draft by selecting the
      // matching sidebar row, so the sidebar highlight and the toolbar agree.
      const row = sidebarRows.find((candidate) => candidate.id === notebookID);
      if (row) setSelectedRow(row);
      return;
    }
    if (!editable) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const moved = await moveDocumentToNotebook(selectedDocument.id, notebookID);
      setSelectedDocument(moved);
      const name = notebookChoices.find((option) => option.id === notebookID)?.name ?? notebookID;
      setMessage(`Filed “${moved.title}” in “${name}”.`);
      void refreshSidebar();
      // The note may have left the notebook the current results describe.
      if (activeQuery !== '') void paged.start(activeQuery);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  // Selecting a sidebar row both runs its query and sets the creation target;
  // selecting a tag runs a query that names no notebook, so the target resets.
  function onSelectRow(row: SidebarRow) {
    setSelectedRow(row);
    runSearch(row.query);
  }

  function onSelectTagQuery(nextQuery: string) {
    setSelectedRow(null);
    runSearch(nextQuery);
  }

  function resetEditor() {
    setSelectedDocument(null);
    setTitle('New note');
    setBody(defaultBody);
    setResources([]);
    setLinks([]);
    setBacklinks([]);
    setRemoteMedia([]);
    setMessage(null);
    setError(null);
  }

  // ----- Theme controls -----
  function toggleMode() {
    const next: ThemeMode = mode === 'light' ? 'dark' : 'light';
    setMode(next);
    saveMode(next);
  }

  function onSelectThemeFor(target: ThemeMode, themeName: string) {
    saveChoice(target, themeName);
    setMode((current) => current);
    setCustomThemes((themes) => [...themes]);
  }

  function onCreateTheme() {
    const name = newThemeName.trim();
    if (!name) return;
    if (allThemes(customThemes).some((theme) => theme.name.toLowerCase() === name.toLowerCase())) {
      setError(`A theme named “${name}” already exists.`);
      return;
    }
    const clone: Theme = { name, base: activeTheme.base, tokens: { ...activeTheme.tokens } };
    const next = [...customThemes, clone];
    setCustomThemes(next);
    saveCustomThemes(next);
    setNewThemeName('');
  }

  function onDeleteTheme(name: string) {
    const next = customThemes.filter((theme) => theme.name !== name);
    setCustomThemes(next);
    saveCustomThemes(next);
  }

  function onEditThemeToken(name: string, token: keyof ThemeTokens, value: string) {
    const next = customThemes.map((theme) => (theme.name === name ? { ...theme, tokens: { ...theme.tokens, [token]: value } } : theme));
    setCustomThemes(next);
    saveCustomThemes(next);
  }

  const onOpenHit = useCallback((hit: SearchHit) => void openDocumentByID(hit.id), [openDocumentByID]);

  return (
    <main className="app-shell">
      <header className="app-header">
        <h1>Notrios</h1>
        <div className="header-actions">
          <p className="status">{statusText}</p>
          <button type="button" className="icon-button" title="Toggle light/dark theme" aria-label="Toggle light/dark theme" onClick={toggleMode}>
            {mode === 'light' ? '🌙' : '☀️'}
          </button>
          <button type="button" className="icon-button" title="Theme settings" aria-label="Theme settings" onClick={() => setShowThemes((v) => !v)}>
            🎨
          </button>
        </div>
      </header>

      {showThemes && (
        <section className="theme-panel" aria-label="Theme settings">
          <div className="theme-selects">
            <label>
              Light mode uses
              <select value={loadChoice('light')} onChange={(event) => onSelectThemeFor('light', event.target.value)}>
                {allThemes(customThemes).map((theme) => (
                  <option key={theme.name} value={theme.name}>
                    {theme.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Dark mode uses
              <select value={loadChoice('dark')} onChange={(event) => onSelectThemeFor('dark', event.target.value)}>
                {allThemes(customThemes).map((theme) => (
                  <option key={theme.name} value={theme.name}>
                    {theme.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className="theme-create">
            <input value={newThemeName} placeholder="New theme name (copies the current theme)" onChange={(event) => setNewThemeName(event.target.value)} />
            <button type="button" onClick={onCreateTheme} disabled={!newThemeName.trim()}>
              Create custom theme
            </button>
          </div>
          {customThemes.map((theme) => (
            <details key={theme.name} className="theme-editor">
              <summary>
                {theme.name} <span className="muted">({theme.base} base)</span>
                <button type="button" className="text-button" onClick={() => onDeleteTheme(theme.name)}>
                  delete
                </button>
              </summary>
              <div className="theme-tokens">
                {(Object.keys(theme.tokens) as Array<keyof ThemeTokens>).map((token) => (
                  <label key={token}>
                    {token}
                    <input
                      type="color"
                      value={/^#[0-9a-fA-F]{6}$/.test(theme.tokens[token]) ? theme.tokens[token] : '#888888'}
                      onChange={(event) => onEditThemeToken(theme.name, token, event.target.value)}
                    />
                  </label>
                ))}
              </div>
            </details>
          ))}
        </section>
      )}

      {(error || message) && (
        <div className={error ? 'notice error' : 'notice'} role="status">
          {error ?? message}
        </div>
      )}

      <div className="workspace" ref={workspaceRef} data-testid="workspace">
        <SidebarPane
          rows={sidebarRows}
          tags={tags}
          activeQuery={activeQuery}
          selectedRowID={selectedRow?.id ?? null}
          onSelectRow={onSelectRow}
          onSelectQuery={onSelectTagQuery}
          onDeleteNotebook={(row) => void onDeleteNotebookRow(row)}
        />
        <PaneSplitter
          label="Resize sidebar"
          value={clamped.sidebar}
          min={MIN_WIDTHS.sidebar}
          max={containerWidth - MIN_WIDTHS.search - MIN_WIDTHS.editor - MIN_WIDTHS.preview}
          {...splitterHandlers(0)}
        />
        <SearchPane
          query={query}
          onQueryChange={setQuery}
          onSubmit={() => runSearch(query)}
          paged={paged}
          onOpenHit={onOpenHit}
          selectedDocumentID={selectedDocument?.id ?? null}
          busy={busy}
          onNewNote={resetEditor}
          newNoteNotebookName={creationNotebookName}
        />
        <PaneSplitter
          label="Resize search results"
          value={clamped.search}
          min={MIN_WIDTHS.search}
          max={containerWidth - MIN_WIDTHS.sidebar - MIN_WIDTHS.editor - MIN_WIDTHS.preview}
          {...splitterHandlers(1)}
        />
        <EditorPane
          title={title}
          onTitleChange={setTitle}
          body={body}
          onBodyChange={setBody}
          selectedDocument={selectedDocument}
          editable={editable}
          busy={busy}
          themeBase={activeTheme.base}
          onSave={() => void onSaveDocument()}
          onUploadAndAttach={(file) => void onUploadAndAttachResource(file)}
          onEditorUploadImages={(files, callback) => void onEditorUploadImages(files, callback)}
          links={links}
          backlinks={backlinks}
          resources={resources}
          remoteMedia={remoteMedia}
          onLocalizeRemoteMedia={() => void onLocalizeRemoteMedia()}
          onOpenDocument={(id) => void openDocumentByID(id)}
          trashed={trashed}
          notebookOptions={notebookChoices}
          notebookID={toolbarNotebookID}
          defaultNotebookName={defaultNotebookName}
          onSelectNotebook={(id) => void onSelectNotebook(id)}
          onDelete={() => void onDeleteDocument()}
          onRestore={() => void onRestoreDocument()}
          onPurge={() => void onPurgeDocument()}
        />
        <PaneSplitter
          label="Resize editor"
          value={clamped.editor}
          min={MIN_WIDTHS.editor}
          max={containerWidth - MIN_WIDTHS.sidebar - MIN_WIDTHS.search - MIN_WIDTHS.preview}
          {...splitterHandlers(2)}
        />
        <PreviewPane
          body={body}
          themeBase={activeTheme.base}
          onOpenDocument={(id) => void openDocumentByID(id)}
          onOpenStableLink={(uri) => void openStableLink(uri)}
          onError={setError}
        />
      </div>
      {/* Pane widths as CSS custom properties for the fixed-width panes. */}
      <style>{`.workspace > .sidebar-pane{width:${clamped.sidebar}px}.workspace > .search-pane{width:${clamped.search}px}.workspace > .editor-pane{width:${clamped.editor}px}.workspace > .preview-pane{width:${Math.max(MIN_WIDTHS.preview, previewWidth(clamped, containerWidth))}px}`}</style>
    </main>
  );
}
