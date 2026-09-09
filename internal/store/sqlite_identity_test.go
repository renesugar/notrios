package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseIdentityPersistsAndReplicaRotationIsExplicit(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "identity.sqlite")
	st, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.DatabaseID, "db_") || !strings.HasPrefix(first.ReplicaID, "replica_") {
		t.Fatalf("unexpected identity: %+v", first)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := st.GetDatabaseIdentity(ctx)
	if err != nil || again.DatabaseID != first.DatabaseID || again.ReplicaID != first.ReplicaID {
		t.Fatalf("bootstrap changed identity: first=%+v again=%+v err=%v", first, again, err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.GetDatabaseIdentity(ctx)
	if err != nil || persisted.DatabaseID != first.DatabaseID || persisted.ReplicaID != first.ReplicaID {
		t.Fatalf("reopen changed identity: first=%+v persisted=%+v err=%v", first, persisted, err)
	}
	rotated, err := reopened.RotateReplicaIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.DatabaseID != first.DatabaseID || rotated.ReplicaID == first.ReplicaID || !strings.HasPrefix(rotated.ReplicaID, "replica_") {
		t.Fatalf("bad replica rotation: before=%+v after=%+v", first, rotated)
	}
}

func TestSchemaV11UpgradeCreatesIdentityExactlyOnce(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(ctx, `DROP TABLE database_identity; PRAGMA user_version = 11;`); err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil || identity.DatabaseID == "" || identity.ReplicaID == "" {
		t.Fatalf("identity after v11 upgrade: %+v err=%v", identity, err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	stable, err := st.GetDatabaseIdentity(ctx)
	if err != nil || stable.DatabaseID != identity.DatabaseID || stable.ReplicaID != identity.ReplicaID {
		t.Fatalf("upgrade identity was not idempotent: before=%+v after=%+v err=%v", identity, stable, err)
	}
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("status after upgrade: %+v err=%v", status, err)
	}
}
