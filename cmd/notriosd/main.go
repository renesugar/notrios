package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/recoll"
	"github.com/renesugar/notrios/internal/store"
)

func main() {
	configPath := flag.String("config", "", "configuration file path; defaults to config/config.example.yaml when present")
	addrOverride := flag.String("addr", "", "HTTP listen address override")
	dbOverride := flag.String("db", "", "SQLite database path override, or :memory: for temporary storage")
	flag.Parse()

	cfg, err := loadRuntimeConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if strings.TrimSpace(*addrOverride) != "" {
		cfg.Server.ListenAddr = *addrOverride
	}
	if strings.TrimSpace(*dbOverride) != "" {
		cfg.Data.DatabasePath = *dbOverride
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		log.Fatal(err)
	}

	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		log.Fatal(err)
	}

	handler := httpapi.NewServerWithOptions(httpapi.ServerOptions{
		Store:  st,
		Config: cfg,
	})

	if cfg.SearchSidecar.Enabled {
		startSearchSidecar(cfg, st, handler)
	}

	server := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("notriosd listening on http://%s using db %s", cfg.Server.ListenAddr, cfg.Data.DatabasePath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// startSearchSidecar wires the optional Recoll sidecar: generated config, an
// initial projection full-sync plus index pass, merged search results, and a
// background loop that drains the projection outbox and reindexes. Any
// missing binary degrades to FTS5-only search without failing startup.
func startSearchSidecar(cfg config.Config, st store.Store, handler *httpapi.Server) {
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
	if report, err := projection.FullSync(ctx, st, writer); err != nil {
		log.Printf("projection full sync failed: %v", err)
	} else {
		log.Printf("startup %s", report)
	}
	if err := sidecar.Index(ctx); err != nil {
		log.Printf("recollindex failed; continuing with FTS5 only: %v", err)
		return
	}
	handler.AttachSidecar(sidecar)
	log.Printf("search sidecar active: recoll config %s indexing %s", cfg.SearchSidecar.IndexDir, cfg.Data.ProjectionDir)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			report, err := projection.SyncOutbox(ctx, st, writer, 200)
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

func loadRuntimeConfig(path string) (config.Config, error) {
	if strings.TrimSpace(path) != "" {
		return config.Load(path)
	}
	return config.LoadDefaultOrExample()
}
