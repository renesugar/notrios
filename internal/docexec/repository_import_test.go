package docexec

// This file owns the small, deterministic source fixtures used by the
// documentation import examples.  Keeping them here makes the examples
// independent of a user's private exports while still exercising the real
// import parsers.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func (h *repositoryExamples) prepareDocumentationImport(ctx context.Context, fixture *repositoryFixture, kind string) error {
	var commands []string
	switch kind {
	case "cli-import-joplin-apply":
		commands = append(commands, fmt.Sprintf("notriosctl import joplin-raw --dry-run --write-config %q %q", filepath.Join(fixture.imports.Joplin, "import-config.json"), fixture.imports.Joplin))
	case "cli-import-preserve":
		commands = append(commands,
			fmt.Sprintf("notriosctl import joplin-raw --dry-run --write-config %q %q", filepath.Join(fixture.root, "joplin-plan.json"), fixture.imports.Joplin),
			fmt.Sprintf("notriosctl import obsidian --dry-run --write-config %q %q", filepath.Join(fixture.root, "obsidian-plan.json"), fixture.imports.Obsidian))
	}
	for _, command := range commands {
		output, status, err := runPinnedShell(ctx, fixture.root, h.cli, command)
		if err != nil {
			return fmt.Errorf("prepare documentation import exit %d: %w\n%s", status, err, output)
		}
	}
	return nil
}

func checkDocumentationImportPostcondition(ctx context.Context, fixture *repositoryFixture, kind, output string) (bool, bool, string) {
	phrases := map[string][]string{
		"cli-import-joplin-apply": {"Joplin Fixture"},
		"cli-import-chatgpt":      {"ChatGPT Fixture"},
		"cli-import-claude":       {"Claude Fixture"},
		"cli-import-obsidian":     {"Obsidian Fixture"},
		"cli-import-twitter":      {"Twitter fixture note"},
		"cli-import-archive-v1":   {"Archive Fixture"},
		"cli-import-preserve":     {"Joplin Fixture", "Obsidian Fixture"},
	}
	if kind == "cli-import-dual-dry" || kind == "cli-import-plans" {
		st, closeStore, err := openFixtureStore(fixture)
		if err != nil {
			return true, false, err.Error()
		}
		defer closeStore()
		if err := AssertDocumentationImportDryRunPreservesState(ctx, st, fixture.importBase); err != nil {
			return true, false, err.Error()
		}
		if kind == "cli-import-plans" && (!fileExists(filepath.Join(fixture.root, "joplin-plan.json")) || !fileExists(filepath.Join(fixture.root, "obsidian-plan.json"))) {
			return true, false, "dry-run import plans were not written to their explicit scratch paths"
		}
		return true, strings.Contains(output, "dry_run") || strings.Contains(output, "notes_seen"), "dry-run import report was vacuous"
	}
	wanted, handled := phrases[kind]
	if !handled {
		return false, false, ""
	}
	st, closeStore, err := openFixtureStore(fixture)
	if err != nil {
		return true, false, err.Error()
	}
	defer closeStore()
	for _, phrase := range wanted {
		if err := AssertDocumentationImportCreated(ctx, st, phrase); err != nil {
			return true, false, fmt.Sprintf("%s: %v", phrase, err)
		}
	}
	return true, strings.TrimSpace(output) != "", "import emitted no result report"
}

// DocumentationImportFixtures names all supported external import sources.
// The paths are absolute when root is absolute and are suitable for command
// substitution in repositoryExamples.resolve.
type DocumentationImportFixtures struct {
	Joplin, Obsidian, Twitter, ChatGPT, Claude, ArchiveV1 string
}

// SeedDocumentationImportFixtures creates minimal valid exports beneath root.
func SeedDocumentationImportFixtures(root string) (DocumentationImportFixtures, error) {
	f := DocumentationImportFixtures{
		Joplin: filepath.Join(root, "joplin-export"), Obsidian: filepath.Join(root, "obsidian-vault"),
		Twitter: filepath.Join(root, "twitter-archive"), ChatGPT: filepath.Join(root, "chatgpt-export"),
		Claude: filepath.Join(root, "claude-export"), ArchiveV1: filepath.Join(root, "archive-v1"),
	}
	files := map[string]string{
		filepath.Join(f.Joplin, "folder.md"):                      "Imported Notes\n\nid: folder-fixture\ntype_: 2\n",
		filepath.Join(f.Joplin, "note.md"):                        "Joplin Fixture\n\nA searchable imported note.\n\nid: joplin-fixture\nparent_id: folder-fixture\ntype_: 1\n",
		filepath.Join(f.Obsidian, "Projects", "Fixture.md"):       "---\ntags: [fixture]\n---\n# Obsidian Fixture\n\nA searchable vault note.\n",
		filepath.Join(f.Twitter, "data", "account.js"):            `window.YTD.account.part0 = [{"account":{"username":"fixture","accountId":"1","accountDisplayName":"Fixture User"}}]`,
		filepath.Join(f.Twitter, "data", "tweets.js"):             `window.YTD.tweets.part0 = [{"tweet":{"id_str":"fixture-1","full_text":"Twitter fixture note","created_at":"Mon Jul 13 12:00:00 +0000 2026","entities":{"hashtags":[],"urls":[]}}}]`,
		filepath.Join(f.ChatGPT, "conversations.json"):            `[{"title":"ChatGPT Fixture","create_time":1783958400,"conversation_id":"chatgpt-fixture","current_node":"n1","mapping":{"n1":{"id":"n1","parent":"","message":{"id":"m1","author":{"role":"user"},"create_time":1783958400,"content":{"content_type":"text","parts":["ChatGPT fixture note"]}}}}}]`,
		filepath.Join(f.Claude, "conversations.json"):             `[{"uuid":"claude-fixture","name":"Claude Fixture","created_at":"2026-07-13T18:00:00Z","chat_messages":[{"uuid":"m1","sender":"human","created_at":"2026-07-13T18:00:01Z","text":"Claude fixture note"}]}]`,
		filepath.Join(f.ArchiveV1, "manifest.json"):               `{"format":"notrios-archive","version":1,"query":"","notes":1}`,
		filepath.Join(f.ArchiveV1, "notebooks.json"):              `[{"path":"Imported Archive"}]`,
		filepath.Join(f.ArchiveV1, "notes", "archive-fixture.md"): "---\nid: archive-fixture\ntitle: Archive Fixture\nnotebook: Imported Archive\n---\nAn archived fixture note.\n",
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return f, err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return f, err
		}
	}
	return f, nil
}

// DocumentationImportSnapshot is a cheap canonical-state fence. It is used
// before and after dry runs: a dry run may write its config beside the source,
// but must not alter this state.
type DocumentationImportSnapshot struct{ Documents, Notebooks int }

func SnapshotDocumentationImportState(ctx context.Context, st store.Store) (DocumentationImportSnapshot, error) {
	nbs, err := st.ListNotebooks(ctx)
	if err != nil {
		return DocumentationImportSnapshot{}, err
	}
	res, err := st.Search(ctx, store.SearchRequest{Limit: 10000})
	if err != nil {
		return DocumentationImportSnapshot{}, err
	}
	return DocumentationImportSnapshot{Documents: len(res.Hits), Notebooks: len(nbs)}, nil
}

// AssertDocumentationImportDryRunPreservesState verifies the invariant shared
// by all importers without coupling the harness to any report struct.
func AssertDocumentationImportDryRunPreservesState(ctx context.Context, st store.Store, before DocumentationImportSnapshot) error {
	after, err := SnapshotDocumentationImportState(ctx, st)
	if err != nil {
		return err
	}
	if after != before {
		return errors.New("documentation import dry-run changed canonical state")
	}
	return nil
}

// AssertDocumentationImportCreated checks that an actual import produced a
// searchable note and (for notebook-bearing sources) a corresponding notebook.
func AssertDocumentationImportCreated(ctx context.Context, st store.Store, phrase string) error {
	res, err := st.Search(ctx, store.SearchRequest{Query: phrase, Limit: 10})
	if err != nil {
		return err
	}
	if len(res.Hits) == 0 {
		return errors.New("documentation import produced no searchable note")
	}
	return nil
}

func TestImportDocumentationFixtures(t *testing.T) {
	paths, err := SeedDocumentationImportFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.Joplin, paths.Obsidian, paths.Twitter, paths.ChatGPT, paths.Claude, paths.ArchiveV1} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("fixture %s: %v", path, err)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(paths.ArchiveV1, "manifest.json")); err != nil {
		t.Fatal(err)
	} else {
		var manifest map[string]any
		if err := json.Unmarshal(raw, &manifest); err != nil || manifest["version"] != float64(1) {
			t.Fatalf("archive manifest invalid: %s", raw)
		}
	}
}
