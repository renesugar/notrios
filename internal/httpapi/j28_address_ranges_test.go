package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/addressrange"
	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

// J28: the server carries the configuration's security.remote_media block to
// the scan, check and localize paths, and reports it.

func j28Server(t *testing.T, security config.RemoteMediaSecurityConfig) (*Server, *store.SQLiteStore) {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.RemoteMedia.QuarantineDir = t.TempDir()
	cfg.Security.RemoteMedia = security
	return NewServerWithOptions(ServerOptions{Store: st, Config: cfg}), st
}

func j28MediaPolicy(t *testing.T, s *Server) api.MediaPolicyStatus {
	t.Helper()
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/media-policy", nil))
	var policy api.MediaPolicyStatus
	if rr.Code != http.StatusOK || json.NewDecoder(rr.Body).Decode(&policy) != nil {
		t.Fatalf("GET media-policy: %d %s", rr.Code, rr.Body.String())
	}
	return policy
}

func TestJ28MediaPolicyReportsTheDefaultSet(t *testing.T) {
	s, _ := j28Server(t, config.RemoteMediaSecurityConfig{})
	policy := j28MediaPolicy(t, s)
	if policy.AddressRangesOrigin != "default" || strings.Join(policy.RefusedAddressRanges, ",") != strings.Join(addressrange.Default, ",") {
		t.Errorf("origin=%s refused=%v; want the default set", policy.AddressRangesOrigin, policy.RefusedAddressRanges)
	}
	if policy.PermittedAddressRanges == nil || len(policy.PermittedAddressRanges) != 0 || policy.Warnings == nil || len(policy.Warnings) != 0 {
		t.Errorf("permitted=%#v warnings=%#v; want empty lists, not null", policy.PermittedAddressRanges, policy.Warnings)
	}
}

func TestJ28AStatedSetReachesScanCheckAndLocalize(t *testing.T) {
	s, st := j28Server(t, config.RemoteMediaSecurityConfig{
		RefusedAddressRanges:   []string{"203.0.113.0/24", "10.0.0.0/8"},
		PermittedAddressRanges: []string{"10.1.0.0/16"},
	})

	policy := j28MediaPolicy(t, s)
	if policy.AddressRangesOrigin != "configuration" || strings.Join(policy.RefusedAddressRanges, ",") != "203.0.113.0/24,10.0.0.0/8" ||
		strings.Join(policy.PermittedAddressRanges, ",") != "10.1.0.0/16" {
		t.Errorf("policy = %+v", policy)
	}
	warnings := strings.Join(policy.Warnings, "\n")
	if !strings.Contains(warnings, "omits 29 default range(s)") || !strings.Contains(warnings, "lets remote media reach: 10.1.0.0/16") {
		t.Errorf("warnings = %s", warnings)
	}

	// check-url: 127.0.0.1 is outside the stated set, so it falls to the
	// default action; 203.0.113.5 is refused by name; the exception is not.
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/media-policy/check-url",
		strings.NewReader(`{"urls":["http://127.0.0.1/a.png","http://203.0.113.5/b.png","http://10.1.2.3/c.png","http://10.2.0.1/d.png"]}`)))
	result := decodeScan(t, rr)
	got := []string{}
	for _, decision := range result.Media {
		got = append(got, decision.Action)
	}
	if strings.Join(got, ",") != "review,block,review,block" || !strings.Contains(result.Media[1].Reason, "203.0.113.0/24") {
		t.Errorf("check-url decisions = %+v; want the stated set, not the default", result.Media)
	}

	// localize (dry run) uses its own policy, built from the same block.
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
		Title: "J28", Body: "![a](http://127.0.0.1/a.png)\n![b](http://203.0.113.5/b.png)\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+doc.ID+"/remote-media/localize", strings.NewReader(`{"dry_run":true}`)))
	var localized api.RemoteMediaResult
	if rr.Code != http.StatusOK || json.NewDecoder(rr.Body).Decode(&localized) != nil {
		t.Fatalf("localize: %d %s", rr.Code, rr.Body.String())
	}
	if len(localized.Review) != 1 || len(localized.Blocked) != 1 || !strings.Contains(fmt.Sprint(localized.Blocked[0]["reason"]), "203.0.113.0/24") {
		t.Errorf("localize review=%v blocked=%v; want 127.0.0.1 in review and 203.0.113.5 blocked by the stated set", localized.Review, localized.Blocked)
	}
}
