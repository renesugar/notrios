package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/notes-companion/internal/store"
)

func TestResourceContentUsesSafeDownloadHeaders(t *testing.T) {
	st, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	res, err := st.CreateResource(context.Background(), store.CreateResourceRequest{
		CollectionID: "default",
		Filename:     "../bad\r\nname.txt",
		MIMEType:     "text/plain",
		Content:      strings.NewReader("safe resource content"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	s := NewServerWithStore(st)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/"+res.ID+"/content?download=1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}
	disposition := rr.Header().Get("Content-Disposition")
	if !strings.HasPrefix(disposition, "attachment;") {
		t.Fatalf("expected attachment disposition, got %q", disposition)
	}
	if strings.Contains(disposition, "..") || strings.Contains(disposition, "\r") || strings.Contains(disposition, "\n") {
		t.Fatalf("unsafe content disposition: %q", disposition)
	}
}
