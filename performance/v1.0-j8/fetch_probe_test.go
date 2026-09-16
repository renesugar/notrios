package j8

// J8-A, the half that needs a server: redirects, lying servers, size caps,
// quarantine handling and provenance.
//
// Every server here listens on loopback and the probes set
// AllowPrivateNetworks so the loopback destination is reachable; that switch is
// what lets the rest of the pipeline be exercised at all. The SSRF checks
// themselves are probed with the switch off, in policy_probe_test.go.
//
// These probes observe. A gap is recorded as a finding in README.md and
// deferred to its own plan item; J8 changes no product code.

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/media"
)

// onePixelPNG is the smallest real PNG: enough for DetectContentType to say
// image/png rather than guessing.
var onePixelPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
	0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
	0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00,
	0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
}

type recorded struct {
	attempts []media.Attempt
}

func (r *recorded) RecordMediaAttempt(_ context.Context, attempt media.Attempt) error {
	r.attempts = append(r.attempts, attempt)
	return nil
}

// loopbackFetcher builds a fetcher that may reach a loopback test server, with
// its own quarantine directory.
func loopbackFetcher(t *testing.T, adjust func(*config.RemoteMediaConfig)) (*media.Fetcher, string, *recorded) {
	t.Helper()
	quarantine := filepath.Join(t.TempDir(), "quarantine")
	cfg := config.RemoteMediaConfig{
		DefaultAction:        media.ActionAllow,
		BlockedSchemes:       []string{"file", "data", "javascript", "ftp"},
		AllowPrivateNetworks: true,
		QuarantineDir:        quarantine,
		FetchTimeoutSeconds:  5,
		MaxRedirects:         3,
		MaxBytes:             map[string]int64{"image": 64 * 1024, "video": 64 * 1024, "pdf": 64 * 1024},
	}
	if adjust != nil {
		adjust(&cfg)
	}
	recorder := &recorded{}
	fetcher, err := media.NewFetcher(cfg, recorder)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	return fetcher, quarantine, recorder
}

func quarantineFiles(t *testing.T, dir string) []string {
	t.Helper()
	names := []string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		names = append(names, filepath.Base(path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// TestJ8AServerThatLiesAboutItsContent records what happens when the bytes and
// the headers disagree: the policy says the type is sniffed, so a lying header
// must not decide.
func TestJ8AServerThatLiesAboutItsContent(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		body    []byte
		wantOK  bool
		because string
		// finding names the J8 finding a difference belongs to. A difference
		// with a finding is recorded, not failed: J8 records and defers.
		finding string
	}{
		{"html served as image/png", "image/png", []byte("<html><body>not an image</body></html>"), false,
			"a positive sniff of text/html must beat the header", ""},
		{"a real png with no header", "", onePixelPNG, true, "the bytes are an image", ""},
		{"a real png mislabelled as text/plain", "text/plain", onePixelPNG, true,
			"the sniff is positive, so the header does not matter", ""},
		{"text bytes served as image/jpeg", "image/jpeg", []byte("#!/bin/sh\nrm -rf /\n"), false,
			"text/plain is inconclusive, so the lying header decides the type", "J8-F4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if c.header != "" {
					w.Header().Set("Content-Type", c.header)
				}
				w.Write(c.body)
			}))
			defer server.Close()
			fetcher, quarantine, _ := loopbackFetcher(t, nil)
			results := fetcher.Quarantine(context.Background(), media.QuarantineRequest{URLs: []string{server.URL + "/photo.png"}})
			result := results[0]
			t.Logf("%-40s -> %s (%s) content_type=%q", c.name, result.Status, result.Reason, result.ContentType)
			quarantined := result.Status == media.StatusQuarantined
			switch {
			case quarantined == c.wantOK:
			case c.finding != "":
				t.Logf("FINDING %s: %s -> quarantined as %q; %s", c.finding, c.name, result.ContentType, c.because)
			default:
				t.Errorf("%s: status %s, wanted quarantined=%v because %s", c.name, result.Status, c.wantOK, c.because)
			}
			if !quarantined && len(quarantineFiles(t, quarantine)) != 0 {
				t.Errorf("%s: a refused fetch left bytes in quarantine: %v", c.name, quarantineFiles(t, quarantine))
			}
		})
	}
}

// TestJ8SizeCapsHoldWhenTheServerLies records whether the cap is enforced from
// the bytes rather than from what the server claims.
func TestJ8SizeCapsHoldWhenTheServerLies(t *testing.T) {
	big := make([]byte, 256*1024)
	copy(big, onePixelPNG)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		// Claim a small body and send a large one.
		w.Header().Set("Content-Length", "64")
		w.WriteHeader(http.StatusOK)
		w.Write(big)
	}))
	defer server.Close()

	fetcher, quarantine, _ := loopbackFetcher(t, nil)
	result := fetcher.Quarantine(context.Background(), media.QuarantineRequest{URLs: []string{server.URL + "/big.png"}})[0]
	t.Logf("a 256 KiB body under a 64 KiB cap, declaring Content-Length 64 -> %s (%s) size=%d",
		result.Status, result.Reason, result.SizeBytes)
	// What this probe establishes is the outcome, not the mechanism: the
	// transport refuses a body that contradicts its own Content-Length before
	// the cap is reached, and nothing is left behind either way. The cap itself,
	// enforced while streaming an honest body, is proven by
	// TestQuarantineEnforcesSizeCapWhileStreaming in internal/media.
	if result.Status != media.StatusRefused {
		t.Errorf("a body over the cap was quarantined: %s", result.Reason)
	}
	if files := quarantineFiles(t, quarantine); len(files) != 0 {
		t.Errorf("an over-cap fetch left bytes in quarantine: %v", files)
	}
}

// TestJ8RedirectsAreRefusedWhereTheyShouldBe records what a redirect chain can
// reach: a hop the policy blocks, and more hops than the limit.
func TestJ8RedirectsAreRefusedWhereTheyShouldBe(t *testing.T) {
	image := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(onePixelPNG)
	}))
	defer image.Close()

	t.Run("a hop to a blocked host", func(t *testing.T) {
		blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write(onePixelPNG)
		}))
		defer blocked.Close()
		hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, blocked.URL+"/final.png", http.StatusFound)
		}))
		defer hop.Close()

		blockedHost := strings.TrimPrefix(blocked.URL, "http://")
		if host, _, ok := strings.Cut(blockedHost, ":"); ok {
			blockedHost = host
		}
		fetcher, quarantine, _ := loopbackFetcher(t, func(cfg *config.RemoteMediaConfig) {
			// Block the destination by address, which is what the redirect
			// lands on.
			cfg.BlockedDomains = []string{blockedHost}
		})
		result := fetcher.Quarantine(context.Background(), media.QuarantineRequest{URLs: []string{hop.URL + "/start.png"}})[0]
		t.Logf("redirect into a blocked host -> %s (%s)", result.Status, result.Reason)
		if result.Status != media.StatusRefused {
			t.Errorf("a redirect reached a blocked host: %s", result.Reason)
		}
		if files := quarantineFiles(t, quarantine); len(files) != 0 {
			t.Errorf("a refused redirect left bytes in quarantine: %v", files)
		}
	})

	t.Run("more hops than the limit", func(t *testing.T) {
		var chain *httptest.Server
		hops := 0
		chain = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hops++
			http.Redirect(w, r, chain.URL+fmt.Sprintf("/hop-%d.png", hops), http.StatusFound)
		}))
		defer chain.Close()

		fetcher, _, _ := loopbackFetcher(t, nil) // MaxRedirects 3
		result := fetcher.Quarantine(context.Background(), media.QuarantineRequest{URLs: []string{chain.URL + "/start.png"}})[0]
		t.Logf("an endless redirect chain under a 3-hop limit -> %s (%s)", result.Status, result.Reason)
		if result.Status != media.StatusRefused {
			t.Errorf("an endless redirect chain was followed to a fetch: %s", result.Reason)
		}
	})
}

// TestJ8QuarantineHoldsWhatItShould records where admitted bytes land, under
// what permissions, and what names them.
func TestJ8QuarantineHoldsWhatItShould(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(onePixelPNG)
	}))
	defer server.Close()

	fetcher, quarantine, recorder := loopbackFetcher(t, nil)
	result := fetcher.Quarantine(context.Background(), media.QuarantineRequest{
		DocumentID: "doc_probe",
		URLs:       []string{server.URL + "/photo.png"},
	})[0]
	if result.Status != media.StatusQuarantined {
		t.Fatalf("the probe image was refused: %s", result.Reason)
	}

	info, err := os.Stat(quarantine)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("quarantine directory mode: %v", info.Mode().Perm())
	if info.Mode().Perm() != 0o700 {
		t.Errorf("the quarantine directory is %v; it holds untrusted bytes and should be owner-only", info.Mode().Perm())
	}

	files := quarantineFiles(t, quarantine)
	t.Logf("quarantined as: %v (sha256=%s)", files, result.SHA256)
	for _, name := range files {
		if !strings.HasPrefix(name, "sha256-") {
			t.Errorf("a quarantined file is named %q; the policy says content addresses, not server-chosen names", name)
		}
		if strings.Contains(name, "photo") {
			t.Errorf("a quarantined file carries the server's name %q", name)
		}
	}

	// Provenance: what the audit trail holds for an admission.
	if len(recorder.attempts) != 1 {
		t.Fatalf("expected one recorded attempt, got %d", len(recorder.attempts))
	}
	attempt := recorder.attempts[0]
	t.Logf("recorded: document=%q original=%q final=%q decision=%q status=%q type=%q size=%d sha256=%q path=%q",
		attempt.DocumentID, attempt.OriginalURL, attempt.FinalURL, attempt.Decision,
		attempt.Status, attempt.ContentType, attempt.SizeBytes, attempt.SHA256, attempt.QuarantinePath)
	for name, value := range map[string]string{
		"original URL": attempt.OriginalURL,
		"status":       attempt.Status,
		"content type": attempt.ContentType,
		"sha256":       attempt.SHA256,
	} {
		if strings.TrimSpace(value) == "" {
			t.Errorf("the policy requires provenance, and %s is empty", name)
		}
	}
}

// TestJ8ARefusalIsRecordedToo records whether a refused fetch leaves an audit
// trail: the policy requires the decision to be stored, not only the success.
func TestJ8ARefusalIsRecordedToo(t *testing.T) {
	fetcher, _, recorder := loopbackFetcher(t, func(cfg *config.RemoteMediaConfig) {
		cfg.DefaultAction = media.ActionReview
	})
	results := fetcher.Quarantine(context.Background(), media.QuarantineRequest{
		DocumentID: "doc_probe",
		URLs:       []string{"https://images.example.org/never-fetched.png"},
	})
	t.Logf("a review-action URL without opt-in -> %s (%s)", results[0].Status, results[0].Reason)
	if len(recorder.attempts) != 1 {
		t.Fatalf("a refusal must be recorded: got %d attempts", len(recorder.attempts))
	}
	attempt := recorder.attempts[0]
	t.Logf("recorded refusal: decision=%q status=%q reason=%q", attempt.Decision, attempt.Status, attempt.Reason)
	if attempt.Status != media.StatusRefused || attempt.Decision != media.ActionReview {
		t.Errorf("the refusal was recorded as status=%q decision=%q", attempt.Status, attempt.Decision)
	}
}
