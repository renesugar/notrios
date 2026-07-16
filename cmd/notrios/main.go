// notrios is the built-in GUI executable (Notrios redesign task R13). One
// binary contains the GUI and the service:
//
//	notrios              start the local service and open the GUI on it
//	notrios -no-gui      run the service headless (same behavior as notriosd)
//	notrios -gui-only    open only the GUI as a pure REST client, optionally
//	                     against a remote service (-remote http://host:port);
//	                     this exercises the API exactly like a third-party
//	                     client would
//
// The webview itself is compiled in with `-tags "gui desktop production webkit2_41"` (see
// gui_wails.go); without the tag the binary still runs -no-gui mode and
// explains how to get the GUI.
package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/service"
)

func main() {
	configPath := flag.String("config", "", "configuration file path; defaults to config/config.example.yaml when present")
	addrOverride := flag.String("addr", "", "HTTP listen address override")
	dbOverride := flag.String("db", "", "SQLite database path override, or :memory: for temporary storage")
	noGUI := flag.Bool("no-gui", false, "run the service without the built-in GUI (use any REST/MCP client)")
	guiOnly := flag.Bool("gui-only", false, "run only the GUI as a REST client against an already-running service")
	remote := flag.String("remote", "", "service base URL for -gui-only (default http://<listen_addr> from config)")
	flag.Parse()

	if *noGUI && *guiOnly {
		log.Fatal("-no-gui and -gui-only are mutually exclusive")
	}

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

	switch {
	case *guiOnly:
		base := strings.TrimSpace(*remote)
		if base == "" {
			base = "http://" + cfg.Server.ListenAddr
		}
		target, err := url.Parse(base)
		if err != nil || target.Host == "" {
			log.Fatalf("invalid -remote URL %q", base)
		}
		log.Printf("notrios GUI connecting to %s (gui-only mode)", target)
		proxy := httputil.NewSingleHostReverseProxy(target)
		if err := runGUI(proxy); err != nil {
			log.Fatal(err)
		}

	case *noGUI:
		svc, err := service.New(cfg)
		if err != nil {
			log.Fatal(err)
		}
		defer svc.Close()
		log.Printf("notrios (no-gui) listening on http://%s using db %s", cfg.Server.ListenAddr, cfg.Data.DatabasePath)
		if err := svc.HTTPServer().ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}

	default:
		svc, err := service.New(cfg)
		if err != nil {
			log.Fatal(err)
		}
		defer svc.Close()
		// The service also listens on its TCP address so MCP clients and
		// third-party GUIs can connect while the built-in GUI is open.
		go func() {
			log.Printf("notrios service listening on http://%s using db %s", cfg.Server.ListenAddr, cfg.Data.DatabasePath)
			if err := svc.HTTPServer().ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("service listener stopped: %v", err)
			}
		}()
		if err := runGUI(svc.Handler); err != nil {
			log.Fatal(err)
		}
	}
}

func loadRuntimeConfig(path string) (config.Config, error) {
	if strings.TrimSpace(path) != "" {
		return config.Load(path)
	}
	return config.LoadDefaultOrExample()
}
