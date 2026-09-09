export interface StatusResponse {
  service: string;
  version: string;
  status: string;
  profile?: string;
  profile_id?: string;
  database?: string;
  config_path?: string;
  database_info?: {
    driver?: string;
    path?: string;
    state?: string;
    // The logical database identity external notrios:// links carry. The
    // per-copy replica ID is deliberately not reported by the service.
    database_id?: string;
    schema_version?: number;
  };
  storage?: {
    data_directory?: string;
    database_path?: string;
    asset_store?: string;
    projection_dir?: string;
    search_sidecar_index_dir?: string;
  };
  capabilities?: Record<string, boolean>;
  limits?: Record<string, number>;
  search_sidecar: {
    configured: boolean;
    available: boolean;
    active: boolean;
    state: string;
    backlog: number;
    failed_jobs: number;
    last_sync_at?: string;
    last_index_at?: string;
    last_reconciliation_at?: string;
    last_error?: string;
    reconciliation?: {
      complete: boolean;
      canonical: number;
      scanned: number;
      missing: number;
      stale: number;
      orphaned: number;
      repaired: number;
      failed: number;
    };
  };
}

export interface SearchHit {
  id: string;
  uri: string;
  source: string;
  sources?: string[];
  collection_id?: string;
  title?: string;
  snippet?: string;
  score?: number;
  editable?: boolean;
  metadata?: Record<string, unknown>;
}

export interface SearchResponse {
  hits: SearchHit[];
  next_cursor?: string;
  total?: number;
}

export interface DocumentRecord {
  id: string;
  uri: string;
  collection_id: string;
  notebook_id?: string;
  /** Server-authoritative capability: false for protected (Help) and trashed notes. */
  editable?: boolean;
  title: string;
  body_mime_type: string;
  body?: string;
  current_revision_id: string;
  metadata?: Record<string, unknown>;
  tags?: string[];
  created_at?: string;
  updated_at?: string;
  deleted_at?: string;
}



export interface SourcePosition {
  start_byte?: number;
  end_byte?: number;
  line?: number;
  column?: number;
}

export interface DocumentLink {
  id: string;
  source_document_id: string;
  target_document_id?: string;
  target_resource_id?: string;
  target_uri?: string;
  relation_type: string;
  source_format?: string;
  raw_target?: string;
  display_text?: string;
  anchor_type?: string;
  anchor_value?: string;
  context?: string;
  resolution_status: string;
  source_position?: SourcePosition;
}

export interface DocumentLinkPage {
  outgoing?: DocumentLink[];
  incoming?: DocumentLink[];
  next_cursor?: string;
}

export interface ResourceRecord {
  id: string;
  uri: string;
  collection_id: string;
  filename?: string;
  mime_type: string;
  size_bytes?: number;
  sha256?: string;
  created_at?: string;
}

export interface ResourceReference {
  document_id: string;
  resource_id: string;
  resource?: ResourceRecord;
  relation_type: string;
  ordinal?: number;
  anchor?: Record<string, unknown>;
}

export interface ResourceReferencePage {
  resources: ResourceReference[];
  next_cursor?: string;
}

export interface CreateDocumentRequest {
  collection_id?: string;
  /**
   * Where the note is filed. Omitted, the service uses the default "Notes"
   * notebook — which is the right answer when the sidebar has a search
   * notebook selected, not a fallback worth apologising for.
   */
  notebook_id?: string;
  title: string;
  body: string;
  body_mime_type?: string;
  tags?: string[];
  metadata?: Record<string, unknown>;
}

export interface UpdateDocumentRequest {
  title: string;
  body: string;
  body_mime_type?: string;
  base_revision_id: string;
}

async function parseJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = await response.json() as { error?: { message?: string } };
      if (body.error?.message) {
        message = body.error.message;
      }
    } catch {
      // Keep the HTTP status message when the body is not JSON.
    }
    throw new Error(message);
  }
  return response.json() as Promise<T>;
}

export async function getStatus(): Promise<StatusResponse> {
  const response = await fetch('/api/v1/status');
  return parseJSON<StatusResponse>(response);
}

export interface StableLinkResolution {
  uri: string;
  status: string;
  database_id: string;
  local_database_id: string;
  document_id: string;
  anchor?: string;
  document_uri?: string;
  title?: string;
  notebook_id?: string;
}

/**
 * Asks the service which note a notrios:// link names. The service answers for
 * the database it has open and takes no database selector, so a link belonging
 * to another library comes back as `foreign_database` rather than silently
 * matching a local ID.
 */
export async function resolveStableLink(uri: string): Promise<StableLinkResolution> {
  const response = await fetch('/api/v1/links/resolve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ uri }),
  });
  return parseJSON<StableLinkResolution>(response);
}

export async function createDocument(request: CreateDocumentRequest): Promise<DocumentRecord> {
  const response = await fetch('/api/v1/documents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      collection_id: request.collection_id ?? 'default',
      notebook_id: request.notebook_id || undefined,
      title: request.title,
      body: request.body,
      body_mime_type: request.body_mime_type ?? 'text/markdown',
      tags: request.tags ?? [],
      metadata: request.metadata ?? {},
    }),
  });
  return parseJSON<DocumentRecord>(response);
}

/** One note in a batch, with the revision a trash is preconditioned on. */
export interface BatchItem {
  document_id: string;
  base_revision_id?: string;
}

export interface BatchItemResult {
  document_id: string;
  status: string;
  reason?: string;
  error?: string;
  new_document_id?: string;
}

export interface BatchResult {
  request_key?: string;
  operation: string;
  mode: string;
  items: BatchItemResult[];
  applied: number;
  skipped: number;
  failed: number;
  rolled_back: number;
  replayed: boolean;
}

export interface BatchRequest {
  request_key?: string;
  operation: 'move' | 'add_tags' | 'remove_tags' | 'trash' | 'restore' | 'duplicate';
  mode?: 'atomic' | 'best_effort';
  items: BatchItem[];
  notebook_id?: string;
  tags?: string[];
}

/**
 * Runs one bounded organiser transaction over a set of notes.
 *
 * A run that happened is a 200 even when every item failed, because per-item
 * failure is the report's content rather than the request's fate — so a caller
 * has to read `failed` and `rolled_back` rather than trusting the status code.
 * Only a malformed request is a 4xx, and `parseJSON` turns that into a thrown
 * error carrying the service's own message.
 */
export async function runBatch(request: BatchRequest): Promise<BatchResult> {
  const response = await fetch('/api/v1/batch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  return parseJSON<BatchResult>(response);
}

export async function getDocument(documentID: string): Promise<DocumentRecord> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}`);
  return parseJSON<DocumentRecord>(response);
}

export async function updateDocument(documentID: string, request: UpdateDocumentRequest): Promise<DocumentRecord> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      title: request.title,
      body: request.body,
      body_mime_type: request.body_mime_type ?? 'text/markdown',
      base_revision_id: request.base_revision_id,
    }),
  });
  return parseJSON<DocumentRecord>(response);
}

export async function getDocumentBody(documentID: string): Promise<string> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/body`);
  if (!response.ok) {
    throw new Error(`body request failed: ${response.status}`);
  }
  return response.text();
}

export async function search(query: string, limit = 20, cursor = ''): Promise<SearchResponse> {
  const response = await fetch('/api/v1/search', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, limit, cursor: cursor || undefined }),
  });
  return parseJSON<SearchResponse>(response);
}

export interface Notebook {
  id: string;
  parent_id?: string;
  name: string;
  icon_emoji?: string;
  builtin: boolean;
  position: number;
}

export interface NotebookTreeNode extends Notebook {
  children?: NotebookTreeNode[];
}

export interface SearchNotebook {
  id: string;
  name: string;
  icon_emoji?: string;
  query: string;
  builtin: boolean;
  sort_anchor: string;
}

export interface TagRecord {
  id: string;
  name: string;
  note_count: number;
}

export async function getNotebookTree(): Promise<NotebookTreeNode[]> {
  const response = await fetch('/api/v1/notebooks/tree');
  const payload = await parseJSON<{ notebooks: NotebookTreeNode[] }>(response);
  return payload.notebooks ?? [];
}

export async function listSearchNotebooks(): Promise<SearchNotebook[]> {
  const response = await fetch('/api/v1/search-notebooks');
  const payload = await parseJSON<{ search_notebooks: SearchNotebook[] }>(response);
  return payload.search_notebooks ?? [];
}

/**
 * Saves a query as a search notebook.
 *
 * The query is passed in rather than read from the search box, because the two
 * can differ: somebody types, searches, then types again while reading the
 * results. Saving what is in the box would produce a notebook whose contents
 * are not the results they were looking at when they decided to keep them.
 */
export async function createSearchNotebook(name: string, query: string): Promise<SearchNotebook> {
  const response = await fetch('/api/v1/search-notebooks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, query }),
  });
  return parseJSON<SearchNotebook>(response);
}

/** One tag the rename would change, and what it would do to it. */
export interface TagRenameChange {
  tag_id: string;
  from: string;
  to: string;
  action: string;
  merged_into_tag_id?: string;
  notes: number;
  /** How many of those notes do not already carry the destination tag. */
  notes_gained: number;
}

export interface TagRenameResult {
  from: string;
  to: string;
  include_children: boolean;
  dry_run: boolean;
  changes: TagRenameChange[];
  notes: number;
  warnings: string[];
}

/**
 * Renames a tag, and by default only says what that would do.
 *
 * `dry_run` is sent explicitly rather than left to the service's default,
 * because the difference between looking and changing every note carrying a tag
 * should be visible at the call site rather than in a document.
 *
 * The report is not a prediction: the service performs the rename in a
 * transaction and rolls it back for a dry run, so what is shown and what would
 * happen cannot drift apart.
 */
export async function renameTag(
  from: string,
  to: string,
  includeChildren: boolean,
  dryRun: boolean,
): Promise<TagRenameResult> {
  const response = await fetch('/api/v1/tags/rename', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ from, to, include_children: includeChildren, dry_run: dryRun }),
  });
  return parseJSON<TagRenameResult>(response);
}

/**
 * One thing lint found. Deliberately content-free: a finding locates a problem
 * without quoting a note, and the offending target is a fingerprint rather than
 * the value itself.
 */
export interface LintFinding {
  check: string;
  document_id?: string;
  resource_id?: string;
  line?: number;
  column?: number;
  target_sha256?: string;
  detail?: string;
}

export interface LintCheckResult {
  check: string;
  /** The complete count, unaffected by the detail cap. */
  count: number;
  findings: LintFinding[];
  truncated: boolean;
}

export interface LintReport {
  version: number;
  collection_id: string;
  detail_limit: number;
  checks: LintCheckResult[];
  total_findings: number;
  /** Covers every finding, including those the detail cap hid. */
  report_sha256: string;
  warnings: string[];
}

export interface GarbageCollectionReport {
  dry_run: boolean;
  as_of: string;
  eligible: unknown[];
  retained: unknown[];
  removed: unknown[];
  referenced_resource_count: number;
  blobs_removed: number;
  bytes_removed: number;
  warnings: string[];
}

/** Reads the lint report. There is no apply: fixing is a command-line action. */
export async function getLintReport(): Promise<LintReport> {
  return parseJSON<LintReport>(await fetch('/api/v1/admin/lint/report'));
}

/**
 * Reads what garbage collection would reclaim.
 *
 * Always a plan and never a deletion: the service has intentionally no REST
 * apply, so nothing this returns has been removed.
 */
export async function getGarbageReport(): Promise<GarbageCollectionReport> {
  return parseJSON<GarbageCollectionReport>(await fetch('/api/v1/admin/gc/report'));
}

export async function listTags(): Promise<TagRecord[]> {
  const response = await fetch('/api/v1/tags');
  const payload = await parseJSON<{ tags: TagRecord[] }>(response);
  return payload.tags ?? [];
}

// The three calls behind the editor's tag control.
//
// Tagging was reachable over REST and MCP and from neither surface a person
// uses -- the gap v0.8 H14 opened with, found by reading the documentation and
// then again by comparing the two surfaces mechanically. The command line
// gained `tags add`, `tags remove` and `tags list`; these are the other half.
export async function listDocumentTags(documentID: string): Promise<string[]> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/tags`);
  const payload = await parseJSON<{ tags: TagRecord[] }>(response);
  return (payload.tags ?? []).map((tag) => tag.name);
}

export async function addDocumentTag(documentID: string, tag: string): Promise<void> {
  const response = await fetch(
    `/api/v1/documents/${encodeURIComponent(documentID)}/tags/${encodeURIComponent(tag)}`,
    { method: 'POST' },
  );
  await parseJSON<unknown>(response);
}

export async function removeDocumentTag(documentID: string, tag: string): Promise<void> {
  const response = await fetch(
    `/api/v1/documents/${encodeURIComponent(documentID)}/tags/${encodeURIComponent(tag)}`,
    { method: 'DELETE' },
  );
  // A tag the note does not carry answers 404, and that is not an error worth
  // showing: the control only offers removal of tags it is displaying, so a
  // 404 here means somebody else got there first and the end state is the one
  // the user wanted.
  if (response.status === 404) return;
  await parseJSON<unknown>(response);
}

export async function uploadResource(file: File, collectionID = 'default'): Promise<ResourceRecord> {
  const params = new URLSearchParams({ collection_id: collectionID, filename: file.name });
  const response = await fetch(`/api/v1/resources?${params.toString()}`, {
    method: 'POST',
    headers: { 'Content-Type': file.type || 'application/octet-stream' },
    body: file,
  });
  return parseJSON<ResourceRecord>(response);
}

export async function attachResource(documentID: string, resourceID: string, relationType = 'attachment'): Promise<ResourceReference> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/resources/${encodeURIComponent(resourceID)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ relation_type: relationType }),
  });
  return parseJSON<ResourceReference>(response);
}

export async function listDocumentResources(documentID: string): Promise<ResourceReferencePage> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/resources`);
  return parseJSON<ResourceReferencePage>(response);
}

export async function listDocumentLinks(documentID: string, direction = 'both'): Promise<DocumentLinkPage> {
  const params = new URLSearchParams({ direction });
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/links?${params.toString()}`);
  return parseJSON<DocumentLinkPage>(response);
}

export interface RemoteMediaDecision {
  url: string;
  media_class: string;
  action: 'allow' | 'block' | 'review';
  reason: string;
  line?: number;
}

export interface RemoteMediaScanResult {
  document_id?: string;
  media: RemoteMediaDecision[];
  counts: Record<string, number>;
}

/** Static policy scan of a note's remote media — nothing is downloaded. */
export async function scanRemoteMedia(documentID: string): Promise<RemoteMediaScanResult> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/remote-media/scan`, { method: 'POST' });
  return parseJSON<RemoteMediaScanResult>(response);
}

export interface RemoteMediaLocalizeResult {
  revision_id?: string;
  localized: Array<Record<string, unknown>>;
  blocked: Array<Record<string, unknown>>;
  review: Array<Record<string, unknown>>;
  failed: Array<Record<string, unknown>>;
}

/**
 * Server-side localization: quarantine-fetch policy-allowed remote media,
 * store it as local resources, and rewrite the note to resource:// URIs in
 * a new revision (guarded by the base revision).
 */
export async function localizeRemoteMedia(
  documentID: string,
  baseRevisionID: string,
  options: { dryRun?: boolean; allowReview?: boolean } = {},
): Promise<RemoteMediaLocalizeResult> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/remote-media/localize`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      base_revision_id: baseRevisionID,
      dry_run: options.dryRun ?? false,
      allow_review: options.allowReview ?? false,
    }),
  });
  return parseJSON<RemoteMediaLocalizeResult>(response);
}

export function resourceContentURL(resourceID: string, download = true): string {
  const suffix = download ? '?download=1' : '';
  return `/api/v1/resources/${encodeURIComponent(resourceID)}/content${suffix}`;
}

// --- Editor link intelligence (v0.5 E5) -------------------------------------
//
// Both calls are read-only and bounded, and both are made while someone is
// typing. Neither writes a revision, and a failure of either degrades the
// editor to no suggestions and no markers rather than to an error: link
// intelligence is an assist, and an assist that interrupts typing is worse
// than one that is quietly absent.

export interface DocumentSuggestion {
  document_id: string;
  title: string;
  uri: string;
  notebook_id?: string;
  /** `title_prefix` matched the start of the title, `word_prefix` a later word. */
  match: string;
}

export interface DocumentSuggestionResponse {
  suggestions: DocumentSuggestion[];
  truncated?: boolean;
  limit: number;
}

export interface CheckedLink {
  raw_target: string;
  display_text?: string;
  relation_type?: string;
  source_format?: string;
  anchor_type?: string;
  anchor_value?: string;
  /** resolved | unresolved | ambiguous | external | invalid | stale_anchor */
  status: string;
  target_document_id?: string;
  target_resource_id?: string;
  target_uri?: string;
  /** The URI a title-resolved link already points at, offered as a repair. */
  canonical_target?: string;
  start_byte: number;
  end_byte: number;
  line: number;
  column: number;
}

export interface CheckLinksResponse {
  links: CheckedLink[];
  total: number;
  unresolved: number;
  truncated?: boolean;
}

/** Bounded link-target suggestions. The query must be at least two characters. */
export async function suggestLinkTargets(
  query: string,
  options: { limit?: number; excludeDocumentID?: string; signal?: AbortSignal } = {},
): Promise<DocumentSuggestionResponse> {
  const params = new URLSearchParams({ q: query });
  if (options.limit) params.set('limit', String(options.limit));
  if (options.excludeDocumentID) params.set('exclude_document_id', options.excludeDocumentID);
  const response = await fetch(`/api/v1/links/suggest?${params.toString()}`, { signal: options.signal });
  return parseJSON<DocumentSuggestionResponse>(response);
}

/**
 * Asks whether the links in an unsaved buffer resolve. The body goes to the
 * server because extracting Markdown links is the canonical parser's job:
 * a second implementation here would give markers that disagree with what a
 * save records. Nothing is stored by this call.
 */
export async function checkBufferLinks(
  body: string,
  options: { documentID?: string; signal?: AbortSignal } = {},
): Promise<CheckLinksResponse> {
  const response = await fetch('/api/v1/links/check', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ body, document_id: options.documentID ?? '' }),
    signal: options.signal,
  });
  return parseJSON<CheckLinksResponse>(response);
}

// --- Embedded query blocks (v0.5 E7) ----------------------------------------

export interface NoteQuerySpec {
  query: string;
  fields: string[];
  sort: string;
  limit: number;
}

export interface NoteQueryRow {
  document_id: string;
  uri: string;
  title: string;
  notebook?: string;
  tags?: string[];
  updated_at?: string;
  snippet?: string;
}

export interface NoteQueryResult {
  spec: NoteQuerySpec;
  rows: NoteQueryRow[];
  truncated: boolean;
  /** Set when the block could not run. Part of a 200 response, not a failure. */
  error?: string;
}

/**
 * Evaluates one ```note-query block. The block's text is parsed by the service
 * with the same Q1 parser the search box uses, so a block can express nothing
 * its author could not already search for.
 */
export async function runNoteQuery(
  block: string,
  options: { collectionID?: string; signal?: AbortSignal } = {},
): Promise<NoteQueryResult> {
  const response = await fetch('/api/v1/note-queries/run', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ block, collection_id: options.collectionID ?? '' }),
    signal: options.signal,
  });
  return parseJSON<NoteQueryResult>(response);
}

// --- Trash-first deletion and organizer operations (v0.5 E8) -----------------
//
// Deleting a note in Notrios moves it to the Trash. That is a store rule, not a
// client convention, and these calls are the GUI's way of reaching it — before
// E8 the only way to undo a deletion was the CLI.

/** Throws with the service's message for a non-2xx response with no body. */
async function expectNoContent(response: Response): Promise<void> {
  if (response.ok) return;
  let message = `${response.status} ${response.statusText}`;
  try {
    const body = (await response.json()) as { error?: { message?: string } };
    if (body.error?.message) message = body.error.message;
  } catch {
    // Keep the HTTP status message when the body is not JSON.
  }
  throw new Error(message);
}

/**
 * Moves a note to the Trash. The base revision is a precondition: a note edited
 * elsewhere since it was opened fails rather than being deleted out from under
 * the other writer.
 */
export async function deleteDocument(documentID: string, baseRevisionID: string): Promise<void> {
  const params = new URLSearchParams({ base_revision_id: baseRevisionID });
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}?${params.toString()}`, { method: 'DELETE' });
  await expectNoContent(response);
}

/** Brings a trashed note back, in the notebook it is currently assigned to. */
export async function restoreTrashedDocument(documentID: string): Promise<DocumentRecord> {
  const response = await fetch(`/api/v1/trash/${encodeURIComponent(documentID)}/restore`, { method: 'POST' });
  return parseJSON<DocumentRecord>(response);
}

/**
 * Permanently deletes a trashed note. The service requires a confirmation
 * header naming the exact note, so a purge cannot be triggered by a stray
 * request; the header is echoed here rather than invented, and the service is
 * still the one that decides.
 */
export async function purgeDocument(documentID: string): Promise<void> {
  const response = await fetch(`/api/v1/trash/${encodeURIComponent(documentID)}`, {
    method: 'DELETE',
    headers: { 'X-Notrios-Confirmation': `purge-document:${documentID}` },
  });
  await expectNoContent(response);
}

export interface NotebookDeletionPreview {
  notebook_id: string;
  name: string;
  notebooks: number;
  descendant_names: string[];
  truncated?: boolean;
  notes: number;
  trashed_notes: number;
  /** Where every note in the subtree ends up, so a later restore has a home. */
  rehome_notebook_id: string;
  deletable: boolean;
  reason?: string;
}

/** What deleting a notebook would do, asked before asking the user. */
export async function previewNotebookDeletion(notebookID: string): Promise<NotebookDeletionPreview> {
  const response = await fetch(`/api/v1/notebooks/${encodeURIComponent(notebookID)}/deletion-preview`);
  return parseJSON<NotebookDeletionPreview>(response);
}

/**
 * Files a note into another notebook.
 *
 * Not revision-scoped: a move changes where a note lives, not what it says, so
 * it takes effect immediately and does not write a revision. The service
 * refuses moves into and out of the protected Help notebook.
 */
export async function moveDocumentToNotebook(documentID: string, notebookID: string): Promise<DocumentRecord> {
  const response = await fetch(`/api/v1/documents/${encodeURIComponent(documentID)}/notebook`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ notebook_id: notebookID }),
  });
  return parseJSON<DocumentRecord>(response);
}

/** Deletes a notebook and its descendants; their notes move to the Trash. */
export async function deleteNotebook(notebookID: string): Promise<void> {
  const response = await fetch(`/api/v1/notebooks/${encodeURIComponent(notebookID)}`, { method: 'DELETE' });
  await expectNoContent(response);
}

/**
 * A note's neighbourhood in the link graph.
 *
 * This is the half of a graph view that stays useful in a large library. A
 * global canvas degrades into an unreadable hairball past a few thousand notes;
 * a map of what surrounds *this* note does not, because its size is bounded by
 * the note rather than by the library.
 */
export interface GraphNode {
  id: string;
  uri?: string;
  /** `document`, `resource`, or a link resolution status for a broken target. */
  kind: string;
  label?: string;
  /** Hops from the note in the editor. The note itself is 0. */
  depth: number;
}

export interface GraphEdge {
  id: string;
  source_id: string;
  target_id: string;
  kind?: string;
  status?: string;
  raw_target?: string;
}

export interface GraphResponse {
  nodes: GraphNode[];
  edges: GraphEdge[];
  /** A ceiling stopped the expansion; `truncated_by` names which one. */
  truncated?: boolean;
  truncated_by?: string;
  requested_depth: number;
  completed_depth: number;
}

/**
 * Depths the local graph offers.
 *
 * The service allows up to 5, but one and two hops are what stay readable: at
 * depth 3 a well-linked note reaches a sizeable fraction of the library and the
 * view stops answering the question it was opened to answer. The ceiling is
 * shown rather than silently applied.
 */
export const LOCAL_GRAPH_DEPTHS = [1, 2] as const;

export async function getLocalGraph(
  documentID: string,
  depth: number,
  signal?: AbortSignal,
): Promise<GraphResponse> {
  const response = await fetch('/api/v1/graph', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ roots: [documentID], depth, direction: 'both' }),
    signal,
  });
  return parseJSON<GraphResponse>(response);
}

export interface SyncUIProfile {
  name: string;
  profile_id?: string;
  database_id?: string;
  public_base_url?: string;
  sync_target: 'none' | 'directory' | 'rest';
  active: boolean;
}

export interface SyncUIJob {
  id: string;
  kind: string;
  state: string;
  phase?: string;
  processed: number;
  total: number;
  summary?: Record<string, unknown>;
  error?: string;
  cancel_requested?: boolean;
  created_at: string;
}

export interface SyncUIPeer {
  replica_id: string;
  status: string;
  behind_operations: number;
  snapshot_permitted: boolean;
  enrolled_at?: string;
  revoked_at?: string;
  last_acknowledged?: string;
  warning_at?: string;
  horizon_at?: string;
  full_resync_required?: boolean;
}

export interface SyncRetentionSubject {
  replica_id: string;
  current_sequence: number;
  current_floor: number;
  age_floor: number;
  snapshot_floor: number;
  acknowledged_floor: number;
  eligible_floor: number;
  eligible_operations: number;
  eligible_bytes: number;
}

export interface SyncPeerRetentionStatus {
  replica_id: string;
  status: string;
  last_acknowledged?: string;
  warning_at?: string;
  horizon_at?: string;
  warning: boolean;
  beyond_horizon: boolean;
  full_resync_required: boolean;
}

export interface SyncRepairPlan {
  ready: boolean;
  snapshot_id?: string;
  snapshot_vector: Record<string, number>;
  newer_operations: number;
  install_automatic: false;
}

export interface SyncRetentionReport {
  dry_run: boolean;
  applied: boolean;
  as_of: string;
  history_seconds: number;
  warning_seconds: number;
  snapshot_id?: string;
  snapshot_created_at?: string;
  subjects: SyncRetentionSubject[];
  peers: SyncPeerRetentionStatus[];
  tombstones: Array<{ document_id: string; replica_id: string; sequence: number; purged_at: string }>;
  eligible_operations: number;
  eligible_bytes: number;
  removed_operations: number;
  collected_tombstones: number;
  digest: string;
  warnings: string[];
  repair: SyncRepairPlan;
}

export interface SyncRetirementPreview {
  dry_run: true;
  peer: SyncPeerRetentionStatus;
  confirmation: string;
  consequences: string[];
}

export function setSyncSnapshotPermission(replicaID: string, permitted: boolean): Promise<{ replica_id: string; snapshot_permitted: boolean }> {
  return syncUIJSON(`/api/v1/sync-ui/peers/${encodeURIComponent(replicaID)}/snapshot-permission`, 'POST', { permitted });
}

export interface SyncUIConflictSummary {
  id: string;
  document_id: string;
  base_revision_id?: string;
  revision_a: string;
  revision_b: string;
  kind: string;
  created_at: string;
}

export interface SyncConflictRevision {
  id: string;
  title: string;
  body: string;
}

export interface SyncUIConflictDetail extends SyncUIConflictSummary {
  title: string;
  body_mime_type: string;
  base: SyncConflictRevision;
  local: SyncConflictRevision;
  remote: SyncConflictRevision;
  region_count: number;
  regions_clipped?: boolean;
}

export interface SyncUIResource {
  id: string;
  filename?: string;
  mime_type: string;
  size_bytes: number;
  availability: string;
  pinned: boolean;
  requested: boolean;
}

export interface SyncUIRepair {
  id: string;
  kind: string;
  subject_id: string;
  details?: Record<string, unknown>;
}

export interface SyncUIStatus {
  status: 'setup' | 'ready' | 'syncing' | 'offline' | 'attention' | string;
  active_profile: { name: string; profile_id?: string; database_id: string; replica_id: string };
  profiles: SyncUIProfile[];
  configuration: {
    target: 'none' | 'directory' | 'rest';
    directory?: string;
    rest_base_url?: string;
    rest_inbound_enabled: boolean;
    restart_required: boolean;
  };
  journal_enabled: boolean;
  secret_store: { available: boolean; configured: boolean; name: string; warning: string };
  peers: SyncUIPeer[];
  jobs: SyncUIJob[];
  conflicts: SyncUIConflictSummary[];
  conflicts_truncated?: boolean;
  resources: SyncUIResource[];
  resources_truncated?: boolean;
  repairs: SyncUIRepair[];
  repairs_truncated?: boolean;
  retention: {
    history_seconds: number;
    snapshot_id?: string;
    eligible_operations: number;
    eligible_tombstones: number;
    digest: string;
    repair: SyncRepairPlan;
  };
}

export async function getSyncUIStatus(): Promise<SyncUIStatus> {
  return parseJSON<SyncUIStatus>(await fetch('/api/v1/sync-ui', { cache: 'no-store' }));
}

async function syncUIJSON<T>(path: string, method: 'POST' | 'PUT', body: unknown): Promise<T> {
  return parseJSON<T>(await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }));
}

export function initializeSync(): Promise<{ restart_required: boolean; warning: string }> {
  return syncUIJSON('/api/v1/sync-ui/initialize', 'POST', {});
}

export function saveSyncConfiguration(configuration: { target: string; directory?: string; rest_base_url?: string; rest_inbound_enabled?: boolean }): Promise<{ restart_required: boolean; message: string }> {
  return syncUIJSON('/api/v1/sync-ui/configuration', 'PUT', configuration);
}

export function discoverSyncPeers(): Promise<{ peers?: string[]; candidates?: Array<Record<string, unknown>>; scanned_artifacts?: number }> {
  return syncUIJSON('/api/v1/sync-ui/discover', 'POST', {});
}

export function createSyncInvitation(label = ''): Promise<{ code: string; expires_at: string; single_use: boolean }> {
  return syncUIJSON('/api/v1/sync-ui/invitations', 'POST', { label, ttl_minutes: 15 });
}

export function pairSyncPeer(baseURL: string, code: string): Promise<{ paired_with: string }> {
  return syncUIJSON('/api/v1/sync-ui/pair', 'POST', { base_url: baseURL, code });
}

export function previewSyncPeerRetirement(replicaID: string): Promise<SyncRetirementPreview> {
  return syncUIJSON(`/api/v1/sync-ui/peers/${encodeURIComponent(replicaID)}/retirement-preview`, 'POST', {});
}

export function retireSyncPeer(replicaID: string, confirmation: string, reason: string): Promise<{ replica_id: string; status: string; unacknowledged_peers: string[] }> {
  return syncUIJSON(`/api/v1/sync-ui/peers/${encodeURIComponent(replicaID)}/retire`, 'POST', { confirmation, reason });
}

export async function getSyncRetention(): Promise<SyncRetentionReport> {
  return parseJSON<SyncRetentionReport>(await fetch('/api/v1/sync-ui/retention', { cache: 'no-store' }));
}

export function startSync(kind: 'incremental' | 'resource_fetch' = 'incremental'): Promise<Record<string, unknown>> {
  return syncUIJSON('/api/v1/jobs/sync/start', 'POST', { kind });
}

export function requestSyncRecovery(action: 'catchup' | 'reset'): Promise<{ job: SyncUIJob; destructive_review_required: boolean }> {
  return syncUIJSON('/api/v1/sync-ui/recovery', 'POST', { action });
}

export function cancelSyncJob(jobID: string): Promise<SyncUIJob> {
  return syncUIJSON(`/api/v1/jobs/${encodeURIComponent(jobID)}/cancel`, 'POST', {});
}

export function retrySyncJob(jobID: string): Promise<Record<string, unknown>> {
  return syncUIJSON(`/api/v1/jobs/${encodeURIComponent(jobID)}/retry`, 'POST', {});
}

export async function getSyncConflict(conflictID: string): Promise<SyncUIConflictDetail> {
  return parseJSON<SyncUIConflictDetail>(await fetch(`/api/v1/sync-ui/conflicts/${encodeURIComponent(conflictID)}`, { cache: 'no-store' }));
}

export function resolveSyncConflict(conflictID: string, title: string, body: string): Promise<DocumentRecord> {
  return syncUIJSON(`/api/v1/sync-ui/conflicts/${encodeURIComponent(conflictID)}/resolve`, 'POST', { title, body });
}

export function setSyncResourceIntent(resourceID: string, pinned: boolean, requested: boolean): Promise<Record<string, unknown>> {
  return syncUIJSON(`/api/v1/sync-ui/resources/${encodeURIComponent(resourceID)}/intent`, 'POST', { pinned, requested });
}

export async function createPasswordBackup(password: string): Promise<Blob> {
  const response = await fetch('/api/v1/sync-ui/backups', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  });
  if (!response.ok) await parseJSON(response);
  return response.blob();
}

export interface BackupReview {
  verified: boolean;
  database_id: string;
  snapshot_id: string;
  schema_version: number;
  objects: number;
  database_bytes: number;
  external_bytes: number;
  ready_for_destructive_review: boolean;
  applied: false;
  next: string;
}

export async function inspectPasswordBackup(file: File, password: string): Promise<BackupReview> {
  const form = new FormData();
  form.append('password', password);
  form.append('backup', file, file.name);
  return parseJSON<BackupReview>(await fetch('/api/v1/sync-ui/backups/inspect', { method: 'POST', body: form }));
}
