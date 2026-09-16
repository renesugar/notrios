// Package claude imports a Claude data export into the canonical store
// (Notrios redesign task R11; J26 made it read the archive as downloaded).
// Each conversation becomes one Markdown note (messages in order) in a "Claude"
// notebook, with provenance rows carrying the conversation UUID as the thread
// ID, and each project becomes a note of its own.
package claude

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/importers/archivesource"
	"github.com/renesugar/notrios/internal/importers/conversationnote"
	"github.com/renesugar/notrios/internal/store"
)

// NotebookID is the deterministic notebook for imported Claude conversations.
const NotebookID = "nb_claude"

// Options controls one import run.
type Options struct {
	CollectionID string
	NotebookName string // defaults to "Claude"
	DryRun       bool
}

// Report is the JSON-serializable import result.
type Report struct {
	ConversationsSeen int      `json:"conversations_seen"`
	NotesImported     int      `json:"notes_imported"`
	NotesUpdated      int      `json:"notes_updated"`
	NotesUnchanged    int      `json:"notes_unchanged"`
	MessagesImported  int      `json:"messages_imported"`
	MessagesSkipped   int      `json:"messages_skipped"`
	DryRun            bool     `json:"dry_run,omitempty"`
	NotebookID        string   `json:"notebook_id,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`

	// SourceFormat is "zip" for the archive as downloaded, or "directory".
	SourceFormat string `json:"source_format"`
	// ArchiveFiles names the conversation files read, in shard order, and
	// BatchArchives the batch ZIPs when a folder held several.
	ArchiveFiles    []string `json:"archive_files"`
	BatchArchives   []string `json:"batch_archives,omitempty"`
	CodeBlocks      int      `json:"code_blocks"`
	AttachmentsSeen int      `json:"attachments_seen"`
	// FilesReferenced counts file references that name a file the archive does
	// not carry: a Claude export holds no file bytes.
	FilesReferenced int `json:"files_referenced"`
	// MachinerySkipped counts thinking and tool blocks left out of notes by
	// owner decision (J26-E).
	MachinerySkipped int `json:"machinery_skipped"`
	ProjectsSeen     int `json:"projects_seen"`
	ProjectsImported int `json:"projects_imported"`
	ProjectDocs      int `json:"project_docs"`
	// ArchiveEntriesRejected counts ZIP entries refused as unsafe.
	ArchiveEntriesRejected int `json:"archive_entries_rejected"`
}

type conversation struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	Summary   string    `json:"summary"`
	CreatedAt string    `json:"created_at"`
	Messages  []message `json:"chat_messages"`
}

type message struct {
	UUID      string `json:"uuid"`
	Text      string `json:"text"`
	Sender    string `json:"sender"`
	CreatedAt string `json:"created_at"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Attachments []struct {
		FileName         string `json:"file_name"`
		FileSize         int64  `json:"file_size"`
		FileType         string `json:"file_type"`
		ExtractedContent string `json:"extracted_content"`
	} `json:"attachments"`
	Files []struct {
		FileUUID string `json:"file_uuid"`
		FileName string `json:"file_name"`
	} `json:"files"`
}

type project struct {
	UUID           string `json:"uuid"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PromptTemplate string `json:"prompt_template"`
	CreatedAt      string `json:"created_at"`
	Docs           []struct {
		UUID     string `json:"uuid"`
		Filename string `json:"filename"`
		Content  string `json:"content"`
	} `json:"docs"`
}

// Import reads a Claude export: the ZIP as downloaded, a folder holding the
// batch ZIPs of one export, an extracted folder, or a conversations.json.
func Import(ctx context.Context, st store.Store, source string, options Options) (Report, error) {
	report := Report{DryRun: options.DryRun}
	if strings.TrimSpace(options.CollectionID) == "" {
		options.CollectionID = "default"
	}
	if strings.TrimSpace(options.NotebookName) == "" {
		options.NotebookName = "Claude"
	}

	archives, err := openArchives(source, &report)
	if err != nil {
		return report, err
	}
	defer func() {
		for _, archive := range archives {
			archive.Close()
		}
	}()
	report.SourceFormat = archives[0].Kind()

	notebookID := ""
	if !options.DryRun {
		if notebookID, err = ensureNotebook(ctx, st, NotebookID, options.NotebookName, "✳️"); err != nil {
			return report, err
		}
		report.NotebookID = notebookID
	}

	for _, archive := range archives {
		names, err := archive.List("")
		if err != nil {
			return report, err
		}
		for _, name := range conversationFiles(names) {
			report.ArchiveFiles = append(report.ArchiveFiles, name)
			if err := eachEntry(archive, name, func(conv conversation) error {
				report.ConversationsSeen++
				if options.DryRun {
					report.NotesImported++
					return nil
				}
				return importConversation(ctx, st, conv, notebookID, options, &report)
			}); err != nil {
				return report, err
			}
		}
		if err := importProjects(ctx, st, archive, notebookID, options, &report); err != nil {
			return report, err
		}
	}
	return report, nil
}

// openArchives opens the export: one archive, or the batch ZIPs of a split
// export when a folder holds them.
func openArchives(source string, report *Report) ([]archivesource.Source, error) {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		entries, err := os.ReadDir(source)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				names = append(names, entry.Name())
			}
		}
		if batches := batchSiblings(names); len(batches) > 0 {
			sources := make([]archivesource.Source, 0, len(batches))
			for _, batch := range batches {
				archive, rejected, err := archivesource.Open(filepath.Join(source, batch), archiveSpec())
				if err != nil {
					for _, opened := range sources {
						opened.Close()
					}
					return nil, err
				}
				report.ArchiveEntriesRejected += rejected
				report.BatchArchives = append(report.BatchArchives, batch)
				sources = append(sources, archive)
			}
			return sources, nil
		}
	} else if err == nil && conversationsFileRE.MatchString(filepath.Base(source)) {
		// A path straight to conversations.json: its directory is the export.
		source = filepath.Dir(source)
	}
	archive, rejected, err := archivesource.Open(source, archiveSpec())
	if err != nil {
		return nil, err
	}
	report.ArchiveEntriesRejected += rejected
	if rejected > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%d archive entries were refused: a name escaping the archive, or not a regular file", rejected))
	}
	return []archivesource.Source{archive}, nil
}

// eachEntry streams one conversations file, handing over each conversation.
func eachEntry(archive archivesource.Source, name string, each func(conversation) error) error {
	file, err := archive.Open(name, archivesource.Limits.DataFileBytes)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = archivesource.DecodeArray(file, name, each)
	return err
}

func importConversation(ctx context.Context, st store.Store, conv conversation, notebookID string, options Options, report *Report) error {
	if strings.TrimSpace(conv.UUID) == "" {
		report.Warnings = append(report.Warnings, "conversation without a uuid skipped")
		return nil
	}
	docID := "doc_claude_" + sanitizeID(conv.UUID)
	created := parseISO(conv.CreatedAt)
	title := strings.TrimSpace(conv.Name)
	if title == "" {
		title = "Claude conversation " + created.UTC().Format("2006-01-02")
	}

	note := &conversationnote.Builder{}
	for _, msg := range conv.Messages {
		text, machinery := msg.render()
		note.Machinery(machinery)
		if strings.TrimSpace(text) == "" && len(msg.Attachments) == 0 && len(msg.Files) == 0 {
			note.Empty()
			continue
		}
		heading := conversationnote.Heading(msg.Sender, "")
		when := ""
		if ts := parseISO(msg.CreatedAt); !ts.IsZero() {
			when = ts.UTC().Format("2006-01-02 15:04 UTC")
		}
		note.Section(heading, when)
		note.Text(text)
		for _, attachment := range msg.Attachments {
			// A Claude export carries an attachment's extracted text, not its
			// bytes, so the note keeps the text under the file's name.
			note.Attachment(attachment.FileName, "")
			note.Code("", "", attachment.ExtractedContent)
		}
		for _, file := range msg.Files {
			note.Attachment(file.FileName, "")
			report.FilesReferenced++
		}
	}

	counts := note.Counts()
	report.MessagesImported += counts.Messages
	report.MessagesSkipped += counts.Empty
	report.CodeBlocks += counts.CodeBlocks
	report.AttachmentsSeen += counts.Attachments
	report.MachinerySkipped += counts.Machinery

	return upsertConversationNote(ctx, st, upsertNote{
		DocID: docID, Title: title, Body: note.Body(),
		SourceSystem: "claude", ExternalID: conv.UUID,
		NotebookID: notebookID, CollectionID: options.CollectionID,
		PublishedAt: publishedString(created),
	}, report)
}

// render returns a message's prose and how many machinery blocks it left out.
func (m message) render() (string, int) {
	if strings.TrimSpace(m.Text) != "" {
		return m.Text, 0
	}
	parts := []string{}
	machinery := 0
	for _, block := range m.Content {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				parts = append(parts, block.Text)
			}
		default:
			// thinking, tool_use, tool_result, token_budget: counted, not kept.
			machinery++
		}
	}
	return strings.Join(parts, "\n\n"), machinery
}

// importProjects imports each project in the export as its own note: what it
// was for, its prompt template, and each doc (owner decision, J26-D).
func importProjects(ctx context.Context, st store.Store, archive archivesource.Source, notebookID string, options Options, report *Report) error {
	names, err := archive.List("projects")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		file, err := archive.Open("projects/"+name, archivesource.Limits.DataFileBytes)
		if err != nil {
			return err
		}
		var p project
		err = archivesource.DecodeObject(file, "projects/"+name, &p)
		file.Close()
		if err != nil {
			return err
		}
		report.ProjectsSeen++
		report.ProjectDocs += len(p.Docs)
		if options.DryRun || strings.TrimSpace(p.UUID) == "" {
			continue
		}
		note := &conversationnote.Builder{}
		note.Section("Project", "")
		note.Text(p.Description)
		if strings.TrimSpace(p.PromptTemplate) != "" {
			note.Code("Prompt template", "", p.PromptTemplate)
		}
		for _, doc := range p.Docs {
			note.Section(doc.Filename, "")
			note.Text(doc.Content)
		}
		title := strings.TrimSpace(p.Name)
		if title == "" {
			title = "Claude project"
		}
		before := report.NotesImported
		if err := upsertConversationNote(ctx, st, upsertNote{
			DocID: "doc_claude_project_" + sanitizeID(p.UUID), Title: title, Body: note.Body(),
			SourceSystem: "claude", ExternalID: "project:" + p.UUID,
			NotebookID: notebookID, CollectionID: options.CollectionID,
			PublishedAt: publishedString(parseISO(p.CreatedAt)),
		}, report); err != nil {
			return err
		}
		if report.NotesImported > before {
			report.ProjectsImported++
		}
	}
	return nil
}

// upsertNote carries the shared conversation-note upsert parameters.
type upsertNote struct {
	DocID        string
	Title        string
	Body         string
	SourceSystem string
	ExternalID   string
	NotebookID   string
	CollectionID string
	PublishedAt  string
}

// upsertConversationNote mirrors the chatgpt importer's semantics: unchanged
// detection, revision-checked updates, no resurrection of user-trashed notes,
// and a provenance row whose thread ID is the conversation.
func upsertConversationNote(ctx context.Context, st store.Store, note upsertNote, report *Report) error {
	existing, err := st.GetDocument(ctx, note.DocID)
	switch {
	case err == nil:
		if existing.Title == note.Title && existing.Body == note.Body {
			report.NotesUnchanged++
		} else {
			if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
				ID: note.DocID, Title: note.Title, Body: note.Body, BodyMIMEType: "text/markdown",
				BaseRevisionID: existing.CurrentRevisionID, Message: "import update from " + note.SourceSystem + " export",
			}); err != nil {
				return err
			}
			report.NotesUpdated++
		}
	case errors.Is(err, store.ErrNotFound):
		if _, srcErr := st.FindDocumentBySource(ctx, note.SourceSystem, note.ExternalID); srcErr == nil {
			report.NotesUnchanged++
		} else if !errors.Is(srcErr, store.ErrNotFound) {
			return srcErr
		} else {
			if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
				PreferredID: note.DocID, CollectionID: note.CollectionID, NotebookID: note.NotebookID,
				Title: note.Title, Body: note.Body, BodyMIMEType: "text/markdown",
				Message: "import from " + note.SourceSystem + " export",
			}); err != nil {
				return err
			}
			report.NotesImported++
		}
	default:
		return err
	}
	_, err = st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{
		DocumentID:   note.DocID,
		SourceSystem: note.SourceSystem,
		ExternalID:   note.ExternalID,
		ThreadID:     note.ExternalID,
		PublishedAt:  note.PublishedAt,
	})
	return err
}

func parseISO(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func publishedString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func ensureNotebook(ctx context.Context, st store.Store, preferredID, name, emoji string) (string, error) {
	nb, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: preferredID, Name: name, IconEmoji: emoji})
	if err == nil {
		return nb.ID, nil
	}
	if errors.Is(err, store.ErrNameConflict) {
		notebooks, listErr := st.ListNotebooks(ctx)
		if listErr != nil {
			return "", listErr
		}
		for _, existing := range notebooks {
			if strings.EqualFold(existing.Name, name) && existing.ParentID == "" {
				return existing.ID, nil
			}
		}
	}
	if existing, getErr := st.GetNotebook(ctx, preferredID); getErr == nil {
		return existing.ID, nil
	}
	return "", err
}

func sanitizeID(name string) string {
	name = strings.ToLower(name)
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, name)
	return strings.Trim(mapped, "_")
}
