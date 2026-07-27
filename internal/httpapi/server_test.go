package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

func TestHealth(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "ok" {
		t.Fatalf("unexpected body %q", rr.Body.String())
	}
}

func TestStatusReportsConfigurationAndSchema(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	cfg := config.Default()
	cfg.ConfigPath = "config/test.yaml"
	cfg.Data.DatabasePath = ":memory:"
	cfg.Data.AssetStore = "/tmp/notes-test-assets"
	cfg.SearchSidecar.Enabled = true
	s := NewServerWithOptions(ServerOptions{Store: st, Config: cfg})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got api.StatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if got.Status != "running" || got.DatabaseInfo.Driver != "sqlite" || got.DatabaseInfo.SchemaVersion != 9 {
		t.Fatalf("unexpected database status: %+v", got)
	}
	if got.ConfigPath != "config/test.yaml" || got.Storage.AssetStore != "/tmp/notes-test-assets" {
		t.Fatalf("unexpected config/storage status: %+v", got)
	}
	if !got.Capabilities["documents.create"] || !got.Capabilities["search.fts5"] || !got.Capabilities["search_sidecar"] {
		t.Fatalf("expected capability flags to be reported: %+v", got.Capabilities)
	}
	if got.MediaPolicy == nil {
		t.Fatalf("status must report the media policy: %+v", got)
	}
	if got.MediaPolicy.DefaultAction != "review" || got.MediaPolicy.AllowPrivateNetworks {
		t.Fatalf("unexpected media policy defaults: %+v", got.MediaPolicy)
	}
	if got.MediaPolicy.BlockedSchemes == 0 || got.MediaPolicy.MaxRedirects == 0 || got.MediaPolicy.FetchTimeoutSeconds == 0 {
		t.Fatalf("media policy limits missing: %+v", got.MediaPolicy)
	}
	if got.MediaPolicy.MaxBytes["image"] == 0 || got.MediaPolicy.QuarantineDir == "" {
		t.Fatalf("media policy size caps/quarantine missing: %+v", got.MediaPolicy)
	}
}

func TestSearchRejectsInvalidJSON(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader("{"))
	rr := httptest.NewRecorder()

	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestSearchRejectsLargeLimit(t *testing.T) {
	s := NewServer()
	body := bytes.NewBufferString(`{"query":"sqlite","limit":101}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", body)
	rr := httptest.NewRecorder()

	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateDocumentScaffold(t *testing.T) {
	s := NewServer()
	body := bytes.NewBufferString(`{"collection_id":"default","title":"Test","body":"# Test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", body)
	rr := httptest.NewRecorder()

	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var got api.Document
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.CollectionID != "default" || got.Title != "Test" {
		t.Fatalf("unexpected document: %#v", got)
	}
}

func TestPlaceholderRoutes(t *testing.T) {
	s := NewServer()
	cases := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/api/v1/collections", http.StatusOK},
		{http.MethodGet, "/api/v1/documents/doc1", http.StatusOK},
		{http.MethodGet, "/api/v1/documents/doc1/resources", http.StatusOK},
		{http.MethodGet, "/api/v1/documents/doc1/links", http.StatusOK},
		{http.MethodGet, "/api/v1/documents/doc1/outline", http.StatusOK},
		{http.MethodGet, "/api/v1/resources/res1", http.StatusOK},
		{http.MethodGet, "/api/v1/jobs/job1", http.StatusOK},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("%s %s: expected %d, got %d", tc.method, tc.path, tc.want, rr.Code)
		}
	}
}

func TestDocumentCreateReadSearchWithSQLiteStore(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	s := NewServerWithStore(st)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewBufferString(`{"title":"Joplin RAW import","body":"Joplin RAW exports are converted to Markdown frontmatter before indexing."}`))
	createRR := httptest.NewRecorder()
	s.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRR.Code, createRR.Body.String())
	}
	var created api.Document
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == "" || created.CurrentRevisionID == "" {
		t.Fatalf("created document missing IDs: %+v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+created.ID, nil)
	getRR := httptest.NewRecorder()
	s.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRR.Code, getRR.Body.String())
	}

	searchReq := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewBufferString(`{"query":"RAW frontmatter","limit":5}`))
	searchRR := httptest.NewRecorder()
	s.ServeHTTP(searchRR, searchReq)
	if searchRR.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", searchRR.Code, searchRR.Body.String())
	}
	var search api.SearchResponse
	if err := json.NewDecoder(searchRR.Body).Decode(&search); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if len(search.Hits) != 1 || search.Hits[0].ID != created.ID {
		t.Fatalf("unexpected hits: %+v", search.Hits)
	}
}

func TestDocumentUpdateRevisionConflictAndDeleteWithSQLiteStore(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	s := NewServerWithStore(st)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewBufferString(`{"title":"Draft","body":"alpha body"}`))
	createRR := httptest.NewRecorder()
	s.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRR.Code, createRR.Body.String())
	}
	var created api.Document
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	updateBody := `{"title":"Draft updated","body":"beta body","base_revision_id":"` + created.CurrentRevisionID + `"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/documents/"+created.ID, bytes.NewBufferString(updateBody))
	updateRR := httptest.NewRecorder()
	s.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updateRR.Code, updateRR.Body.String())
	}
	var updated api.Document
	if err := json.NewDecoder(updateRR.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.CurrentRevisionID == created.CurrentRevisionID || updated.Body != "beta body" {
		t.Fatalf("unexpected updated document: %+v", updated)
	}
	if got := updateRR.Header().Get("ETag"); got == "" {
		t.Fatalf("update should return ETag")
	}

	staleReq := httptest.NewRequest(http.MethodPut, "/api/v1/documents/"+created.ID, bytes.NewBufferString(updateBody))
	staleRR := httptest.NewRecorder()
	s.ServeHTTP(staleRR, staleReq)
	if staleRR.Code != http.StatusConflict {
		t.Fatalf("stale update should return 409, got %d: %s", staleRR.Code, staleRR.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+created.ID, nil)
	deleteReq.Header.Set("If-Match", `"`+updated.CurrentRevisionID+`"`)
	deleteRR := httptest.NewRecorder()
	s.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleteRR.Code, deleteRR.Body.String())
	}
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+created.ID, nil)
	getRR := httptest.NewRecorder()
	s.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusNotFound {
		t.Fatalf("deleted document get should be 404, got %d", getRR.Code)
	}
}

func TestDocumentRevisionsAndRestoreWithSQLiteStore(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	s := NewServerWithStore(st)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewBufferString(`{"title":"Versioned","body":"first body"}`))
	createRR := httptest.NewRecorder()
	s.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRR.Code, createRR.Body.String())
	}
	var created api.Document
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	originalRevisionID := created.CurrentRevisionID

	updateBody := `{"title":"Versioned","body":"second body","base_revision_id":"` + originalRevisionID + `"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/documents/"+created.ID, bytes.NewBufferString(updateBody))
	updateRR := httptest.NewRecorder()
	s.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updateRR.Code, updateRR.Body.String())
	}
	var updated api.Document
	if err := json.NewDecoder(updateRR.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+created.ID+"/revisions", nil)
	listRR := httptest.NewRecorder()
	s.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("revision list status=%d body=%s", listRR.Code, listRR.Body.String())
	}
	var page api.RevisionPage
	if err := json.NewDecoder(listRR.Body).Decode(&page); err != nil {
		t.Fatalf("decode revision page: %v", err)
	}
	if len(page.Revisions) != 2 || page.Revisions[0].Body != "" {
		t.Fatalf("expected 2 revision summaries without bodies, got %+v", page.Revisions)
	}

	getRevReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+created.ID+"/revisions/"+originalRevisionID, nil)
	getRevRR := httptest.NewRecorder()
	s.ServeHTTP(getRevRR, getRevReq)
	if getRevRR.Code != http.StatusOK {
		t.Fatalf("get revision status=%d body=%s", getRevRR.Code, getRevRR.Body.String())
	}
	var original api.DocumentRevision
	if err := json.NewDecoder(getRevRR.Body).Decode(&original); err != nil {
		t.Fatalf("decode original revision: %v", err)
	}
	if original.Body != "first body" {
		t.Fatalf("unexpected original body: %+v", original)
	}

	restoreBody := `{"base_revision_id":"` + updated.CurrentRevisionID + `"}`
	restoreReq := httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+created.ID+"/revisions/"+originalRevisionID+"/restore", bytes.NewBufferString(restoreBody))
	restoreRR := httptest.NewRecorder()
	s.ServeHTTP(restoreRR, restoreReq)
	if restoreRR.Code != http.StatusOK {
		t.Fatalf("restore status=%d body=%s", restoreRR.Code, restoreRR.Body.String())
	}
	var restored api.Document
	if err := json.NewDecoder(restoreRR.Body).Decode(&restored); err != nil {
		t.Fatalf("decode restored: %v", err)
	}
	if restored.Body != "first body" || restored.CurrentRevisionID == updated.CurrentRevisionID {
		t.Fatalf("unexpected restored document: %+v", restored)
	}
}

func TestResourceHTTPUploadAttachDownloadAndSafeDelete(t *testing.T) {
	st, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	s := NewServerWithOptions(ServerOptions{Store: st, Config: config.Default()})

	createDoc := httptest.NewRequest(http.MethodPost, "/api/v1/documents", strings.NewReader(`{"title":"with resource","body":"body"}`))
	createDoc.Header.Set("Content-Type", "application/json")
	docRR := httptest.NewRecorder()
	s.ServeHTTP(docRR, createDoc)
	if docRR.Code != http.StatusCreated {
		t.Fatalf("create doc status=%d body=%s", docRR.Code, docRR.Body.String())
	}
	var doc api.Document
	if err := json.NewDecoder(docRR.Body).Decode(&doc); err != nil {
		t.Fatalf("decode doc: %v", err)
	}

	upload := httptest.NewRequest(http.MethodPost, "/api/v1/resources?filename=diagram.txt", strings.NewReader("hello resource"))
	upload.Header.Set("Content-Type", "text/plain")
	uploadRR := httptest.NewRecorder()
	s.ServeHTTP(uploadRR, upload)
	if uploadRR.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", uploadRR.Code, uploadRR.Body.String())
	}
	var res api.Resource
	if err := json.NewDecoder(uploadRR.Body).Decode(&res); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	if res.ID == "" || res.SHA256 == "" || res.SizeBytes != int64(len("hello resource")) {
		t.Fatalf("unexpected resource: %+v", res)
	}

	attach := httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+doc.ID+"/resources/"+res.ID, strings.NewReader(`{"relation_type":"embedded","ordinal":2}`))
	attach.Header.Set("Content-Type", "application/json")
	attachRR := httptest.NewRecorder()
	s.ServeHTTP(attachRR, attach)
	if attachRR.Code != http.StatusOK {
		t.Fatalf("attach status=%d body=%s", attachRR.Code, attachRR.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+doc.ID+"/resources", nil)
	listRR := httptest.NewRecorder()
	s.ServeHTTP(listRR, list)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRR.Code, listRR.Body.String())
	}
	var page api.ResourceReferencePage
	if err := json.NewDecoder(listRR.Body).Decode(&page); err != nil {
		t.Fatalf("decode resource page: %v", err)
	}
	if len(page.Resources) != 1 || page.Resources[0].ResourceID != res.ID || page.Resources[0].Resource == nil {
		t.Fatalf("unexpected resource refs: %+v", page.Resources)
	}

	download := httptest.NewRequest(http.MethodGet, "/api/v1/resources/"+res.ID+"/content?download=1", nil)
	downloadRR := httptest.NewRecorder()
	s.ServeHTTP(downloadRR, download)
	if downloadRR.Code != http.StatusOK {
		t.Fatalf("download status=%d body=%s", downloadRR.Code, downloadRR.Body.String())
	}
	if downloadRR.Body.String() != "hello resource" || !strings.Contains(downloadRR.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("unexpected download headers/body: headers=%v body=%q", downloadRR.Header(), downloadRR.Body.String())
	}

	deleteReferenced := httptest.NewRequest(http.MethodDelete, "/api/v1/resources/"+res.ID, nil)
	deleteReferencedRR := httptest.NewRecorder()
	s.ServeHTTP(deleteReferencedRR, deleteReferenced)
	if deleteReferencedRR.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected confirmation requirement, got %d", deleteReferencedRR.Code)
	}
	deleteReferenced = httptest.NewRequest(http.MethodDelete, "/api/v1/resources/"+res.ID, nil)
	deleteReferenced.Header.Set("X-Notrios-Confirmation", "delete-resource:"+res.ID)
	deleteReferencedRR = httptest.NewRecorder()
	s.ServeHTTP(deleteReferencedRR, deleteReferenced)
	if deleteReferencedRR.Code != http.StatusConflict {
		t.Fatalf("expected conflict deleting referenced resource, got %d", deleteReferencedRR.Code)
	}

	detach := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+doc.ID+"/resources/"+res.ID, nil)
	detachRR := httptest.NewRecorder()
	s.ServeHTTP(detachRR, detach)
	if detachRR.Code != http.StatusNoContent {
		t.Fatalf("detach status=%d body=%s", detachRR.Code, detachRR.Body.String())
	}
	deleteFree := httptest.NewRequest(http.MethodDelete, "/api/v1/resources/"+res.ID, nil)
	deleteFree.Header.Set("X-Notrios-Confirmation", "delete-resource:"+res.ID)
	deleteFreeRR := httptest.NewRecorder()
	s.ServeHTTP(deleteFreeRR, deleteFree)
	if deleteFreeRR.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleteFreeRR.Code, deleteFreeRR.Body.String())
	}
}

func TestResourceReportHTTP(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_report", Title: "Report"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	first, err := st.CreateResource(ctx, store.CreateResourceRequest{PreferredID: "res_report_a", Filename: "a.txt", Content: strings.NewReader("duplicate")})
	if err != nil {
		t.Fatalf("CreateResource first: %v", err)
	}
	if _, err := st.CreateResource(ctx, store.CreateResourceRequest{PreferredID: "res_report_b", Filename: "b.txt", Content: strings.NewReader("duplicate")}); err != nil {
		t.Fatalf("CreateResource second: %v", err)
	}
	if _, err := st.CreateResource(ctx, store.CreateResourceRequest{PreferredID: "res_report_orphan", Filename: "orphan.txt", Content: strings.NewReader("orphan")}); err != nil {
		t.Fatalf("CreateResource orphan: %v", err)
	}
	if _, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{DocumentID: doc.ID, ResourceID: first.ID}); err != nil {
		t.Fatalf("AttachDocumentResource: %v", err)
	}

	s := NewServerWithStore(st)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/resources/reports/reference", nil)
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("report status=%d body=%s", response.Code, response.Body.String())
	}
	var report api.ResourceReport
	if err := json.NewDecoder(response.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if len(report.ExactDuplicates) != 1 || report.ExactDuplicates[0].ResourceCount != 2 {
		t.Fatalf("exact duplicates: %+v", report.ExactDuplicates)
	}
	if len(report.UnreferencedBlobs) != 1 || len(report.UnreferencedBlobs[0].Resources) != 1 ||
		report.UnreferencedBlobs[0].Resources[0].ID != "res_report_orphan" {
		t.Fatalf("unreferenced blobs: %+v", report.UnreferencedBlobs)
	}
	if report.Perceptual.HookEnabled {
		t.Fatalf("default hook must be inert: %+v", report.Perceptual)
	}
}

func TestGarbageCollectionReportHTTPIsReadOnly(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	resource, err := st.CreateResource(ctx, store.CreateResourceRequest{
		PreferredID: "res_gc_http", Filename: "gc.txt", Content: strings.NewReader("gc report"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	cfg := config.Default()
	cfg.Retention.UnreferencedResourceDays = 0
	cfg.Retention.PurgedResourceDays = 0
	s := NewServerWithOptions(ServerOptions{Store: st, Config: cfg})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/gc/report", nil)
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GC report status=%d body=%s", response.Code, response.Body.String())
	}
	var report api.GarbageCollectionReport
	if err := json.NewDecoder(response.Body).Decode(&report); err != nil {
		t.Fatalf("decode GC report: %v", err)
	}
	if !report.DryRun || len(report.Eligible) != 1 || report.Eligible[0].Resource.ID != resource.ID ||
		len(report.Removed) != 0 || report.Policy.Gate != "local" {
		t.Fatalf("unexpected GC report: %+v", report)
	}
	if _, err := st.GetResource(ctx, resource.ID); err != nil {
		t.Fatalf("REST report must not delete: %v", err)
	}
}

func TestDocumentLinksAndGraphWithSQLiteStore(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	s := NewServerWithStore(st)

	createTarget := httptest.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewBufferString(`{"title":"Target","body":"target"}`))
	createTargetRR := httptest.NewRecorder()
	s.ServeHTTP(createTargetRR, createTarget)
	if createTargetRR.Code != http.StatusCreated {
		t.Fatalf("target create status=%d body=%s", createTargetRR.Code, createTargetRR.Body.String())
	}
	var target api.Document
	if err := json.NewDecoder(createTargetRR.Body).Decode(&target); err != nil {
		t.Fatalf("decode target: %v", err)
	}

	body := `{"title":"Source","body":"See [[Target]] and [external](https://example.com)."}`
	createSource := httptest.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewBufferString(body))
	createSourceRR := httptest.NewRecorder()
	s.ServeHTTP(createSourceRR, createSource)
	if createSourceRR.Code != http.StatusCreated {
		t.Fatalf("source create status=%d body=%s", createSourceRR.Code, createSourceRR.Body.String())
	}
	var source api.Document
	if err := json.NewDecoder(createSourceRR.Body).Decode(&source); err != nil {
		t.Fatalf("decode source: %v", err)
	}

	linksReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+source.ID+"/links?direction=both", nil)
	linksRR := httptest.NewRecorder()
	s.ServeHTTP(linksRR, linksReq)
	if linksRR.Code != http.StatusOK {
		t.Fatalf("links status=%d body=%s", linksRR.Code, linksRR.Body.String())
	}
	var links api.DocumentLinkPage
	if err := json.NewDecoder(linksRR.Body).Decode(&links); err != nil {
		t.Fatalf("decode links: %v", err)
	}
	if len(links.Outgoing) != 2 {
		t.Fatalf("expected two outgoing links, got %#v", links.Outgoing)
	}

	backlinksReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+target.ID+"/links?direction=incoming", nil)
	backlinksRR := httptest.NewRecorder()
	s.ServeHTTP(backlinksRR, backlinksReq)
	if backlinksRR.Code != http.StatusOK {
		t.Fatalf("backlinks status=%d body=%s", backlinksRR.Code, backlinksRR.Body.String())
	}
	var backlinks api.DocumentLinkPage
	if err := json.NewDecoder(backlinksRR.Body).Decode(&backlinks); err != nil {
		t.Fatalf("decode backlinks: %v", err)
	}
	if len(backlinks.Incoming) != 1 || backlinks.Incoming[0].SourceDocumentID != source.ID {
		t.Fatalf("unexpected backlinks: %#v", backlinks.Incoming)
	}

	graphBody := `{"roots":["` + source.ID + `"],"direction":"outgoing","max_nodes":10,"max_edges":10}`
	graphReq := httptest.NewRequest(http.MethodPost, "/api/v1/graph", bytes.NewBufferString(graphBody))
	graphRR := httptest.NewRecorder()
	s.ServeHTTP(graphRR, graphReq)
	if graphRR.Code != http.StatusOK {
		t.Fatalf("graph status=%d body=%s", graphRR.Code, graphRR.Body.String())
	}
	var graph api.GraphResponse
	if err := json.NewDecoder(graphRR.Body).Decode(&graph); err != nil {
		t.Fatalf("decode graph: %v", err)
	}
	if len(graph.Nodes) < 2 || len(graph.Edges) < 2 {
		t.Fatalf("expected graph nodes and edges, got %#v", graph)
	}
}

func TestMCPInitializeAndToolsList(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	s := NewServerWithStore(st)

	initReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	initRR := httptest.NewRecorder()
	s.ServeHTTP(initRR, initReq)
	if initRR.Code != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", initRR.Code, initRR.Body.String())
	}
	if !strings.Contains(initRR.Body.String(), "notrios") || !strings.Contains(initRR.Body.String(), "tools") {
		t.Fatalf("initialize response missing server info/tools: %s", initRR.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	listRR := httptest.NewRecorder()
	s.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("tools/list status=%d body=%s", listRR.Code, listRR.Body.String())
	}
	if !strings.Contains(listRR.Body.String(), "search_documents") || !strings.Contains(listRR.Body.String(), "get_document") {
		t.Fatalf("tools/list missing expected tools: %s", listRR.Body.String())
	}
}

func TestMCPReadOnlyToolsUseStore(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	s := NewServerWithStore(st)
	created, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{Title: "MCP Links", Body: "# Heading One\n\nSearchable MCP body with [[MCP Links]] and [external](https://example.com).\n"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	searchBody := `{"jsonrpc":"2.0","id":"search","method":"tools/call","params":{"name":"search_documents","arguments":{"query":"Searchable MCP","limit":3}}}`
	searchRR := httptest.NewRecorder()
	s.ServeHTTP(searchRR, httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(searchBody)))
	if searchRR.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", searchRR.Code, searchRR.Body.String())
	}
	if !strings.Contains(searchRR.Body.String(), created.ID) || !strings.Contains(searchRR.Body.String(), "structuredContent") {
		t.Fatalf("search response missing created doc/structured content: %s", searchRR.Body.String())
	}

	getBody := `{"jsonrpc":"2.0","id":"get","method":"tools/call","params":{"name":"get_document","arguments":{"uri":"` + created.URI + `","max_bytes":20}}}`
	getRR := httptest.NewRecorder()
	s.ServeHTTP(getRR, httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(getBody)))
	if getRR.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRR.Code, getRR.Body.String())
	}
	if !strings.Contains(getRR.Body.String(), "body_truncated") || !strings.Contains(getRR.Body.String(), "untrusted_data") {
		t.Fatalf("get_document response should mark/truncate body: %s", getRR.Body.String())
	}

	linksBody := `{"jsonrpc":"2.0","id":"links","method":"tools/call","params":{"name":"list_document_links","arguments":{"document_id":"` + created.ID + `","direction":"both"}}}`
	linksRR := httptest.NewRecorder()
	s.ServeHTTP(linksRR, httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(linksBody)))
	if linksRR.Code != http.StatusOK {
		t.Fatalf("links status=%d body=%s", linksRR.Code, linksRR.Body.String())
	}
	if !strings.Contains(linksRR.Body.String(), "outgoing") || !strings.Contains(linksRR.Body.String(), "external") {
		t.Fatalf("list_document_links response missing links: %s", linksRR.Body.String())
	}

	outlineBody := `{"jsonrpc":"2.0","id":"outline","method":"tools/call","params":{"name":"get_document_outline","arguments":{"document_id":"` + created.ID + `"}}}`
	outlineRR := httptest.NewRecorder()
	s.ServeHTTP(outlineRR, httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(outlineBody)))
	if outlineRR.Code != http.StatusOK {
		t.Fatalf("outline status=%d body=%s", outlineRR.Code, outlineRR.Body.String())
	}
	if !strings.Contains(outlineRR.Body.String(), "Heading One") || !strings.Contains(outlineRR.Body.String(), "heading-one") {
		t.Fatalf("outline response missing heading: %s", outlineRR.Body.String())
	}
}

func TestMCPCanBeDisabled(t *testing.T) {
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	cfg := config.Default()
	cfg.MCP.Enabled = false
	s := NewServerWithOptions(ServerOptions{Store: st, Config: cfg})
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("disabled GET /mcp should be 404, got %d: %s", rr.Code, rr.Body.String())
	}
	postReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	postRR := httptest.NewRecorder()
	s.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusOK || !strings.Contains(strings.ToLower(postRR.Body.String()), "mcp") {
		t.Fatalf("disabled POST /mcp should return JSON-RPC error, got %d: %s", postRR.Code, postRR.Body.String())
	}
}
