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

const restBlockBody = "# Title\n\nFirst paragraph.\n\n- an item\n\nTagged paragraph. ^marked\n"

func TestDocumentBlocksEndpoint(t *testing.T) {
	s, st := newSelectionServer(t)
	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Blocks", Body: restBlockBody})
	if err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, s, http.MethodGet, "/api/v1/documents/"+doc.ID+"/blocks", "")
	if response.Code != http.StatusOK {
		t.Fatalf("blocks: %d %s", response.Code, response.Body.String())
	}
	var body api.DocumentBlocksResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Blocks) != 4 || body.DocumentID != doc.ID {
		t.Fatalf("unexpected blocks: %+v", body)
	}
	if body.Blocks[0].Kind != "heading" || body.Blocks[0].HeadingLevel != 1 {
		t.Fatalf("first block: %+v", body.Blocks[0])
	}
	if body.Blocks[3].Marker != "marked" {
		t.Fatalf("authored marker missing: %+v", body.Blocks[3])
	}

	// The response addresses content; it does not hand out content. A caller
	// that wants the text reads the body, which is already an authorized read.
	raw := response.Body.String()
	for _, text := range []string{"First paragraph", "Tagged paragraph", "an item"} {
		if strings.Contains(raw, text) {
			t.Fatalf("block listing returned note text (%q): %s", text, raw)
		}
	}
}

func TestDocumentBlocksEndpointRejectsUnknownDocuments(t *testing.T) {
	s, _ := newSelectionServer(t)
	if response := doJSON(t, s, http.MethodGet, "/api/v1/documents/doc_missing/blocks", ""); response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d %s", response.Code, response.Body.String())
	}
}

// Content-based identity has a visible consequence: an anchor into a block
// whose text was rewritten no longer resolves, and the service must say so
// rather than silently opening the note at the top.
func TestStableLinkResolvesBlockAnchorsAndReportsStaleOnes(t *testing.T) {
	s, st := newSelectionServer(t)
	ctx := context.Background()
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Blocks", Body: restBlockBody})
	if err != nil {
		t.Fatal(err)
	}

	resolved := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format(identity.DatabaseID, doc.ID)+`#^marked"}`)
	body := decodeResolveResponse(t, resolved.Body.String())
	if body.Status != store.StableLinkResolved || body.BlockID == "" || body.BlockKind != "paragraph" {
		t.Fatalf("block anchor did not resolve: %+v", body)
	}

	stale := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format(identity.DatabaseID, doc.ID)+`#^no-such-block"}`)
	staleBody := decodeResolveResponse(t, stale.Body.String())
	if staleBody.Status != store.StableLinkStaleAnchor {
		t.Fatalf("expected a stale-anchor status: %+v", staleBody)
	}
	// The note itself is still reported, so a client can offer to open it.
	if staleBody.DocumentID != doc.ID || staleBody.Title != "Blocks" {
		t.Fatalf("a stale anchor should still name the note: %+v", staleBody)
	}
	if staleBody.BlockID != "" {
		t.Fatalf("a stale anchor must not report a block: %+v", staleBody)
	}
}

const restHeadingBody = "# Getting Started\n\nIntro.\n\n## Install & Setup\n\nSteps.\n"

func TestBlocksEndpointExposesHeadingSlugs(t *testing.T) {
	s, st := newSelectionServer(t)
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{Title: "Headings", Body: restHeadingBody})
	if err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, s, http.MethodGet, "/api/v1/documents/"+doc.ID+"/blocks", "")
	var body api.DocumentBlocksResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	slugs := []string{}
	for _, block := range body.Blocks {
		if block.HeadingSlug != "" {
			slugs = append(slugs, block.HeadingSlug)
		}
	}
	if strings.Join(slugs, ",") != "getting-started,install-setup" {
		t.Fatalf("heading slugs: %v", slugs)
	}
}

// A heading anchor resolves in a stable link, and a renamed heading is reported
// as stale rather than silently opening the top of the note.
func TestStableLinkResolvesHeadingAnchors(t *testing.T) {
	s, st := newSelectionServer(t)
	ctx := context.Background()
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Headings", Body: restHeadingBody})
	if err != nil {
		t.Fatal(err)
	}

	resolved := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format(identity.DatabaseID, doc.ID)+`#install-setup"}`)
	body := decodeResolveResponse(t, resolved.Body.String())
	if body.Status != store.StableLinkResolved || body.BlockKind != "heading" || body.BlockID == "" {
		t.Fatalf("heading anchor did not resolve: %+v", body)
	}

	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: doc.ID, Title: "Headings", Body: "# Getting Started\n\nIntro.\n\n## Installation\n\nSteps.\n",
		BaseRevisionID: doc.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	stale := doJSON(t, s, http.MethodPost, "/api/v1/links/resolve",
		`{"uri":"`+stablelink.Format(identity.DatabaseID, doc.ID)+`#install-setup"}`)
	if got := decodeResolveResponse(t, stale.Body.String()).Status; got != store.StableLinkStaleAnchor {
		t.Fatalf("a renamed heading should report a stale anchor, got %q", got)
	}
}
