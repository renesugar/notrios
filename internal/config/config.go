package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config contains the runtime settings used by notesd and notesctl.
// It intentionally avoids third-party YAML dependencies until the project
// chooses and pins the long-term configuration library.
type Config struct {
	ConfigPath string       `json:"config_path,omitempty"`
	Server     ServerConfig `json:"server"`
	Data       DataConfig   `json:"data"`
	Search     SearchConfig `json:"search"`
	MCP        MCPConfig    `json:"mcp"`
	Sist2      Sist2Config  `json:"sist2"`
}

type ServerConfig struct {
	ListenAddr    string `json:"listen_addr"`
	PublicBaseURL string `json:"public_base_url"`
}

type DataConfig struct {
	Directory     string `json:"directory"`
	DatabasePath  string `json:"database_path"`
	AssetStore    string `json:"asset_store"`
	ProjectionDir string `json:"projection_dir"`
}

type SearchConfig struct {
	DefaultLimit int `json:"default_limit"`
	MaxLimit     int `json:"max_limit"`
	MaxOffset    int `json:"max_offset"`
}

type MCPConfig struct {
	Enabled          bool   `json:"enabled"`
	DefaultProfile   string `json:"default_profile"`
	MaxResults       int    `json:"max_results"`
	MaxDocumentBytes int    `json:"max_document_bytes"`
}

type Sist2Config struct {
	Enabled  bool   `json:"enabled"`
	Binary   string `json:"binary"`
	IndexDir string `json:"index_dir"`
}

// Default returns a complete local-development configuration.
func Default() Config {
	return Config{
		Server: ServerConfig{
			ListenAddr:    "127.0.0.1:8080",
			PublicBaseURL: "http://127.0.0.1:8080",
		},
		Data: DataConfig{
			Directory:     "./data",
			DatabasePath:  "./data/notes.sqlite",
			AssetStore:    "./data/assets",
			ProjectionDir: "./data/projections",
		},
		Search: SearchConfig{
			DefaultLimit: 20,
			MaxLimit:     100,
			MaxOffset:    10000,
		},
		MCP: MCPConfig{
			Enabled:          true,
			DefaultProfile:   "read-only",
			MaxResults:       10,
			MaxDocumentBytes: 65536,
		},
		Sist2: Sist2Config{
			Enabled:  false,
			Binary:   "sist2",
			IndexDir: "./data/sist2",
		},
	}
}

// Load reads a tiny, documented subset of YAML used by config/config.example.yaml.
// Unsupported keys are ignored so future config files can be introduced without
// breaking older binaries. A production implementation may replace this with a
// pinned YAML/TOML library after dependency policy is settled.
func Load(path string) (Config, error) {
	cfg := Default()
	path = strings.TrimSpace(path)
	if path == "" {
		return cfg, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()
	cfg.ConfigPath = path

	scanner := bufio.NewScanner(file)
	section := ""
	subsection := ""
	for scanner.Scan() {
		raw := scanner.Text()
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := leadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			// List values are retained as policy documentation for now. Remote media
			// enforcement will parse these once that service is implemented.
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return Config{}, fmt.Errorf("parse config %q: invalid line %q", path, raw)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			if indent == 0 {
				section = key
				subsection = ""
			} else {
				subsection = key
			}
			continue
		}
		if indent <= 2 {
			subsection = ""
		}
		applyScalar(&cfg, section, subsection, key, parseScalar(value))
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return cfg, nil
}

// LoadDefaultOrExample loads config/config.example.yaml when it exists. This is
// useful for the source checkout while still allowing installed binaries to run
// with compiled defaults when the example file is absent.
func LoadDefaultOrExample() (Config, error) {
	const examplePath = "config/config.example.yaml"
	if _, err := os.Stat(examplePath); err == nil {
		return Load(examplePath)
	} else if err != nil && !os.IsNotExist(err) {
		return Config{}, err
	}
	return Default(), nil
}

// EnsureDirectories creates the local storage roots required by the MVP service.
func EnsureDirectories(cfg Config) error {
	paths := []string{
		cfg.Data.Directory,
		filepath.Dir(cfg.Data.DatabasePath),
		cfg.Data.AssetStore,
		cfg.Data.ProjectionDir,
		cfg.Sist2.IndexDir,
	}
	seen := map[string]bool{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || path == ":memory:" || seen[path] {
			continue
		}
		seen[path] = true
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", path, err)
		}
	}
	return nil
}

func stripComment(line string) string {
	inQuote := false
	for i, r := range line {
		switch r {
		case '"':
			inQuote = !inQuote
		case '#':
			if !inQuote {
				return line[:i]
			}
		}
	}
	return line
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func parseScalar(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
		unquoted, err := strconv.Unquote(value)
		if err == nil {
			return unquoted
		}
	}
	return value
}

func applyScalar(cfg *Config, section, subsection, key, value string) {
	switch section {
	case "server":
		applyServer(&cfg.Server, key, value)
	case "data":
		applyData(&cfg.Data, key, value)
	case "search":
		applySearch(&cfg.Search, key, value)
	case "mcp":
		applyMCP(&cfg.MCP, key, value)
	case "sist2":
		applySist2(&cfg.Sist2, key, value)
	case "remote_media":
		_ = subsection
		// Documented for later implementation. Keep parsing permissive now.
	}
}

func applyServer(cfg *ServerConfig, key, value string) {
	switch key {
	case "listen_addr":
		cfg.ListenAddr = value
	case "public_base_url":
		cfg.PublicBaseURL = value
	}
}

func applyData(cfg *DataConfig, key, value string) {
	switch key {
	case "directory":
		cfg.Directory = value
	case "database_path":
		cfg.DatabasePath = value
	case "asset_store":
		cfg.AssetStore = value
	case "projection_dir":
		cfg.ProjectionDir = value
	}
}

func applySearch(cfg *SearchConfig, key, value string) {
	switch key {
	case "default_limit":
		cfg.DefaultLimit = parseInt(value, cfg.DefaultLimit)
	case "max_limit":
		cfg.MaxLimit = parseInt(value, cfg.MaxLimit)
	case "max_offset":
		cfg.MaxOffset = parseInt(value, cfg.MaxOffset)
	}
}

func applyMCP(cfg *MCPConfig, key, value string) {
	switch key {
	case "enabled":
		cfg.Enabled = parseBool(value, cfg.Enabled)
	case "default_profile":
		cfg.DefaultProfile = value
	case "max_results":
		cfg.MaxResults = parseInt(value, cfg.MaxResults)
	case "max_document_bytes":
		cfg.MaxDocumentBytes = parseInt(value, cfg.MaxDocumentBytes)
	}
}

func applySist2(cfg *Sist2Config, key, value string) {
	switch key {
	case "enabled":
		cfg.Enabled = parseBool(value, cfg.Enabled)
	case "binary":
		cfg.Binary = value
	case "index_dir":
		cfg.IndexDir = value
	}
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseBool(value string, fallback bool) bool {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
