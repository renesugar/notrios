package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func decodeLintReport(t *testing.T, body string) store.LintReport {
	t.Helper()
	var report store.LintReport
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&report); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return report
}

func TestLintReportEndpoint(t *testing.T) {
	s, st := newSelectionServer(t)
	ctx := context.Background()
	if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Problems", Body: "[broken](document://default/documents/doc_gone)\n\n![](https://example.com/x.png)\n",
	}); err != nil {
		t.Fatal(err)
	}

	response := doJSON(t, s, http.MethodGet, "/api/v1/admin/lint/report", "")
	if response.Code != http.StatusOK {
		t.Fatalf("lint report: %d %s", response.Code, response.Body.String())
	}
	raw := response.Body.String()
	report := decodeLintReport(t, raw)
	if report.TotalFindings < 2 || len(report.ReportSHA256) != 64 {
		t.Fatalf("unexpected report: %+v", report)
	}
	// The report locates problems; it must not quote note content or targets.
	for _, forbidden := range []string{"doc_gone", "example.com", "Problems"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("lint report leaked %q: %s", forbidden, raw)
		}
	}
}

func TestLintReportEndpointValidatesInput(t *testing.T) {
	s, _ := newSelectionServer(t)
	cases := map[string]int{
		"/api/v1/admin/lint/report?detail_limit=0":        http.StatusBadRequest,
		"/api/v1/admin/lint/report?detail_limit=100000":   http.StatusBadRequest,
		"/api/v1/admin/lint/report?checks=delete_the_lot": http.StatusBadRequest,
		"/api/v1/admin/lint/report?checks=missing_title":  http.StatusOK,
	}
	for path, want := range cases {
		if got := doJSON(t, s, http.MethodGet, path, "").Code; got != want {
			t.Fatalf("%s: got %d, want %d", path, got, want)
		}
	}
}

// Lint is a report. There is no REST apply surface, exactly as with the
// garbage-collection report.
func TestLintHasNoWriteEndpoint(t *testing.T) {
	s, _ := newSelectionServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if got := doJSON(t, s, method, "/api/v1/admin/lint/report", `{}`).Code; got != http.StatusMethodNotAllowed && got != http.StatusNotFound {
			t.Fatalf("%s on the lint report returned %d; it must not be writable", method, got)
		}
	}
}
