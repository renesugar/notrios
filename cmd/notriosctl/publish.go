package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/publish"
	"github.com/renesugar/notrios/internal/store"
)

// runPublish is the reviewed publication workflow: save a profile, plan it,
// read the plan, then run it against the digest you read.
func runPublish(args []string) {
	if len(args) == 0 {
		printPublishUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "profile":
		runPublishProfile(args[1:])
	case "plan":
		runPublishPlan(args[1:])
	case "run":
		runPublishRun(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown publish command %q\n", args[0])
		printPublishUsage()
		os.Exit(2)
	}
}

func printPublishUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl publish profile save --name <profile> [--target publication_handoff|subset_transfer]
        [--notebooks id,id] [--tags a,b] [--query "..."] [--documents id,id] [--match any|all]
        [--exclude-tags a,b] [--private-tags a,b] [--link-action plain_text|redact|report|retain]
        [--include-provenance] [--include-source-bundles] [--max-resource-bytes N] [--description text]
  notriosctl publish profile list [--profiles path]
  notriosctl publish profile delete --name <profile> [--profiles path]
  notriosctl publish plan --profile <profile> [--detail-limit 100]
  notriosctl publish run --profile <profile> --reviewed-plan <sha256> <out-dir>
`)
}

func runPublishProfile(args []string) {
	if len(args) == 0 {
		printPublishUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "save":
		runPublishProfileSave(args[1:])
	case "list":
		runPublishProfileList(args[1:])
	case "delete":
		runPublishProfileDelete(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown publish profile command %q\n", args[0])
		printPublishUsage()
		os.Exit(2)
	}
}

// publishFlags are the flags every publish command shares.
type publishFlags struct {
	configPath   *string
	dbPath       *string
	assetStore   *string
	profilesPath *string
}

func registerPublishFlags(fs *flag.FlagSet) publishFlags {
	return publishFlags{
		configPath:   fs.String("config", "", "optional config file"),
		dbPath:       fs.String("db", "", "SQLite database path override"),
		assetStore:   fs.String("asset-store", "", "asset store directory override"),
		profilesPath: fs.String("profiles", "", "publish profile file (default: <data-dir>/publish-profiles.json)"),
	}
}

func (f publishFlags) resolveProfilePath() string {
	if trimmed := strings.TrimSpace(*f.profilesPath); trimmed != "" {
		return trimmed
	}
	cfg := loadConfigForProfile(*f.configPath, *f.dbPath, *f.assetStore)
	return publish.DefaultPath(cfg.Data.Directory)
}

func runPublishProfileSave(args []string) {
	fs := flag.NewFlagSet("notriosctl publish profile save", flag.ExitOnError)
	shared := registerPublishFlags(fs)
	name := fs.String("name", "", "required: profile name")
	description := fs.String("description", "", "optional description")
	target := fs.String("target", store.SelectionTargetPublicationHandoff, "publication_handoff or subset_transfer")
	notebooks := fs.String("notebooks", "", "comma-separated notebook IDs (recursive)")
	tags := fs.String("tags", "", "comma-separated tag names")
	query := fs.String("query", "", "query-language scope")
	documents := fs.String("documents", "", "comma-separated document IDs")
	match := fs.String("match", "any", "combine populated selector types with any or all")
	excludeTags := fs.String("exclude-tags", "", "tags whose notes are never published")
	privateTags := fs.String("private-tags", "", "tags treated as private for link decisions")
	linkAction := fs.String("link-action", store.SelectionLinkActionPlainText, "retain, report, plain_text, or redact")
	includeProvenance := fs.Bool("include-provenance", false, "publish source provenance (off by default)")
	includeBundles := fs.Bool("include-source-bundles", false, "publish exact source bundles (off by default)")
	maxResourceBytes := fs.Int64("max-resource-bytes", 0, "exclude resources larger than this (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*name) == "" {
		printPublishUsage()
		os.Exit(2)
	}

	policy := store.PrivacyPolicy{
		ExcludeTags:      splitCommaList(*excludeTags),
		PrivateTags:      splitCommaList(*privateTags),
		LinkAction:       strings.TrimSpace(*linkAction),
		MaxResourceBytes: *maxResourceBytes,
	}
	// Only an explicitly requested opt-in overrides a secure default, so a
	// flag left alone can never widen what a publication carries.
	if *includeProvenance {
		value := true
		policy.IncludeProvenance = &value
	}
	if *includeBundles {
		value := true
		policy.IncludeSourceBundles = &value
	}

	now := time.Now().UTC()
	profile := publish.Profile{
		Name:        strings.TrimSpace(*name),
		Description: strings.TrimSpace(*description),
		Target:      strings.TrimSpace(*target),
		Selection: store.SelectionSpec{
			NotebookIDs: splitCommaList(*notebooks),
			Tags:        splitCommaList(*tags),
			Query:       strings.TrimSpace(*query),
			DocumentIDs: splitCommaList(*documents),
			Match:       strings.TrimSpace(*match),
		},
		Policy:    policy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	path := shared.resolveProfilePath()
	file, err := publish.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	updated, err := file.Upsert(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := updated.Save(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"profiles_path": path,
		"saved":         profile.Name,
		"target":        profile.Target,
		"profiles":      len(updated.Profiles),
		"next_step":     "review it with `notriosctl publish plan --profile " + profile.Name + "`",
	})
}

func runPublishProfileList(args []string) {
	fs := flag.NewFlagSet("notriosctl publish profile list", flag.ExitOnError)
	shared := registerPublishFlags(fs)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	path := shared.resolveProfilePath()
	file, err := publish.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{"profiles_path": path, "profiles": file.Profiles})
}

func runPublishProfileDelete(args []string) {
	fs := flag.NewFlagSet("notriosctl publish profile delete", flag.ExitOnError)
	shared := registerPublishFlags(fs)
	name := fs.String("name", "", "required: profile name")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*name) == "" {
		printPublishUsage()
		os.Exit(2)
	}
	path := shared.resolveProfilePath()
	file, err := publish.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	updated, err := file.Remove(strings.TrimSpace(*name))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := updated.Save(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{"profiles_path": path, "deleted": strings.TrimSpace(*name), "profiles": len(updated.Profiles)})
}

// runPublishPlan is the review step. It reads the library and writes nothing,
// reporting what would be published, what would be withheld, and why.
func runPublishPlan(args []string) {
	fs := flag.NewFlagSet("notriosctl publish plan", flag.ExitOnError)
	shared := registerPublishFlags(fs)
	name := fs.String("profile", "", "required: profile name")
	detailLimit := fs.Int("detail-limit", 100, "maximum examples per detail array")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*name) == "" {
		printPublishUsage()
		os.Exit(2)
	}
	profile := loadPublishProfile(shared, *name)
	st := openStoreFromFlags(*shared.configPath, *shared.dbPath, *shared.assetStore)
	defer st.Close()
	plan, err := st.PlanSelection(context.Background(), profile.PlanRequest(*detailLimit))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"profile":   profile.Name,
		"target":    profile.Target,
		"plan":      plan,
		"next_step": "after reviewing, publish with `--reviewed-plan " + plan.ManifestSHA256 + "`",
	})
}

// runPublishRun performs the publication, but only against a plan digest the
// caller has actually seen. If anything about the library changed since the
// review — a note gained a private tag, a new note joined the notebook — the
// digest no longer matches and nothing is written.
func runPublishRun(args []string) {
	fs := flag.NewFlagSet("notriosctl publish run", flag.ExitOnError)
	shared := registerPublishFlags(fs)
	name := fs.String("profile", "", "required: profile name")
	reviewed := fs.String("reviewed-plan", "", "required: the manifest_sha256 printed by `publish plan`")
	overwrite := fs.Bool("overwrite", false, "replace an existing complete archive in the destination")
	skipVerify := fs.Bool("no-verify", false, "skip the verification pass after publishing the manifest")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*name) == "" || fs.NArg() != 1 {
		printPublishUsage()
		os.Exit(2)
	}
	profile := loadPublishProfile(shared, *name)
	st := openStoreFromFlags(*shared.configPath, *shared.dbPath, *shared.assetStore)
	defer st.Close()
	ctx := context.Background()

	// Re-plan now and compare against what the operator reviewed. The plan is
	// cheap next to an export and it is the only way to know the reviewed
	// decision still describes this library.
	plan, err := st.PlanSelection(ctx, profile.PlanRequest(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := publish.RequireReviewedPlan(*reviewed, plan.ManifestSHA256); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report, err := archivev2.Export(ctx, st, fs.Arg(0), archivev2.ExportOptions{
		Target:    profile.Target,
		Selection: profile.Selection,
		Policy:    profile.Policy,
		Overwrite: *overwrite,

		SkipVerification: *skipVerify,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if report.SelectionManifestSHA256 != plan.ManifestSHA256 {
		// The export binds the plan digest into its manifest. A mismatch here
		// would mean the library changed between the check and the write.
		fmt.Fprintf(os.Stderr, "the library changed while publishing: reviewed %s, archived %s\n",
			plan.ManifestSHA256, report.SelectionManifestSHA256)
		os.Exit(1)
	}
	printJSON(map[string]any{"profile": profile.Name, "destination": fs.Arg(0), "report": report})
}

func loadPublishProfile(shared publishFlags, name string) publish.Profile {
	path := shared.resolveProfilePath()
	file, err := publish.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	profile, err := file.ByName(strings.TrimSpace(name))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return profile
}
