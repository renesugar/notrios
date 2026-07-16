// Package claude imports a Claude data-export conversations.json into the
// canonical store (Notrios redesign task R11). Each conversation becomes one
// Markdown note (messages in order) in a "Claude" notebook, with provenance
// rows carrying the conversation UUID as the thread ID.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
}

type conversation struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
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
}

// Import reads conversations.json (a path to the file, or to a directory
// containing it).
func Import(ctx context.Context, st store.Store, source string, options Options) (Report, error) {
	report := Report{DryRun: options.DryRun}
	if strings.TrimSpace(options.CollectionID) == "" {
		options.CollectionID = "default"
	}
	if strings.TrimSpace(options.NotebookName) == "" {
		options.NotebookName = "Claude"
	}
	conversations, err := loadConversations(source)
	if err != nil {
		return report, err
	}
	report.ConversationsSeen = len(conversations)
	if options.DryRun {
		report.NotesImported = len(conversations)
		return report, nil
	}
	notebookID, err := ensureNotebook(ctx, st, NotebookID, options.NotebookName, "✳️")
	if err != nil {
		return report, err
	}
	report.NotebookID = notebookID

	for _, conv := range conversations {
		if err := importConversation(ctx, st, conv, notebookID, options, &report); err != nil {
			return report, err
		}
	}
	return report, nil
}

func loadConversations(source string) ([]conversation, error) {
	path := source
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		path = filepath.Join(source, "conversations.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Claude export: %w", err)
	}
	var conversations []conversation
	if err := json.Unmarshal(raw, &conversations); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return conversations, nil
}

func (m message) text() string {
	if strings.TrimSpace(m.Text) != "" {
		return m.Text
	}
	parts := []string{}
	for _, block := range m.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func importConversation(ctx context.Context, st store.Store, conv conversation, notebookID string, options Options, report *Report) error {
	if strings.TrimSpace(conv.UUID) == "" {
		report.Warnings = append(report.Warnings, "conversation without a uuid skipped")
		return nil
	}
	docID := "doc_claude_" + sanitizeID(conv.UUID)
	title := strings.TrimSpace(conv.Name)
	created := parseISO(conv.CreatedAt)
	if title == "" {
		title = "Claude conversation " + created.UTC().Format("2006-01-02")
	}

	var b strings.Builder
	for _, msg := range conv.Messages {
		text := strings.TrimSpace(msg.text())
		if text == "" {
			report.MessagesSkipped++
			continue
		}
		heading := "Assistant"
		if strings.EqualFold(msg.Sender, "human") {
			heading = "User"
		}
		if ts := parseISO(msg.CreatedAt); !ts.IsZero() {
			heading += " — " + ts.UTC().Format("2006-01-02 15:04 UTC")
		}
		b.WriteString("## " + heading + "\n\n" + text + "\n\n")
		report.MessagesImported++
	}
	body := strings.TrimSpace(b.String()) + "\n"

	return upsertConversationNote(ctx, st, upsertNote{
		DocID: docID, Title: title, Body: body,
		SourceSystem: "claude", ExternalID: conv.UUID,
		NotebookID: notebookID, CollectionID: options.CollectionID,
		PublishedAt: publishedString(created),
	}, report)
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
