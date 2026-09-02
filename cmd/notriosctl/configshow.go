package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/paths"
)

// `notriosctl config show` prints the configuration this instance actually
// resolved, and where each path came from.
//
// `doctor` reported which file was loaded and three derived paths. It could not
// answer "what is the effective remote-media policy?" or "why is my database
// there?" -- and after H4 the answer to the second is frequently "because
// nothing said otherwise and it was resolved", which is a different fact from
// "because your config file says so" and needs to read differently.
//
// No secret is printed. sync.credential_ref is a reference to a native
// credential store, never a credential, and is shown for that reason; if a
// value ever holds material rather than a name, it does not belong here.
func runConfigShow(args []string) {
	fs := flag.NewFlagSet("notriosctl config show", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	asJSON := fs.Bool("json", false, "machine-readable output")
	noRedact := fs.Bool("no-redact", false, "print the home directory instead of ~")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	cfg, origins, err := config.LoadWithOrigins(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	home := ""
	if !*noRedact {
		if resolved, err := os.UserHomeDir(); err == nil {
			home = resolved
		}
	}
	show := func(value string) string { return paths.Redact(value, home) }

	source := cfg.ConfigPath
	if source == "" {
		source = "built-in defaults"
	}

	settings := []struct{ key, value string }{
		{"server.listen_addr", cfg.Server.ListenAddr},
		{"server.public_base_url", cfg.Server.PublicBaseURL},
		{"server.web_dir", show(cfg.Server.WebDir)},
		{"data.directory", show(cfg.Data.Directory)},
		{"data.database_path", show(cfg.Data.DatabasePath)},
		{"data.asset_store", show(cfg.Data.AssetStore)},
		{"data.state_dir", show(cfg.Data.StateDir)},
		{"data.cache_dir", show(cfg.Data.CacheDir)},
		{"data.runtime_dir", show(cfg.Data.RuntimeDir)},
		{"data.projection_dir", show(cfg.Data.ProjectionDir)},
		{"search_sidecar.enabled", fmt.Sprintf("%t", cfg.SearchSidecar.Enabled)},
		{"search_sidecar.index_dir", show(cfg.SearchSidecar.IndexDir)},
		{"remote_media.default_action", cfg.RemoteMedia.DefaultAction},
		{"remote_media.allow_private_networks", fmt.Sprintf("%t", cfg.RemoteMedia.AllowPrivateNetworks)},
		{"remote_media.quarantine_dir", show(cfg.RemoteMedia.QuarantineDir)},
		{"sync.target", cfg.Sync.Target},
		{"sync.rest.enabled", fmt.Sprintf("%t", cfg.Sync.REST.Enabled)},
		{"sync.credential_ref", cfg.Sync.CredentialRef},
		{"mcp.enabled", fmt.Sprintf("%t", cfg.MCP.Enabled)},
		{"mcp.default_scope", cfg.MCP.DefaultScope},
	}

	// A key the loader did not record is a compiled default: origins covers
	// every path setting and every key the file stated, so anything absent was
	// never touched by either.
	originOf := func(key string) string {
		if origin, ok := origins[key]; ok {
			return origin
		}
		return config.OriginCompiled
	}

	if *asJSON {
		rows := make([]map[string]string, 0, len(settings))
		for _, setting := range settings {
			rows = append(rows, map[string]string{
				"key": setting.key, "value": setting.value, "origin": originOf(setting.key),
			})
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(map[string]any{
			"source": show(source), "settings": rows, "redacted": home != "",
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("source: %s\n\n", show(source))
	width := 0
	for _, setting := range settings {
		if len(setting.key) > width {
			width = len(setting.key)
		}
	}
	for _, setting := range settings {
		value := setting.value
		if strings.TrimSpace(value) == "" {
			value = "(unset)"
		}
		fmt.Printf("  %-*s  %-40s  %s\n", width, setting.key, value, originOf(setting.key))
	}

	fmt.Printf("\norigin: %s = stated in the configuration file, %s = resolved from the platform roots, %s = built-in default\n",
		config.OriginFile, config.OriginResolved, config.OriginCompiled)
}
