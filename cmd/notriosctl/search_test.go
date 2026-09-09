package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

type searchResult struct {
	Query      string           `json:"query"`
	Hits       []map[string]any `json:"hits"`
	NextCursor string           `json:"next_cursor"`
	Count      int              `json:"count"`
}

func searchLibrary(t *testing.T) (string, string, []string) {
	t.Helper()
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}
	for i := 1; i <= 5; i++ {
		runCLIIn(t, sandbox, binary, append([]string{
			"notes", "create", "--title", fmt.Sprintf("Reed beds %d", i),
			"--body", fmt.Sprintf("Seen at dusk, note %d.", i)}, roots...)...)
	}
	tagged := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "create", "--title", "Marsh", "--body", "wetland notes"}, roots...)...)
	var note struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(tagged.stdout), &note); err != nil {
		t.Fatalf("reading the note id: %v", err)
	}
	runCLIIn(t, sandbox, binary, append([]string{
		"tags", "add", "--document", note.DocumentID, "--tag", "todo"}, roots...)...)
	return binary, sandbox, roots
}

func search(t *testing.T, binary, sandbox string, args ...string) searchResult {
	t.Helper()
	result := runCLIIn(t, sandbox, binary, args...)
	if result.exitCode != 0 {
		t.Fatalf("search %v exited %d: %s", args, result.exitCode, result.stderr)
	}
	var parsed searchResult
	if err := json.Unmarshal([]byte(result.stdout), &parsed); err != nil {
		t.Fatalf("search output is not parseable: %v\n%s", err, result.stdout)
	}
	return parsed
}

// TestSearchFindsNotesAndReturnsIdsToActOn is the reason the command exists:
// nothing at a terminal produced note identifiers, so every other command could
// only be used on an id somebody already had.
func TestSearchFindsNotesAndReturnsIdsToActOn(t *testing.T) {
	binary, sandbox, roots := searchLibrary(t)

	found := search(t, binary, sandbox, append([]string{"search"}, append(roots, "dusk")...)...)
	if len(found.Hits) == 0 {
		t.Fatal("a text search found nothing")
	}
	id, _ := found.Hits[0]["document_id"].(string)
	if id == "" {
		t.Fatalf("a hit carries no document id: %v", found.Hits[0])
	}
	for _, field := range []string{"title", "notebook_id", "collection_id", "updated_at", "uri", "snippet"} {
		if _, ok := found.Hits[0][field]; !ok {
			t.Errorf("a hit omits %q: %v", field, found.Hits[0])
		}
	}

	// The id a search returns is one another command accepts. That round trip
	// is the whole point, so it is checked rather than assumed.
	shown := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", id, "--json"}, roots...)...)
	if shown.exitCode != 0 {
		t.Errorf("the id a search returned was rejected by notes show: %s", shown.stderr)
	}

	// The query language, not a substring match.
	tagged := search(t, binary, sandbox, append([]string{"search"}, append(roots, "tag:todo")...)...)
	if len(tagged.Hits) != 1 {
		t.Errorf("tag:todo matched %d notes, want 1", len(tagged.Hits))
	}
}

// TestSearchCountAgreesWithPaging is the property that makes --count worth
// having: it must be the number of notes paging would return, or it is a second
// answer to the same question.
func TestSearchCountAgreesWithPaging(t *testing.T) {
	binary, sandbox, roots := searchLibrary(t)

	for _, query := range []string{"dusk", "tag:todo", "notes"} {
		paged := 0
		cursor := ""
		for page := 0; page < 10; page++ {
			args := append([]string{"search", "--limit", "2"}, roots...)
			if cursor != "" {
				args = append(args, "--cursor", cursor)
			}
			result := search(t, binary, sandbox, append(args, query)...)
			paged += len(result.Hits)
			cursor = result.NextCursor
			if cursor == "" {
				break
			}
		}
		counted := search(t, binary, sandbox, append(append([]string{"search", "--count"}, roots...), query)...)
		if counted.Count != paged {
			t.Errorf("%q: --count said %d and paging returned %d", query, counted.Count, paged)
		}
	}
}

// TestSearchRefusesFlagsThatDoNotApplyToACount keeps the command from accepting
// input that changes nothing.
func TestSearchRefusesFlagsThatDoNotApplyToACount(t *testing.T) {
	binary, sandbox, roots := searchLibrary(t)

	for _, extra := range [][]string{{"--limit", "5"}, {"--cursor", "abc"}, {"--links"}} {
		args := append(append([]string{"search", "--count"}, roots...), extra...)
		result := runCLIIn(t, sandbox, binary, append(args, "dusk")...)
		if result.exitCode == 0 {
			t.Errorf("--count with %v was accepted", extra)
		}
	}
}

// TestSearchReportsABadQueryAsAQueryProblem guards the message. A parser error
// printed bare gives no indication that the thing at fault is what was typed.
func TestSearchReportsABadQueryAsAQueryProblem(t *testing.T) {
	binary, sandbox, roots := searchLibrary(t)

	result := runCLIIn(t, sandbox, binary, append(append([]string{"search"}, roots...), "((unclosed")...)
	if result.exitCode != 2 {
		t.Errorf("a malformed query exited %d, want 2", result.exitCode)
	}
	if !strings.Contains(result.stderr, "query") {
		t.Errorf("the refusal does not say the query was the problem: %s", result.stderr)
	}
	if strings.TrimSpace(result.stdout) != "" {
		t.Errorf("a refused query wrote hits to standard output: %q", result.stdout)
	}
}

// TestSearchLinksAreTheOnesLinkPrints keeps one link form rather than two.
func TestSearchLinksAreTheOnesLinkPrints(t *testing.T) {
	binary, sandbox, roots := searchLibrary(t)

	found := search(t, binary, sandbox, append(append([]string{"search", "--links"}, roots...), "wetland")...)
	if len(found.Hits) == 0 {
		t.Fatal("no hit to compare links for")
	}
	id, _ := found.Hits[0]["document_id"].(string)
	uri, _ := found.Hits[0]["uri"].(string)
	if !strings.HasPrefix(uri, "notrios://") {
		t.Errorf("--links did not produce a notrios:// link: %q", uri)
	}

	linked := runCLIIn(t, sandbox, binary, append(append([]string{"link"}, roots...), id)...)
	var link struct {
		StableURI string `json:"stable_uri"`
	}
	if err := json.Unmarshal([]byte(linked.stdout), &link); err != nil {
		t.Fatalf("reading the link: %v\n%s", err, linked.stdout)
	}
	if link.StableURI != uri {
		t.Errorf("search printed %q and link printed %q", uri, link.StableURI)
	}
}

// TestSearchExcludesTrashUnlessAsked keeps the boundary the item stated.
func TestSearchExcludesTrashUnlessAsked(t *testing.T) {
	binary, sandbox, roots := searchLibrary(t)

	before := search(t, binary, sandbox, append(append([]string{"search"}, roots...), "wetland")...)
	if len(before.Hits) != 1 {
		t.Fatalf("expected one wetland note, got %d", len(before.Hits))
	}
	id, _ := before.Hits[0]["document_id"].(string)
	if deleted := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "delete", "--document", id}, roots...)...); deleted.exitCode != 0 {
		t.Fatalf("trashing the note: %s", deleted.stderr)
	}

	after := search(t, binary, sandbox, append(append([]string{"search"}, roots...), "wetland")...)
	if len(after.Hits) != 0 {
		t.Errorf("a trashed note was still found: %v", after.Hits)
	}
	trashed := search(t, binary, sandbox, append(append([]string{"search"}, roots...), "is:trashed")...)
	if len(trashed.Hits) != 1 {
		t.Errorf("is:trashed found %d notes, want the one in Trash", len(trashed.Hits))
	}
}
