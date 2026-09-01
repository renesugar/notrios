package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/paths"
)

// Config contains the runtime settings used by notriosd and notriosctl.
// It intentionally avoids third-party YAML dependencies until the project
// chooses and pins the long-term configuration library.
//
//notrios:doc user configuration-keys
//notrios:help service configuration-reference
//notrios:enumerates go:github.com/renesugar/notrios/internal/config#Config
type Config struct {
	ConfigPath    string              `json:"config_path,omitempty"`
	Profile       ProfileConfig       `json:"profile"`
	Server        ServerConfig        `json:"server"`
	Data          DataConfig          `json:"data"`
	Sync          SyncConfig          `json:"sync"`
	Search        SearchConfig        `json:"search"`
	MCP           MCPConfig           `json:"mcp"`
	SearchSidecar SearchSidecarConfig `json:"search_sidecar"`
	RemoteMedia   RemoteMediaConfig   `json:"remote_media"`
	Retention     RetentionConfig     `json:"retention"`
}

// ProfileConfig binds a generated per-profile config file back to the local
// registry entry that owns it. It is local routing state and is never synced.
type ProfileConfig struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	RegistryPath string `json:"registry_path,omitempty"`
}

// SyncConfig reserves the local transport choice and credential-store
// reference for later v0.7 slices. A non-none target establishes G4's explicit
// local journal boundary at service startup; no transport or credential use is
// implemented yet. Target "none" remains the safe, complete default.
type SyncConfig struct {
	Target        string         `json:"target"`
	Directory     string         `json:"directory,omitempty"`
	RESTBaseURL   string         `json:"rest_base_url,omitempty"`
	CredentialRef string         `json:"credential_ref,omitempty"`
	REST          SyncRESTConfig `json:"rest"`
}

// SyncRESTConfig is v0.7 G13's transport policy for the sync surface.
//
// It is off by default, and that default is the product: Notrios is a local
// application whose REST API has no general authentication, and turning on a
// peer-authenticated surface is a decision a user makes rather than a state
// they arrive in.
type SyncRESTConfig struct {
	// Enabled exposes /api/v1/sync/... to authenticated peers. Nothing else on
	// the API becomes remotely reachable or remotely authorized by it.
	Enabled bool `json:"enabled"`
	// RequireTLS refuses to serve the sync surface over plaintext on anything
	// but a loopback address. It defaults to true and should stay true: a peer
	// credential is a signature rather than a bearer token, so plaintext does
	// not leak a reusable secret, but every byte of every note would be in the
	// clear once G14 carries data.
	RequireTLS bool `json:"require_tls"`
	// TLSCertFile and TLSKeyFile enable HTTPS for the whole service.
	TLSCertFile string `json:"tls_cert_file,omitempty"`
	TLSKeyFile  string `json:"tls_key_file,omitempty"`
	// MaxBodyBytes bounds an authenticated request body.
	MaxBodyBytes int64 `json:"max_body_bytes,omitempty"`
	// RequestsPerMinute and Burst bound one peer; FailuresPerMinute bounds how
	// fast one address can guess.
	RequestsPerMinute int `json:"requests_per_minute,omitempty"`
	Burst             int `json:"burst,omitempty"`
	FailuresPerMinute int `json:"failures_per_minute,omitempty"`
	// KeyFile is the local sync key material. Empty uses the default path.
	KeyFile string `json:"key_file,omitempty"`
}

type ServerConfig struct {
	ListenAddr    string `json:"listen_addr"`
	PublicBaseURL string `json:"public_base_url"`
	// WebDir is the directory holding the built web interface (the one with
	// index.html). Empty searches the default locations: the working
	// directory, then the executable's own directory and its parent. Set it
	// when the binary lives apart from its assets.
	WebDir string `json:"web_dir,omitempty"`
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
}

type MCPConfig struct {
	Enabled bool `json:"enabled"`
	// DefaultScope is the MCP tool scope: search-only, read-only, editor, or
	// organizer. Empty means read-only.
	DefaultScope string `json:"default_scope"`
	// DefaultProfile is the deprecated former name of DefaultScope. It is still
	// read so existing configs keep working; when both are set the narrower
	// wins, because a key that silently stops applying must never widen what an
	// agent may do.
	DefaultProfile string `json:"default_profile"`
	// SyncScope is orthogonal to the content scope. Empty/disabled is the
	// default; status permits bounded operational reads and control additionally
	// permits ordinary incremental and resource-fetch jobs.
	SyncScope        string `json:"sync_scope"`
	MaxResults       int    `json:"max_results"`
	MaxDocumentBytes int    `json:"max_document_bytes"`
}

// SearchSidecarConfig configures the optional derived search sidecar
// (Recoll; formerly sist2). The sidecar is an external user-installed
// process and is never linked into the service.
type SearchSidecarConfig struct {
	Enabled  bool   `json:"enabled"`
	Binary   string `json:"binary"`
	IndexDir string `json:"index_dir"`
}

// RetentionConfig controls when unreferenced logical resources become
// eligible for garbage collection. Resources orphaned by a permanent note
// purge receive a separate, longer recovery window.
type RetentionConfig struct {
	UnreferencedResourceDays int `json:"unreferenced_resource_days"`
	PurgedResourceDays       int `json:"purged_resource_days"`
	SyncHistoryDays          int `json:"sync_history_days"`
	SyncPeerWarningDays      int `json:"sync_peer_warning_days"`
}

func (c RetentionConfig) UnreferencedDuration() time.Duration {
	return retentionDays(c.UnreferencedResourceDays)
}

func (c RetentionConfig) PurgedResourceDuration() time.Duration {
	return retentionDays(c.PurgedResourceDays)
}

func (c RetentionConfig) SyncHistoryDuration() time.Duration {
	return retentionDays(c.SyncHistoryDays)
}

func (c RetentionConfig) SyncPeerWarningDuration() time.Duration {
	return retentionDays(c.SyncPeerWarningDays)
}

func retentionDays(days int) time.Duration {
	if days <= 0 {
		return 0
	}
	const maxDays = int((1<<63 - 1) / int64(24*time.Hour))
	if days > maxDays {
		days = maxDays
	}
	return time.Duration(days) * 24 * time.Hour
}

// RemoteMediaConfig is the remote-media localization policy
// (SECURITY_AND_MEDIA_POLICY.md): what may be downloaded, from where, and
// under which limits. The zero policy is restrictive — unknown domains fall
// back to DefaultAction ("review" means: report, never auto-download).
type RemoteMediaConfig struct {
	// DefaultAction applies to URLs matched by no domain list: allow, block,
	// or review. Invalid configured values fall back to "review".
	DefaultAction        string   `json:"default_action"`
	AllowPrivateNetworks bool     `json:"allow_private_networks"`
	MaxRedirects         int      `json:"max_redirects"`
	FetchTimeoutSeconds  int      `json:"fetch_timeout_seconds"`
	BlockedSchemes       []string `json:"blocked_schemes"`
	BlockedDomains       []string `json:"blocked_domains"`
	AllowedDomains       []string `json:"allowed_domains"`
	ReviewDomains        []string `json:"review_domains"`
	// MaxBytes caps download sizes per media class (image/video/pdf),
	// parsed from human-readable values like "20MB".
	MaxBytes map[string]int64 `json:"max_bytes"`
	// QuarantineDir holds fetched bytes before policy admission to the
	// content-addressed asset store; it is never served.
	QuarantineDir string `json:"quarantine_dir"`
}

// MediaActions are the valid policy decisions for domains and DefaultAction.
var MediaActions = map[string]bool{"allow": true, "block": true, "review": true}

// Default returns the canonical local-development defaults for every runtime
// configuration group.
//
//notrios:doc user configuration-defaults
//notrios:help service configuration-reference
//notrios:enumerates go:github.com/renesugar/notrios/internal/config#Default
func Default() Config {
	return Config{
		Sync: SyncConfig{Target: "none", REST: SyncRESTConfig{RequireTLS: true}},
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
		},
		MCP: MCPConfig{
			Enabled: true,
			// Both scope keys are left empty on purpose. The resolver treats
			// "neither set" as read-only, and pre-filling the deprecated key
			// would make every default config look half-migrated and emit a
			// deprecation warning nobody caused.
			DefaultScope:     "",
			DefaultProfile:   "",
			MaxResults:       10,
			MaxDocumentBytes: 65536,
		},
		SearchSidecar: SearchSidecarConfig{
			Enabled:  false,
			Binary:   "recollindex",
			IndexDir: "./data/search-index",
		},
		RemoteMedia: RemoteMediaConfig{
			DefaultAction:        "review",
			AllowPrivateNetworks: false,
			MaxRedirects:         5,
			FetchTimeoutSeconds:  30,
			BlockedSchemes:       []string{"file", "data", "javascript", "ftp"},
			MaxBytes: map[string]int64{
				"image": 20 * 1024 * 1024,
				"video": 200 * 1024 * 1024,
				"pdf":   100 * 1024 * 1024,
			},
			QuarantineDir: "./data/quarantine",
		},
		Retention: RetentionConfig{
			UnreferencedResourceDays: 30,
			PurgedResourceDays:       90,
			SyncHistoryDays:          90,
			SyncPeerWarningDays:      30,
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
	// Tracks which config lists have received their first dash item, so a
	// configured list replaces the compiled default instead of appending.
	seenLists := map[string]bool{}
	for scanner.Scan() {
		raw := scanner.Text()
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := leadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			item := parseScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
			applyListItem(&cfg, section, subsection, item, seenLists)
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

// ExampleRelativePath is the checkout's sample configuration, read only in
// source mode.
const ExampleRelativePath = "config/config.example.yaml"

// LoadDefault loads the configuration for a process that was given no --config.
//
// The order is: the user's own <config>/config.yaml, then -- in a source
// checkout only -- the checkout's config/config.example.yaml, then compiled
// defaults.
//
// The source-mode restriction is the point of the change. This function used to
// stat config/config.example.yaml relative to the process working directory,
// which is right in a checkout and wrong once the binary is installed: the file
// that decides the database path, the listen address, the public base URL and
// the remote-media policy, including whether private networks may be fetched,
// was taken from whatever directory the user happened to be standing in.
//
// A missing or unreadable user config is not an error; a *malformed* one is.
// Falling back to compiled defaults because a file failed to parse would start
// the service with a policy the user did not choose and did not know about.
func LoadDefault() (Config, error) {
	if root, err := paths.ConfigRoot(); err == nil {
		candidate := filepath.Join(root, "config.yaml")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return Load(candidate)
		} else if !os.IsNotExist(statErr) {
			return Config{}, fmt.Errorf("read %s: %w", candidate, statErr)
		}
	}

	if executableDir, err := paths.ExecutableDir(); err == nil {
		if root, ok := paths.FindSourceCheckout(executableDir); ok {
			return loadExampleFrom(root)
		}
	}
	if workingDir, err := os.Getwd(); err == nil {
		if root, ok := paths.FindSourceCheckout(workingDir); ok {
			return loadExampleFrom(root)
		}
	}

	return Default(), nil
}

func loadExampleFrom(checkoutRoot string) (Config, error) {
	candidate := filepath.Join(checkoutRoot, ExampleRelativePath)
	if _, err := os.Stat(candidate); err == nil {
		return Load(candidate)
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("read %s: %w", candidate, err)
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
		cfg.SearchSidecar.IndexDir,
		cfg.RemoteMedia.QuarantineDir,
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
	case "profile":
		applyProfile(&cfg.Profile, key, value)
	case "data":
		applyData(&cfg.Data, key, value)
	case "sync":
		applySync(&cfg.Sync, subsection, key, value)
	case "search":
		applySearch(&cfg.Search, key, value)
	case "mcp":
		applyMCP(&cfg.MCP, key, value)
	case "search_sidecar":
		applySearchSidecar(&cfg.SearchSidecar, key, value)
	case "remote_media":
		applyRemoteMedia(&cfg.RemoteMedia, subsection, key, value)
	case "retention":
		applyRetention(&cfg.Retention, key, value)
	}
}

func applyProfile(cfg *ProfileConfig, key, value string) {
	switch key {
	case "id":
		cfg.ID = value
	case "name":
		cfg.Name = value
	case "registry_path":
		cfg.RegistryPath = value
	}
}

func applySync(cfg *SyncConfig, subsection, key, value string) {
	if subsection == "rest" {
		applySyncREST(&cfg.REST, key, value)
		return
	}
	switch key {
	case "target":
		cfg.Target = strings.ToLower(strings.TrimSpace(value))
	case "directory":
		cfg.Directory = value
	case "rest_base_url":
		cfg.RESTBaseURL = value
	case "credential_ref":
		cfg.CredentialRef = value
	}
}

func applySyncREST(cfg *SyncRESTConfig, key, value string) {
	switch key {
	case "enabled":
		cfg.Enabled = parseBool(value, cfg.Enabled)
	case "require_tls":
		cfg.RequireTLS = parseBool(value, cfg.RequireTLS)
	case "tls_cert_file":
		cfg.TLSCertFile = value
	case "tls_key_file":
		cfg.TLSKeyFile = value
	case "key_file":
		cfg.KeyFile = value
	case "max_body_bytes":
		if size, ok := parseByteSize(value); ok {
			cfg.MaxBodyBytes = size
		}
	case "requests_per_minute":
		cfg.RequestsPerMinute = parseNonNegativeInt(value, cfg.RequestsPerMinute)
	case "burst":
		cfg.Burst = parseNonNegativeInt(value, cfg.Burst)
	case "failures_per_minute":
		cfg.FailuresPerMinute = parseNonNegativeInt(value, cfg.FailuresPerMinute)
	}
}

func applyRetention(cfg *RetentionConfig, key, value string) {
	switch key {
	case "unreferenced_resource_days":
		cfg.UnreferencedResourceDays = parseNonNegativeInt(value, cfg.UnreferencedResourceDays)
	case "purged_resource_days":
		cfg.PurgedResourceDays = parseNonNegativeInt(value, cfg.PurgedResourceDays)
	case "sync_history_days":
		cfg.SyncHistoryDays = parseNonNegativeInt(value, cfg.SyncHistoryDays)
	case "sync_peer_warning_days":
		cfg.SyncPeerWarningDays = parseNonNegativeInt(value, cfg.SyncPeerWarningDays)
	}
}

func applyRemoteMedia(cfg *RemoteMediaConfig, subsection, key, value string) {
	if subsection == "max_bytes" {
		if size, ok := parseByteSize(value); ok {
			if cfg.MaxBytes == nil {
				cfg.MaxBytes = map[string]int64{}
			}
			cfg.MaxBytes[key] = size
		}
		return
	}
	switch key {
	case "default_action":
		if action := strings.ToLower(strings.TrimSpace(value)); MediaActions[action] {
			cfg.DefaultAction = action
		}
	case "allow_private_networks":
		cfg.AllowPrivateNetworks = parseBool(value, cfg.AllowPrivateNetworks)
	case "max_redirects":
		cfg.MaxRedirects = parseInt(value, cfg.MaxRedirects)
	case "fetch_timeout_seconds":
		cfg.FetchTimeoutSeconds = parseInt(value, cfg.FetchTimeoutSeconds)
	case "quarantine_dir":
		cfg.QuarantineDir = value
	case "blocked_schemes", "blocked_domains", "allowed_domains", "review_domains":
		if items, ok := parseInlineList(value); ok {
			if target := mediaListTarget(cfg, key); target != nil {
				*target = items
			}
		}
	}
}

// applyListItem consumes one "- value" list entry under a section/subsection.
// Only remote_media lists are recognized; the first configured item replaces
// the compiled default list.
func applyListItem(cfg *Config, section, subsection, item string, seenLists map[string]bool) {
	if section != "remote_media" || item == "" {
		return
	}
	target := mediaListTarget(&cfg.RemoteMedia, subsection)
	if target == nil {
		return
	}
	seenKey := section + "." + subsection
	if !seenLists[seenKey] {
		*target = nil
		seenLists[seenKey] = true
	}
	*target = append(*target, item)
}

func mediaListTarget(cfg *RemoteMediaConfig, key string) *[]string {
	switch key {
	case "blocked_schemes":
		return &cfg.BlockedSchemes
	case "blocked_domains":
		return &cfg.BlockedDomains
	case "allowed_domains":
		return &cfg.AllowedDomains
	case "review_domains":
		return &cfg.ReviewDomains
	}
	return nil
}

// parseInlineList parses a flow-style YAML list: ["file", "ftp", "data"].
func parseInlineList(value string) ([]string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, false
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return nil, true
	}
	parts := strings.Split(inner, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := parseScalar(strings.TrimSpace(part)); item != "" {
			items = append(items, item)
		}
	}
	return items, true
}

// parseByteSize parses human-readable sizes ("20MB", "512kb", "1048576").
func parseByteSize(value string) (int64, bool) {
	v := strings.ToUpper(strings.TrimSpace(value))
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(v, "GB"):
		multiplier, v = 1<<30, v[:len(v)-2]
	case strings.HasSuffix(v, "MB"):
		multiplier, v = 1<<20, v[:len(v)-2]
	case strings.HasSuffix(v, "KB"):
		multiplier, v = 1<<10, v[:len(v)-2]
	case strings.HasSuffix(v, "B"):
		v = v[:len(v)-1]
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n * multiplier, true
}

func applyServer(cfg *ServerConfig, key, value string) {
	switch key {
	case "listen_addr":
		cfg.ListenAddr = value
	case "public_base_url":
		cfg.PublicBaseURL = value
	case "web_dir":
		cfg.WebDir = value
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
	}
}

func applyMCP(cfg *MCPConfig, key, value string) {
	switch key {
	case "enabled":
		cfg.Enabled = parseBool(value, cfg.Enabled)
	case "default_scope":
		cfg.DefaultScope = value
	case "default_profile":
		// Deprecated spelling, still read so existing configs keep working.
		cfg.DefaultProfile = value
	case "sync_scope":
		cfg.SyncScope = strings.ToLower(strings.TrimSpace(value))
	case "max_results":
		cfg.MaxResults = parseInt(value, cfg.MaxResults)
	case "max_document_bytes":
		cfg.MaxDocumentBytes = parseInt(value, cfg.MaxDocumentBytes)
	}
}

func applySearchSidecar(cfg *SearchSidecarConfig, key, value string) {
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

func parseNonNegativeInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
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
