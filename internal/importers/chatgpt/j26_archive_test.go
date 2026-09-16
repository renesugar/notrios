package chatgpt

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J26: ChatGPT is exported in two shapes, and both arrive as a ZIP. Every
// fixture here is synthetic; no real archive content is in the repository.

const (
	// pngBytes and pdfBytes begin with the magic an asset's type is sniffed
	// from, for the Privacy Portal's .dat files that no record names.
	pngBytes = "\x89PNG\r\n\x1a\n........................"
	pdfBytes = "%PDF-1.4\n%âãÏÓ\n1 0 obj\n"
)

// j26Conversation builds one conversation: a user message, an assistant code
// block, its execution output, a thought (machinery), and a message carrying
// two assets, one by asset pointer and one by attachment.
func j26Conversation(id, title, pointerID, attachmentID string) string {
	return `{
      "id": "` + id + `", "conversation_id": "` + id + `", "title": "` + title + `",
      "create_time": 1783958400, "current_node": "n5",
      "mapping": {
        "n1": {"id": "n1", "parent": "", "message": {"id": "m1", "author": {"role": "user"}, "create_time": 1783958400,
          "content": {"content_type": "text", "parts": ["Please plot the series."]}}},
        "n2": {"id": "n2", "parent": "n1", "message": {"id": "m2", "author": {"role": "assistant"}, "create_time": 1783958401,
          "content": {"content_type": "thoughts", "parts": ["The user wants a chart."]}}},
        "n3": {"id": "n3", "parent": "n2", "message": {"id": "m3", "author": {"role": "assistant"}, "create_time": 1783958402,
          "content": {"content_type": "code", "language": "python", "text": "print(sum(series))"}}},
        "n4": {"id": "n4", "parent": "n3", "message": {"id": "m4", "author": {"role": "tool"}, "create_time": 1783958403,
          "content": {"content_type": "execution_output", "text": "42"}}},
        "n5": {"id": "n5", "parent": "n4", "message": {"id": "m5", "author": {"role": "user"}, "create_time": 1783958404,
          "content": {"content_type": "multimodal_text", "parts": ["Here it is", {"content_type": "image_asset_pointer", "asset_pointer": "file-service://` + pointerID + `"}]},
          "metadata": {"attachments": [{"id": "` + attachmentID + `", "name": "notes.txt", "mime_type": "text/plain"}]}}}
      }
    }`
}

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

func j26WriteDir(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// directExport is the export from ChatGPT itself: conversations.json beside the
// assets, whose names carry the real file name under either id prefix, and one
// asset in the user-<id> subfolder.
func directExport() map[string]string {
	return map[string]string{
		"conversations.json":          "[\n" + j26Conversation("c1", "Plotting", "file-AAA", "file_bbb") + "\n]",
		"conversations-000.json":      "[\n" + j26Conversation("c2", "Second shard", "file-AAA", "file_bbb") + "\n]",
		"file-AAA-chart.png":          pngBytes,
		"user-XYZ/file_bbb-notes.txt": "the notes",
		"chat.html":                   "<html>rendered</html>",
	}
}

func TestJ26ImportsTheChatGPTExportZipAndFolderAlike(t *testing.T) {
	files := directExport()
	sources := map[string]string{
		"zip":    j26WriteZip(t, filepath.Join(t.TempDir(), "chatgpt_data_export.zip"), files),
		"folder": j26WriteDir(t, files),
	}
	for kind, source := range sources {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			st := newTestStore(t)
			report, err := Import(ctx, st, source, Options{})
			if err != nil {
				t.Fatalf("Import: %v", err)
			}
			if report.ConversationsSeen != 2 || report.NotesImported != 2 {
				t.Errorf("conversations_seen=%d notes_imported=%d, want 2 each: every conversations shard is read",
					report.ConversationsSeen, report.NotesImported)
			}
			if want := "conversations.json,conversations-000.json"; strings.Join(report.ArchiveFiles, ",") != want {
				t.Errorf("archive_files=%v, want %s in shard order", report.ArchiveFiles, want)
			}
			document, err := st.GetDocument(ctx, "doc_chatgpt_c1")
			if err != nil {
				t.Fatal(err)
			}
			body := document.Body
			if !strings.Contains(body, "```python\nprint(sum(series))\n```") {
				t.Errorf("code must be a fenced block with its language:\n%s", body)
			}
			if !strings.Contains(body, "**Output**") || !strings.Contains(body, "42") {
				t.Errorf("execution output must be rendered:\n%s", body)
			}
			if strings.Contains(body, "The user wants a chart") {
				t.Error("thinking must not be imported (owner decision, J26-E)")
			}
			if report.MachinerySkipped != 2 {
				t.Errorf("machinery_skipped=%d, want the two thoughts counted", report.MachinerySkipped)
			}
			if report.CodeBlocks != 4 {
				t.Errorf("code_blocks=%d, want the code and output of both conversations", report.CodeBlocks)
			}
			if report.AssetsImported != 2 || report.AssetsMissing != 0 {
				t.Errorf("assets_imported=%d assets_missing=%d, want 2 and 0 (both id prefixes, one in a subfolder)",
					report.AssetsImported, report.AssetsMissing)
			}
			if !strings.Contains(body, "chart.png") || !strings.Contains(body, "notes.txt") {
				t.Errorf("attachments must be named in the note:\n%s", body)
			}
			if !strings.Contains(body, "resource://") {
				t.Errorf("an imported asset must be embedded as a resource:\n%s", body)
			}
		})
	}
}

func TestJ26AcceptsAPathStraightToConversationsJSON(t *testing.T) {
	root := j26WriteDir(t, directExport())
	report, err := Import(context.Background(), newTestStore(t), filepath.Join(root, "conversations.json"), Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.ConversationsSeen != 2 {
		t.Fatalf("conversations_seen=%d, want 2", report.ConversationsSeen)
	}
}

// privacyPortalExport is the export from the OpenAI Privacy Portal: an outer
// ZIP whose User Online Activity folder holds the conversations and the file
// library as ZIPs of their own, with assets stripped to file-<id>.dat.
func privacyPortalExport(t *testing.T) string {
	t.Helper()
	conversations := map[string]string{
		"conversations-000.json": "[\n" + j26Conversation("p1", "From the portal", "file-DAT1", "file_LIB2") + "\n]",
		"conversations-001.json": "[\n" + j26Conversation("p2", "Second shard", "file_SNIFF3", "file_SNIFF3") + "\n]",
		// The asset map names one .dat file; library_files.json names another;
		// the third has no record at all and is sniffed.
		"conversation_asset_file_names.json": `{"file-DAT1.dat": "diagram.png"}`,
		"library_files.json":                 `[{"file_id": "file_LIB2", "file_name": "report", "file_extension": "pdf", "mime_type": "application/pdf"}]`,
		"file-DAT1.dat":                      pngBytes,
		"file_LIB2.dat":                      pdfBytes,
		"file_SNIFF3.dat":                    pngBytes,
		"chat.html":                          "<html>rendered with .dat names</html>",
	}
	library := map[string]string{
		"personal/files/holiday.jpeg": "\xff\xd8\xff\xe0JFIF................",
	}
	dir := t.TempDir()
	conversationsZip, err := os.ReadFile(j26WriteZip(t, filepath.Join(dir, "conv.zip"), conversations))
	if err != nil {
		t.Fatal(err)
	}
	libraryZip, err := os.ReadFile(j26WriteZip(t, filepath.Join(dir, "files.zip"), library))
	if err != nil {
		t.Fatal(err)
	}
	return j26WriteZip(t, filepath.Join(t.TempDir(), "OpenAI-export.zip"), map[string]string{
		"report.html": "<html>portal report</html>",
		"User Online Activity/Conversations__digest-chatgpt-0001.zip": string(conversationsZip),
		"User Online Activity/Files__digest-files-0001.zip":           string(libraryZip),
		"User Online Activity/Ads__digest-ads-0001.zip":               "",
	})
}

func TestJ26ImportsThePrivacyPortalExportFromItsNestedZips(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	report, err := Import(ctx, st, privacyPortalExport(t), Options{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.SourceFormat != "privacy-portal-zip" {
		t.Errorf("source_format=%q, want privacy-portal-zip", report.SourceFormat)
	}
	if len(report.NestedArchives) != 2 {
		t.Errorf("nested_archives=%v, want the conversations and files ZIPs", report.NestedArchives)
	}
	if report.ConversationsSeen != 2 {
		t.Errorf("conversations_seen=%d, want both shards inside the nested ZIP", report.ConversationsSeen)
	}
	if report.AssetNamesFromMap != 1 || report.AssetNamesFromLibrary != 1 || report.AssetNamesSniffed != 1 {
		t.Errorf("asset names: map=%d library=%d sniffed=%d, want 1 each",
			report.AssetNamesFromMap, report.AssetNamesFromLibrary, report.AssetNamesSniffed)
	}
	first, err := st.GetDocument(ctx, "doc_chatgpt_p1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Body, "diagram.png") {
		t.Errorf("a .dat asset must take the name the asset map gives it:\n%s", first.Body)
	}
	if !strings.Contains(first.Body, "report.pdf") {
		t.Errorf("a .dat asset must take the name and extension library_files.json gives it:\n%s", first.Body)
	}
	second, err := st.GetDocument(ctx, "doc_chatgpt_p2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(second.Body, ".png") {
		t.Errorf("an unnamed .dat asset must take the extension its bytes imply:\n%s", second.Body)
	}
	if report.LibraryFilesSeen != 1 || report.LibraryFilesImported != 1 || report.LibraryStubNotes != 1 {
		t.Errorf("library seen=%d imported=%d stubs=%d, want 1 each: a file no conversation references still gets a note",
			report.LibraryFilesSeen, report.LibraryFilesImported, report.LibraryStubNotes)
	}
	if _, err := st.GetNotebook(ctx, FilesNotebookID); err != nil {
		t.Errorf("the unreferenced library files need their own notebook: %v", err)
	}
}

func TestJ26ReImportOfAnArchiveChangesNothing(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	source := privacyPortalExport(t)
	if _, err := Import(ctx, st, source, Options{}); err != nil {
		t.Fatal(err)
	}
	again, err := Import(ctx, st, source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if again.NotesImported != 0 || again.NotesUpdated != 0 || again.NotesUnchanged == 0 {
		t.Fatalf("re-import: imported=%d updated=%d unchanged=%d, want everything unchanged",
			again.NotesImported, again.NotesUpdated, again.NotesUnchanged)
	}
}

func TestJ26RefusesSomethingThatIsNeitherExport(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(plain, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Import(context.Background(), newTestStore(t), plain, Options{DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "neither a ChatGPT export ZIP") {
		t.Fatalf("err=%v, want a clear refusal", err)
	}
}
