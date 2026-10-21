package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestLinksReportQuotesForTheOwner: the command prints what REST withholds --
// the link as written and the candidates' titles and stable URLs -- because its
// output stays on the owner's machine (v1.0 J40-D, decisions 1 and 5).
func TestLinksReportQuotesForTheOwner(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}
	for _, note := range [][2]string{{"Twin", "# Twin"}, {"Twin", "# Twin"}, {"Writer", "Which [[Twin|one]]?"}} {
		if created := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", note[0], "--body", note[1]}, roots...)...); created.exitCode != 0 {
			t.Fatalf("notes create: %s", created.stderr)
		}
	}

	printed := runCLIIn(t, sandbox, binary, append([]string{"links", "report", "--candidate-limit", "1"}, roots...)...)
	if printed.exitCode != 0 {
		t.Fatalf("links report exited %d: %s", printed.exitCode, printed.stderr)
	}
	var report struct {
		AmbiguousLinks int `json:"ambiguous_links"`
		Notes          []struct {
			Title     string `json:"title"`
			StableURI string `json:"stable_uri"`
			Links     []struct {
				Written        string `json:"written"`
				CandidateCount int    `json:"candidate_count"`
				Truncated      bool   `json:"truncated"`
				Candidates     []struct {
					Title     string `json:"title"`
					StableURI string `json:"stable_uri"`
				} `json:"candidates"`
			} `json:"links"`
		} `json:"notes"`
	}
	if err := json.Unmarshal([]byte(printed.stdout), &report); err != nil {
		t.Fatalf("links report printed unreadable JSON: %v\n%s", err, printed.stdout)
	}
	if report.AmbiguousLinks != 1 || len(report.Notes) != 1 || report.Notes[0].Title != "Writer" ||
		!strings.HasPrefix(report.Notes[0].StableURI, "notrios://") {
		t.Fatalf("the note, by title and stable URL: %s", printed.stdout)
	}
	link := report.Notes[0].Links[0]
	if link.Written != "[[Twin|one]]" || link.CandidateCount != 2 || !link.Truncated || len(link.Candidates) != 1 ||
		link.Candidates[0].Title != "Twin" || !strings.HasPrefix(link.Candidates[0].StableURI, "notrios://") {
		t.Fatalf("the link as written, and one candidate of two: %+v", link)
	}

	for _, bad := range [][]string{{"--detail-limit", "0"}, {"--candidate-limit", "101"}, {"extra"}} {
		if refused := runCLIIn(t, sandbox, binary, append(append([]string{"links", "report"}, bad...), roots...)...); refused.exitCode != 2 {
			t.Fatalf("links report %v exited %d, want 2", bad, refused.exitCode)
		}
	}

	written := runCLIIn(t, sandbox, binary, append([]string{"links", "report", "--write-note", "--quiet"}, roots...)...)
	if written.exitCode != 0 || written.stdout != "" {
		t.Fatalf("--write-note --quiet: exit %d, printed %q, %s", written.exitCode, written.stdout, written.stderr)
	}
	shown := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", "doc_link_report"}, roots...)...)
	if shown.exitCode != 0 || !strings.Contains(shown.stdout, `\[\[Twin\|one\]\]`) {
		t.Fatalf("the Reports note quotes the link: exit %d\n%s%s", shown.exitCode, shown.stdout, shown.stderr)
	}
}
