package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/renesugar/notrios/internal/api"
)

// Creating a collection and then reading it back.
//
// The reading back is the point. Both handlers were stubs: POST validated its
// input and echoed it as 201 without touching the store, and GET answered
// `default` from a placeholder and 404'd everything else as "not available in
// scaffold server". Every existing test asserted the 201 and stopped, so a
// handler that created nothing looked exactly like one that worked. The bug
// surfaced only when an importer named a collection and hit the foreign key.
func TestCollectionIsReadableAfterItIsCreated(t *testing.T) {
	s := newNotebookServer(t)

	created := doJSON(t, s, http.MethodPost, "/api/v1/collections",
		`{"id":"joplin-raw-2026-07","name":"Joplin, July 2026","description":"a migration"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}

	read := doJSON(t, s, http.MethodGet, "/api/v1/collections/joplin-raw-2026-07", "")
	if read.Code != http.StatusOK {
		t.Fatalf("read back: %d %s", read.Code, read.Body.String())
	}
	var collection api.Collection
	if err := json.NewDecoder(read.Body).Decode(&collection); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if collection.ID != "joplin-raw-2026-07" || collection.Name != "Joplin, July 2026" {
		t.Fatalf("read back a different collection: %+v", collection)
	}
	if collection.Description != "a migration" {
		// The placeholder answered "Placeholder collection for scaffold
		// validation" to every request, so a description that survives the
		// round trip is what distinguishes a real read from that.
		t.Fatalf("the description did not survive: %+v", collection)
	}

	// And it is listed, which is how the importers' --collection value becomes
	// discoverable at all.
	list := doJSON(t, s, http.MethodGet, "/api/v1/collections", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d", list.Code)
	}
	var listed struct {
		Collections []api.Collection `json:"collections"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, item := range listed.Collections {
		if item.ID == "joplin-raw-2026-07" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the created collection is not listed: %+v", listed.Collections)
	}
}

func TestCollectionRefusalsAreReported(t *testing.T) {
	s := newNotebookServer(t)

	if rr := doJSON(t, s, http.MethodGet, "/api/v1/collections/never-made", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("an unknown collection is 404, not a placeholder: %d %s", rr.Code, rr.Body.String())
	}
	first := doJSON(t, s, http.MethodPost, "/api/v1/collections", `{"id":"twitter-archive","name":"Twitter"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("create: %d", first.Code)
	}
	// Creating one twice is a conflict rather than a silent overwrite: the id
	// is chosen by a person at import time and reusing one by accident should
	// not quietly rename somebody else's provenance.
	again := doJSON(t, s, http.MethodPost, "/api/v1/collections", `{"id":"twitter-archive","name":"Something else"}`)
	if again.Code != http.StatusConflict {
		t.Fatalf("a duplicate id is a conflict: %d %s", again.Code, again.Body.String())
	}
	if rr := doJSON(t, s, http.MethodPost, "/api/v1/collections", `{"name":"No id"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("an id is required: %d", rr.Code)
	}
}
