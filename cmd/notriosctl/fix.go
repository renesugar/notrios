package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/localize"
	"github.com/renesugar/notrios/internal/store"
)

// fixKindRemoteMedia is driven here rather than inside the store: localizing a
// remote image is a network operation that must go through the media policy,
// quarantine, and hash checks, and that engine already owns the rewrite and its
// revision precondition.
const fixKindRemoteMedia = "unlocalized_remote_media"

// runFix repairs what lint reports, for the small set of findings that can be
// repaired mechanically.
//
// Dry run is the default, as it is for garbage collection: the operator reads
// the exact edits before any note changes. Applying is per note against the
// revision the plan was computed from, so a note edited in the meantime fails
// instead of being overwritten.
func runFix(args []string) {
	fs := flag.NewFlagSet("notriosctl fix", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	kinds := fs.String("kinds", "", "comma-separated fix kinds (default: non_canonical_link_target)")
	documentID := fs.String("document", "", "restrict the run to one note")
	maxDocuments := fs.Int("max-documents", 0, "maximum notes per run (0 = default 1000)")
	apply := fs.Bool("apply", false, "write the fixes (default: dry run)")
	allowReview := fs.Bool("allow-review", false, "remote media only: also localize URLs whose policy decision is review")
	list := fs.Bool("list-kinds", false, "print the available fix kinds and exit")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *list {
		printJSON(map[string]any{
			"kinds":   append(store.FixKinds(), fixKindRemoteMedia),
			"default": store.DefaultFixKinds(),
			"notes": map[string]string{
				store.FixMissingAltText: "opt-in: fills alt text from the resource filename, which is a starting point rather than a description",
				fixKindRemoteMedia:      "downloads through the media policy, quarantine, and hash checks; never a plain fetch",
			},
		})
		return
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl fix [--kinds a,b] [--document id] [--apply]")
		fs.PrintDefaults()
		os.Exit(2)
	}

	requested := splitCommaList(*kinds)
	storeKinds, mediaRequested := splitFixKinds(requested)
	cfg := loadConfigForProfile(*configPath, *dbPath, *assetStore)
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	report := map[string]any{"apply": *apply}
	if len(storeKinds) > 0 || len(requested) == 0 {
		plan, err := st.PlanWorkspaceFix(ctx, store.FixRequest{
			CollectionID: strings.TrimSpace(*collectionID),
			Kinds:        storeKinds,
			DocumentID:   strings.TrimSpace(*documentID),
			MaxDocuments: *maxDocuments,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		report["plan"] = plan
		if *apply {
			report["results"] = applyFixPlan(ctx, st, plan)
		}
	}
	if mediaRequested {
		report["remote_media"] = localizeRemoteMedia(ctx, cfg, st, strings.TrimSpace(*documentID), *apply, *allowReview)
	}
	printJSON(report)
}

// applyFixPlan repairs one note at a time and reports each outcome. A failure
// on one note does not abandon the rest: each is its own transaction with its
// own precondition, and hiding the successes behind one failure would be worse
// than reporting both.
func applyFixPlan(ctx context.Context, st *store.SQLiteStore, plan store.FixPlan) []store.FixApplyResult {
	results := []store.FixApplyResult{}
	for _, document := range plan.Documents {
		result, err := st.ApplyDocumentFix(ctx, document)
		if err != nil {
			result.DocumentID = document.DocumentID
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

// localizeRemoteMedia runs the existing localization engine over the notes that
// still carry remote images. It is the same code path as `notriosctl localize`,
// so the media policy, SSRF protections, size and MIME checks, and exact-hash
// rules all apply exactly as they do there.
func localizeRemoteMedia(ctx context.Context, cfg config.Config, st *store.SQLiteStore, documentID string, apply, allowReview bool) map[string]any {
	report := map[string]any{"documents": []map[string]any{}}
	lint, err := st.LintWorkspace(ctx, store.LintRequest{
		Checks:      []string{store.LintUnlocalizedRemoteMedia},
		DetailLimit: store.MaxLintDetailItems,
	})
	if err != nil {
		report["error"] = err.Error()
		return report
	}
	targets := []string{}
	seen := map[string]bool{}
	for _, check := range lint.Checks {
		for _, finding := range check.Findings {
			if finding.DocumentID == "" || seen[finding.DocumentID] {
				continue
			}
			if documentID != "" && finding.DocumentID != documentID {
				continue
			}
			seen[finding.DocumentID] = true
			targets = append(targets, finding.DocumentID)
		}
	}
	report["notes_with_remote_media"] = len(targets)
	if lint.Checks[0].Truncated {
		report["truncated"] = true
	}

	localizer := localize.New(cfg.RemoteMedia, st)
	documents := []map[string]any{}
	for _, target := range targets {
		result, err := localizer.LocalizeDocument(ctx, localize.Options{
			DocumentID:  target,
			DryRun:      !apply,
			AllowReview: allowReview,
		})
		entry := map[string]any{"document_id": target}
		if err != nil {
			entry["error"] = err.Error()
		} else {
			entry["localized"] = len(result.Localized)
			entry["blocked"] = len(result.Blocked)
			entry["review"] = len(result.Review)
			entry["failed"] = len(result.Failed)
			if result.RevisionID != "" {
				entry["revision_id"] = result.RevisionID
			}
		}
		documents = append(documents, entry)
	}
	report["documents"] = documents
	return report
}

// splitFixKinds separates the body-rewriting kinds the store plans from the
// remote-media kind the localization engine owns.
func splitFixKinds(requested []string) ([]string, bool) {
	storeKinds := []string{}
	media := false
	for _, kind := range requested {
		if strings.EqualFold(strings.TrimSpace(kind), fixKindRemoteMedia) {
			media = true
			continue
		}
		storeKinds = append(storeKinds, kind)
	}
	return storeKinds, media
}
