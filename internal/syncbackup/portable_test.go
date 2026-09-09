package syncbackup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/synccatchup"
)

func TestPortablePasswordBackupRoundTripAndWrongPassword(t *testing.T) {
	root := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Portable", Body: "encrypted body\n"}); err != nil {
		t.Fatal(err)
	}
	resource, err := st.CreateResource(ctx, store.CreateResourceRequest{
		Filename: "proof.bin", MIMEType: "application/octet-stream", Content: strings.NewReader("resource bytes"),
	})
	if err != nil || resource.SHA256 == "" {
		t.Fatalf("resource: %+v %v", resource, err)
	}

	created, err := syncbackup.CreatePortable(ctx, st, st.AssetRoot(), filepath.Join(root, "created"), "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(created.Path)
	if err != nil || info.Mode().Perm() != 0o600 || created.Header.WrappingMode != "password" {
		t.Fatalf("portable file: info=%v report=%+v err=%v", info, created, err)
	}
	if _, err := syncbackup.InspectPortable(ctx, created.Path, filepath.Join(root, "wrong"), "wrong password"); !errors.Is(err, synccatchup.ErrBadPassword) {
		t.Fatalf("wrong password error = %v", err)
	}
	opened, err := syncbackup.InspectPortable(ctx, created.Path, filepath.Join(root, "opened"), "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !opened.Verify.ReadyForInstall || opened.Verify.DatabaseID != created.Verify.DatabaseID ||
		opened.Verify.CommitSHA256 != created.Verify.CommitSHA256 || opened.Verify.Objects != created.Verify.Objects {
		t.Fatalf("opened report does not match: created=%+v opened=%+v", created.Verify, opened.Verify)
	}
}
