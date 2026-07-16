// Package service wires configuration, the SQLite store, the HTTP API
// handler, and the optional Recoll search sidecar into one runnable unit.
// notriosd (headless) and notrios (built-in GUI) share this startup path.
package service

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/recoll"
	"github.com/renesugar/notrios/internal/store"
)

// Service is a started Notrios backend.
type Service struct {
	Config  config.Config
	Store   *store.SQLiteStore
	Handler *httpapi.Server
	stop    chan struct{}
}

// New creates storage directories, opens and bootstraps the store, builds the
// HTTP handler, and starts the optional search sidecar.
func New(cfg config.Config) (*Service, error) {
	if err := config.EnsureDirectories(cfg); err != nil {
		return nil, err
	}
	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		return nil, err
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		_ = st.Close()
		return nil, err
	}
	handler := httpapi.NewServerWithOptions(httpapi.ServerOptions{Store: st, Config: cfg})
	svc := &Service{Config: cfg, Store: st, Handler: handler, stop: make(chan struct{})}
	if cfg.SearchSidecar.Enabled {
		svc.startSearchSidecar()
	}
	return svc, nil
}

// HTTPServer returns a configured *http.Server for the service handler.
func (s *Service) HTTPServer() *http.Server {
	return &http.Server{
		Addr:              s.Config.Server.ListenAddr,
		Handler:           s.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// Close stops background work and the store.
func (s *Service) Close() error {
	close(s.stop)
	return s.Store.Close()
}

// startSearchSidecar wires the optional Recoll sidecar: generated config, an
// initial projection full-sync plus index pass, merged search results, and a
// background loop that drains the projection outbox and reindexes. Any
// missing binary degrades to FTS5-only search without failing startup.
func (s *Service) startSearchSidecar() {
	cfg := s.Config
	sidecar := recoll.New(cfg.SearchSidecar.IndexDir, cfg.Data.ProjectionDir, cfg.SearchSidecar.Binary)
	if !sidecar.Available() {
		log.Printf("search sidecar enabled but %q/recollq not found; continuing with FTS5 only", cfg.SearchSidecar.Binary)
		return
	}
	if err := sidecar.EnsureConfig(); err != nil {
		log.Printf("search sidecar config generation failed; continuing with FTS5 only: %v", err)
		return
	}
	writer := projection.Writer{Dir: cfg.Data.ProjectionDir}
	ctx := context.Background()
	if report, err := projection.FullSync(ctx, s.Store, writer); err != nil {
		log.Printf("projection full sync failed: %v", err)
	} else {
		log.Printf("startup %s", report)
	}
	if err := sidecar.Index(ctx); err != nil {
		log.Printf("recollindex failed; continuing with FTS5 only: %v", err)
		return
	}
	s.Handler.AttachSidecar(sidecar)
	log.Printf("search sidecar active: recoll config %s indexing %s", cfg.SearchSidecar.IndexDir, cfg.Data.ProjectionDir)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
			}
			report, err := projection.SyncOutbox(ctx, s.Store, writer, 200)
			if err != nil {
				log.Printf("projection outbox sync failed: %v", err)
				continue
			}
			if report.Written == 0 && report.Removed == 0 {
				continue
			}
			log.Printf("incremental %s", report)
			if err := sidecar.Index(ctx); err != nil {
				log.Printf("recollindex failed: %v", err)
			}
		}
	}()
}
