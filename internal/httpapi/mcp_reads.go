package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/renesugar/notrios/internal/store"
)

// The read surfaces v0.6 F3 brings to MCP.
//
// The rule the slice settled: **read-shaped surfaces become tools; write-shaped
// and whole-library ones stay off, with the reason recorded.** These are the
// read-shaped ones. What stays off, and why, is in `docs/api/mcp.md`.
//
// Every one of them is bounded by the same Store ceilings the REST and GUI
// callers hit — a tool cannot ask for more than a person can.

// mcpGetDocumentBlocks lists a note's addressable blocks.
//
// Blocks are how a model cites *part* of a note precisely: a block ID names
// exactly the content it was derived from, so a citation breaks loudly when the
// text changes rather than silently pointing at something else.
func (s *Server) mcpGetDocumentBlocks(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		DocumentID string `json:"document_id,omitempty"`
		URI        string `json:"uri,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	documentID := firstNonEmpty(args.DocumentID, documentIDFromURI(args.URI))
	if documentID == "" {
		return mcpToolResult{}, fmt.Errorf("document_id or document:// URI is required")
	}
	blocks, err := s.store.ListDocumentBlocks(r.Context(), documentID)
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(map[string]any{"document_id": documentID, "blocks": toAPIBlocks(blocks)})
}

// mcpGetGraph walks links outward from one or more notes.
func (s *Server) mcpGetGraph(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		DocumentIDs []string `json:"document_ids,omitempty"`
		DocumentID  string   `json:"document_id,omitempty"`
		Depth       int      `json:"depth,omitempty"`
		Direction   string   `json:"direction,omitempty"`
		Limit       int      `json:"limit,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	roots := args.DocumentIDs
	if len(roots) == 0 && strings.TrimSpace(args.DocumentID) != "" {
		roots = []string{args.DocumentID}
	}
	result, err := s.store.Graph(r.Context(), store.GraphRequest{
		Roots:     roots,
		Depth:     args.Depth,
		Direction: args.Direction,
		MaxNodes:  args.Limit,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(result)
}

// mcpFindGraphPath finds a shortest route between two notes.
func (s *Server) mcpFindGraphPath(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		FromDocumentID string `json:"from_document_id,omitempty"`
		ToDocumentID   string `json:"to_document_id,omitempty"`
		MaxDepth       int    `json:"max_depth,omitempty"`
		Direction      string `json:"direction,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	result, err := s.store.GraphPath(r.Context(), store.GraphPathRequest{
		From:      args.FromDocumentID,
		To:        args.ToDocumentID,
		MaxDepth:  args.MaxDepth,
		Direction: args.Direction,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(result)
}

// mcpGetGraphReport reports orphans, isolates, and in-degree hubs.
func (s *Server) mcpGetGraphReport(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		CollectionID string `json:"collection_id,omitempty"`
		Limit        int    `json:"limit,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	result, err := s.store.GraphReport(r.Context(), store.GraphReportRequest{
		CollectionID: args.CollectionID,
		Limit:        args.Limit,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(result)
}

// mcpRunNoteQuery evaluates one embedded query block.
//
// The block reaches the same Q1 parser every search surface uses, so this adds
// no expressive power over `search_documents` — it is here so a model reading a
// note containing a block can resolve what the block shows.
func (s *Server) mcpRunNoteQuery(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		Block        string `json:"block,omitempty"`
		CollectionID string `json:"collection_id,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	result, err := s.store.RunNoteQuery(r.Context(), store.NoteQueryRequest{
		Block:        args.Block,
		CollectionID: args.CollectionID,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(result)
}

// mcpGetLintReport returns the read-only workspace lint report.
//
// Lint is read-shaped and content-free by construction: findings carry a
// document ID, a line and column, a reason code, and a SHA-256 of the offending
// target — never the target text, because a broken wikilink's raw text is
// frequently the title of a private note. `notriosctl fix` stays off MCP; a
// model may see what is broken without being able to rewrite notes in bulk.
func (s *Server) mcpGetLintReport(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		CollectionID string   `json:"collection_id,omitempty"`
		Checks       []string `json:"checks,omitempty"`
		DetailLimit  int      `json:"detail_limit,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	detailLimit := args.DetailLimit
	if detailLimit <= 0 || detailLimit > effectiveMCPMaxResults(s.config.MCP.MaxResults) {
		detailLimit = effectiveMCPMaxResults(s.config.MCP.MaxResults)
	}
	result, err := s.store.LintWorkspace(r.Context(), store.LintRequest{
		CollectionID: args.CollectionID,
		Checks:       args.Checks,
		DetailLimit:  detailLimit,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(result)
}

// mcpReadResource returns a resource's metadata, and optionally a bounded slice
// of its bytes.
//
// The default is **metadata plus a `resource://` URI and nothing else**.
// Streaming arbitrary attachment bytes into a model's context is precisely what
// the bulk-versus-control-plane split exists to prevent, and a PDF or an image
// is not something a model should receive by accident.
//
// Bytes are returned only when asked for, only for text-like MIME types, and
// only within `mcp.max_document_bytes`. `offset` and `length` make it a range
// read — the same shape as `get_note_line_range`, and the reason REST gained
// `Range` in this slice: a caller decides from the metadata how much to pull.
func (s *Server) mcpReadResource(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		ResourceID  string `json:"resource_id,omitempty"`
		URI         string `json:"uri,omitempty"`
		IncludeText bool   `json:"include_text,omitempty"`
		Offset      int64  `json:"offset,omitempty"`
		Length      int64  `json:"length,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	resourceID := firstNonEmpty(args.ResourceID, resourceIDFromURI(args.URI))
	if resourceID == "" {
		return mcpToolResult{}, fmt.Errorf("resource_id or resource:// URI is required")
	}

	res, content, err := s.store.OpenResourceContent(r.Context(), resourceID)
	if err != nil {
		return mcpToolResult{}, err
	}
	defer content.Close()

	out := map[string]any{
		"resource_id": res.ID,
		"uri":         res.URI,
		"filename":    res.Filename,
		"mime_type":   res.MIMEType,
		"size_bytes":  res.SizeBytes,
		"sha256":      res.SHA256,
		"text":        nil,
	}
	if !args.IncludeText {
		out["note"] = "metadata only; pass include_text to read a bounded slice of a text-like resource"
		return mcpStructured(out)
	}
	if !isTextLikeMIME(res.MIMEType) {
		out["note"] = fmt.Sprintf("%q is not a text-like resource; fetch its bytes over REST if you need them", res.MIMEType)
		return mcpStructured(out)
	}

	maxBytes := int64(effectiveMCPMaxDocumentBytes(s.config.MCP.MaxDocumentBytes))
	length := args.Length
	if length <= 0 || length > maxBytes {
		length = maxBytes
	}
	if args.Offset > 0 {
		seeker, seekable := content.(io.Seeker)
		if !seekable {
			return mcpToolResult{}, fmt.Errorf("this resource cannot be read from an offset")
		}
		if _, err := seeker.Seek(args.Offset, io.SeekStart); err != nil {
			return mcpToolResult{}, err
		}
	}
	// One byte over the limit tells truncation from an exact fit, which a
	// caller needs in order to know whether to ask for the next slice.
	buffer := make([]byte, length+1)
	read, err := io.ReadFull(content, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return mcpToolResult{}, err
	}
	truncated := int64(read) > length
	if truncated {
		read = int(length)
	}
	slice := buffer[:read]
	// A slice can land mid-character; dropping the partial rune keeps the text
	// valid UTF-8 rather than handing a model a replacement character it will
	// treat as content.
	for len(slice) > 0 && !utf8.Valid(slice) {
		slice = slice[:len(slice)-1]
	}
	out["text"] = string(slice)
	out["offset"] = args.Offset
	out["returned_bytes"] = len(slice)
	out["truncated"] = truncated
	return mcpStructured(out)
}

// resourceIDFromURI extracts the ID from a resource:// URI, mirroring
// documentIDFromURI. It returns "" for anything that is not one, so a caller
// passing a document URI by mistake gets "resource_id is required" rather than
// a confusing not-found.
func resourceIDFromURI(uri string) string {
	uri = strings.TrimSpace(uri)
	if !strings.HasPrefix(uri, "resource://") {
		return ""
	}
	parts := strings.Split(uri, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "resources" {
			return parts[i+1]
		}
	}
	return ""
}

// isTextLikeMIME reports whether a resource is safe to hand back as text.
//
// An allowlist rather than a "not binary" guess: `text/*` plus the structured
// formats whose bytes really are characters. Anything else — an image, a PDF,
// an archive — is described, never transcribed.
func isTextLikeMIME(mime string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mime))
	if index := strings.IndexByte(normalized, ';'); index >= 0 {
		normalized = strings.TrimSpace(normalized[:index])
	}
	if strings.HasPrefix(normalized, "text/") {
		return true
	}
	switch normalized {
	case "application/json", "application/xml", "application/yaml", "application/x-yaml",
		"application/toml", "application/javascript", "application/sql", "image/svg+xml":
		return true
	}
	return false
}
