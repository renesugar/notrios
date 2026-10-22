// Package chatgpt imports a ChatGPT data export into the canonical store
// (Notrios redesign task R10; J26 made it read the archive as downloaded, in
// both of the shapes OpenAI ships). Each conversation becomes one Markdown note
// in a "ChatGPT" notebook, with provenance rows carrying the conversation ID as
// the thread ID, its code and execution output as fenced blocks, and its
// attachments as resources. Format references:
// github.com/temnoon/openai_export_parser and
// github.com/slyubarskiy/chatgpt-conversation-extractor.
package chatgpt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/importers/archivesource"
	"github.com/renesugar/notrios/internal/importers/conversationnote"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/tempspace"
)

// NotebookID is the deterministic notebook for imported ChatGPT conversations.
const NotebookID = "nb_chatgpt"

// FilesNotebookID holds the file-library files no conversation references
// (owner decision, J26-C).
const FilesNotebookID = "nb_chatgpt_files"

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

	// SourceFormat is "chatgpt-zip", "privacy-portal-zip" or "directory".
	SourceFormat string `json:"source_format"`
	// ArchiveFiles names the conversations files read, in shard order.
	ArchiveFiles []string `json:"archive_files"`
	// NestedArchives names the ZIPs opened inside the archive, for a Privacy
	// Portal export.
	NestedArchives []string `json:"nested_archives,omitempty"`
	CodeBlocks     int      `json:"code_blocks"`
	// MachinerySkipped counts thoughts, reasoning recaps and tool plumbing
	// left out of notes by owner decision (J26-E).
	MachinerySkipped int `json:"machinery_skipped"`
	// AssetsReferenced counts message attachments and image parts;
	// AssetsImported those whose bytes the archive held.
	AssetsReferenced int `json:"assets_referenced"`
	AssetsImported   int `json:"assets_imported"`
	AssetsMissing    int `json:"assets_missing"`
	// AssetNamesFrom says where a .dat file's real name came from.
	AssetNamesFromMap     int `json:"asset_names_from_map"`
	AssetNamesFromLibrary int `json:"asset_names_from_library"`
	AssetNamesSniffed     int `json:"asset_names_sniffed"`
	// The file library, which the Privacy Portal ships as its own ZIP.
	LibraryFilesSeen     int `json:"library_files_seen"`
	LibraryFilesImported int `json:"library_files_imported"`
	LibraryStubNotes     int `json:"library_stub_notes"`
	// ArchiveEntriesRejected counts ZIP entries refused as unsafe.
	ArchiveEntriesRejected int `json:"archive_entries_rejected"`
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
		Language    string `json:"language"`
		Result      string `json:"result"`
	} `json:"content"`
	Metadata struct {
		Attachments []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			MimeType string `json:"mime_type"`
		} `json:"attachments"`
	} `json:"metadata"`
}

// Import reads a ChatGPT export: the ZIP as downloaded, an OpenAI Privacy
// Portal export ZIP, an extracted folder, or a conversations.json.
func Import(ctx context.Context, st store.Store, source string, options Options) (Report, error) {
	report := Report{DryRun: options.DryRun}
	if strings.TrimSpace(options.CollectionID) == "" {
		options.CollectionID = "default"
	}
	if strings.TrimSpace(options.NotebookName) == "" {
		options.NotebookName = "ChatGPT"
	}

	opened, err := openExport(source, store.TempSpaceOf(st), &report)
	if err != nil {
		return report, err
	}
	defer opened.close()

	assets, err := newAssetIndex(opened, &report)
	if err != nil {
		return report, err
	}

	notebookID := ""
	if !options.DryRun {
		if notebookID, err = ensureNotebook(ctx, st, NotebookID, options.NotebookName, "🤖"); err != nil {
			return report, err
		}
		report.NotebookID = notebookID
	}

	names, err := opened.conversations.List("")
	if err != nil {
		return report, err
	}
	run := &importRun{st: st, assets: assets, options: options, report: &report, notebookID: notebookID}
	for _, name := range conversationFiles(names) {
		report.ArchiveFiles = append(report.ArchiveFiles, name)
		file, err := opened.conversations.Open(name, archivesource.Limits.DataFileBytes)
		if err != nil {
			return report, err
		}
		_, err = archivesource.DecodeArray(file, name, func(conv conversation) error {
			report.ConversationsSeen++
			if options.DryRun {
				report.NotesImported++
				run.countDryRun(conv)
				return nil
			}
			return run.importConversation(ctx, conv)
		})
		file.Close()
		if err != nil {
			return report, err
		}
	}
	if err := run.importFileLibrary(ctx, opened); err != nil {
		return report, err
	}
	return report, nil
}

// export is the archive being read: the conversations, and, for a Privacy
// Portal export, the separate file library.
type export struct {
	conversations archivesource.Source
	library       archivesource.Source
	outer         archivesource.Source
}

func (e *export) close() {
	for _, source := range []archivesource.Source{e.library, e.conversations, e.outer} {
		if source != nil {
			source.Close()
		}
	}
}

// openExport opens whichever shape the user handed over. The Privacy Portal
// export is tried first: its conversations live in a ZIP inside the ZIP, so a
// plain ChatGPT reading of it would find no conversations file at all.
func openExport(source string, space *tempspace.Space, report *Report) (*export, error) {
	if info, err := os.Stat(source); err == nil && !info.IsDir() && conversationsFileRE.MatchString(filepath.Base(source)) {
		// A path straight to conversations.json: its directory is the export.
		source = filepath.Dir(source)
	}
	if outer, rejected, err := archivesource.Open(source, portalSpec()); err == nil {
		report.SourceFormat = "privacy-portal-zip"
		report.ArchiveEntriesRejected += rejected
		result := &export{outer: outer}
		names, err := outer.List("")
		if err != nil {
			outer.Close()
			return nil, err
		}
		for _, name := range names {
			switch {
			case portalConversationsRE.MatchString(name) && result.conversations == nil:
				inner, innerRejected, err := archivesource.Nested(space, outer, name, archiveSpec())
				if err != nil {
					result.close()
					return nil, err
				}
				report.ArchiveEntriesRejected += innerRejected
				report.NestedArchives = append(report.NestedArchives, name)
				result.conversations = inner
			case portalFilesRE.MatchString(name) && result.library == nil:
				inner, innerRejected, err := archivesource.Nested(space, outer, name, librarySpec())
				if err != nil {
					// A portal export without a readable file library still
					// imports its conversations.
					report.Warnings = append(report.Warnings, fmt.Sprintf("file library %s: %v", name, err))
					continue
				}
				report.ArchiveEntriesRejected += innerRejected
				report.NestedArchives = append(report.NestedArchives, name)
				result.library = inner
			}
		}
		if result.conversations == nil {
			result.close()
			return nil, fmt.Errorf("no Conversations__*.zip in %s", filepath.Base(source))
		}
		return result, nil
	}

	archive, rejected, err := archivesource.Open(source, archiveSpec())
	if err != nil {
		return nil, err
	}
	report.ArchiveEntriesRejected += rejected
	if rejected > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%d archive entries were refused: a name escaping the archive, or not a regular file", rejected))
	}
	report.SourceFormat = "chatgpt-zip"
	if archive.Kind() == archivesource.KindDirectory {
		report.SourceFormat = archivesource.KindDirectory
	}
	return &export{conversations: archive}, nil
}

func (c conversation) externalID() string {
	if strings.TrimSpace(c.ConversationID) != "" {
		return c.ConversationID
	}
	return c.ID
}

// mainPath returns the conversation's messages along the current-node chain
// (the visible branch after edits and regenerations), oldest first. When
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

// textParts returns a message's prose, and the asset IDs its parts point at.
func (m *message) textParts() (string, []string) {
	pointers := []string{}
	if strings.TrimSpace(m.Content.Text) != "" {
		return m.Content.Text, pointers
	}
	parts := []string{}
	for _, part := range m.Content.Parts {
		switch value := part.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				parts = append(parts, value)
			}
		case map[string]any:
			if pointer, ok := value["asset_pointer"].(string); ok {
				if id := assetID(pointer); id != "" {
					pointers = append(pointers, id)
				}
			}
		}
	}
	return strings.Join(parts, "\n\n"), pointers
}

// importRun carries what one import needs across conversations.
type importRun struct {
	st         store.Store
	assets     *assetIndex
	options    Options
	report     *Report
	notebookID string
	// referenced records the library files a conversation used, so the rest
	// can get their own notes.
	referenced map[string]bool
}

func (r *importRun) countDryRun(conv conversation) {
	for _, msg := range conv.mainPath() {
		_, pointers := msg.textParts()
		r.report.AssetsReferenced += len(pointers) + len(msg.Metadata.Attachments)
		switch msg.Content.ContentType {
		case "thoughts", "reasoning_recap":
			r.report.MachinerySkipped++
		case "code", "execution_output":
			r.report.CodeBlocks++
		}
	}
}

func (r *importRun) importConversation(ctx context.Context, conv conversation) error {
	externalID := conv.externalID()
	if externalID == "" {
		r.report.Warnings = append(r.report.Warnings, "conversation without an ID skipped")
		return nil
	}
	docID := "doc_chatgpt_" + sanitizeID(externalID)
	created := epochToTime(conv.CreateTime)
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "ChatGPT conversation " + created.UTC().Format("2006-01-02")
	}

	note := &conversationnote.Builder{}
	for _, msg := range conv.mainPath() {
		if err := r.renderMessage(ctx, note, msg); err != nil {
			return err
		}
	}

	counts := note.Counts()
	r.report.MessagesImported += counts.Messages
	r.report.MessagesSkipped += counts.Empty
	r.report.CodeBlocks += counts.CodeBlocks
	r.report.MachinerySkipped += counts.Machinery

	return upsertConversationNote(ctx, r.st, upsertNote{
		DocID: docID, Title: title, Body: note.Body(),
		SourceSystem: "chatgpt", ExternalID: externalID,
		NotebookID: r.notebookID, CollectionID: r.options.CollectionID,
		PublishedAt: publishedString(created),
	}, r.report)
}

// renderMessage writes one message into the note: prose, code, output and
// attachments, counting the machinery it leaves out (owner decision, J26-E).
func (r *importRun) renderMessage(ctx context.Context, note *conversationnote.Builder, msg *message) error {
	role := strings.TrimSpace(msg.Author.Role)
	switch msg.Content.ContentType {
	case "thoughts", "reasoning_recap":
		note.Machinery(1)
		return nil
	}
	text, pointers := msg.textParts()
	if strings.TrimSpace(text) == "" && len(pointers) == 0 && len(msg.Metadata.Attachments) == 0 {
		note.Empty()
		return nil
	}
	if role == "system" && strings.TrimSpace(text) == "" {
		note.Empty()
		return nil
	}

	when := ""
	if ts := epochToTime(msg.CreateTime); !ts.IsZero() {
		when = ts.UTC().Format("2006-01-02 15:04 UTC")
	}
	note.Section(conversationnote.Heading(role, msg.Author.Name), when)

	switch msg.Content.ContentType {
	case "code":
		note.Code("", msg.Content.Language, text)
	case "execution_output":
		note.Code("Output", "", firstNonEmpty(msg.Content.Text, msg.Content.Result, text))
	default:
		note.Text(text)
	}

	seen := map[string]bool{}
	for _, id := range pointers {
		if err := r.attach(ctx, note, id, "", seen); err != nil {
			return err
		}
	}
	for _, attachment := range msg.Metadata.Attachments {
		if err := r.attach(ctx, note, assetID(attachment.ID), attachment.Name, seen); err != nil {
			return err
		}
	}
	return nil
}

// attach imports one asset and embeds it, or names it when the archive does not
// carry its bytes.
func (r *importRun) attach(ctx context.Context, note *conversationnote.Builder, id, fallbackName string, seen map[string]bool) error {
	if id == "" || seen[id] {
		return nil
	}
	seen[id] = true
	r.report.AssetsReferenced++
	resourceID, display, err := r.assets.importAsset(ctx, r.st, id, r.options.CollectionID, r.report)
	if err != nil {
		return err
	}
	if resourceID == "" {
		r.report.AssetsMissing++
		note.Attachment(firstNonEmpty(display, fallbackName, id), "")
		return nil
	}
	if r.referenced == nil {
		r.referenced = map[string]bool{}
	}
	r.referenced[id] = true
	note.Attachment(firstNonEmpty(display, fallbackName, id), store.ResourceURI(r.options.CollectionID, resourceID))
	return nil
}

// importFileLibrary imports the Privacy Portal's file library. A file a
// conversation referenced is already attached to that note; the rest get a stub
// note of their own, so nothing in the archive is silently dropped (owner
// decision, J26-C).
func (r *importRun) importFileLibrary(ctx context.Context, opened *export) error {
	if opened.library == nil {
		return nil
	}
	names, err := opened.library.All()
	if err != nil {
		return err
	}
	r.report.LibraryFilesSeen = len(names)
	if r.options.DryRun {
		return nil
	}
	notebookID, err := ensureNotebook(ctx, r.st, FilesNotebookID, "ChatGPT Files", "📎")
	if err != nil {
		return err
	}
	for _, name := range names {
		id := assetID(name)
		if id != "" && r.referenced[id] {
			continue
		}
		resourceID, display, err := importOne(ctx, r.st, opened.library, name, path.Base(name), "", r.options.CollectionID, r.report)
		if err != nil {
			r.report.Warnings = append(r.report.Warnings, fmt.Sprintf("library file %s: %v", path.Base(name), err))
			continue
		}
		r.report.LibraryFilesImported++
		note := &conversationnote.Builder{}
		note.Section("File", "")
		note.Attachment(display, store.ResourceURI(r.options.CollectionID, resourceID))
		docID := "doc_chatgpt_file_" + sanitizeID(firstNonEmpty(id, name))
		before := r.report.NotesImported
		if err := upsertConversationNote(ctx, r.st, upsertNote{
			DocID: docID, Title: display, Body: note.Body(),
			SourceSystem: "chatgpt", ExternalID: "file:" + firstNonEmpty(id, name),
			NotebookID: notebookID, CollectionID: r.options.CollectionID,
		}, r.report); err != nil {
			return err
		}
		if r.report.NotesImported > before {
			r.report.LibraryStubNotes++
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
// detection, updates with revision preconditions, no resurrection of notes the
// user trashed, and a provenance row whose thread ID is the conversation.
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

// sniffMIME reads a file's first bytes to name its type, for an asset whose
// name the archive stripped, and returns a reader with those bytes still in it.
func sniffMIME(file io.Reader) (string, io.Reader, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", nil, err
	}
	head = head[:n]
	mime := strings.TrimSpace(strings.SplitN(http.DetectContentType(head), ";", 2)[0])
	return mime, io.MultiReader(bytes.NewReader(head), file), nil
}
