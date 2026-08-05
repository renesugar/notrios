package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/localize"
	"github.com/renesugar/notrios/internal/media"
	"github.com/renesugar/notrios/internal/query"
	"github.com/renesugar/notrios/internal/recoll"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/version"
)

type Server struct {
	mux           *http.ServeMux
	store         store.Store
	config        config.Config
	sidecar       SidecarSearcher
	sidecarStatus SidecarStatusProvider
	mediaPolicy   *media.Policy
	localizer     *localize.Localizer
	searchCache   *mergedSearchCache
}

// SidecarSearcher is the optional derived search backend (Recoll). Implemented
// by *recoll.Sidecar; nil means FTS5-only search.
type SidecarSearcher interface {
	Search(ctx context.Context, q query.Query, limit int) ([]recoll.Hit, error)
}

type SidecarStatusProvider interface {
	Status(ctx context.Context) recoll.RuntimeStatus
}

// AttachSidecar enables merged sidecar search results.
func (s *Server) AttachSidecar(sidecar SidecarSearcher) {
	s.sidecar = sidecar
}

// AttachSidecarStatus exposes configured/unavailable/degraded state even when
// Recoll is not active enough to participate in search.
func (s *Server) AttachSidecarStatus(sidecar SidecarStatusProvider) {
	s.sidecarStatus = sidecar
}

// ServerOptions configures the HTTP API adapter.
type ServerOptions struct {
	Store  store.Store
	Config config.Config
}

func NewServer() *Server {
	return NewServerWithStore(nil)
}

func NewServerWithStore(st store.Store) *Server {
	return NewServerWithOptions(ServerOptions{Store: st, Config: config.Default()})
}

func NewServerWithOptions(options ServerOptions) *Server {
	cfg := options.Config
	if cfg.Server.ListenAddr == "" {
		cfg = config.Default()
	}
	s := &Server{
		mux:         http.NewServeMux(),
		store:       options.Store,
		config:      cfg,
		mediaPolicy: media.NewPolicy(cfg.RemoteMedia),
		searchCache: newMergedSearchCache(),
	}
	if options.Store != nil {
		// Lazy fetcher inside: no filesystem side effects until first use.
		s.localizer = localize.New(cfg.RemoteMedia, options.Store)
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.mux.HandleFunc("GET /mcp", s.handleMCPInfo)
	s.mux.HandleFunc("POST /mcp", s.handleMCP)
	s.mux.HandleFunc("GET /api/v1/collections", s.handleCollections)
	s.mux.HandleFunc("POST /api/v1/collections", s.handleCreateCollection)
	s.mux.HandleFunc("GET /api/v1/collections/{collection_id}", s.handleCollection)
	s.mux.HandleFunc("PATCH /api/v1/collections/{collection_id}", s.handleCollection)

	s.mux.HandleFunc("GET /api/v1/search", s.handleSearchGET)
	s.mux.HandleFunc("POST /api/v1/search", s.handleSearchPOST)

	s.mux.HandleFunc("POST /api/v1/documents", s.handleCreateDocument)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}", s.handleDocument)
	s.mux.HandleFunc("PUT /api/v1/documents/{document_id}", s.handleDocument)
	s.mux.HandleFunc("PATCH /api/v1/documents/{document_id}", s.handleDocument)
	s.mux.HandleFunc("DELETE /api/v1/documents/{document_id}", s.handleDocument)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/body", s.handleDocumentBody)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/revisions", s.handleDocumentRevisions)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/revisions/{revision_id}", s.handleDocumentRevision)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/revisions/{revision_id}/restore", s.handleRestoreRevision)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/resources", s.handleDocumentResources)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/resources/{resource_id}", s.handleDocumentResource)
	s.mux.HandleFunc("DELETE /api/v1/documents/{document_id}/resources/{resource_id}", s.handleDocumentResource)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/links", s.handleDocumentLinks)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/outline", s.handleDocumentOutline)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/append", s.handleAppendDocument)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/prepend", s.handlePrependDocument)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/lines", s.handleDocumentLines)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/search-in", s.handleDocumentSearchIn)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/remote-media/scan", s.handleRemoteMediaScan)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/remote-media/localize", s.handleRemoteMediaLocalize)
	s.mux.HandleFunc("GET /api/v1/media-policy", s.handleMediaPolicy)
	s.mux.HandleFunc("POST /api/v1/media-policy/check-url", s.handleMediaPolicyCheckURL)

	s.mux.HandleFunc("POST /api/v1/resources", s.handleCreateResource)
	s.mux.HandleFunc("GET /api/v1/resources/reports/reference", s.handleResourceReport)
	s.mux.HandleFunc("HEAD /api/v1/resources/{resource_id}", s.handleResourceHead)
	s.mux.HandleFunc("GET /api/v1/resources/{resource_id}", s.handleResource)
	s.mux.HandleFunc("DELETE /api/v1/resources/{resource_id}", s.handleResource)
	s.mux.HandleFunc("GET /api/v1/resources/{resource_id}/content", s.handleResourceContent)
	s.mux.HandleFunc("GET /api/v1/admin/gc/report", s.handleGarbageCollectionReport)

	s.mux.HandleFunc("GET /api/v1/notebooks", s.handleListNotebooks)
	s.mux.HandleFunc("GET /api/v1/notebooks/tree", s.handleNotebookTree)
	s.mux.HandleFunc("POST /api/v1/notebooks", s.handleCreateNotebook)
	s.mux.HandleFunc("GET /api/v1/notebooks/{notebook_id}", s.handleNotebook)
	s.mux.HandleFunc("PATCH /api/v1/notebooks/{notebook_id}", s.handleNotebook)
	s.mux.HandleFunc("DELETE /api/v1/notebooks/{notebook_id}", s.handleNotebook)
	s.mux.HandleFunc("GET /api/v1/notebooks/{notebook_id}/notes", s.handleNotebookNotes)
	s.mux.HandleFunc("GET /api/v1/tags", s.handleListTags)
	s.mux.HandleFunc("GET /api/v1/documents/{document_id}/tags", s.handleDocumentTags)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/tags/{tag}", s.handleDocumentTag)
	s.mux.HandleFunc("DELETE /api/v1/documents/{document_id}/tags/{tag}", s.handleDocumentTag)
	s.mux.HandleFunc("POST /api/v1/documents/{document_id}/notebook", s.handleMoveDocumentNotebook)
	s.mux.HandleFunc("GET /api/v1/search-notebooks", s.handleListSearchNotebooks)
	s.mux.HandleFunc("POST /api/v1/search-notebooks", s.handleCreateSearchNotebook)
	s.mux.HandleFunc("DELETE /api/v1/search-notebooks/{search_notebook_id}", s.handleDeleteSearchNotebook)
	s.mux.HandleFunc("GET /api/v1/trash", s.handleListTrash)
	s.mux.HandleFunc("POST /api/v1/trash/{document_id}/restore", s.handleRestoreTrashedDocument)
	s.mux.HandleFunc("DELETE /api/v1/trash/{document_id}", s.handlePurgeDocument)

	s.mux.HandleFunc("POST /api/v1/graph", s.handleGraph)
	s.mux.HandleFunc("POST /api/v1/selection/plan", s.handleSelectionPlan)
	s.mux.HandleFunc("POST /api/v1/links/resolve", s.handleResolveStableLink)
	s.mux.HandleFunc("GET /api/v1/jobs/{job_id}", s.handleJob)
	s.mux.HandleFunc("GET /", s.handleWebApp)
}

func (s *Server) handleWebApp(w http.ResponseWriter, r *http.Request) {
	distDir := filepath.Clean("web/dist")
	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		writeError(w, http.StatusNotFound, "web_ui_not_built", "web/dist/index.html not found; run cd web && npm run build or use npm run dev")
		return
	}

	requested := strings.TrimPrefix(r.URL.Path, "/")
	if requested == "" {
		requested = "index.html"
	}
	cleanRequested := filepath.Clean(requested)
	if cleanRequested == "." || strings.HasPrefix(cleanRequested, "..") || filepath.IsAbs(cleanRequested) {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid web asset path")
		return
	}

	assetPath := filepath.Join(distDir, cleanRequested)
	if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, assetPath)
		return
	}
	http.ServeFile(w, r, indexPath)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := "scaffold"
	database := "not-wired"
	databaseInfo := api.DatabaseStatus{Driver: "none", State: "not-wired"}
	if s.store != nil {
		storeStatus, err := s.store.Status(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "status_failed", err.Error())
			return
		}
		status = "running"
		database = storeStatus.Driver
		databaseInfo = api.DatabaseStatus{
			Driver:        storeStatus.Driver,
			Path:          storeStatus.Path,
			State:         storeStatus.State,
			SchemaVersion: storeStatus.SchemaVersion,
		}
		// The logical database ID is what a client needs to build a stable
		// notrios:// link for a note it already holds. The replica ID is
		// deliberately not reported: it identifies this writable copy and has
		// no meaning in a shared link.
		if identity, err := s.store.GetDatabaseIdentity(r.Context()); err == nil {
			databaseInfo.DatabaseID = identity.DatabaseID
		}
	}
	sidecarStatus := api.SearchSidecarStatus{
		Configured: s.config.SearchSidecar.Enabled,
		State:      "disabled",
	}
	if s.config.SearchSidecar.Enabled {
		sidecarStatus.State = "configured"
	}
	if s.sidecarStatus != nil {
		sidecarStatus = toAPISidecarStatus(s.sidecarStatus.Status(r.Context()))
	}
	writeJSON(w, http.StatusOK, api.StatusResponse{
		Service:      "notrios",
		Version:      version.Version,
		Status:       status,
		Database:     database,
		ConfigPath:   s.config.ConfigPath,
		DatabaseInfo: databaseInfo,
		Storage: api.StorageStatus{
			DataDirectory:         s.config.Data.Directory,
			DatabasePath:          s.config.Data.DatabasePath,
			AssetStore:            s.config.Data.AssetStore,
			ProjectionDir:         s.config.Data.ProjectionDir,
			SearchSidecarIndexDir: s.config.SearchSidecar.IndexDir,
		},
		Capabilities: map[string]bool{
			"documents.create":      true,
			"documents.read":        true,
			"documents.update":      s.store != nil,
			"documents.delete":      s.store != nil,
			"search.fts5":           s.store != nil,
			"search.boolean":        s.store != nil,
			"search.category_alias": s.store != nil,
			"selection.plan":        s.store != nil,
			"resources":             s.store != nil,
			"resources.report":      s.store != nil,
			"resources.gc_report":   s.store != nil,
			"links":                 s.store != nil,
			"mcp":                   s.store != nil && s.config.MCP.Enabled,
			"search_sidecar":        s.config.SearchSidecar.Enabled,
		},
		Limits: map[string]int{
			"search_default_limit":                 s.config.Search.DefaultLimit,
			"search_max_limit":                     s.config.Search.MaxLimit,
			"search_cursor_version":                2,
			"search_merged_snapshot_limit":         mergedSearchWindow,
			"search_query_max_bytes":               query.MaxInputBytes,
			"search_query_max_tokens":              query.MaxTokens,
			"search_query_max_depth":               query.MaxDepth,
			"selection_max_selectors":              store.MaxSelectionSelectors,
			"selection_max_explicit_document_ids":  store.MaxSelectionDocumentIDs,
			"selection_rest_max_documents":         restSelectionMaxDocuments,
			"selection_max_detail_items":           store.MaxSelectionDetailItems,
			"retention_unreferenced_resource_days": s.config.Retention.UnreferencedResourceDays,
			"retention_purged_resource_days":       s.config.Retention.PurgedResourceDays,
		},
		MediaPolicy:   s.mediaPolicyStatus(),
		SearchSidecar: sidecarStatus,
	})
}

func toAPISidecarStatus(status recoll.RuntimeStatus) api.SearchSidecarStatus {
	out := api.SearchSidecarStatus{
		Configured: status.Configured, Available: status.Available, Active: status.Active,
		State: status.State, Backlog: status.Backlog, FailedJobs: status.FailedJobs,
		LastError: status.LastError,
	}
	if !status.LastSyncAt.IsZero() {
		out.LastSyncAt = status.LastSyncAt.Format(time.RFC3339)
	}
	if !status.LastIndexAt.IsZero() {
		out.LastIndexAt = status.LastIndexAt.Format(time.RFC3339)
	}
	if !status.LastReconciliationAt.IsZero() {
		out.LastReconciliationAt = status.LastReconciliationAt.Format(time.RFC3339)
		out.Reconciliation = &api.SearchReconciliationStatus{
			Complete: status.Reconciliation.Complete, Canonical: status.Reconciliation.Canonical,
			Scanned: status.Reconciliation.Scanned, Missing: status.Reconciliation.Missing,
			Stale: status.Reconciliation.Stale, Orphaned: status.Reconciliation.Orphaned,
			Repaired: status.Reconciliation.Repaired, Failed: status.Reconciliation.Failed,
		}
	}
	return out
}

func (s *Server) handleCollections(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.CollectionPage{Collections: []api.Collection{defaultCollection()}})
		return
	}
	collections, err := s.store.ListCollections(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "collection_list_failed", err.Error())
		return
	}
	out := make([]api.Collection, 0, len(collections))
	for _, collection := range collections {
		out = append(out, api.Collection{
			ID:           collection.ID,
			Name:         collection.Name,
			Kind:         "managed",
			Description:  collection.Description,
			Capabilities: collection.Capabilities,
		})
	}
	writeJSON(w, http.StatusOK, api.CollectionPage{Collections: out})
}

func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Kind        string `json:"kind"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "id and name are required")
		return
	}
	if req.Kind == "" {
		req.Kind = "managed"
	}
	writeJSON(w, http.StatusCreated, api.Collection{ID: req.ID, Name: req.Name, Kind: req.Kind, Description: req.Description})
}

func (s *Server) handleCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("collection_id")
	if id == "default" {
		writeJSON(w, http.StatusOK, defaultCollection())
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "collection is not available in scaffold server")
}

func (s *Server) handleSearchGET(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseLimit(w, r, 25, 100)
	if !ok {
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.SearchResponse{Hits: []api.SearchHit{}})
		return
	}
	result, err := s.searchMerged(r.Context(), store.SearchRequest{
		CollectionID: r.URL.Query().Get("collection"),
		Query:        r.URL.Query().Get("q"),
		Limit:        limit,
		Cursor:       r.URL.Query().Get("cursor"),
	})
	if writeStoreError(w, err, "search_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPISearchResponse(result))
}

func (s *Server) handleSearchPOST(w http.ResponseWriter, r *http.Request) {
	var req api.SearchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		writeError(w, http.StatusBadRequest, "limit_too_large", "limit must be 100 or less")
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.SearchResponse{Hits: []api.SearchHit{}})
		return
	}
	collectionID := firstCollection(req)
	result, err := s.searchMerged(r.Context(), store.SearchRequest{
		CollectionID: collectionID,
		Query:        req.Query,
		Limit:        req.Limit,
		Cursor:       req.Cursor,
	})
	if writeStoreError(w, err, "search_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPISearchResponse(result))
}

func (s *Server) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	var req api.DocumentMutationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "title is required")
		return
	}
	if s.store == nil {
		if strings.TrimSpace(req.CollectionID) == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "collection_id is required in scaffold mode")
			return
		}
		writeJSON(w, http.StatusCreated, api.Document{
			ID:                "doc_scaffold",
			URI:               "document://" + req.CollectionID + "/documents/doc_scaffold",
			CollectionID:      req.CollectionID,
			Title:             req.Title,
			BodyMIMEType:      defaultString(req.BodyMIMEType, "text/markdown"),
			Body:              req.Body,
			CurrentRevisionID: "rev_scaffold",
			Metadata:          req.Metadata,
			Tags:              req.Tags,
		})
		return
	}
	doc, err := s.store.CreateDocument(r.Context(), store.CreateDocumentRequest{
		CollectionID: req.CollectionID,
		NotebookID:   req.NotebookID,
		Title:        req.Title,
		Body:         req.Body,
		BodyMIMEType: req.BodyMIMEType,
		Message:      req.Message,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "document_create_failed", err.Error())
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusCreated, toAPIDocument(doc))
}

func (s *Server) handleDocument(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	switch r.Method {
	case http.MethodGet:
		s.handleGetDocument(w, r, docID)
	case http.MethodPut:
		s.handlePutDocument(w, r, docID)
	case http.MethodPatch:
		s.handlePatchDocument(w, r, docID)
	case http.MethodDelete:
		s.handleDeleteDocument(w, r, docID)
	}
}

func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request, docID string) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, placeholderDocument(docID))
		return
	}
	doc, err := s.store.GetDocument(r.Context(), docID)
	if writeStoreError(w, err, "document_read_failed") {
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusOK, toAPIDocument(doc))
}

func (s *Server) handlePutDocument(w http.ResponseWriter, r *http.Request, docID string) {
	if s.store != nil && s.guardHelpNote(w, r, docID) {
		return
	}
	var req api.DocumentMutationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	baseRevisionID := firstNonEmpty(req.BaseRevisionID, revisionFromIfMatch(r.Header.Get("If-Match")))
	if strings.TrimSpace(baseRevisionID) == "" {
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "base_revision_id or If-Match is required")
		return
	}
	if s.store == nil {
		doc := placeholderDocument(docID)
		doc.Title = defaultString(req.Title, doc.Title)
		doc.Body = req.Body
		writeJSON(w, http.StatusOK, doc)
		return
	}
	doc, err := s.store.UpdateDocument(r.Context(), store.UpdateDocumentRequest{
		ID:             docID,
		Title:          req.Title,
		Body:           req.Body,
		BodyMIMEType:   req.BodyMIMEType,
		BaseRevisionID: baseRevisionID,
		Message:        req.Message,
	})
	if writeStoreError(w, err, "document_update_failed") {
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusOK, toAPIDocument(doc))
}

func (s *Server) handlePatchDocument(w http.ResponseWriter, r *http.Request, docID string) {
	if s.store != nil && s.guardHelpNote(w, r, docID) {
		return
	}
	var req api.DocumentPatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	baseRevisionID := firstNonEmpty(req.BaseRevisionID, revisionFromIfMatch(r.Header.Get("If-Match")))
	if strings.TrimSpace(baseRevisionID) == "" {
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "base_revision_id or If-Match is required")
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, placeholderDocument(docID))
		return
	}
	current, err := s.store.GetDocument(r.Context(), docID)
	if writeStoreError(w, err, "document_read_failed") {
		return
	}
	title := current.Title
	if strings.TrimSpace(req.Title) != "" {
		title = req.Title
	}
	body, err := applySurgicalEdits(current.Body, req.Edits)
	if err != nil {
		writeError(w, http.StatusBadRequest, "edit_failed", err.Error())
		return
	}
	if req.DryRun {
		current.Title = title
		current.Body = body
		writeJSON(w, http.StatusOK, toAPIDocument(current))
		return
	}
	doc, err := s.store.UpdateDocument(r.Context(), store.UpdateDocumentRequest{
		ID:             docID,
		Title:          title,
		Body:           body,
		BodyMIMEType:   current.BodyMIMEType,
		BaseRevisionID: baseRevisionID,
		Message:        "patch",
	})
	if writeStoreError(w, err, "document_patch_failed") {
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusOK, toAPIDocument(doc))
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request, docID string) {
	if s.store != nil && s.guardHelpNote(w, r, docID) {
		return
	}
	baseRevisionID := firstNonEmpty(r.URL.Query().Get("base_revision_id"), revisionFromIfMatch(r.Header.Get("If-Match")))
	if strings.TrimSpace(baseRevisionID) == "" {
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "base_revision_id query parameter or If-Match is required")
		return
	}
	if s.store == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	err := s.store.DeleteDocument(r.Context(), store.DeleteDocumentRequest{ID: docID, BaseRevisionID: baseRevisionID})
	if writeStoreError(w, err, "document_delete_failed") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDocumentBody(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	if s.store == nil {
		_, _ = w.Write([]byte("# Scaffold document\n\nPersistence is not wired yet.\n"))
		return
	}
	doc, err := s.store.GetDocument(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "document_read_failed") {
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	_, _ = w.Write([]byte(doc.Body))
}

func (s *Server) handleDocumentRevisions(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.RevisionPage{Revisions: []api.DocumentRevision{}})
		return
	}
	revisions, err := s.store.ListDocumentRevisions(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "revision_list_failed") {
		return
	}
	out := make([]api.DocumentRevision, 0, len(revisions))
	for _, revision := range revisions {
		apiRevision := toAPIRevision(revision)
		apiRevision.Body = ""
		out = append(out, apiRevision)
	}
	writeJSON(w, http.StatusOK, api.RevisionPage{Revisions: out})
}

func (s *Server) handleDocumentRevision(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.DocumentRevision{ID: r.PathValue("revision_id"), DocumentID: r.PathValue("document_id"), Title: "Scaffold revision"})
		return
	}
	revision, err := s.store.GetDocumentRevision(r.Context(), r.PathValue("document_id"), r.PathValue("revision_id"))
	if writeStoreError(w, err, "revision_read_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIRevision(revision))
}

func (s *Server) handleRestoreRevision(w http.ResponseWriter, r *http.Request) {
	var req api.RestoreRevisionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	baseRevisionID := firstNonEmpty(req.BaseRevisionID, revisionFromIfMatch(r.Header.Get("If-Match")))
	if strings.TrimSpace(baseRevisionID) == "" {
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "base_revision_id or If-Match is required")
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, placeholderDocument(r.PathValue("document_id")))
		return
	}
	doc, err := s.store.RestoreDocumentRevision(r.Context(), store.RestoreRevisionRequest{
		DocumentID:     r.PathValue("document_id"),
		RevisionID:     r.PathValue("revision_id"),
		BaseRevisionID: baseRevisionID,
		Message:        req.Message,
	})
	if writeStoreError(w, err, "revision_restore_failed") {
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusOK, toAPIDocument(doc))
}

func (s *Server) handleDocumentResources(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.ResourceReferencePage{Resources: []api.ResourceReference{}})
		return
	}
	refs, err := s.store.ListDocumentResources(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "resource_ref_list_failed") {
		return
	}
	out := make([]api.ResourceReference, 0, len(refs))
	for _, ref := range refs {
		out = append(out, toAPIResourceReference(ref))
	}
	writeJSON(w, http.StatusOK, api.ResourceReferencePage{Resources: out})
}

func (s *Server) handleDocumentResource(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	resourceID := r.PathValue("resource_id")
	if r.Method == http.MethodDelete {
		if s.store != nil {
			if err := s.store.DetachDocumentResource(r.Context(), docID, resourceID); writeStoreError(w, err, "resource_detach_failed") {
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req api.AttachResourceRequest
	if !decodeOptionalJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.ResourceReference{
			DocumentID:   docID,
			ResourceID:   resourceID,
			RelationType: defaultString(req.RelationType, "attachment"),
		})
		return
	}
	anchorJSON := "{}"
	if len(req.Anchor) > 0 {
		encoded, err := json.Marshal(req.Anchor)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_anchor", "anchor must be JSON-serializable")
			return
		}
		anchorJSON = string(encoded)
	}
	ref, err := s.store.AttachDocumentResource(r.Context(), store.AttachResourceRequest{
		DocumentID:   docID,
		ResourceID:   resourceID,
		RelationType: req.RelationType,
		Ordinal:      req.Ordinal,
		AnchorJSON:   anchorJSON,
	})
	if writeStoreError(w, err, "resource_attach_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIResourceReference(ref))
}

func (s *Server) handleDocumentLinks(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.DocumentLinkPage{Outgoing: []api.DocumentLink{}, Incoming: []api.DocumentLink{}})
		return
	}
	page, err := s.store.ListDocumentLinks(r.Context(), r.PathValue("document_id"), r.URL.Query().Get("direction"))
	if writeStoreError(w, err, "link_list_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPILinkPage(page))
}

func (s *Server) handleDocumentOutline(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.DocumentOutline{DocumentID: docID, Headings: []api.DocumentHeading{}})
		return
	}
	doc, err := s.store.GetDocument(r.Context(), docID)
	if writeStoreError(w, err, "document_read_failed") {
		return
	}
	writeJSON(w, http.StatusOK, extractDocumentOutline(doc.ID, doc.Body))
}

// handleRemoteMediaScan reports the policy decision for every remote-media
// URL in a note without downloading anything (v0.3 task H2). An optional
// request body with "urls" evaluates an explicit URL list instead of the
// stored document body — e.g. for unsaved editor drafts.
func (s *Server) handleRemoteMediaScan(w http.ResponseWriter, r *http.Request) {
	var request api.RemoteMediaRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}
	if len(request.URLs) > 0 {
		writeJSON(w, http.StatusOK, scanResultFromDecisions("", s.mediaPolicy.EvaluateURLs(request.URLs)))
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "persistence is not configured")
		return
	}
	documentID := r.PathValue("document_id")
	doc, err := s.store.GetDocument(r.Context(), documentID)
	if writeStoreError(w, err, "document_read_failed") {
		return
	}
	writeJSON(w, http.StatusOK, scanResultFromDecisions(doc.ID, s.mediaPolicy.ScanBody(doc.Body)))
}

// handleMediaPolicy reports the active remote-media policy (same shape as
// the status media_policy block).
func (s *Server) handleMediaPolicy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mediaPolicyStatus())
}

func (s *Server) mediaPolicyStatus() *api.MediaPolicyStatus {
	return &api.MediaPolicyStatus{
		DefaultAction:        s.config.RemoteMedia.DefaultAction,
		AllowPrivateNetworks: s.config.RemoteMedia.AllowPrivateNetworks,
		MaxRedirects:         s.config.RemoteMedia.MaxRedirects,
		FetchTimeoutSeconds:  s.config.RemoteMedia.FetchTimeoutSeconds,
		BlockedSchemes:       len(s.config.RemoteMedia.BlockedSchemes),
		AllowedDomains:       len(s.config.RemoteMedia.AllowedDomains),
		BlockedDomains:       len(s.config.RemoteMedia.BlockedDomains),
		ReviewDomains:        len(s.config.RemoteMedia.ReviewDomains),
		MaxBytes:             s.config.RemoteMedia.MaxBytes,
		QuarantineDir:        s.config.RemoteMedia.QuarantineDir,
	}
}

// handleMediaPolicyCheckURL evaluates explicit URLs against the policy
// without touching any document (and without downloading anything).
func (s *Server) handleMediaPolicyCheckURL(w http.ResponseWriter, r *http.Request) {
	var request api.RemoteMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if len(request.URLs) == 0 {
		writeError(w, http.StatusBadRequest, "missing_urls", "provide at least one URL in \"urls\"")
		return
	}
	writeJSON(w, http.StatusOK, scanResultFromDecisions("", s.mediaPolicy.EvaluateURLs(request.URLs)))
}

func scanResultFromDecisions(documentID string, decisions []media.Decision) api.RemoteMediaScanResult {
	result := api.RemoteMediaScanResult{
		DocumentID: documentID,
		Media:      make([]api.RemoteMediaDecision, 0, len(decisions)),
		Counts:     map[string]int{"allow": 0, "block": 0, "review": 0},
	}
	for _, decision := range decisions {
		result.Media = append(result.Media, api.RemoteMediaDecision{
			URL:        decision.URL,
			MediaClass: decision.MediaClass,
			Action:     decision.Action,
			Reason:     decision.Reason,
			Line:       decision.Line,
		})
		result.Counts[decision.Action]++
	}
	return result
}

// handleRemoteMediaLocalize converts a note's remote media into local
// resources through the shared localize engine (v0.3 task H4): quarantine
// fetch → exact-hash rule check → content-addressed admission → Markdown
// rewrite in a new revision. Dry runs never fetch or write.
func (s *Server) handleRemoteMediaLocalize(w http.ResponseWriter, r *http.Request) {
	if s.localizer == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "persistence is not configured")
		return
	}
	var request api.RemoteMediaRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}
	baseRevisionID := firstNonEmpty(request.BaseRevisionID, revisionFromIfMatch(r.Header.Get("If-Match")))
	if baseRevisionID == "" && !request.DryRun {
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "base_revision_id or If-Match is required")
		return
	}
	result, err := s.localizer.LocalizeDocument(r.Context(), localize.Options{
		DocumentID:     r.PathValue("document_id"),
		BaseRevisionID: baseRevisionID,
		DryRun:         request.DryRun,
		AllowReview:    request.AllowReview,
	})
	if errors.Is(err, localize.ErrReadOnly) {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if writeStoreError(w, err, "localize_failed") {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateResource(w http.ResponseWriter, r *http.Request) {
	filename := firstNonEmpty(r.URL.Query().Get("filename"), filenameFromContentDisposition(r.Header.Get("Content-Disposition")))
	mimeType := firstNonEmpty(r.Header.Get("Content-Type"), store.MIMETypeFromFilename(filename), "application/octet-stream")
	collectionID := firstNonEmpty(r.URL.Query().Get("collection_id"), "default")
	if s.store == nil {
		writeJSON(w, http.StatusCreated, api.Resource{ID: "res_scaffold", URI: "resource://default/resources/res_scaffold", CollectionID: collectionID, Filename: filename, MIMEType: mimeType})
		return
	}
	res, err := s.store.CreateResource(r.Context(), store.CreateResourceRequest{
		CollectionID: collectionID,
		Filename:     filename,
		MIMEType:     mimeType,
		Content:      r.Body,
	})
	if writeStoreError(w, err, "resource_create_failed") {
		return
	}
	writeJSON(w, http.StatusCreated, toAPIResource(res))
}

func (s *Server) handleResourceReport(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.ResourceReport{
			ExactDuplicates:   []api.ExactDuplicateGroup{},
			UnreferencedBlobs: []api.UnreferencedBlob{},
			NotebookUsage:     []api.NotebookResourceUsage{},
			Perceptual: api.PerceptualHashReport{
				PolicyReviews:  []api.PerceptualPolicyReview{},
				NearDuplicates: []api.NearDuplicateReview{},
			},
		})
		return
	}
	report, err := s.store.ResourceReport(r.Context())
	if writeStoreError(w, err, "resource_report_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIResourceReport(report))
}

func (s *Server) handleResourceHead(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		return
	}
	res, err := s.store.GetResource(r.Context(), r.PathValue("resource_id"))
	if writeStoreError(w, err, "resource_read_failed") {
		return
	}
	setResourceHeaders(w, res, false)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleResource(w http.ResponseWriter, r *http.Request) {
	resourceID := r.PathValue("resource_id")
	if r.Method == http.MethodDelete {
		if !requireConfirmation(w, r, "delete-resource:"+resourceID) {
			return
		}
		if s.store != nil {
			if err := s.store.DeleteResource(r.Context(), resourceID); writeStoreError(w, err, "resource_delete_failed") {
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.Resource{ID: resourceID, URI: "resource://default/resources/" + resourceID, CollectionID: "default", MIMEType: "application/octet-stream"})
		return
	}
	res, err := s.store.GetResource(r.Context(), resourceID)
	if writeStoreError(w, err, "resource_read_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIResource(res))
}

func (s *Server) handleGarbageCollectionReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	report, err := s.store.GarbageCollect(r.Context(), store.GarbageCollectionRequest{
		Policy: store.GarbageCollectionPolicy{
			UnreferencedFor:   s.config.Retention.UnreferencedDuration(),
			PurgedResourceFor: s.config.Retention.PurgedResourceDuration(),
		},
	})
	if writeStoreError(w, err, "gc_report_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIGarbageCollectionReport(report))
}

func (s *Server) handleResourceContent(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("scaffold resource content\n"))
		return
	}
	res, content, err := s.store.OpenResourceContent(r.Context(), r.PathValue("resource_id"))
	if writeStoreError(w, err, "resource_content_failed") {
		return
	}
	defer content.Close()
	setResourceHeaders(w, res, wantsDownload(r))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, content)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	var req api.GraphRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.GraphResponse{Nodes: []map[string]any{}, Edges: []map[string]any{}})
		return
	}
	graph, err := s.store.Graph(r.Context(), store.GraphRequest{
		Roots:            req.Roots,
		Direction:        req.Direction,
		Depth:            req.Depth,
		IncludeResources: req.IncludeResources,
		MaxNodes:         req.MaxNodes,
		MaxEdges:         req.MaxEdges,
	})
	if writeStoreError(w, err, "graph_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIGraph(graph))
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.JobStatus{ID: r.PathValue("job_id"), Kind: "scaffold", Status: "unknown"})
}

func defaultCollection() api.Collection {
	return api.Collection{
		ID:           "default",
		Name:         "Default",
		Kind:         "managed",
		Description:  "Placeholder collection for scaffold validation.",
		Capabilities: []string{"documents", "search", "resources", "links"},
	}
}

func placeholderDocument(id string) api.Document {
	return api.Document{
		ID:                id,
		URI:               "document://default/documents/" + id,
		CollectionID:      "default",
		Title:             "Scaffold document",
		BodyMIMEType:      "text/markdown",
		Body:              "# Scaffold document\n\nPersistence is not wired yet.\n",
		CurrentRevisionID: "rev_scaffold",
	}
}

func emptyRemoteMediaResult() api.RemoteMediaResult {
	return api.RemoteMediaResult{
		Localized: []map[string]any{},
		Blocked:   []map[string]any{},
		Review:    []map[string]any{},
		Failed:    []map[string]any{},
	}
}

func parseLimit(w http.ResponseWriter, r *http.Request, def, max int) (int, bool) {
	limit := def
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
			return 0, false
		}
		limit = parsed
	}
	if limit > max {
		writeError(w, http.StatusBadRequest, "limit_too_large", "limit is too large")
		return 0, false
	}
	return limit, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	err := json.NewDecoder(r.Body).Decode(v)
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
	return false
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstCollection(req api.SearchRequest) string {
	if len(req.Collections) > 0 && strings.TrimSpace(req.Collections[0]) != "" {
		return req.Collections[0]
	}
	return req.Collection
}

func toAPIDocument(doc store.Document) api.Document {
	deletedAt := ""
	if !doc.DeletedAt.IsZero() {
		deletedAt = doc.DeletedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return api.Document{
		ID:                doc.ID,
		URI:               doc.URI,
		CollectionID:      doc.CollectionID,
		NotebookID:        doc.NotebookID,
		Editable:          doc.NotebookID != store.HelpNotebookID && doc.DeletedAt.IsZero(),
		Title:             doc.Title,
		BodyMIMEType:      doc.BodyMIMEType,
		Body:              doc.Body,
		CurrentRevisionID: doc.CurrentRevisionID,
		CreatedAt:         doc.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         doc.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		DeletedAt:         deletedAt,
	}
}

func toAPISearchResponse(result store.SearchResponse) api.SearchResponse {
	hits := make([]api.SearchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, api.SearchHit{
			ID:           hit.ID,
			URI:          hit.URI,
			Source:       "managed-notes",
			Sources:      append([]string(nil), hit.SearchSources...),
			CollectionID: hit.CollectionID,
			Title:        hit.Title,
			Snippet:      hit.Snippet,
			Score:        hit.Score,
			Editable:     hit.NotebookID != store.HelpNotebookID,
		})
	}
	return api.SearchResponse{Hits: hits, NextCursor: result.NextCursor, Truncated: result.Truncated}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, api.ErrorEnvelope{Error: api.APIError{Code: code, Message: message}})
}

func toAPIRevision(revision store.DocumentRevision) api.DocumentRevision {
	return api.DocumentRevision{
		ID:           revision.ID,
		DocumentID:   revision.DocumentID,
		Title:        revision.Title,
		Body:         revision.Body,
		BodyMIMEType: revision.BodyMIMEType,
		Message:      revision.Message,
		CreatedAt:    revision.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toAPIResource(resource store.Resource) api.Resource {
	return api.Resource{
		ID:           resource.ID,
		URI:          resource.URI,
		CollectionID: resource.CollectionID,
		Filename:     resource.Filename,
		MIMEType:     resource.MIMEType,
		SizeBytes:    resource.SizeBytes,
		SHA256:       resource.SHA256,
		CreatedAt:    resource.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toAPIResourceReference(ref store.ResourceReference) api.ResourceReference {
	resource := toAPIResource(ref.Resource)
	anchor := map[string]any{}
	if strings.TrimSpace(ref.AnchorJSON) != "" {
		_ = json.Unmarshal([]byte(ref.AnchorJSON), &anchor)
	}
	return api.ResourceReference{
		DocumentID:   ref.DocumentID,
		ResourceID:   ref.ResourceID,
		Resource:     &resource,
		RelationType: ref.RelationType,
		Ordinal:      ref.Ordinal,
		Anchor:       anchor,
	}
}

func toAPIResourceReport(report store.ResourceReport) api.ResourceReport {
	out := api.ResourceReport{
		ExactDuplicates:   make([]api.ExactDuplicateGroup, 0, len(report.ExactDuplicates)),
		UnreferencedBlobs: make([]api.UnreferencedBlob, 0, len(report.UnreferencedBlobs)),
		NotebookUsage:     make([]api.NotebookResourceUsage, 0, len(report.NotebookUsage)),
		Perceptual: api.PerceptualHashReport{
			HookEnabled:    report.Perceptual.HookEnabled,
			Algorithm:      report.Perceptual.Algorithm,
			StoredHashes:   report.Perceptual.StoredHashes,
			PolicyReviews:  make([]api.PerceptualPolicyReview, 0, len(report.Perceptual.PolicyReviews)),
			NearDuplicates: make([]api.NearDuplicateReview, 0, len(report.Perceptual.NearDuplicates)),
		},
	}
	for _, group := range report.ExactDuplicates {
		converted := api.ExactDuplicateGroup{
			SHA256:          group.SHA256,
			MIMEType:        group.MIMEType,
			SizeBytes:       group.SizeBytes,
			ResourceCount:   group.ResourceCount,
			ReferenceCount:  group.ReferenceCount,
			CollectionIDs:   append([]string(nil), group.CollectionIDs...),
			CrossCollection: group.CrossCollection,
			Resources:       make([]api.ResourceReportItem, 0, len(group.Resources)),
		}
		for _, item := range group.Resources {
			converted.Resources = append(converted.Resources, api.ResourceReportItem{
				Resource:       toAPIResource(item.Resource),
				ReferenceCount: item.ReferenceCount,
			})
		}
		out.ExactDuplicates = append(out.ExactDuplicates, converted)
	}
	for _, blob := range report.UnreferencedBlobs {
		converted := api.UnreferencedBlob{
			SHA256:    blob.SHA256,
			MIMEType:  blob.MIMEType,
			SizeBytes: blob.SizeBytes,
			Resources: make([]api.Resource, 0, len(blob.Resources)),
		}
		for _, resource := range blob.Resources {
			converted.Resources = append(converted.Resources, toAPIResource(resource))
		}
		out.UnreferencedBlobs = append(out.UnreferencedBlobs, converted)
	}
	for _, usage := range report.NotebookUsage {
		out.NotebookUsage = append(out.NotebookUsage, api.NotebookResourceUsage{
			NotebookID:      usage.NotebookID,
			NotebookName:    usage.NotebookName,
			DocumentCount:   usage.DocumentCount,
			ReferenceCount:  usage.ReferenceCount,
			ResourceCount:   usage.ResourceCount,
			UniqueBlobCount: usage.UniqueBlobCount,
			ReferencedBytes: usage.ReferencedBytes,
			UniqueBytes:     usage.UniqueBytes,
		})
	}
	for _, review := range report.Perceptual.PolicyReviews {
		out.Perceptual.PolicyReviews = append(out.Perceptual.PolicyReviews, api.PerceptualPolicyReview{
			Algorithm:   review.Algorithm,
			Hash:        review.Hash,
			BlobSHA256:  review.BlobSHA256,
			ResourceIDs: append([]string(nil), review.ResourceIDs...),
			Reason:      review.Reason,
		})
	}
	for _, review := range report.Perceptual.NearDuplicates {
		out.Perceptual.NearDuplicates = append(out.Perceptual.NearDuplicates, api.NearDuplicateReview{
			Algorithm:        review.Algorithm,
			LeftBlobSHA256:   review.LeftBlobSHA256,
			RightBlobSHA256:  review.RightBlobSHA256,
			LeftResourceIDs:  append([]string(nil), review.LeftResourceIDs...),
			RightResourceIDs: append([]string(nil), review.RightResourceIDs...),
			Distance:         review.Distance,
			Reason:           review.Reason,
		})
	}
	return out
}

func toAPIGarbageCollectionReport(report store.GarbageCollectionReport) api.GarbageCollectionReport {
	out := api.GarbageCollectionReport{
		DryRun: report.DryRun,
		AsOf:   report.AsOf.Format("2006-01-02T15:04:05Z07:00"),
		Policy: api.GarbageCollectionPolicy{
			UnreferencedSeconds:   report.Policy.UnreferencedSeconds,
			PurgedResourceSeconds: report.Policy.PurgedResourceSeconds,
			Gate:                  report.Policy.Gate,
		},
		Eligible:                toAPIGarbageCollectionCandidates(report.Eligible),
		Retained:                toAPIGarbageCollectionCandidates(report.Retained),
		Removed:                 toAPIGarbageCollectionCandidates(report.Removed),
		ReferencedResourceCount: report.ReferencedResourceCount,
		BlobsRemoved:            report.BlobsRemoved,
		BytesRemoved:            report.BytesRemoved,
		Warnings:                append([]string(nil), report.Warnings...),
	}
	return out
}

func toAPIGarbageCollectionCandidates(candidates []store.GarbageCollectionCandidate) []api.GarbageCollectionCandidate {
	out := make([]api.GarbageCollectionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		eligibleAt := ""
		if !candidate.EligibleAt.IsZero() {
			eligibleAt = candidate.EligibleAt.Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, api.GarbageCollectionCandidate{
			Resource:           toAPIResource(candidate.Resource),
			UnreferencedAt:     candidate.UnreferencedAt.Format("2006-01-02T15:04:05Z07:00"),
			UnreferencedReason: candidate.UnreferencedReason,
			RetentionSeconds:   candidate.RetentionSeconds,
			EligibleAt:         eligibleAt,
			Decision:           candidate.Decision,
		})
	}
	return out
}

func requireConfirmation(w http.ResponseWriter, r *http.Request, expected string) bool {
	if strings.TrimSpace(r.Header.Get("X-Notrios-Confirmation")) == expected {
		return true
	}
	writeError(w, http.StatusPreconditionRequired, "confirmation_required",
		"set X-Notrios-Confirmation to "+expected)
	return false
}

func toAPILinkPage(page store.DocumentLinkPage) api.DocumentLinkPage {
	return api.DocumentLinkPage{
		Outgoing: toAPILinks(page.Outgoing),
		Incoming: toAPILinks(page.Incoming),
	}
}

func toAPILinks(links []store.DocumentLink) []api.DocumentLink {
	out := make([]api.DocumentLink, 0, len(links))
	for _, link := range links {
		out = append(out, api.DocumentLink{
			ID:               strconv.FormatInt(link.ID, 10),
			SourceDocumentID: link.SourceDocumentID,
			TargetDocumentID: link.TargetDocumentID,
			TargetResourceID: link.TargetResourceID,
			TargetURI:        link.TargetURI,
			RelationType:     link.RelationType,
			SourceFormat:     link.SourceFormat,
			RawTarget:        link.RawTarget,
			DisplayText:      link.DisplayText,
			AnchorType:       link.AnchorType,
			AnchorValue:      link.AnchorValue,
			Context:          link.Context,
			ResolutionStatus: link.ResolutionStatus,
			SourcePosition: api.SourcePosition{
				StartByte: link.SourceStartByte,
				EndByte:   link.SourceEndByte,
				Line:      link.SourceLine,
				Column:    link.SourceColumn,
			},
		})
	}
	return out
}

func toAPIGraph(graph store.GraphResponse) api.GraphResponse {
	nodes := make([]map[string]any, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes = append(nodes, map[string]any{
			"id":    node.ID,
			"uri":   node.URI,
			"kind":  node.Kind,
			"label": node.Label,
		})
	}
	edges := make([]map[string]any, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		edges = append(edges, map[string]any{
			"id":         edge.ID,
			"source_id":  edge.SourceID,
			"target_id":  edge.TargetID,
			"kind":       edge.Kind,
			"status":     edge.Status,
			"raw_target": edge.RawTarget,
		})
	}
	return api.GraphResponse{Nodes: nodes, Edges: edges, Truncated: graph.Truncated}
}

func setResourceHeaders(w http.ResponseWriter, resource store.Resource, download bool) {
	contentType := defaultString(resource.MIMEType, "application/octet-stream")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if resource.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resource.SizeBytes, 10))
	}
	if resource.Filename != "" {
		disposition := "inline"
		if download {
			disposition = "attachment"
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": safeDownloadFilename(resource.Filename)}))
	}
}

func safeDownloadFilename(filename string) string {
	filename = strings.TrimSpace(filepath.Base(strings.ReplaceAll(filename, "\\", "/")))
	if filename == "" || filename == "." || filename == ".." {
		return "resource"
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case 0, '/', '\\', '\r', '\n':
			return -1
		default:
			return r
		}
	}, filename)
}

func filenameFromContentDisposition(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return params["filename"]
}

func wantsDownload(r *http.Request) bool {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("download")))
	return value == "1" || value == "true" || value == "yes"
}

func writeStoreError(w http.ResponseWriter, err error, fallbackCode string) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "requested object was not found")
	case errors.Is(err, store.ErrPreconditionRequired):
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "base_revision_id or If-Match is required")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "operation conflicts with the current resource state")
	case errors.Is(err, store.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, store.ErrInvalidCursor):
		writeError(w, http.StatusBadRequest, "cursor_invalid", err.Error())
	case errors.Is(err, store.ErrNameConflict):
		writeError(w, http.StatusConflict, "name_conflict", err.Error())
	case errors.Is(err, store.ErrProtected):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, fallbackCode, err.Error())
	}
	return true
}

func setRevisionETag(w http.ResponseWriter, revisionID string) {
	if strings.TrimSpace(revisionID) == "" {
		return
	}
	w.Header().Set("ETag", `"`+revisionID+`"`)
}

func revisionFromIfMatch(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "*" {
		return ""
	}
	value = strings.TrimPrefix(value, "W/")
	value = strings.Trim(value, `"`)
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func applySurgicalEdits(body string, edits []api.SurgicalEdit) (string, error) {
	out := body
	for _, edit := range edits {
		if edit.Search == "" {
			return "", errors.New("patch edits require a non-empty search string")
		}
		count := strings.Count(out, edit.Search)
		if count == 0 {
			return "", errors.New("patch search text was not found")
		}
		if edit.ReplaceAll {
			out = strings.ReplaceAll(out, edit.Search, edit.Replace)
			continue
		}
		// Match joplin-mcp editNote semantics: an ambiguous match must be
		// disambiguated with more context or replace_all.
		if count > 1 {
			return "", fmt.Errorf("patch search text matches %d locations; add context or set replace_all", count)
		}
		out = strings.Replace(out, edit.Search, edit.Replace, 1)
	}
	return out, nil
}
