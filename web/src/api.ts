export interface StatusResponse {
  service: string;
  version: string;
  status: string;
  database?: string;
  config_path?: string;
  database_info?: {
    driver?: string;
    path?: string;
    state?: string;
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
}

export interface SearchHit {
  id: string;
  uri: string;
  source: string;
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

export async function createDocument(request: CreateDocumentRequest): Promise<DocumentRecord> {
  const response = await fetch('/api/v1/documents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      collection_id: request.collection_id ?? 'default',
      title: request.title,
      body: request.body,
      body_mime_type: request.body_mime_type ?? 'text/markdown',
      tags: request.tags ?? [],
      metadata: request.metadata ?? {},
    }),
  });
  return parseJSON<DocumentRecord>(response);
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

export async function listTags(): Promise<TagRecord[]> {
  const response = await fetch('/api/v1/tags');
  const payload = await parseJSON<{ tags: TagRecord[] }>(response);
  return payload.tags ?? [];
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

export function resourceContentURL(resourceID: string, download = true): string {
  const suffix = download ? '?download=1' : '';
  return `/api/v1/resources/${encodeURIComponent(resourceID)}/content${suffix}`;
}
