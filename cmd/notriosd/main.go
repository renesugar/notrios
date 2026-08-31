package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/service"
)

func main() {
	configPath := flag.String("config", "", "configuration file path; defaults to config/config.example.yaml when present")
	addrOverride := flag.String("addr", "", "HTTP listen address override")
	dbOverride := flag.String("db", "", "SQLite database path override, or :memory: for temporary storage")
	webDir := flag.String("web-dir", "", "directory holding the built web interface (default: search the working directory, then the executable's directory and its parent)")
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
	if strings.TrimSpace(*webDir) != "" {
		cfg.Server.WebDir = *webDir
	}

	svc, err := service.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer svc.Close()

	// Reported, not enforced: REST and MCP are the point of the headless
	// service and work without an interface. Saying so at startup beats a
	// browser discovering it later.
	if root, err := httpapi.ResolveWebRoot(cfg.Server.WebDir); err == nil {
		log.Printf("notriosd serving the web interface from %s", root)
	} else {
		log.Printf("notriosd starting without a web interface (REST and MCP still work): %v", err)
	}
	certificate, _ := svc.TLSFiles()
	scheme := "http"
	if certificate != "" {
		scheme = "https"
	}
	log.Printf("notriosd listening on %s://%s using db %s", scheme, cfg.Server.ListenAddr, cfg.Data.DatabasePath)
	serveErr := svc.ListenAndServe()
	if serveErr != nil && serveErr != http.ErrServerClosed {
		log.Fatal(serveErr)
	}
}

func loadRuntimeConfig(path string) (config.Config, error) {
	if strings.TrimSpace(path) != "" {
		return config.Load(path)
	}
	return config.LoadDefaultOrExample()
}
