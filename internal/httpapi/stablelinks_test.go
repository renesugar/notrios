package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/stablelink"
	"github.com/renesugar/notrios/internal/store"
)

func decodeResolveResponse(t *testing.T, body string) api.StableLinkResolveResponse {
	t.Helper()
	var response api.StableLinkResolveResponse
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&response); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return response
}

func TestStatusReportsTheLogicalDatabaseIDButNotTheReplicaID(t *testing.T) {
	s, st := newSelectionServer(t)
	identity, err := st.GetDatabaseIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, s, http.MethodGet, "/api/v1/status", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	var status api.StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.DatabaseInfo.DatabaseID != identity.DatabaseID {
		t.Fatalf("status should carry the logical database ID: %+v", status.DatabaseInfo)
	}
	// The replica ID identifies this writable copy and means nothing in a
	// shared link; publishing it invites clients to build links around it.
	if strings.Contains(response.Body.String(), identity.ReplicaID) {
		t.Fatalf("status leaked the replica ID: %s", response.Body.String())
	}
}

func TestResolveStableLinkEndpoint(t *testing.T) {
	s, st := newSelectionServer(t)
	ctx := context.Background()
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Linked note", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}

	resolved := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format(identity.DatabaseID, doc.ID)+`"}`)
	if resolved.Code != http.StatusOK {
		t.Fatalf("resolve: %d %s", resolved.Code, resolved.Body.String())
	}
	body := decodeResolveResponse(t, resolved.Body.String())
	if body.Status != store.StableLinkResolved || body.DocumentID != doc.ID || body.Title != "Linked note" {
		t.Fatalf("unexpected resolution: %+v", body)
	}

	// A link for another database is answered honestly and reveals nothing
	// about local state, even though this exact document ID exists here.
	foreign := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format("db_elsewhere", doc.ID)+`"}`)
	foreignBody := decodeResolveResponse(t, foreign.Body.String())
	if foreignBody.Status != store.StableLinkForeignDatabase {
		t.Fatalf("foreign link: %+v", foreignBody)
	}
	if foreignBody.Title != "" || foreignBody.DocumentURI != "" || foreignBody.NotebookID != "" {
		t.Fatalf("foreign link leaked local note state: %+v", foreignBody)
	}

	stale := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format(identity.DatabaseID, "doc_missing")+`"}`)
	if got := decodeResolveResponse(t, stale.Body.String()).Status; got != store.StableLinkStaleTarget {
		t.Fatalf("stale target status: %q", got)
	}
}

func TestResolveStableLinkEndpointRejectsBadInput(t *testing.T) {
	s, _ := newSelectionServer(t)
	cases := map[string]string{
		"empty":          `{"uri":""}`,
		"missing field":  `{}`,
		"foreign scheme": `{"uri":"document://default/documents/doc_a"}`,
		"malformed":      `{"uri":"notrios://databases/db_a/documents/doc_b/extra"}`,
		"oversized":      `{"uri":"notrios://databases/db_a/documents/` + strings.Repeat("d", 4096) + `"}`,
	}
	for name, payload := range cases {
		response := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve", payload)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d %s", name, response.Code, response.Body.String())
		}
	}
}

// The endpoint resolves against the database this service has open. It must
// not accept a path, a profile, or any other way to make the service consult a
// different database: that routing decision belongs to the local registry.
func TestResolveStableLinkEndpointTakesNoDatabaseSelector(t *testing.T) {
	s, st := newSelectionServer(t)
	identity, err := st.GetDatabaseIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format(identity.DatabaseID, "doc_x")+`","database_path":"/etc/passwd","profile":"other"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", response.Code, response.Body.String())
	}
	body := decodeResolveResponse(t, response.Body.String())
	if body.LocalDatabaseID != identity.DatabaseID {
		t.Fatalf("another database answered: %+v", body)
	}
}
