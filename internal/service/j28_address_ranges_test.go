package service

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
)

// J28: service start refuses ranges it cannot apply, logs what a stated set
// omits, and serves the stated set to the status view.

func j28ServiceConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Data.Directory = dir
	config.UseDataDirectory(&cfg, dir, nil)
	cfg.Data.DatabasePath = filepath.Join(dir, "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(dir, "assets")
	cfg.Data.ProjectionDir = filepath.Join(dir, "projections")
	return cfg
}

func TestJ28ServiceStartLogsAStatedSetAndServesIt(t *testing.T) {
	cfg := j28ServiceConfig(t)
	cfg.Security.RemoteMedia.RefusedAddressRanges = []string{"127.0.0.0/8"}
	cfg.Security.RemoteMedia.PermittedAddressRanges = []string{}

	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	svc, err := New(cfg)
	log.SetOutput(previous)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if !strings.Contains(logged.String(), "remote media: security.remote_media.refused_address_ranges omits 30 default range(s)") {
		t.Errorf("service start must name the omitted default ranges; log:\n%s", logged.String())
	}

	response := httptest.NewRecorder()
	svc.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))
	var status api.StatusResponse
	if response.Code != http.StatusOK || json.NewDecoder(response.Body).Decode(&status) != nil || status.MediaPolicy == nil {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	if status.MediaPolicy.AddressRangesOrigin != "configuration" || strings.Join(status.MediaPolicy.RefusedAddressRanges, ",") != "127.0.0.0/8" {
		t.Errorf("media_policy = %+v; want the stated set", status.MediaPolicy)
	}
}

func TestJ28ServiceRefusesRangesItCannotApply(t *testing.T) {
	cfg := j28ServiceConfig(t)
	cfg.Security.RemoteMedia.PermittedAddressRanges = []string{"::1"}
	if svc, err := New(cfg); err == nil {
		_ = svc.Close()
		t.Fatal("a loopback exception built in code must stop the service starting")
	} else if !strings.Contains(err.Error(), "remote-media address ranges") {
		t.Errorf("err = %v", err)
	}
}
