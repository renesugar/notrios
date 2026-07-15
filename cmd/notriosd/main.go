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

	server := &http.Server{
		Addr: cfg.Server.ListenAddr,
		Handler: httpapi.NewServerWithOptions(httpapi.ServerOptions{
			Store:  st,
			Config: cfg,
		}),
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

func loadRuntimeConfig(path string) (config.Config, error) {
	if strings.TrimSpace(path) != "" {
		return config.Load(path)
	}
	return config.LoadDefaultOrExample()
}
