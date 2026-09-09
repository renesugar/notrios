package store

import (
	"context"
	"strings"
	"testing"
)

func TestSchemaV9KeysetIndexesAndQueryPlans(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	tests := []struct {
		name  string
		sql   string
		args  []string
		index string
	}{
		{
			name: "all notes",
			sql: `SELECT id FROM documents
				WHERE collection_id = ? AND deleted_at IS NULL
				ORDER BY updated_at DESC, id DESC LIMIT 101`,
			args:  []string{"default"},
			index: "documents_collection_state_updated_idx",
		},
		{
			name: "notebook",
			sql: `SELECT id FROM documents
				WHERE notebook_id = ? AND deleted_at IS NULL
				ORDER BY updated_at DESC, id DESC LIMIT 101`,
			args:  []string{DefaultNotebookID},
			index: "documents_notebook_state_updated_idx",
		},
		{
			name: "trash",
			sql: `SELECT id FROM documents
				WHERE deleted_at IS NOT NULL
				ORDER BY deleted_at DESC, id DESC LIMIT 101`,
			index: "documents_trash_deleted_idx",
		},
		{
			name: "tag membership",
			sql: `SELECT document_id FROM note_tags
				WHERE tag_id = ? ORDER BY document_id`,
			args:  []string{"tag_missing"},
			index: "note_tags_tag_document_idx",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := st.explainQueryPlan(ctx, test.sql, test.args...)
			if err != nil {
				t.Fatalf("explain: %v", err)
			}
			if !strings.Contains(strings.Join(plan, "\n"), test.index) {
				t.Fatalf("query plan does not use %s: %v", test.index, plan)
			}
		})
	}
}
