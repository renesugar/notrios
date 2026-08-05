package httpapi

import (
	"net/http"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// handleDocumentBlocks lists a note's addressable blocks.
//
// Block IDs are content-derived (`PROJECT_DECISIONS.md` 17), so a client that
// copies one is copying an anchor to exactly this text: it keeps working when
// the block moves and stops working when the block is rewritten. The response
// carries no block text — a caller that wants the content reads the note body,
// which is already an authorized read.
func (s *Server) handleDocumentBlocks(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("document_id")
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "block listing requires the canonical store")
		return
	}
	blocks, err := s.store.ListDocumentBlocks(r.Context(), documentID)
	if writeStoreError(w, err, "blocks_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.DocumentBlocksResponse{
		DocumentID: documentID,
		Blocks:     toAPIBlocks(blocks),
	})
}

func toAPIBlocks(blocks []store.DocumentBlock) []api.DocumentBlock {
	out := make([]api.DocumentBlock, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, api.DocumentBlock{
			ID:            block.ID,
			DocumentID:    block.DocumentID,
			Ordinal:       block.Ordinal,
			Kind:          block.Kind,
			HeadingLevel:  block.HeadingLevel,
			Marker:        block.Marker,
			ContentSHA256: block.ContentSHA256,
			StartByte:     block.StartByte,
			EndByte:       block.EndByte,
			Backlinks:     block.Backlinks,
		})
	}
	return out
}
