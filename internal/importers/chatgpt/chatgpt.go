// Package chatgpt imports a ChatGPT data-export conversations.json into the
// canonical store (Notrios redesign task R10). Each conversation becomes one
// Markdown note (messages as sections along the current-node main path, in
// order) in a "ChatGPT" notebook, with provenance rows carrying the
// conversation ID as the thread ID. Format references:
// github.com/temnoon/openai_export_parser and
// github.com/slyubarskiy/chatgpt-conversation-extractor.
package chatgpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// NotebookID is the deterministic notebook for imported ChatGPT conversations.
const NotebookID = "nb_chatgpt"

// Options controls one import run.
type Options struct {
	CollectionID string
	NotebookName string // defaults to "ChatGPT"
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
	Title          string                 `json:"title"`
	CreateTime     float64                `json:"create_time"`
	UpdateTime     float64                `json:"update_time"`
	ConversationID string                 `json:"conversation_id"`
	ID             string                 `json:"id"`
	CurrentNode    string                 `json:"current_node"`
	Mapping        map[string]mappingNode `json:"mapping"`
}

type mappingNode struct {
	ID      string   `json:"id"`
	Parent  string   `json:"parent"`
	Message *message `json:"message"`
}

type message struct {
	ID     string `json:"id"`
	Author struct {
		Role string `json:"role"`
		Name string `json:"name"`
	} `json:"author"`
	CreateTime float64 `json:"create_time"`
	Content    struct {
		ContentType string `json:"content_type"`
		Parts       []any  `json:"parts"`
		Text        string `json:"text"`
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
		options.NotebookName = "ChatGPT"
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
	notebookID, err := ensureNotebook(ctx, st, NotebookID, options.NotebookName, "🤖")
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
		return nil, fmt.Errorf("read ChatGPT export: %w", err)
	}
	var conversations []conversation
	if err := json.Unmarshal(raw, &conversations); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return conversations, nil
}

func (c conversation) externalID() string {
	if strings.TrimSpace(c.ConversationID) != "" {
		return c.ConversationID
	}
	return c.ID
}

// mainPath returns the conversation's messages along the current-node chain
// (the visible branch after edits/regenerations), oldest first. When
// current_node is missing, all messages are used in create-time order.
func (c conversation) mainPath() []*message {
	messages := []*message{}
	if node, ok := c.Mapping[c.CurrentNode]; ok && c.CurrentNode != "" {
		for {
			if node.Message != nil {
				messages = append(messages, node.Message)
			}
			parent, ok := c.Mapping[node.Parent]
			if !ok || node.Parent == "" {
				break
			}
			node = parent
		}
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
		return messages
	}
	for _, node := range c.Mapping {
		if node.Message != nil {
			messages = append(messages, node.Message)
		}
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreateTime < messages[j].CreateTime })
	return messages
}

func (m *message) text() string {
	if m.Content.Text != "" {
		return m.Content.Text
	}
	parts := []string{}
	for _, part := range m.Content.Parts {
		if s, ok := part.(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

func importConversation(ctx context.Context, st store.Store, conv conversation, notebookID string, options Options, report *Report) error {
	externalID := conv.externalID()
	if externalID == "" {
		report.Warnings = append(report.Warnings, "conversation without an ID skipped")
		return nil
	}
	docID := "doc_chatgpt_" + sanitizeID(externalID)
	created := epochToTime(conv.CreateTime)
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "ChatGPT conversation " + created.UTC().Format("2006-01-02")
	}

	var b strings.Builder
	for _, msg := range conv.mainPath() {
		text := strings.TrimSpace(msg.text())
		role := strings.TrimSpace(msg.Author.Role)
		if text == "" || role == "system" || role == "tool" {
			report.MessagesSkipped++
			continue
		}
		heading := roleHeading(role, msg.Author.Name)
		if ts := epochToTime(msg.CreateTime); !ts.IsZero() {
			heading += " — " + ts.UTC().Format("2006-01-02 15:04 UTC")
		}
		b.WriteString("## " + heading + "\n\n" + text + "\n\n")
		report.MessagesImported++
	}
	body := strings.TrimSpace(b.String()) + "\n"

	if err := upsertConversationNote(ctx, st, upsertNote{
		DocID: docID, Title: title, Body: body,
		SourceSystem: "chatgpt", ExternalID: externalID,
		NotebookID: notebookID, CollectionID: options.CollectionID,
		PublishedAt: publishedString(created),
	}, report); err != nil {
		return err
	}
	return nil
}

func roleHeading(role, name string) string {
	if name != "" {
		return name
	}
	switch role {
	case "user":
		return "User"
	case "assistant":
		return "Assistant"
	default:
		if role == "" {
			return "Message"
		}
		return strings.ToUpper(role[:1]) + role[1:]
	}
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

// upsertConversationNote applies the shared importer semantics: unchanged
// detection, updates with revision preconditions, no resurrection of notes
// the user trashed, and a provenance row whose thread ID is the conversation.
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
			// The note exists but sits in the user's Trash: refresh
			// provenance only, never resurrect.
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

func epochToTime(epoch float64) time.Time {
	if epoch <= 0 {
		return time.Time{}
	}
	return time.Unix(int64(epoch), 0)
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
