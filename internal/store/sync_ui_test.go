package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSyncConflictDetailAndTwoParentResolution(t *testing.T) {
	left, right, _ := makeDivergedDocument(t,
		"the quick brown fox\n", "the SWIFT brown fox\n", "the RAPID brown fox\n")
	exchange(t, left, right)
	page, err := left.ListSyncConflicts(context.Background(), 10)
	if err != nil || len(page.Conflicts) != 1 {
		t.Fatalf("conflicts=%+v err=%v", page, err)
	}
	detail, err := left.GetSyncConflictDetail(context.Background(), page.Conflicts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Base.Body != "the quick brown fox\n" || detail.Local.Body == detail.Remote.Body || detail.RegionCount == 0 {
		t.Fatalf("conflict detail = %+v", detail)
	}
	resolved, err := left.ResolveSyncConflict(context.Background(), ResolveSyncConflictRequest{
		ConflictID: detail.ID, Title: "Shared note", Body: "the AGREED brown fox\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Body != "the AGREED brown fox\n" {
		t.Fatalf("resolved body = %q", resolved.Body)
	}
	if after, err := left.ListSyncConflicts(context.Background(), 10); err != nil || len(after.Conflicts) != 0 {
		t.Fatalf("conflict remained: %+v %v", after, err)
	}
	rows := queryRows(t, left, `SELECT parent_revision_ids FROM document_revisions WHERE id = ?`, resolved.CurrentRevisionID)
	var parents []string
	if len(rows) != 1 || json.Unmarshal([]byte(rows[0]["parent_revision_ids"]), &parents) != nil || len(parents) != 2 {
		t.Fatalf("resolution is not a two-parent revision: %+v", rows)
	}
}

func TestSyncResourceIntentAndRepairReports(t *testing.T) {
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{Filename: "remote.pdf", MIMEType: "application/pdf", Content: strings.NewReader("bytes")})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(ctx, `UPDATE blobs SET availability='unavailable', storage_path='' WHERE sha256='`+resource.SHA256+`';
		INSERT INTO sync_repair_events(id, event_type, subject_id, details_json, hlc_wall_ms, hlc_logical, replica_id, sequence)
		VALUES('repair_ui', 'notebook.cycle', 'nb_a', '{"effective":"nb_recovered"}', 1, 0, 'rep_a', 1);`); err != nil {
		t.Fatal(err)
	}
	items, truncated, err := st.ListSyncResourceStatus(ctx, 10)
	if err != nil || truncated || len(items) != 1 || items[0].ID != resource.ID || items[0].Requested {
		t.Fatalf("resources=%+v truncated=%t err=%v", items, truncated, err)
	}
	if err := st.SetSyncResourceIntent(ctx, resource.ID, true, true); err != nil {
		t.Fatal(err)
	}
	items, _, _ = st.ListSyncResourceStatus(ctx, 10)
	if !items[0].Pinned || !items[0].Requested {
		t.Fatalf("intent not recorded: %+v", items[0])
	}
	repairs, truncated, err := st.ListSyncRepairEvents(ctx, 10)
	if err != nil || truncated || len(repairs) != 1 || repairs[0].Kind != "notebook.cycle" || repairs[0].Details["effective"] != "nb_recovered" {
		t.Fatalf("repairs=%+v truncated=%t err=%v", repairs, truncated, err)
	}
}
