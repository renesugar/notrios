package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/profiles"
	"github.com/renesugar/notrios/internal/stablelink"
	"github.com/renesugar/notrios/internal/store"
)

// runProfile manages the local database registry that turns the logical
// database ID inside a stable link into a database on this machine.
func runProfile(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl profile register|list|forget [options]")
		os.Exit(2)
	}
	switch args[0] {
	case "register":
		runProfileRegister(args[1:])
	case "list":
		runProfileList(args[1:])
	case "forget":
		runProfileForget(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown profile command %q\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: notriosctl profile register|list|forget [options]")
		os.Exit(2)
	}
}

// runProfileRegister records a database under a local name. The database ID is
// read out of the database itself and never taken from a flag: a registry
// entry claiming an identity its database does not have would send links to
// the wrong notes.
func runProfileRegister(args []string) {
	fs := flag.NewFlagSet("notriosctl profile register", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	registryPath := fs.String("registry", "", "profile registry path (default: the user registry)")
	name := fs.String("name", "", "required: local profile name")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*name) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl profile register --name <profile> [--db path] [--registry path]")
		fs.PrintDefaults()
		os.Exit(2)
	}

	cfg := loadConfigForProfile(*configPath, *dbPath, *assetStore)
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	identity, err := st.GetDatabaseIdentity(context.Background())
	st.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	absoluteDB, err := filepath.Abs(cfg.Data.DatabasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	absoluteAssets := ""
	if strings.TrimSpace(cfg.Data.AssetStore) != "" {
		if absoluteAssets, err = filepath.Abs(cfg.Data.AssetStore); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	path := registryPathOrDefault(*registryPath)
	registry, err := profiles.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	updated, err := registry.Upsert(profiles.Profile{
		Name:         strings.TrimSpace(*name),
		DatabaseID:   identity.DatabaseID,
		DatabasePath: absoluteDB,
		AssetStore:   absoluteAssets,
		RegisteredAt: time.Now().UTC(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := updated.Save(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"registry":      path,
		"name":          strings.TrimSpace(*name),
		"database_id":   identity.DatabaseID,
		"replica_id":    identity.ReplicaID,
		"database_path": absoluteDB,
		"profiles":      len(updated.Profiles),
	})
}

func runProfileList(args []string) {
	fs := flag.NewFlagSet("notriosctl profile list", flag.ExitOnError)
	registryPath := fs.String("registry", "", "profile registry path (default: the user registry)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	path := registryPathOrDefault(*registryPath)
	registry, err := profiles.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{"registry": path, "profiles": registry.Profiles})
}

// runProfileForget removes a registry entry. It never touches the database the
// entry named.
func runProfileForget(args []string) {
	fs := flag.NewFlagSet("notriosctl profile forget", flag.ExitOnError)
	registryPath := fs.String("registry", "", "profile registry path (default: the user registry)")
	name := fs.String("name", "", "required: local profile name")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*name) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl profile forget --name <profile> [--registry path]")
		os.Exit(2)
	}
	path := registryPathOrDefault(*registryPath)
	registry, err := profiles.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	updated, err := registry.Remove(strings.TrimSpace(*name))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := updated.Save(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{"registry": path, "forgot": strings.TrimSpace(*name), "profiles": len(updated.Profiles)})
}

// runLink prints the external link for one of this database's notes, optionally
// anchored at a heading or a block.
func runLink(args []string) {
	fs := flag.NewFlagSet("notriosctl link", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	anchor := fs.String("anchor", "", "heading slug, heading text, block ID, or ^marker to anchor the link at")
	list := fs.Bool("list-anchors", false, "print the note's addressable anchors instead of a link")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl link [--db path] <document-id>")
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()
	documentID := fs.Arg(0)
	document, err := st.GetDocument(ctx, documentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	uri, err := st.StableDocumentURI(ctx, documentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *list {
		blocks, err := st.ListDocumentBlocks(ctx, documentID)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		anchors := []map[string]any{}
		for _, block := range blocks {
			entry := map[string]any{"kind": block.Kind, "ordinal": block.Ordinal, "block_id": block.ID, "backlinks": block.Backlinks}
			switch {
			case block.Marker != "":
				entry["anchor"] = "^" + block.Marker
			case block.HeadingSlug != "":
				entry["anchor"] = block.HeadingSlug
			default:
				entry["anchor"] = "^" + block.ID
			}
			if block.HeadingLevel > 0 {
				entry["heading_level"] = block.HeadingLevel
			}
			anchors = append(anchors, entry)
		}
		printJSON(map[string]any{"document_id": document.ID, "stable_uri": uri, "anchors": anchors})
		return
	}

	output := map[string]any{
		"document_id":  document.ID,
		"title":        document.Title,
		"document_uri": document.URI,
		"stable_uri":   uri,
	}
	if trimmed := strings.TrimSpace(*anchor); trimmed != "" {
		// An anchor that does not resolve is refused rather than printed: a
		// stable link is meant to be pasted somewhere permanent, and one that
		// never worked is worse than no link at all.
		block, err := st.FindDocumentBlock(ctx, documentID, strings.TrimPrefix(trimmed, "^"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			fmt.Fprintln(os.Stderr, "run `notriosctl link --list-anchors <document-id>` to see this note's anchors")
			os.Exit(1)
		}
		// Heading anchors are written bare; block anchors keep the caret, which
		// is the spelling every Notrios and Obsidian link already uses.
		suffix := "^" + block.ID
		switch {
		case block.Marker != "":
			suffix = "^" + block.Marker
		case block.HeadingSlug != "":
			suffix = block.HeadingSlug
		}
		output["stable_uri"] = uri + "#" + suffix
		output["document_uri"] = document.URI + "#" + suffix
		output["anchor"] = suffix
		output["anchor_kind"] = block.Kind
		output["block_id"] = block.ID
	}
	printJSON(output)
}

// runOpen resolves an external link on this machine.
//
// Exit codes are part of the contract because an OS protocol handler runs this
// non-interactively: 0 means the link named a note this machine can open, 1
// means it could not be resolved (unknown or ambiguous database, stale note),
// and 2 means the link itself was malformed.
func runOpen(args []string) {
	fs := flag.NewFlagSet("notriosctl open", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "check the link against this database instead of the registry")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	registryPath := fs.String("registry", "", "profile registry path (default: the user registry)")
	profileName := fs.String("profile", "", "resolve through this registered profile (settles ambiguity)")
	launch := fs.Bool("launch", false, "after resolving, open the note in the local web UI with xdg-open")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl open [--profile name] [--db path] [--launch] <notrios-uri>")
		fs.PrintDefaults()
		os.Exit(2)
	}
	link, err := stablelink.Parse(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	profileLabel := ""
	databasePath := strings.TrimSpace(*dbPath)
	assetPath := strings.TrimSpace(*assetStore)
	if databasePath == "" {
		// No explicit database: the registry decides, and only when the
		// answer is unambiguous.
		path := registryPathOrDefault(*registryPath)
		registry, err := profiles.Load(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		profile, err := registry.Resolve(link.DatabaseID, *profileName)
		if err != nil {
			reportUnresolvedLink(link, path, err)
			os.Exit(1)
		}
		profileLabel = profile.Name
		databasePath = profile.DatabasePath
		if assetPath == "" {
			assetPath = profile.AssetStore
		}
	}

	st := openStoreFromFlags(*configPath, databasePath, assetPath)
	defer st.Close()
	resolution, err := st.ResolveStableLink(context.Background(), link.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output := map[string]any{
		"uri":               resolution.URI,
		"status":            resolution.Status,
		"database_id":       resolution.DatabaseID,
		"local_database_id": resolution.LocalDatabaseID,
		"document_id":       resolution.DocumentID,
		"database_path":     databasePath,
	}
	if profileLabel != "" {
		output["profile"] = profileLabel
	}
	if resolution.Anchor != "" {
		output["anchor"] = resolution.Anchor
	}
	if resolution.DocumentURI != "" {
		output["document_uri"] = resolution.DocumentURI
		output["title"] = resolution.Title
	}
	openable := resolution.Status == store.StableLinkResolved || resolution.Status == store.StableLinkTrashed
	if openable {
		output["local_url"] = localNoteURL(*configPath, resolution.DocumentID, resolution.Anchor)
	}
	printJSON(output)
	if !openable {
		fmt.Fprintf(os.Stderr, "link not opened: %s\n", resolution.Status)
		os.Exit(1)
	}
	if *launch {
		url, _ := output["local_url"].(string)
		if err := exec.Command("xdg-open", url).Start(); err != nil {
			fmt.Fprintf(os.Stderr, "xdg-open %s: %v\n", url, err)
			os.Exit(1)
		}
	}
}

// reportUnresolvedLink prints why a link could not be routed. Ambiguity lists
// every candidate rather than choosing one, because the candidates are copies
// of one database and picking silently could edit the wrong copy.
func reportUnresolvedLink(link stablelink.Link, registryPath string, err error) {
	var ambiguity *profiles.AmbiguityError
	if errors.As(err, &ambiguity) {
		candidates := make([]map[string]string, 0, len(ambiguity.Candidates))
		for _, candidate := range ambiguity.Candidates {
			candidates = append(candidates, map[string]string{"profile": candidate.Name, "database_path": candidate.DatabasePath})
		}
		printJSON(map[string]any{
			"uri":         link.String(),
			"status":      "ambiguous_database",
			"database_id": link.DatabaseID,
			"registry":    registryPath,
			"candidates":  candidates,
			"error":       "several local profiles hold this database; choose one with --profile",
		})
		return
	}
	status := "unregistered_database"
	if errors.Is(err, profiles.ErrUnknownProfile) {
		status = "unknown_profile"
	}
	printJSON(map[string]any{
		"uri":         link.String(),
		"status":      status,
		"database_id": link.DatabaseID,
		"registry":    registryPath,
		"error":       err.Error(),
	})
}

// localNoteURL builds the deep link into the local web UI. The service must be
// running for it to open anything; this command deliberately does not start
// one.
func localNoteURL(configPath, documentID, anchor string) string {
	base := "http://127.0.0.1:8080"
	if cfg, err := config.Load(configPath); err == nil {
		if trimmed := strings.TrimSpace(cfg.Server.PublicBaseURL); trimmed != "" {
			base = strings.TrimRight(trimmed, "/")
		} else if addr := strings.TrimSpace(cfg.Server.ListenAddr); addr != "" {
			base = "http://" + addr
		}
	}
	url := base + "/#document=" + documentID
	if anchor != "" {
		url += "&anchor=" + anchor
	}
	return url
}

func registryPathOrDefault(explicit string) string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed
	}
	return profiles.DefaultPath()
}

func loadConfigForProfile(configPath, dbPath, assetStore string) config.Config {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if strings.TrimSpace(dbPath) != "" {
		cfg.Data.DatabasePath = dbPath
	}
	if strings.TrimSpace(assetStore) != "" {
		cfg.Data.AssetStore = assetStore
	}
	return cfg
}

// desktopEntry is the Ubuntu/XDG protocol-handler definition. It is generated
// rather than shipped so it can name the binary the user actually installed.
//
// A config path is embedded when the user supplies one: a desktop launch has
// no working directory or environment of the user's choosing, so a handler
// that fell back to built-in defaults would open the note in whatever service
// happens to run on the default port rather than theirs.
func desktopEntry(binary, configPath string) string {
	command := binary + " open --launch"
	if strings.TrimSpace(configPath) != "" {
		command = fmt.Sprintf("%s open --config %s --launch", binary, configPath)
	}
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Notrios stable link handler
Comment=Open notrios:// links in Notrios
Exec=%s %%u
Terminal=false
NoDisplay=true
MimeType=x-scheme-handler/%s;
`, command, stablelink.Scheme)
}

// runRegisterURLHandler installs the desktop protocol handler on Ubuntu.
//
// Printing is the default and installation requires --apply, because this
// writes into the user's desktop environment and changes what happens when
// they click a link anywhere on the machine. `xdg-mime`/`update-desktop-database`
// are invoked as external tools; nothing else on the system is modified.
func runRegisterURLHandler(args []string) {
	fs := flag.NewFlagSet("notriosctl register-url-handler", flag.ExitOnError)
	configPath := fs.String("config", "", "config file the handler should use (embedded in the desktop entry)")
	binary := fs.String("binary", "", "path to notriosctl (default: this executable)")
	dir := fs.String("dir", "", "applications directory (default: ~/.local/share/applications)")
	apply := fs.Bool("apply", false, "write the desktop entry and register the scheme (default: print only)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "notrios:// handler registration is implemented for Ubuntu/XDG desktops only")
		os.Exit(1)
	}
	executable := strings.TrimSpace(*binary)
	if executable == "" {
		resolved, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		executable = resolved
	}
	if !filepath.IsAbs(executable) {
		absolute, err := filepath.Abs(executable)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		executable = absolute
	}
	directory := strings.TrimSpace(*dir)
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		directory = filepath.Join(home, ".local", "share", "applications")
	}
	entryPath := filepath.Join(directory, "notrios-url-handler.desktop")
	embeddedConfig := strings.TrimSpace(*configPath)
	if embeddedConfig != "" {
		absolute, err := filepath.Abs(embeddedConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		embeddedConfig = absolute
	}
	contents := desktopEntry(executable, embeddedConfig)
	if !*apply {
		printJSON(map[string]any{
			"applied":       false,
			"desktop_entry": entryPath,
			"scheme":        stablelink.Scheme,
			"contents":      contents,
			"next_step":     "re-run with --apply to install it",
		})
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(entryPath, []byte(contents), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	warnings := []string{}
	if err := exec.Command("update-desktop-database", directory).Run(); err != nil {
		warnings = append(warnings, fmt.Sprintf("update-desktop-database: %v", err))
	}
	if err := exec.Command("xdg-mime", "default", "notrios-url-handler.desktop", "x-scheme-handler/"+stablelink.Scheme).Run(); err != nil {
		warnings = append(warnings, fmt.Sprintf("xdg-mime default: %v", err))
	}
	printJSON(map[string]any{
		"applied":       true,
		"desktop_entry": entryPath,
		"scheme":        stablelink.Scheme,
		"binary":        executable,
		"warnings":      warnings,
	})
}
