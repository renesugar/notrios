package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func TestCatchupExercisesBothCarriersAndReplay(t *testing.T) {
	ctx := context.Background()
	hostRoot := t.TempDir()
	hostDB := filepath.Join(hostRoot, "notes.sqlite")
	hostAssets := filepath.Join(hostRoot, "assets")
	host, err := store.OpenSQLiteWithAssetStore(hostDB, hostAssets)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		if _, err := host.CreateDocument(ctx, store.CreateDocumentRequest{
			Title: "Generated acceptance fixture", Body: "generated fixture body\n",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := host.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := catchup(hostDB, hostAssets, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !got.RESTVerified || !got.DirectoryVerified || got.RestoreStage != "complete" ||
		!got.ReplicaRotated || !got.PostSnapshotConverged {
		t.Fatalf("incomplete acceptance: %+v", got)
	}
	if got.RESTDownloadCalls < 1 || got.DirectoryPublishCalls < 1 || got.DirectoryDownloadCalls < 1 {
		t.Fatalf("carrier was not exercised: %+v", got)
	}
}
