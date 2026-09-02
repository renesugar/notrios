package profiles

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/paths"
	"github.com/renesugar/notrios/internal/store"
)

const (
	SyncNone      = "none"
	SyncDirectory = "directory"
	SyncREST      = "rest"
)

var ErrProfileCollision = errors.New("runtime profile collision")

// CreateOptions is the explicit local state needed to create one runtime
// profile. Empty storage paths are derived under the registry's config root.
type CreateOptions struct {
	Name             string
	RegistryPath     string
	DataDirectory    string
	DatabasePath     string
	AssetStore       string
	ListenAddr       string
	PublicBaseURL    string
	SyncTarget       string
	SyncDirectory    string
	SyncRESTBaseURL  string
	CredentialRef    string
	CopiedDatabaseAs string // empty, adopt, or fork
}

type Issue struct {
	Profile string `json:"profile,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationReport struct {
	Registry string  `json:"registry"`
	Valid    bool    `json:"valid"`
	Checked  int     `json:"checked"`
	Issues   []Issue `json:"issues"`
}

// View is the redacted runtime-profile representation returned by CLI show.
type View struct {
	Name                 string `json:"name"`
	ProfileID            string `json:"profile_id"`
	DatabaseID           string `json:"database_id"`
	ReplicaID            string `json:"replica_id"`
	ConfigPath           string `json:"config_path"`
	DatabasePath         string `json:"database_path"`
	AssetStore           string `json:"asset_store"`
	ListenAddr           string `json:"listen_addr"`
	PublicBaseURL        string `json:"public_base_url"`
	SyncTarget           string `json:"sync_target"`
	SyncDirectory        string `json:"sync_directory,omitempty"`
	SyncRESTBaseURL      string `json:"sync_rest_base_url,omitempty"`
	CredentialConfigured bool   `json:"credential_configured"`
}

// Create creates or opens the explicitly named database, binds its current
// identity into a generated owner-only config, validates it against every
// registered profile, and only then publishes the registry entry.
func Create(ctx context.Context, options CreateOptions) (Profile, error) {
	registryPath, err := absoluteClean(options.RegistryPath)
	if err != nil {
		return Profile{}, err
	}
	registry, err := Load(registryPath)
	if err != nil {
		return Profile{}, err
	}
	if _, exists := registry.ByName(options.Name); exists {
		return Profile{}, fmt.Errorf("%w: profile name %q is already registered", ErrProfileCollision, options.Name)
	}

	profileID, err := store.NewID("profile")
	if err != nil {
		return Profile{}, err
	}
	// The generated config sits beside the registry, which is correct: it is a
	// config file and the registry is in the config root.
	configPath := filepath.Join(filepath.Dir(registryPath), "profiles", profileID+".yaml")

	// The profile's *data* does not. It used to default to
	// <config>/profiles/<id>/data, which put a user's database, asset store,
	// projections, search index and quarantine inside ~/.config -- the one root
	// they are most likely to sync with a dotfile manager or commit to a
	// repository. It now defaults under the resolved data root.
	//
	// An explicit --data-dir still wins, and an existing profile is untouched:
	// its paths are recorded in the registry and read from there, so this
	// changes where the *next* profile is created and moves nothing.
	// Deriving this from an explicit --db was tried and reverted: two profiles
	// whose databases sit in one directory would then share every derived root,
	// and the collision check correctly refused them. A profile's cache and
	// state are per-profile by design, keyed by profile ID under the user's
	// roots, which is what the backup and purge categories want.
	dataDir := strings.TrimSpace(options.DataDirectory)
	explicitLocation := dataDir != ""
	if dataDir == "" {
		dataRoot, err := paths.Root(paths.RootData)
		if err != nil {
			return Profile{}, fmt.Errorf("the profile data location could not be resolved: %w", err)
		}
		dataDir = filepath.Join(dataRoot, "profiles", profileID)
	}
	dataDir, err = absoluteClean(dataDir)
	if err != nil {
		return Profile{}, err
	}
	databasePath := strings.TrimSpace(options.DatabasePath)
	if databasePath == "" {
		databasePath = filepath.Join(dataDir, "notes.sqlite")
	}
	databasePath, err = absoluteClean(databasePath)
	if err != nil {
		return Profile{}, err
	}
	assetStore := strings.TrimSpace(options.AssetStore)
	if assetStore == "" {
		assetStore = filepath.Join(dataDir, "assets")
	}
	assetStore, err = absoluteClean(assetStore)
	if err != nil {
		return Profile{}, err
	}
	if err := validate(Profile{Name: options.Name, DatabaseID: "pending", DatabasePath: databasePath}); err != nil {
		return Profile{}, err
	}
	for _, existing := range registry.Profiles {
		if pathsEqual(existing.DatabasePath, databasePath) {
			return Profile{}, fmt.Errorf("%w: database path is already owned by profile %q", ErrProfileCollision, existing.Name)
		}
	}

	cfg := config.Default()
	cfg.ConfigPath = configPath
	cfg.Profile = config.ProfileConfig{ID: profileID, Name: options.Name, RegistryPath: registryPath}
	cfg.Server.ListenAddr = defaultString(options.ListenAddr, "127.0.0.1:8080")
	cfg.Server.PublicBaseURL = defaultString(options.PublicBaseURL, "http://"+cfg.Server.ListenAddr)
	cfg.Data.Directory = dataDir
	cfg.Data.DatabasePath = databasePath
	cfg.Data.AssetStore = assetStore
	// Derived roots follow H3's categories rather than all hanging off the data
	// directory: projections and the search index are rebuildable, so they are
	// cache; the quarantine and the sync spools are state. An explicit
	// --data-dir keeps everything together under it, because a caller who named
	// one directory meant one directory.
	stateDir, cacheDir := dataDir, dataDir
	if !explicitLocation {
		if resolved, err := paths.Root(paths.RootState); err == nil && resolved != "" {
			stateDir, err = absoluteClean(filepath.Join(resolved, "profiles", profileID))
			if err != nil {
				return Profile{}, err
			}
		}
		if resolved, err := paths.Root(paths.RootCache); err == nil && resolved != "" {
			cacheDir, err = absoluteClean(filepath.Join(resolved, "profiles", profileID))
			if err != nil {
				return Profile{}, err
			}
		}
	}
	cfg.Data.StateDir = stateDir
	cfg.Data.CacheDir = cacheDir
	cfg.Data.ProjectionDir = filepath.Join(cacheDir, "projections")
	cfg.SearchSidecar.IndexDir = filepath.Join(cacheDir, "search-index")
	cfg.RemoteMedia.QuarantineDir = filepath.Join(stateDir, "quarantine")
	cfg.Sync = config.SyncConfig{
		Target:        defaultString(strings.ToLower(strings.TrimSpace(options.SyncTarget)), SyncNone),
		Directory:     strings.TrimSpace(options.SyncDirectory),
		RESTBaseURL:   strings.TrimSpace(options.SyncRESTBaseURL),
		CredentialRef: strings.TrimSpace(options.CredentialRef),
	}
	if err := validateRuntimeConfig(cfg); err != nil {
		return Profile{}, err
	}
	if issues := staticCollisions(registry, cfg, ""); len(issues) > 0 {
		return Profile{}, fmt.Errorf("%w: %s", ErrProfileCollision, issues[0].Message)
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		return Profile{}, err
	}
	st, err := store.OpenSQLiteWithAssetStore(databasePath, assetStore)
	if err != nil {
		return Profile{}, err
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		return Profile{}, err
	}
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		return Profile{}, err
	}
	duplicates := replicaOwners(ctx, registry, identity.ReplicaID)
	copyAction := strings.ToLower(strings.TrimSpace(options.CopiedDatabaseAs))
	if len(duplicates) > 0 {
		switch copyAction {
		case "adopt":
			identity, err = st.RotateReplicaIdentity(ctx)
		case "fork":
			var databaseID string
			databaseID, err = store.NewID("db")
			if err == nil {
				identity, err = st.AdoptDatabaseIdentity(ctx, databaseID)
			}
		case "":
			return Profile{}, fmt.Errorf("%w: replica %s is already registered by %s; this looks like a copied database, use --copied-database-as adopt|fork", ErrProfileCollision, identity.ReplicaID, strings.Join(duplicates, ", "))
		default:
			return Profile{}, fmt.Errorf("copied database action must be adopt or fork")
		}
		if err != nil {
			return Profile{}, err
		}
	} else if copyAction != "" {
		return Profile{}, fmt.Errorf("--copied-database-as requires a duplicate replica identity in this registry")
	}

	profile := Profile{
		Name: options.Name, ProfileID: profileID, DatabaseID: identity.DatabaseID,
		ReplicaID: identity.ReplicaID, DatabasePath: databasePath, AssetStore: assetStore,
		ConfigPath: configPath, RegisteredAt: time.Now().UTC(),
	}
	updated, err := registry.Upsert(profile)
	if err != nil {
		return Profile{}, err
	}
	if err := config.WriteProfileFile(configPath, cfg); err != nil {
		return Profile{}, err
	}
	if err := updated.Save(registryPath); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func Show(registryPath, name string) (View, error) {
	registry, err := Load(registryPath)
	if err != nil {
		return View{}, err
	}
	profile, ok := registry.ByName(name)
	if !ok {
		return View{}, fmt.Errorf("%w: %q", ErrUnknownProfile, name)
	}
	if profile.ConfigPath == "" {
		return View{}, fmt.Errorf("%w: %q is a legacy stable-link routing entry, not a runtime profile", ErrInvalidProfile, name)
	}
	cfg, err := config.Load(profile.ConfigPath)
	if err != nil {
		return View{}, err
	}
	return View{
		Name: profile.Name, ProfileID: profile.ProfileID, DatabaseID: profile.DatabaseID,
		ReplicaID: profile.ReplicaID, ConfigPath: profile.ConfigPath,
		DatabasePath: cfg.Data.DatabasePath, AssetStore: cfg.Data.AssetStore,
		ListenAddr: cfg.Server.ListenAddr, PublicBaseURL: cfg.Server.PublicBaseURL,
		SyncTarget: cfg.Sync.Target, SyncDirectory: cfg.Sync.Directory,
		SyncRESTBaseURL: cfg.Sync.RESTBaseURL, CredentialConfigured: cfg.Sync.CredentialRef != "",
	}, nil
}

// Summary is one row of `profile list`: enough to pick a profile and see which
// port it will use, without opening its database.
type Summary struct {
	Name         string `json:"name"`
	ProfileID    string `json:"profile_id,omitempty"`
	DatabaseID   string `json:"database_id,omitempty"`
	DatabasePath string `json:"database_path,omitempty"`
	ConfigPath   string `json:"config_path,omitempty"`
	ListenAddr   string `json:"listen_addr,omitempty"`
	SyncTarget   string `json:"sync_target,omitempty"`
	// Kind is "runtime" for a full profile or "routing" for a legacy
	// stable-link entry that has no config and cannot be started.
	Kind string `json:"kind"`
	// Problem explains why this row is missing its details, when it is. A
	// listing is the first thing a user runs, so one unreadable profile
	// reports itself rather than hiding the other nine behind an error.
	Problem string `json:"problem,omitempty"`
}

// List summarizes every registered profile.
//
// The listen address is read from each profile's own config rather than stored
// in the registry: the config file is the thing that decides the port, and a
// second copy in the registry would be free to disagree with it the moment
// somebody edited the file.
func List(registryPath string) ([]Summary, error) {
	registry, err := Load(registryPath)
	if err != nil {
		return nil, err
	}
	summaries := make([]Summary, 0, len(registry.Profiles))
	for _, profile := range registry.Profiles {
		summary := Summary{
			Name: profile.Name, ProfileID: profile.ProfileID,
			DatabaseID: profile.DatabaseID, DatabasePath: profile.DatabasePath,
			ConfigPath: profile.ConfigPath, Kind: "runtime",
		}
		if strings.TrimSpace(profile.ConfigPath) == "" {
			summary.Kind = "routing"
			summary.Problem = "legacy stable-link routing entry: no runtime config, cannot be started"
			summaries = append(summaries, summary)
			continue
		}
		cfg, err := config.Load(profile.ConfigPath)
		if err != nil {
			summary.Problem = fmt.Sprintf("config could not be read: %v", err)
			summaries = append(summaries, summary)
			continue
		}
		summary.ListenAddr = cfg.Server.ListenAddr
		summary.SyncTarget = cfg.Sync.Target
		if cfg.Data.DatabasePath != "" {
			summary.DatabasePath = cfg.Data.DatabasePath
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// Validate checks explicit registry/config/database bindings only. It never
// scans for unregistered databases or opens an arbitrary discovered path.
func Validate(ctx context.Context, registryPath, onlyName string) ValidationReport {
	report := ValidationReport{Registry: registryPath, Valid: true, Issues: []Issue{}}
	registry, err := Load(registryPath)
	if err != nil {
		report.Valid = false
		report.Issues = append(report.Issues, Issue{Code: "registry_invalid", Message: err.Error()})
		return report
	}
	if info, err := os.Stat(registryPath); err == nil && info.Mode().Perm() != 0o600 {
		report.Issues = append(report.Issues, Issue{Code: "registry_permissions", Message: "registry permissions must be 0600"})
	}
	selected := map[string]bool{}
	for _, profile := range registry.Profiles {
		if onlyName == "" || strings.EqualFold(profile.Name, onlyName) {
			selected[strings.ToLower(profile.Name)] = true
			report.Checked++
		}
	}
	if onlyName != "" && report.Checked == 0 {
		report.Issues = append(report.Issues, Issue{Profile: onlyName, Code: "unknown_profile", Message: "no such profile"})
	}

	configs := map[string]config.Config{}
	identities := map[string]store.DatabaseIdentity{}
	for _, profile := range registry.Profiles {
		identity, identityErr := readIdentity(ctx, profile.DatabasePath, profile.AssetStore)
		if identityErr != nil {
			addSelectedIssue(&report, selected, profile.Name, "stale_database", identityErr.Error())
		} else {
			identities[profile.Name] = identity
			if profile.ReplicaID != "" && (identity.DatabaseID != profile.DatabaseID || identity.ReplicaID != profile.ReplicaID) {
				addSelectedIssue(&report, selected, profile.Name, "identity_changed", "database identity no longer matches the registered profile; explicitly re-register after adopt or fork")
			}
		}
		if profile.ConfigPath == "" {
			continue
		}
		cfg, cfgErr := config.Load(profile.ConfigPath)
		if cfgErr != nil {
			addSelectedIssue(&report, selected, profile.Name, "stale_config", cfgErr.Error())
			continue
		}
		configs[profile.Name] = cfg
		if info, statErr := os.Stat(profile.ConfigPath); statErr != nil || info.Mode().Perm() != 0o600 {
			addSelectedIssue(&report, selected, profile.Name, "config_permissions", "profile config permissions must be 0600")
		}
		if cfg.Profile.ID != profile.ProfileID || !strings.EqualFold(cfg.Profile.Name, profile.Name) || !pathsEqual(cfg.Profile.RegistryPath, registryPath) {
			addSelectedIssue(&report, selected, profile.Name, "stale_binding", "profile config does not match its registry binding")
		}
		if err := validateRuntimeConfig(cfg); err != nil {
			addSelectedIssue(&report, selected, profile.Name, "invalid_config", err.Error())
		}
		if !pathsEqual(cfg.Data.DatabasePath, profile.DatabasePath) || !pathsEqual(cfg.Data.AssetStore, profile.AssetStore) {
			addSelectedIssue(&report, selected, profile.Name, "stale_paths", "registry and profile config storage paths differ")
		}
	}
	for _, issue := range allCollisions(registry, configs, identities) {
		if onlyName == "" || strings.Contains(strings.ToLower(issue.Message), strings.ToLower(onlyName)) || strings.EqualFold(issue.Profile, onlyName) {
			report.Issues = append(report.Issues, issue)
		}
	}
	report.Valid = len(report.Issues) == 0
	return report
}

// ValidateStartup binds a service created from a generated config to the
// registry entry and database identity it was approved for.
func ValidateStartup(ctx context.Context, cfg config.Config) error {
	if strings.TrimSpace(cfg.Profile.ID) == "" {
		return nil // backward-compatible unmanaged/default config
	}
	report := Validate(ctx, cfg.Profile.RegistryPath, cfg.Profile.Name)
	if !report.Valid {
		return fmt.Errorf("runtime profile %q failed validation: %s: %s", cfg.Profile.Name, report.Issues[0].Code, report.Issues[0].Message)
	}
	registry, err := Load(cfg.Profile.RegistryPath)
	if err != nil {
		return err
	}
	profile, ok := registry.ByName(cfg.Profile.Name)
	if !ok || profile.ProfileID != cfg.Profile.ID || !pathsEqual(profile.ConfigPath, cfg.ConfigPath) ||
		!pathsEqual(profile.DatabasePath, cfg.Data.DatabasePath) || !pathsEqual(profile.AssetStore, cfg.Data.AssetStore) {
		return fmt.Errorf("runtime profile %q config is not the registered config", cfg.Profile.Name)
	}
	return nil
}

func CheckListenAvailable(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen address %s is unavailable: %w", addr, err)
	}
	return listener.Close()
}

func readIdentity(ctx context.Context, databasePath, assetStore string) (store.DatabaseIdentity, error) {
	if _, err := os.Stat(databasePath); err != nil {
		return store.DatabaseIdentity{}, fmt.Errorf("database %s: %w", databasePath, err)
	}
	st, err := store.OpenSQLiteWithAssetStore(databasePath, assetStore)
	if err != nil {
		return store.DatabaseIdentity{}, err
	}
	defer st.Close()
	return st.GetDatabaseIdentity(ctx)
}

func replicaOwners(ctx context.Context, registry Registry, replicaID string) []string {
	owners := []string{}
	for _, profile := range registry.Profiles {
		identity, err := readIdentity(ctx, profile.DatabasePath, profile.AssetStore)
		if err == nil && identity.ReplicaID == replicaID {
			owners = append(owners, profile.Name)
		}
	}
	sort.Strings(owners)
	return owners
}

func validateRuntimeConfig(cfg config.Config) error {
	if strings.TrimSpace(cfg.Profile.ID) == "" || strings.TrimSpace(cfg.Profile.Name) == "" || strings.TrimSpace(cfg.Profile.RegistryPath) == "" {
		return fmt.Errorf("runtime profile identity, name, and registry path are required")
	}
	for _, path := range []string{cfg.ConfigPath, cfg.Profile.RegistryPath, cfg.Data.Directory, cfg.Data.DatabasePath, cfg.Data.AssetStore, cfg.Data.ProjectionDir, cfg.SearchSidecar.IndexDir, cfg.RemoteMedia.QuarantineDir} {
		if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) {
			return fmt.Errorf("runtime profile paths must be absolute")
		}
	}
	host, port, err := net.SplitHostPort(cfg.Server.ListenAddr)
	if err != nil || port == "" {
		return fmt.Errorf("listen_addr must be host:port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("listen_addr port must be between 1 and 65535")
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("G3 runtime profiles are loopback-only")
		}
	}
	parsedURL, err := url.Parse(cfg.Server.PublicBaseURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return fmt.Errorf("public_base_url must be an absolute HTTP(S) URL")
	}
	switch cfg.Sync.Target {
	case SyncNone:
		if cfg.Sync.Directory != "" || cfg.Sync.RESTBaseURL != "" || cfg.Sync.CredentialRef != "" {
			return fmt.Errorf("sync target none may not carry transport or credential settings")
		}
	case SyncDirectory:
		if !filepath.IsAbs(cfg.Sync.Directory) || cfg.Sync.RESTBaseURL != "" {
			return fmt.Errorf("directory sync requires one absolute directory and no REST URL")
		}
	case SyncREST:
		u, parseErr := url.Parse(cfg.Sync.RESTBaseURL)
		if parseErr != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || cfg.Sync.Directory != "" {
			return fmt.Errorf("REST sync requires one absolute REST URL and no directory")
		}
		if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
			return fmt.Errorf("non-loopback REST sync URLs require HTTPS")
		}
	default:
		return fmt.Errorf("sync target must be none, directory, or rest")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func staticCollisions(registry Registry, candidate config.Config, skipName string) []Issue {
	configs := map[string]config.Config{"__candidate__": candidate}
	copyRegistry := registry
	copyRegistry.Profiles = append(copyRegistry.Profiles, Profile{Name: "__candidate__", DatabasePath: candidate.Data.DatabasePath, AssetStore: candidate.Data.AssetStore, ConfigPath: candidate.ConfigPath})
	for _, profile := range registry.Profiles {
		if strings.EqualFold(profile.Name, skipName) || profile.ConfigPath == "" {
			continue
		}
		if cfg, err := config.Load(profile.ConfigPath); err == nil {
			configs[profile.Name] = cfg
		}
	}
	return allCollisions(copyRegistry, configs, nil)
}

func allCollisions(registry Registry, configs map[string]config.Config, identities map[string]store.DatabaseIdentity) []Issue {
	issues := []Issue{}
	for i := 0; i < len(registry.Profiles); i++ {
		left := registry.Profiles[i]
		for j := i + 1; j < len(registry.Profiles); j++ {
			right := registry.Profiles[j]
			if pathsEqual(left.DatabasePath, right.DatabasePath) {
				issues = append(issues, collisionIssue("database_path_collision", left, right, left.DatabasePath))
			}
			if left.AssetStore != "" && pathsEqual(left.AssetStore, right.AssetStore) {
				issues = append(issues, collisionIssue("asset_path_collision", left, right, left.AssetStore))
			}
			for _, lp := range []string{left.DatabasePath, left.AssetStore} {
				for _, rp := range []string{right.DatabasePath, right.AssetStore} {
					if lp != "" && rp != "" && pathsOverlap(lp, rp) && !pathsEqual(lp, rp) {
						issues = append(issues, collisionIssue("storage_path_collision", left, right, lp))
					}
				}
			}
			leftCfg, leftOK := configs[left.Name]
			rightCfg, rightOK := configs[right.Name]
			if leftOK && rightOK {
				if normalizeListen(leftCfg.Server.ListenAddr) == normalizeListen(rightCfg.Server.ListenAddr) {
					issues = append(issues, collisionIssue("port_collision", left, right, leftCfg.Server.ListenAddr))
				}
				leftPaths := runtimePaths(left, leftCfg)
				rightPaths := runtimePaths(right, rightCfg)
				for _, lp := range leftPaths {
					for _, rp := range rightPaths {
						if pathsOverlap(lp, rp) {
							issues = append(issues, collisionIssue("runtime_path_collision", left, right, lp))
						}
					}
				}
			}
			if identities != nil {
				li, lok := identities[left.Name]
				ri, rok := identities[right.Name]
				if lok && rok && li.ReplicaID == ri.ReplicaID {
					issues = append(issues, collisionIssue("duplicate_replica_id", left, right, li.ReplicaID))
				}
			}
		}
	}
	return issues
}

func runtimePaths(profile Profile, cfg config.Config) []string {
	paths := []string{profile.ConfigPath, cfg.Data.Directory, cfg.Data.DatabasePath, cfg.Data.AssetStore, cfg.Data.ProjectionDir, cfg.SearchSidecar.IndexDir, cfg.RemoteMedia.QuarantineDir}
	if cfg.Sync.Target == SyncDirectory {
		paths = append(paths, cfg.Sync.Directory)
	}
	return paths
}

func collisionIssue(code string, left, right Profile, value string) Issue {
	return Issue{Profile: left.Name, Code: code, Message: fmt.Sprintf("profiles %q and %q collide on %s", left.Name, right.Name, value)}
}

func addSelectedIssue(report *ValidationReport, selected map[string]bool, profile, code, message string) {
	if selected[strings.ToLower(profile)] {
		report.Issues = append(report.Issues, Issue{Profile: profile, Code: code, Message: message})
	}
}

func absoluteClean(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(abs); resolveErr == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(abs), nil
}

func pathsEqual(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	l, _ := absoluteClean(left)
	r, _ := absoluteClean(right)
	return l == r
}

func pathsOverlap(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	l, _ := absoluteClean(left)
	r, _ := absoluteClean(right)
	if l == r {
		return true
	}
	for _, pair := range [][2]string{{l, r}, {r, l}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func normalizeListen(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return strings.ToLower(addr)
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		host = "loopback"
	}
	return strings.ToLower(host) + ":" + port
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
