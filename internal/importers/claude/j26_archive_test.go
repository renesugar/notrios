package claude

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J26: a Claude export arrives as a ZIP, and a large one as several batch ZIPs.
// Every fixture here is synthetic; no real archive content is in the repository.

// j26Conversation builds one conversation: a plain message, a message whose
// content blocks are mostly machinery, an attachment carrying extracted text,
// and a file reference whose bytes the export does not hold.
func j26Conversation(uuid, name string) string {
	return `{
      "uuid": "` + uuid + `", "name": "` + name + `", "summary": "",
      "created_at": "2026-07-13T18:00:00Z", "updated_at": "2026-07-13T18:10:00Z",
      "chat_messages": [
        {"uuid": "m1", "sender": "human", "created_at": "2026-07-13T18:00:01Z", "text": "Read this and summarise it.",
         "content": [{"type": "text", "text": "Read this and summarise it."}],
         "attachments": [{"file_name": "report.txt", "file_size": 12, "file_type": "text/plain", "extracted_content": "the report body"}],
         "files": [{"file_uuid": "f-1", "file_name": "diagram.png"}]},
        {"uuid": "m2", "sender": "assistant", "created_at": "2026-07-13T18:00:05Z", "text": "",
         "content": [
           {"type": "thinking", "text": "Considering the request."},
           {"type": "tool_use", "text": "search"},
           {"type": "tool_result", "text": "results"},
           {"type": "text", "text": "Here is the summary."}
         ],
         "attachments": [], "files": []}
      ]
    }`
}

const j26Project = `{
  "uuid": "p-1", "name": "Research", "description": "Everything about the study.",
  "prompt_template": "You are a careful reader.", "created_at": "2026-07-01T09:00:00Z",
  "is_private": true, "is_starter_project": false,
  "docs": [{"uuid": "d-1", "filename": "method.md", "content": "Sample selection and limits."}]
}`

func j26WriteZip(t *testing.T, path string, files map[string]string) string {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for name, body := range files {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func claudeExport(conversationUUIDs ...string) map[string]string {
	conversations := make([]string, 0, len(conversationUUIDs))
	for i, uuid := range conversationUUIDs {
		conversations = append(conversations, j26Conversation(uuid, "Conversation "+string(rune('A'+i))))
	}
	return map[string]string{
		"conversations.json": "[\n" + strings.Join(conversations, ",\n") + "\n]",
		"users.json":         `[{"uuid": "u-1", "full_name": "A User"}]`,
		"projects/p-1.json":  j26Project,
	}
}

func TestJ26ImportsTheClaudeExportZip(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	path := j26WriteZip(t, filepath.Join(t.TempDir(), "claude-data-abc-1783442080-38379786-batch-0000.zip"), claudeExport("c-1", "c-2"))
	report, err := Import(ctx, st, path, Options{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.SourceFormat != "zip" || report.ConversationsSeen != 2 || report.NotesImported != 3 {
		t.Errorf("source_format=%q conversations_seen=%d notes_imported=%d, want zip, 2, and 3 (two conversations and the project)",
			report.SourceFormat, report.ConversationsSeen, report.NotesImported)
	}
	document, err := st.GetDocument(ctx, "doc_claude_c_1")
	if err != nil {
		t.Fatal(err)
	}
	body := document.Body
	if !strings.Contains(body, "Here is the summary.") {
		t.Errorf("assistant text must be imported:\n%s", body)
	}
	if strings.Contains(body, "Considering the request") || strings.Contains(body, "results") {
		t.Error("thinking and tool blocks must not be imported (owner decision, J26-E)")
	}
	if report.MachinerySkipped != 6 {
		t.Errorf("machinery_skipped=%d, want the three machinery blocks of both conversations", report.MachinerySkipped)
	}
	if !strings.Contains(body, "report.txt") || !strings.Contains(body, "the report body") {
		t.Errorf("an attachment's extracted text must be kept under its name:\n%s", body)
	}
	if !strings.Contains(body, "diagram.png") || !strings.Contains(body, "not in the archive") {
		t.Errorf("a file the export names but does not carry must say so:\n%s", body)
	}
	if report.AttachmentsSeen != 4 || report.FilesReferenced != 2 {
		t.Errorf("attachments_seen=%d files_referenced=%d, want 4 and 2", report.AttachmentsSeen, report.FilesReferenced)
	}

	project, err := st.GetDocument(ctx, "doc_claude_project_p_1")
	if err != nil {
		t.Fatalf("each project becomes a note (owner decision): %v", err)
	}
	if !strings.Contains(project.Body, "Everything about the study") ||
		!strings.Contains(project.Body, "You are a careful reader") ||
		!strings.Contains(project.Body, "Sample selection and limits") {
		t.Errorf("a project note carries its description, prompt template and docs:\n%s", project.Body)
	}
	if report.ProjectsSeen != 1 || report.ProjectsImported != 1 || report.ProjectDocs != 1 {
		t.Errorf("projects seen=%d imported=%d docs=%d, want 1 each", report.ProjectsSeen, report.ProjectsImported, report.ProjectDocs)
	}
}

func TestJ26ReadsEveryBatchZipOfOneExport(t *testing.T) {
	folder := t.TempDir()
	j26WriteZip(t, filepath.Join(folder, "claude-data-abc-1783442080-38379786-batch-0000.zip"), claudeExport("b-1"))
	second := claudeExport("b-2")
	delete(second, "projects/p-1.json")
	j26WriteZip(t, filepath.Join(folder, "claude-data-abc-1783442080-38379786-batch-0001.zip"), second)

	report, err := Import(context.Background(), newTestStore(t), folder, Options{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.ConversationsSeen != 2 {
		t.Errorf("conversations_seen=%d, want both batches read", report.ConversationsSeen)
	}
	if want := "claude-data-abc-1783442080-38379786-batch-0000.zip,claude-data-abc-1783442080-38379786-batch-0001.zip"; strings.Join(report.BatchArchives, ",") != want {
		t.Errorf("batch_archives=%v, want both in batch order", report.BatchArchives)
	}
}

func TestJ26ClaudeReImportChangesNothing(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	path := j26WriteZip(t, filepath.Join(t.TempDir(), "claude-data-abc-1-1-batch-0000.zip"), claudeExport("c-1"))
	if _, err := Import(ctx, st, path, Options{}); err != nil {
		t.Fatal(err)
	}
	again, err := Import(ctx, st, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if again.NotesImported != 0 || again.NotesUpdated != 0 || again.NotesUnchanged != 2 {
		t.Fatalf("re-import: imported=%d updated=%d unchanged=%d, want the conversation and the project unchanged",
			again.NotesImported, again.NotesUpdated, again.NotesUnchanged)
	}
}

func TestJ26RefusesSomethingThatIsNotAClaudeExport(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(plain, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Import(context.Background(), newTestStore(t), plain, Options{DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "neither a Claude export ZIP") {
		t.Fatalf("err=%v, want a clear refusal", err)
	}
}
