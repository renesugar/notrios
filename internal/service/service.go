// Package service wires configuration, the SQLite store, the HTTP API
// handler, and the optional Recoll search sidecar into one runnable unit.
// notriosd (headless) and notrios (built-in GUI) share this startup path.
package service

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/recoll"
	"github.com/renesugar/notrios/internal/store"
)

// Service is a started Notrios backend.
type Service struct {
	Config    config.Config
	Store     *store.SQLiteStore
	Handler   *httpapi.Server
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
	workers   sync.WaitGroup
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
	ctx, cancel := context.WithCancel(context.Background())
	svc := &Service{Config: cfg, Store: st, Handler: handler, ctx: ctx, cancel: cancel}
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
	s.closeOnce.Do(func() {
		s.cancel()
		s.workers.Wait()
		s.closeErr = s.Store.Close()
	})
	return s.closeErr
}

// startSearchSidecar wires the optional Recoll sidecar: generated config, an
// initial exact reconciliation plus index pass, merged search results, and a
// background loop that drains bounded outbox batches, periodically reconciles,
// and reindexes. Any
// missing binary degrades to FTS5-only search without failing startup.
func (s *Service) startSearchSidecar() {
	cfg := s.Config
	sidecar := recoll.New(cfg.SearchSidecar.IndexDir, cfg.Data.ProjectionDir, cfg.SearchSidecar.Binary)
	s.Handler.AttachSidecarStatus(sidecar)
	if !sidecar.Available() {
		log.Printf("search sidecar enabled but %q/recollq not found; continuing with FTS5 only", cfg.SearchSidecar.Binary)
		return
	}
	if err := sidecar.EnsureConfig(); err != nil {
		sidecar.RecordError(err)
		log.Printf("search sidecar config generation failed; continuing with FTS5 only: %v", err)
		return
	}
	writer := projection.Writer{Dir: cfg.Data.ProjectionDir}
	if report, err := projection.Reconcile(s.ctx, s.Store, writer, 200); err != nil {
		sidecar.RecordError(err)
		log.Printf("projection reconciliation failed: %v", err)
	} else {
		sidecar.RecordReconciliation(report)
		log.Printf("startup projection reconciliation: canonical=%d missing=%d stale=%d orphaned=%d repaired=%d failed=%d",
			report.Canonical, report.Missing, report.Stale, report.Orphaned, report.Repaired, report.Failed)
	}
	if report, err := projection.DrainOutbox(s.ctx, s.Store, writer, 200, 20); err != nil {
		sidecar.RecordError(err)
		log.Printf("projection outbox sync failed: %v", err)
	} else {
		sidecar.RecordProjection(report)
		s.recordSidecarQueue(sidecar)
	}
	if err := sidecar.Index(s.ctx); err != nil {
		log.Printf("recollindex failed; continuing with FTS5 only: %v", err)
		return
	}
	sidecar.SetActive(true)
	s.Handler.AttachSidecar(sidecar)
	log.Printf("search sidecar active: recoll config %s indexing %s", cfg.SearchSidecar.IndexDir, cfg.Data.ProjectionDir)

	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		syncTicker := time.NewTicker(30 * time.Second)
		reconcileTicker := time.NewTicker(10 * time.Minute)
		defer syncTicker.Stop()
		defer reconcileTicker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-syncTicker.C:
				s.runSidecarPass(sidecar, writer, false)
			case <-reconcileTicker.C:
				s.runSidecarPass(sidecar, writer, true)
			}
		}
	}()
}

func (s *Service) runSidecarPass(sidecar *recoll.Sidecar, writer projection.Writer, reconcile bool) {
	changed := false
	report, err := projection.DrainOutbox(s.ctx, s.Store, writer, 200, 20)
	if err != nil {
		sidecar.RecordError(err)
		log.Printf("projection outbox sync failed: %v", err)
	} else {
		sidecar.RecordProjection(report)
		changed = report.Written > 0 || report.Removed > 0
		if report.Jobs > 0 {
			log.Printf("incremental %s", report)
		}
	}
	s.recordSidecarQueue(sidecar)
	if reconcile {
		reconcileReport, reconcileErr := projection.Reconcile(s.ctx, s.Store, writer, 200)
		if reconcileErr != nil {
			sidecar.RecordError(reconcileErr)
			log.Printf("projection reconciliation failed: %v", reconcileErr)
		} else {
			sidecar.RecordReconciliation(reconcileReport)
			changed = changed || reconcileReport.Repaired > 0
			log.Printf("projection reconciliation: canonical=%d missing=%d stale=%d orphaned=%d repaired=%d failed=%d",
				reconcileReport.Canonical, reconcileReport.Missing, reconcileReport.Stale,
				reconcileReport.Orphaned, reconcileReport.Repaired, reconcileReport.Failed)
		}
	}
	if changed {
		if err := sidecar.Index(s.ctx); err != nil {
			log.Printf("recollindex failed: %v", err)
		}
	}
}

func (s *Service) recordSidecarQueue(sidecar *recoll.Sidecar) {
	queue, err := s.Store.ProjectionQueueStatus(s.ctx)
	if err != nil {
		sidecar.RecordError(err)
		return
	}
	sidecar.RecordQueue(queue.Pending, queue.Failed)
}
