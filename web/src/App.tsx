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
  listDocumentTags,
  addDocumentTag,
  removeDocumentTag,
  createSearchNotebook,
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
import { clearDraft, discardPrompt, draftDiffers, loadDraft, saveDraft, type Draft } from './draft';
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
import { runBatch, type BatchRequest, type BatchResult } from './api';
import { usePagedSearch } from './usePagedSearch';
import { errorMessage } from './preview-utils';
import { PaneSplitter } from './components/PaneSplitter';
import { SidebarPane } from './components/SidebarPane';
import { SearchPane } from './components/SearchPane';
import { EditorPane } from './components/EditorPane';
import { PreviewPane } from './components/PreviewPane';
import { SelectionPanel } from './components/SelectionPanel';
import { SyncCenter } from './components/SyncCenter';
import { LibraryTransfer, transferBridge } from './components/LibraryTransfer';
import { reportUnsavedChanges, useNativeBridgeReady } from './desktop';
import { AboutDialog } from './components/AboutDialog';
import { TagRename } from './components/TagRename';
import { LibraryHealth } from './components/LibraryHealth';

// What an untouched new note holds. Named because the draft check compares
// against it: an editor still showing the template is not unsaved work.
const NEW_NOTE_TITLE = 'New note';
const defaultBody = `# New note\n\nWrite Markdown here. Link other notes with:\n\n[Related note](document://default/documents/<document-id>)\n`;

declare global {
  interface Window {
    __notriosOpenHelp?: number;
    __notriosOpenTransfer?: number;
    __notriosOpenAbout?: number;
  }
}

export function App() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [query, setQuery] = useState('');
  const [activeQuery, setActiveQuery] = useState('');
  const [selectedDocument, setSelectedDocument] = useState<DocumentRecord | null>(null);
  const [title, setTitle] = useState(NEW_NOTE_TITLE);
  const [body, setBody] = useState(defaultBody);
  const [resources, setResources] = useState<ResourceReference[]>([]);
  // The open note's tags. Held here rather than inside the toolbar control
  // because the same list is what a later sidebar count would read, and two
  // components asking the server separately would disagree the moment one of
  // them changed something.
  const [documentTags, setDocumentTags] = useState<string[]>([]);
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
  const [showSyncCenter, setShowSyncCenter] = useState(false);
  const [showTransfer, setShowTransfer] = useState(false);
  const [showAbout, setShowAbout] = useState(false);
  const [renamingTag, setRenamingTag] = useState<string | null>(null);
  const [showHealth, setShowHealth] = useState(false);
  // Not read once. Wails injects the bridge after the webview starts, so a
  // check at mount can run first and conclude this is a browser -- leaving the
  // control disabled in the desktop application until the window is reopened.
  // That was observed intermittently, which is the worst way to find out an
  // assumption was wrong.
  const bridgeReady = useNativeBridgeReady();
  const transferAvailable = bridgeReady && transferBridge() !== undefined;

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

  // ----- Acting on several notes at once -----
  //
  // The missing primitive was selection, not the operations: `POST
  // /api/v1/batch` has moved, tagged, untagged, trashed, restored and
  // duplicated a set since v0.6, and this pane tracked one note.
  const [checked, setChecked] = useState<ReadonlySet<string>>(new Set());
  // Read through a ref so the toggle keeps one identity for the life of the
  // app: it is a prop of the search pane's every row, and a handler that
  // changed whenever a page of results arrived would rebuild all of them.
  const pagedHitsRef = useRef(paged.hits);
  pagedHitsRef.current = paged.hits;
  const [batchReport, setBatchReport] = useState<BatchResult | null>(null);
  const [batchError, setBatchError] = useState('');
  const lastCheckedRef = useRef<string | null>(null);

  const onToggleChecked = useCallback((documentID: string, extend: boolean) => {
    setBatchReport(null);
    setBatchError('');
    setChecked((current) => {
      const next = new Set(current);
      // Shift extends from the last one touched, over the results as they are
      // listed rather than as they were clicked -- which is what makes "these
      // twelve" one gesture instead of twelve.
      const anchor = lastCheckedRef.current;
      if (extend && anchor && anchor !== documentID) {
        const ids = pagedHitsRef.current.map((hit) => hit.id);
        const from = ids.indexOf(anchor);
        const to = ids.indexOf(documentID);
        if (from >= 0 && to >= 0) {
          const [low, high] = from < to ? [from, to] : [to, from];
          for (const id of ids.slice(low, high + 1)) next.add(id);
          lastCheckedRef.current = documentID;
          return next;
        }
      }
      if (next.has(documentID)) next.delete(documentID);
      else next.add(documentID);
      lastCheckedRef.current = documentID;
      return next;
    });
  }, []);

  const clearChecked = useCallback(() => {
    setChecked(new Set());
    setBatchReport(null);
    setBatchError('');
    lastCheckedRef.current = null;
  }, []);

  /**
   * Runs one batch over the checked notes and reports every item.
   *
   * `request_key` is sent on every run. A person who clicks twice because
   * nothing appeared to happen is exactly the case the key exists for: the
   * second run is answered from the ledger rather than performed again, and the
   * report says `replayed` so the interface is not lying about having done the
   * work twice.
   *
   * A run that happened is a 200 even when every item failed, so the report is
   * read rather than the status: `failed` and `rolled_back` are what say
   * whether anything is wrong.
   */
  async function runOverChecked(request: Omit<BatchRequest, 'items'>, describe: string) {
    const ids = pagedHitsRef.current.map((hit) => hit.id).filter((id) => checked.has(id));
    if (ids.length === 0) return;
    // Trash is the one operation the store preconditions on a revision, and a
    // search hit does not carry one. The note in the editor supplies its own;
    // any other is read immediately before the run, which is the same guarantee
    // a person gets when they open a note and then delete it.
    const items = await Promise.all(ids.map(async (id) => {
      if (request.operation !== 'trash') return { document_id: id };
      if (selectedDocument?.id === id) {
        return { document_id: id, base_revision_id: selectedDocument.current_revision_id };
      }
      const current = await getDocument(id);
      return { document_id: id, base_revision_id: current.current_revision_id };
    }));

    setBusy(true);
    setBatchError('');
    setBatchReport(null);
    setError(null);
    setMessage(null);
    try {
      const result = await runBatch({
        ...request,
        request_key: `gui-${request.operation}-${Date.now()}-${ids.length}`,
        items,
      });
      setBatchReport(result);
      setMessage(`${describe}: ${result.applied} applied, ${result.skipped} skipped, ${result.failed} failed.`);
      // The results list and the sidebar both describe what just changed, and
      // the note in the editor may be one of the notes that changed.
      if (activeQuery.trim() !== '') void paged.start(activeQuery);
      void refreshSidebar();
      if (selectedDocument && checked.has(selectedDocument.id)) {
        if (request.operation === 'trash') clearEditor();
        else void openDocumentByID(selectedDocument.id, 'reload the note the batch changed');
      }
    } catch (err) {
      setBatchError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  /**
   * Trashing a set asks about the open note's unsaved work, and only then.
   *
   * The slice this implements said the selection panel must go through the
   * unsaved-draft guard rather than around it, on the reasoning that having the
   * note you were reading disappear is abrupt when it was unsaved. Checked
   * against what the panel actually does: entering and leaving a selection
   * discards nothing -- the draft stays in state and the editor comes back to
   * the same note -- so a confirmation there would ask about a loss that is not
   * happening, which is how people learn to dismiss confirmations without
   * reading them. Trashing the note the editor is holding is the case where the
   * work really does go, so that is where the guard belongs.
   */
  function onTrashChecked() {
    const openNoteIsGoing = selectedDocument !== null && checked.has(selectedDocument.id);
    if (openNoteIsGoing && !confirmDiscard('move it to the Trash with the others')) return;
    if (!window.confirm(`Move ${checked.size} ${checked.size === 1 ? 'note' : 'notes'} to the Trash?\n\n`
      + 'They stay in the Trash until you restore them or delete them permanently.')) return;
    void runOverChecked({ operation: 'trash' }, 'Moved to the Trash');
  }

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
    // A draft outranks a startup link. The link names a note that is safely in
    // the store and can be opened at any time; the draft exists nowhere else,
    // so opening the link over it would be the one irreversible choice here.
    const draft = loadDraft();
    if (draft) restoreDraft(draft, Boolean(deepLink));
    else if (deepLink) void openDocumentByID(deepLink.documentID);
    // The native File menu opens import and export the same way, and sets the
    // same kind of flag first so a menu choice made before React mounts is
    // honoured rather than dropped.
    const openTransfer = () => setShowTransfer(true);
    if (window.__notriosOpenTransfer) {
      delete window.__notriosOpenTransfer;
      openTransfer();
    }
    const openAbout = () => setShowAbout(true);
    if (window.__notriosOpenAbout) {
      delete window.__notriosOpenAbout;
      openAbout();
    }
    window.addEventListener('notrios:open-help', openHelp);
    window.addEventListener('notrios:open-transfer', openTransfer);
    window.addEventListener('notrios:open-about', openAbout);
    return () => {
      window.clearInterval(statusInterval);
      window.removeEventListener('notrios:open-help', openHelp);
      window.removeEventListener('notrios:open-transfer', openTransfer);
      window.removeEventListener('notrios:open-about', openAbout);
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
  // Resolved over the whole tree, not the pickable options: those omit builtin
  // notebooks because they are not valid destinations, but a Help note lives in
  // Help and the control has to say so rather than falling back to "Notes".
  const toolbarNotebookLabel = useMemo(
    () => (toolbarNotebookID ? notebookName(notebooks, toolbarNotebookID) : defaultNotebookName),
    [notebooks, toolbarNotebookID, defaultNotebookName],
  );

  const statusText = useMemo(() => {
    if (!status) return 'loading…';
    const schema = status.database_info?.schema_version ? ` · schema ${status.database_info.schema_version}` : '';
    const profile = status.profile ? ` · profile ${status.profile}` : '';
    const sidecar = status.search_sidecar;
    if (!sidecar?.configured) return `${status.service} ${status.version}${profile}${schema} · Recoll off`;
    const sync = sidecar.last_sync_at ? new Date(sidecar.last_sync_at).toLocaleTimeString() : 'never';
    return `${status.service} ${status.version}${profile}${schema} · Recoll ${sidecar.state} · ${sidecar.backlog} pending · synced ${sync}`;
  }, [status]);

  const editable = selectedDocument ? selectedDocument.editable !== false : true;

  // ----- The unsaved draft -----
  //
  // Notrios saves on purpose rather than continuously, because every save
  // writes a revision that is replicated and never pruned (see `draft.ts`).
  // Two obligations follow, and both belong here rather than in the editor
  // pane: nothing may replace what is in the editor without asking, and what
  // is in the editor has to survive the window being reloaded.
  const draftBaseline = useMemo(
    () => (selectedDocument
      ? { title: selectedDocument.title, body: selectedDocument.body ?? '' }
      : { title: NEW_NOTE_TITLE, body: defaultBody }),
    [selectedDocument],
  );
  // A read-only note cannot be dirty: its fields refuse edits, so a difference
  // here would be one this code invented rather than one the reader typed.
  const draftIsDirty = editable && draftDiffers(draftBaseline, { title, body });
  // False when the draft is too large for the browser's storage, or storage is
  // unavailable. The badge says so rather than promising work is being kept.
  const [draftKept, setDraftKept] = useState(true);
  // Appended to the two confirmations that destroy a note: what is unsaved
  // in the editor goes with it, and the question should say so.
  const unsavedChangesGoToo = draftIsDirty ? '\n\nYour unsaved changes to it are discarded.' : '';

  useEffect(() => {
    if (!draftIsDirty) {
      clearDraft();
      setDraftKept(true);
      return undefined;
    }
    // On a short delay rather than on every keystroke. The draft only has to
    // survive the window going away, and serialising a long note once per
    // character is a cost paid while somebody is typing. What the delay could
    // lose -- the last few characters -- is written by the unload handler
    // below before the window is allowed to go.
    const timer = window.setTimeout(() => {
      setDraftKept(saveDraft({ documentID: selectedDocument?.id ?? null, title, body }));
    }, 300);
    return () => window.clearTimeout(timer);
  }, [draftIsDirty, selectedDocument?.id, title, body]);

  // Reload, or a browser tab closed with unsaved work. The draft is stored
  // either way; this is what keeps the reload from happening unremarked. A
  // Wails window closed from its own title bar is the app's decision and not
  // this handler's, which is why the stored copy matters more than the prompt.
  useEffect(() => {
    if (!draftIsDirty) return undefined;
    const warn = (event: BeforeUnloadEvent) => {
      // Flush first: this is the moment the debounced write exists for.
      const draftState = draftStateRef.current;
      saveDraft({ documentID: draftState.documentID, title: draftState.title, body: draftState.body });
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [draftIsDirty]);

  // The window's close button is not the frontend's to intercept: it does not
  // go through `beforeunload`, so the desktop shell asks instead and has to be
  // told what to ask about. See cmd/notrios/gui_window_state.go.
  useEffect(() => {
    reportUnsavedChanges(draftIsDirty);
  }, [draftIsDirty]);

  // Read through a ref so the guard keeps one identity for the life of the
  // app: it is a dependency of `openDocumentByID`, which is a dependency of
  // the search pane's row handler, and a guard that changed on every
  // keystroke would re-render the result list on every keystroke.
  const draftStateRef = useRef<{ dirty: boolean; title: string; body: string; documentID: string | null }>({
    dirty: false, title: '', body: '', documentID: null,
  });
  draftStateRef.current = { dirty: draftIsDirty, title, body, documentID: selectedDocument?.id ?? null };

  /**
   * confirmDiscard guards everything that replaces the editor's contents.
   * `action` completes "Discard them and …" so the question reads as one
   * sentence and names what is about to happen, like the Trash confirmations.
   */
  const confirmDiscard = useCallback((action: string) => {
    const draftState = draftStateRef.current;
    if (!draftState.dirty) return true;
    return window.confirm(discardPrompt(draftState.title, action));
  }, []);

  /**
   * restoreDraft puts a stored draft back in the editor after a reload. When
   * the draft belongs to a saved note that note is loaded first, so the
   * restored text saves as a revision of it rather than as a second note.
   */
  function restoreDraft(draft: Draft, linkNotFollowed: boolean) {
    const when = draft.savedAt ? new Date(draft.savedAt).toLocaleString() : '';
    const apply = () => {
      setTitle(draft.title);
      setBody(draft.body);
      setMessage(
        `Restored unsaved changes${when ? ` from ${when}` : ''}. Save them, or start a new note to discard them.`
        + (linkNotFollowed ? ' The link this window was opened at was not followed, so the changes were not replaced.' : ''),
      );
    };
    // Applied after the load either way: if the note has since been deleted
    // the load reports that, and the text is still the reader's to save.
    if (draft.documentID) void openDocumentByID(draft.documentID).then(apply);
    else apply();
  }

  // Tag changes are applied and then re-read rather than assumed. The server
  // decides what a tag is called -- it normalises and it may already hold the
  // tag under another case -- so echoing the typed string into local state
  // would show the user something the library does not contain.
  const refreshDocumentTags = useCallback(async (documentID: string | null) => {
    if (!documentID) {
      setDocumentTags([]);
      return;
    }
    try {
      setDocumentTags(await listDocumentTags(documentID));
    } catch {
      // Deliberately silent. This runs whenever a note is opened, and the
      // shared error banner is where the result of something the user just did
      // is shown -- a background read that fails should not replace "Filed
      // this note in Work" with a fetch error. The control shows no tags,
      // which is the honest thing for it to show, and any attempt to add or
      // remove one reports properly because the user asked for that.
      setDocumentTags([]);
    }
  }, []);

  useEffect(() => {
    void refreshDocumentTags(selectedDocument?.id ?? null);
  }, [selectedDocument?.id, refreshDocumentTags]);

  const handleAddTag = useCallback(async (tag: string) => {
    const documentID = selectedDocument?.id;
    if (!documentID) return;
    try {
      await addDocumentTag(documentID, tag);
      await refreshDocumentTags(documentID);
    } catch (error) {
      setError(errorMessage(error));
    }
  }, [selectedDocument?.id, refreshDocumentTags]);

  const handleRemoveTag = useCallback(async (tag: string) => {
    const documentID = selectedDocument?.id;
    if (!documentID) return;
    try {
      await removeDocumentTag(documentID, tag);
      await refreshDocumentTags(documentID);
    } catch (error) {
      setError(errorMessage(error));
    }
  }, [selectedDocument?.id, refreshDocumentTags]);
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

  const openDocumentByID = useCallback(async (documentID: string, discardAction = 'open the other note') => {
    if (!confirmDiscard(discardAction)) return;
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
  }, [confirmDiscard]);

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
      await openDocumentByID(selectedDocument.id, 'reload the note as the service rewrote it');
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
    if (!window.confirm(`Move “${selectedDocument.title}” to the Trash?\n\nIt stays in the Trash until you restore it or delete it permanently.${unsavedChangesGoToo}`)) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    const { id, title: deletedTitle } = selectedDocument;
    try {
      await deleteDocument(id, selectedDocument.current_revision_id);
      paged.removeHit(id);
      clearEditor();
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
    if (!window.confirm(`Permanently delete “${selectedDocument.title}”?\n\nThe note and every revision of it are removed. This cannot be undone.${unsavedChangesGoToo}`)) return;
    setBusy(true);
    setError(null);
    setMessage(null);
    const { id, title: purgedTitle } = selectedDocument;
    try {
      await purgeDocument(id);
      paged.removeHit(id);
      clearEditor();
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

  // Raw: it throws away whatever is in the editor. Every path that reaches it
  // has either asked first or has just deleted the note the draft belonged to.
  function clearEditor() {
    setSelectedDocument(null);
    setTitle(NEW_NOTE_TITLE);
    setBody(defaultBody);
    setResources([]);
    setLinks([]);
    setBacklinks([]);
    setRemoteMedia([]);
    setMessage(null);
    setError(null);
  }

  function onNewNote() {
    if (!confirmDiscard('start a new note')) return;
    clearEditor();
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

  /**
   * Saves the search that ran as a notebook, and refreshes the sidebar so the
   * new one appears where the person will look for it.
   *
   * The error is rethrown rather than swallowed into the page-level notice: the
   * name conflict this most often produces belongs beside the field somebody
   * just typed into, not at the top of the window.
   */
  const onKeepSearch = useCallback(async (name: string) => {
    const saved = await createSearchNotebook(name, activeQuery);
    await refreshSidebar();
    setMessage(`Kept “${saved.name}” in the sidebar.`);
  }, [activeQuery, refreshSidebar]);

  return (
    <main className="app-shell">
      <header className="app-header">
        <h1>Notrios</h1>
        <div className="header-actions">
          <p className="status">{statusText}</p>
          <button type="button" className="sync-header-button" data-testid="sync-header-button"
            title="Open synchronization, pairing, backup, and recovery" onClick={() => setShowSyncCenter(true)}>
            <span aria-hidden="true">↻</span> Sync
          </button>
          {/* Shown always and disabled without the native bridge, rather than
              hidden. Importing, exporting and snapshots name a folder on the
              machine running the library, which a browser window cannot choose
              and no REST route accepts; saying so is more useful than a feature
              that appears not to exist. */}
          {/* Read-only, and available in both modes: unlike import and export
              this needs no local path, so a browser can ask for it too. */}
          <button type="button" className="sync-header-button" data-testid="health-header-button"
            title="What has rotted in this library, and what could be reclaimed"
            onClick={() => setShowHealth(true)}>
            <span aria-hidden="true">✚</span> Health
          </button>
          <button type="button" className="sync-header-button" data-testid="transfer-header-button"
            disabled={!transferAvailable}
            title={transferAvailable
              ? 'Import from Joplin, Obsidian or Notrios; export; take a snapshot'
              : 'Importing, exporting and snapshots need the desktop app: they name a folder on the machine running this library'}
            onClick={() => setShowTransfer(true)}>
            <span aria-hidden="true">⇄</span> Import/Export
          </button>
          <button type="button" className="icon-button" data-testid="theme-mode-toggle" title="Toggle light/dark theme" aria-label="Toggle light/dark theme" onClick={toggleMode}>
            {mode === 'light' ? '🌙' : '☀️'}
          </button>
          <button type="button" className="icon-button" data-testid="theme-settings" title="Theme settings" aria-label="Theme settings" onClick={() => setShowThemes((v) => !v)}>
            🎨
          </button>
        </div>
      </header>

      {showSyncCenter ? <SyncCenter onClose={() => setShowSyncCenter(false)} /> : null}

      {showTransfer ? <LibraryTransfer onClose={() => setShowTransfer(false)} /> : null}

      {showAbout ? <AboutDialog status={status} onClose={() => setShowAbout(false)} /> : null}

      {showHealth ? <LibraryHealth onClose={() => setShowHealth(false)} /> : null}

      {renamingTag ? (
        <TagRename
          tag={renamingTag}
          onClose={() => setRenamingTag(null)}
          onRenamed={(result) => {
            setRenamingTag(null);
            // The sidebar counts and the open search both refer to tags by
            // name, so both are stale the moment one is renamed.
            void refreshSidebar();
            if (activeQuery !== '') void paged.start(activeQuery);
            setMessage(`Renamed ${result.changes.length === 1 ? 'the tag' : `${result.changes.length} tags`}, `
              + `touching ${result.notes} ${result.notes === 1 ? 'note' : 'notes'}.`);
          }}
        />
      ) : null}

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
          onRenameTag={setRenamingTag}
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
          onNewNote={onNewNote}
          newNoteNotebookName={creationNotebookName}
          ranQuery={activeQuery}
          onKeepSearch={onKeepSearch}
          checked={checked}
          onToggleChecked={onToggleChecked}
        />
        <PaneSplitter
          label="Resize search results"
          value={clamped.search}
          min={MIN_WIDTHS.search}
          max={containerWidth - MIN_WIDTHS.sidebar - MIN_WIDTHS.editor - MIN_WIDTHS.preview}
          {...splitterHandlers(1)}
        />
        {checked.size > 0 ? (
          // The editor and the preview are both about one note, and with
          // several chosen there is no one note to show. Replacing them is
          // Joplin's shape and it is the honest one: the alternative is
          // photographing whichever note was clicked last beside a list of
          // notes the next action will change.
          <SelectionPanel
            selected={paged.hits.filter((hit) => checked.has(hit.id))
              .map((hit) => ({ id: hit.id, title: hit.title ?? hit.uri }))}
            notebooks={notebookChoices}
            busy={busy}
            report={batchReport}
            error={batchError}
            onClear={clearChecked}
            onMove={(notebookID) => void runOverChecked({ operation: 'move', notebook_id: notebookID }, 'Moved')}
            onAddTag={(tag) => void runOverChecked({ operation: 'add_tags', tags: [tag] }, 'Tagged')}
            onRemoveTag={(tag) => void runOverChecked({ operation: 'remove_tags', tags: [tag] }, 'Untagged')}
            onDuplicate={() => void runOverChecked({ operation: 'duplicate' }, 'Duplicated')}
            onTrash={onTrashChecked}
            onRestore={() => void runOverChecked({ operation: 'restore' }, 'Restored')}
          />
        ) : (
        <EditorPane
          title={title}
          onTitleChange={setTitle}
          body={body}
          onBodyChange={setBody}
          selectedDocument={selectedDocument}
          editable={editable}
          busy={busy}
          unsaved={draftIsDirty}
          draftKept={draftKept}
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
          tags={documentTags}
          onAddTag={handleAddTag}
          onRemoveTag={handleRemoveTag}
          notebookID={toolbarNotebookID}
          notebookLabel={toolbarNotebookLabel}
          onSelectNotebook={(id) => void onSelectNotebook(id)}
          onDelete={() => void onDeleteDocument()}
          onRestore={() => void onRestoreDocument()}
          onPurge={() => void onPurgeDocument()}
        />
        )}
        {checked.size === 0 && (
          <PaneSplitter
            label="Resize editor"
            value={clamped.editor}
            min={MIN_WIDTHS.editor}
            max={containerWidth - MIN_WIDTHS.sidebar - MIN_WIDTHS.search - MIN_WIDTHS.preview}
            {...splitterHandlers(2)}
          />
        )}
        {checked.size === 0 && (
          <PreviewPane
            body={body}
            themeBase={activeTheme.base}
            onOpenDocument={(id) => void openDocumentByID(id)}
            onOpenStableLink={(uri) => void openStableLink(uri)}
            onError={setError}
          />
        )}
      </div>
      {/* Pane widths as CSS custom properties for the fixed-width panes. */}
      <style>{`.workspace > .sidebar-pane{width:${clamped.sidebar}px}.workspace > .search-pane{width:${clamped.search}px}.workspace > .editor-pane{width:${clamped.editor}px}.workspace > .preview-pane{width:${Math.max(MIN_WIDTHS.preview, previewWidth(clamped, containerWidth))}px}`}</style>
    </main>
  );
}
