package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/service"
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

	svc, err := service.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer svc.Close()

	log.Printf("notriosd listening on http://%s using db %s", cfg.Server.ListenAddr, cfg.Data.DatabasePath)
	if err := svc.HTTPServer().ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func loadRuntimeConfig(path string) (config.Config, error) {
	if strings.TrimSpace(path) != "" {
		return config.Load(path)
	}
	return config.LoadDefaultOrExample()
}
