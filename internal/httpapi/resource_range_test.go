package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// seedResource stores bytes and returns the resource ID.
func seedResource(t *testing.T, s *Server, filename, mime, body string) string {
	t.Helper()
	res, err := s.store.CreateResource(context.Background(), store.CreateResourceRequest{
		CollectionID: "default",
		Filename:     filename,
		MIMEType:     mime,
		Content:      strings.NewReader(body),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	return res.ID
}

func getRange(t *testing.T, s *Server, resourceID, rangeHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/"+resourceID+"/content", nil)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	return rr
}

// Range support (v0.6 F3). It exists so an MCP client can read part of a
// resource after seeing its metadata, rather than being handed all of it or
// none of it.
func TestResourceContentHonoursRange(t *testing.T) {
	s := newNotebookServer(t)
	const body = "0123456789abcdefghij"
	id := seedResource(t, s, "sample.txt", "text/plain", body)

	// No Range: the whole thing, and a header advertising that ranges work.
	whole := getRange(t, s, id, "")
	if whole.Code != http.StatusOK || whole.Body.String() != body {
		t.Fatalf("full read: %d %q", whole.Code, whole.Body.String())
	}
	if whole.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("a rangeable resource must advertise it: %q", whole.Header().Get("Accept-Ranges"))
	}

	cases := []struct {
		name         string
		header       string
		wantStatus   int
		wantBody     string
		wantRangeHdr string
	}{
		{"a middle slice", "bytes=5-9", http.StatusPartialContent, "56789", "bytes 5-9/20"},
		{"an open-ended slice", "bytes=15-", http.StatusPartialContent, "fghij", "bytes 15-19/20"},
		{"a suffix slice", "bytes=-4", http.StatusPartialContent, "ghij", "bytes 16-19/20"},
		{"the whole thing by range", "bytes=0-19", http.StatusPartialContent, body, "bytes 0-19/20"},
	}
	for _, tc := range cases {
		rr := getRange(t, s, id, tc.header)
		if rr.Code != tc.wantStatus {
			t.Fatalf("%s: status %d, want %d", tc.name, rr.Code, tc.wantStatus)
		}
		if rr.Body.String() != tc.wantBody {
			t.Fatalf("%s: body %q, want %q", tc.name, rr.Body.String(), tc.wantBody)
		}
		if got := rr.Header().Get("Content-Range"); got != tc.wantRangeHdr {
			t.Fatalf("%s: Content-Range %q, want %q", tc.name, got, tc.wantRangeHdr)
		}
		// Content-Length must describe the slice, not the resource. The
		// handler sets the full length before delegating, so this asserts the
		// delegation actually overrides it.
		if got := rr.Header().Get("Content-Length"); got != strconv.Itoa(len(tc.wantBody)) {
			t.Fatalf("%s: Content-Length %q, want %d", tc.name, got, len(tc.wantBody))
		}
	}
}

// An unsatisfiable range is 416 with the real size, not a silent full body. A
// caller that guessed wrong has to be able to find out.
func TestResourceContentRejectsUnsatisfiableRange(t *testing.T) {
	s := newNotebookServer(t)
	id := seedResource(t, s, "small.txt", "text/plain", "0123456789")

	rr := getRange(t, s, id, "bytes=500-600")
	if rr.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected 416, got %d %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Range"); got != "bytes */10" {
		t.Fatalf("416 must name the actual size, got %q", got)
	}
}

// Range and ?download=1 have to coexist: one chooses the disposition, the other
// chooses how much. A resumed download needs both.
func TestResourceContentRangeWorksWithDownload(t *testing.T) {
	s := newNotebookServer(t)
	id := seedResource(t, s, "report.txt", "text/plain", "abcdefghij")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/"+id+"/content?download=1", nil)
	req.Header.Set("Range", "bytes=2-4")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent || rr.Body.String() != "cde" {
		t.Fatalf("range with download: %d %q", rr.Code, rr.Body.String())
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("download disposition lost: %q", rr.Header().Get("Content-Disposition"))
	}
	// The stored MIME type survives; ServeContent must not re-sniff over it.
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("stored MIME type lost: %q", got)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff must survive a range response")
	}
}
