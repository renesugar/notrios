package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/linkreport"
	"github.com/renesugar/notrios/internal/store"
)

// v1.0 J40-D, decision 1: REST carries the ambiguous-links report as IDs,
// locations and fingerprints, and MCP does not offer it.
func TestJ40DRESTCarriesNoNoteContent(t *testing.T) {
	s := newNotebookServer(t)
	// Every piece of text a note could hold is marked, so a leak of any one
	// of them is found by looking for its marker.
	twinA := createNote(t, s, "Secretive Twin", "# Twin\n")
	twinB := createNote(t, s, "Secretive Twin", "# Twin\n")
	writer := createNote(t, s, "Confidential writer", "Which [[Secretive Twin|private words]]?\n")

	rr := doJSON(t, s, http.MethodGet, "/api/v1/admin/links/report?detail_limit=5&candidate_limit=1", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("link report: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, marker := range []string{"Secretive", "Confidential", "private words", "Twin", "document://", "notrios://"} {
		if strings.Contains(body, marker) {
			t.Fatalf("REST quoted %q from a note:\n%s", marker, body)
		}
	}
	var report api.LinkReport
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		t.Fatal(err)
	}
	if report.AmbiguousLinks != 1 || len(report.Notes) != 1 || report.Notes[0].DocumentID != writer.ID {
		t.Fatalf("one ambiguous link, by ID: %+v", report)
	}
	link := report.Notes[0].Links[0]
	if link.CandidateCount != 2 || len(link.CandidateIDs) != 1 || !link.Truncated || link.TargetSHA256 == "" ||
		link.FoundBy != linkreport.FoundByLibrary || link.Kind != linkreport.KindTitle {
		t.Fatalf("the link by location and fingerprint: %+v", link)
	}
	if id := link.CandidateIDs[0]; id != twinA.ID && id != twinB.ID {
		t.Fatalf("a candidate by ID: %s", id)
	}

	for _, bad := range []string{"detail_limit=0", "detail_limit=1001", "candidate_limit=101", "candidate_limit=x"} {
		if rr := doJSON(t, s, http.MethodGet, "/api/v1/admin/links/report?"+bad, ""); rr.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", bad, rr.Code)
		}
	}
}

func TestJ40DTheNoteIsWrittenOnRequestAndIsReadOnly(t *testing.T) {
	s := newNotebookServer(t)
	createNote(t, s, "Twin", "# Twin\n")
	createNote(t, s, "Twin", "# Twin\n")
	createNote(t, s, "Writer", "Which [[Twin]]?\n")

	if rr := doJSON(t, s, http.MethodGet, "/api/v1/admin/links/report/note", ""); rr.Code == http.StatusOK {
		t.Fatal("writing the note is a POST, never a read")
	}
	rr := doJSON(t, s, http.MethodPost, "/api/v1/admin/links/report/note", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("write link report note: %d %s", rr.Code, rr.Body.String())
	}
	var written api.LinkReportNoteWritten
	if err := json.NewDecoder(rr.Body).Decode(&written); err != nil {
		t.Fatal(err)
	}
	if written.DocumentID != linkreport.NoteID || written.Title != linkreport.NoteTitle || written.Report.AmbiguousLinks != 1 {
		t.Fatalf("the note and its counts: %+v", written)
	}
	rr = doJSON(t, s, http.MethodGet, "/api/v1/documents/"+linkreport.NoteID, "")
	var note api.Document
	if err := json.NewDecoder(rr.Body).Decode(&note); err != nil {
		t.Fatal(err)
	}
	if note.NotebookID != store.ReportsNotebookID || note.Editable || !strings.Contains(note.Body, `\[\[Twin\]\]`) {
		t.Fatalf("a read-only Reports note quoting the link: %+v", note)
	}
	if rr := doJSON(t, s, http.MethodPut, "/api/v1/documents/"+linkreport.NoteID, `{"title":"Mine now","body":"x"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("an edit of the report = %d, want 403", rr.Code)
	}
}

func TestJ40DMCPDoesNotOfferTheReport(t *testing.T) {
	s := newNotebookServer(t)
	// get_lint_report stays: it is the content-free form, and the reason this
	// report has none of its own.
	sawLint := false
	for _, tool := range s.mcpTools() {
		name := strings.ToLower(tool.Name)
		sawLint = sawLint || name == "get_lint_report"
		if strings.Contains(name, "ambiguous") || strings.Contains(name, "links_report") || strings.Contains(name, "link_report") {
			t.Fatalf("MCP offers %s: %s", tool.Name, tool.Description)
		}
	}
	if !sawLint {
		t.Fatal("get_lint_report is the content-free form an MCP client gets")
	}
}
