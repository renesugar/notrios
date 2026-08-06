package store

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// newLintFixture builds a library with one instance of each detectable problem
// plus healthy content that must not be reported.
func newLintFixture(t *testing.T) (*SQLiteStore, map[string]string) {
	t.Helper()
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}

	healthy, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_healthy", Title: "Healthy", Body: "# Healthy\n\nA paragraph. ^good-anchor\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	ids["healthy"] = healthy.ID

	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_used", Filename: "used.png", MIMEType: "image/png",
		Content: bytes.NewReader([]byte("\x89PNG\r\n\x1a\nused")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_orphan", Filename: "orphan.png", MIMEType: "image/png",
		Content: bytes.NewReader([]byte("\x89PNG\r\n\x1a\norphan")),
	}); err != nil {
		t.Fatal(err)
	}

	// One note carrying most of the problems, each on its own line so the
	// reported locations can be checked.
	body := strings.Join([]string{
		"[broken](document://default/documents/doc_gone)",
		"[stale block](" + healthy.URI + "#^no-such-block)",
		"[good block](" + healthy.URI + "#^good-anchor)",
		"![](https://example.com/remote.png)",
		"![local alt](" + resource.URI + ")",
		"[healthy](" + healthy.URI + ")",
	}, "\n\n")
	problems, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_problems", Title: "Problems", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	ids["problems"] = problems.ID
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{
		DocumentID: problems.ID, ResourceID: resource.ID, RelationType: "embedded",
	}); err != nil {
		t.Fatal(err)
	}

	// CreateDocument substitutes "Untitled" for a blank title, so an empty one
	// can only arrive from a writer that bypasses it — an importer, or a restore
	// whose title-derivation pass did not complete. That is exactly the state
	// this check exists to catch, so the fixture produces it the same way.
	untitled, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_untitled", Title: "Placeholder", Body: "no title\n"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(ctx, `UPDATE documents SET title = '' WHERE id = 'doc_untitled'`); err != nil {
		t.Fatal(err)
	}
	ids["untitled"] = untitled.ID

	// Two notes claiming one external identity.
	for _, id := range []string{"doc_twin_a", "doc_twin_b"} {
		created, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: id, Title: "Twin", Body: "twin\n"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.SetDocumentSource(ctx, SetDocumentSourceRequest{
			DocumentID: created.ID, SourceSystem: "joplin", ExternalID: "shared-external-id",
		}); err != nil {
			t.Fatal(err)
		}
	}
	return st, ids
}

func checkResult(t *testing.T, report LintReport, check string) LintCheckResult {
	t.Helper()
	for _, result := range report.Checks {
		if result.Check == check {
			return result
		}
	}
	t.Fatalf("check %q missing from the report", check)
	return LintCheckResult{}
}

func TestLintFindsEachProblemAndNothingElse(t *testing.T) {
	ctx := context.Background()
	st, ids := newLintFixture(t)
	report, err := st.LintWorkspace(ctx, LintRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != 1 || len(report.ReportSHA256) != 64 {
		t.Fatalf("report shape: %+v", report)
	}

	wants := map[string]int{
		LintBrokenDocumentLink:     1,
		LintAmbiguousLink:          0,
		LintUnresolvedBlockAnchor:  1,
		LintDuplicateSourceID:      2,
		LintMissingTitle:           1,
		LintUnlocalizedRemoteMedia: 1,
		LintMissingAltText:         1,
		LintUnreferencedResource:   1,
	}
	for check, want := range wants {
		if got := checkResult(t, report, check).Count; got != want {
			t.Fatalf("%s: got %d findings, want %d", check, got, want)
		}
	}

	// The healthy note and the used resource must not appear anywhere.
	for _, result := range report.Checks {
		for _, finding := range result.Findings {
			if finding.DocumentID == ids["healthy"] && result.Check != LintProjectionBacklog {
				t.Fatalf("healthy note reported by %s: %+v", result.Check, finding)
			}
			if finding.ResourceID == "res_used" {
				t.Fatalf("referenced resource reported by %s", result.Check)
			}
		}
	}

	// A finding locates the problem: the broken link is on the first line.
	broken := checkResult(t, report, LintBrokenDocumentLink).Findings[0]
	if broken.DocumentID != ids["problems"] || broken.Line != 1 {
		t.Fatalf("broken link finding: %+v", broken)
	}
	if broken.TargetSHA256 == "" || len(broken.TargetSHA256) != 64 {
		t.Fatalf("a finding must fingerprint its target: %+v", broken)
	}
}

// Findings identify and locate; they never quote. A broken wikilink's raw text
// is frequently a private note's title.
func TestLintFindingsCarryNoNoteContent(t *testing.T) {
	ctx := context.Background()
	st, _ := newLintFixture(t)
	report, err := st.LintWorkspace(ctx, LintRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range report.Checks {
		for _, finding := range result.Findings {
			for _, forbidden := range []string{"doc_gone", "example.com", "no-such-block", "Problems", "Healthy", "shared-external-id"} {
				if strings.Contains(finding.TargetSHA256, forbidden) || strings.Contains(finding.Detail, forbidden) {
					t.Fatalf("%s leaked %q: %+v", result.Check, forbidden, finding)
				}
			}
		}
	}
}

func TestLintIsDeterministicAndReadOnly(t *testing.T) {
	ctx := context.Background()
	st, _ := newLintFixture(t)
	before, err := st.LintWorkspace(ctx, LintRequest{})
	if err != nil {
		t.Fatal(err)
	}
	revisionsBefore, err := st.countLocked(`SELECT COUNT(*) FROM document_revisions`)
	if err != nil {
		t.Fatal(err)
	}

	after, err := st.LintWorkspace(ctx, LintRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if before.ReportSHA256 != after.ReportSHA256 || before.TotalFindings != after.TotalFindings {
		t.Fatalf("lint is not deterministic: %s/%d vs %s/%d",
			before.ReportSHA256, before.TotalFindings, after.ReportSHA256, after.TotalFindings)
	}
	revisionsAfter, err := st.countLocked(`SELECT COUNT(*) FROM document_revisions`)
	if err != nil {
		t.Fatal(err)
	}
	if revisionsBefore != revisionsAfter {
		t.Fatalf("lint wrote %d revisions", revisionsAfter-revisionsBefore)
	}
}

// The cap hides examples, never counts: the digest and the totals describe the
// whole library, which is what makes a capped report safe to act on.
func TestLintCapsExamplesWithoutCappingCounts(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		lines = append(lines, "[broken](document://default/documents/doc_missing_"+string(rune('a'+i))+")")
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Many", Body: strings.Join(lines, "\n\n")}); err != nil {
		t.Fatal(err)
	}

	capped, err := st.LintWorkspace(ctx, LintRequest{DetailLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	result := checkResult(t, capped, LintBrokenDocumentLink)
	if result.Count != 25 || len(result.Findings) != 5 || !result.Truncated {
		t.Fatalf("cap should hide examples only: count=%d findings=%d truncated=%t", result.Count, len(result.Findings), result.Truncated)
	}

	// The digest covers findings the cap hid, so it does not change with it.
	full, err := st.LintWorkspace(ctx, LintRequest{DetailLimit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if capped.ReportSHA256 != full.ReportSHA256 {
		t.Fatal("the digest must describe the library, not the page")
	}
}

func TestLintCheckSelectionAndLimits(t *testing.T) {
	ctx := context.Background()
	st, _ := newLintFixture(t)

	only, err := st.LintWorkspace(ctx, LintRequest{Checks: []string{LintMissingTitle}})
	if err != nil {
		t.Fatal(err)
	}
	if len(only.Checks) != 1 || only.Checks[0].Check != LintMissingTitle {
		t.Fatalf("selecting one check ran %d: %+v", len(only.Checks), only.Checks)
	}

	if _, err := st.LintWorkspace(ctx, LintRequest{Checks: []string{"delete_everything"}}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown check: %v", err)
	}
	if _, err := st.LintWorkspace(ctx, LintRequest{DetailLimit: MaxLintDetailItems + 1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized detail limit: %v", err)
	}
}

// A trashed note is not a broken workspace: its links stop being reported when
// it goes to the Trash, and come back if it is restored.
func TestLintIgnoresTrashedNotes(t *testing.T) {
	ctx := context.Background()
	st, ids := newLintFixture(t)
	document, err := st.GetDocument(ctx, ids["problems"])
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: document.ID, BaseRevisionID: document.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	report, err := st.LintWorkspace(ctx, LintRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := checkResult(t, report, LintBrokenDocumentLink).Count; got != 0 {
		t.Fatalf("trashed notes should not be linted, got %d broken links", got)
	}
}
