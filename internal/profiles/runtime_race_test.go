package profiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// What changed when startup stopped opening every registered database, and what
// did not.
//
// v0.8 H13 found the consequence rather than the cause: two daemons launched
// together and one aborted with `stale_database: sqlite exec: database is
// locked` -- a transient lock on its *own* library, contended because the other
// daemon was validating every registered profile including this one.
//
// Racing two daemons does not assert the fix. With two small fresh libraries
// the window is a few milliseconds, and the unfixed code passed five
// consecutive runs; a test that only sometimes fails on a defect is not
// evidence. Nor does making the other database unopenable: every such error is
// already suppressed for a profile the call was not asked about, so the fixed
// and unfixed code agree.
//
// One behaviour does differ, deterministically, and it is the trade-off the
// narrowing makes: an unselected profile's identity now comes from the registry
// rather than from its database. So a database swapped underneath a stale
// registry entry is invisible to a startup that is not starting it, and visible
// to an audit. Pinning that here is the honest assertion -- it says what the
// change gave up, in a form that fails if the change is reverted *or* if the
// audit half is ever narrowed to match.
func TestStartupTrustsTheRegistryForProfilesItIsNotStarting(t *testing.T) {
	isolateRoots(t)
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")

	starting := createRuntimeProfile(t, registry, "starting",
		filepath.Join(root, "starting.sqlite"), filepath.Join(root, "starting-assets"),
		"127.0.0.1:18191", nil)
	bystander := createRuntimeProfile(t, registry, "bystander",
		filepath.Join(root, "bystander.sqlite"), filepath.Join(root, "bystander-assets"),
		"127.0.0.1:18192", nil)

	// Put the starting profile's library where the bystander's belongs. Both
	// databases now carry one replica identity, and the registry still records
	// the two distinct ones it was given.
	library, err := os.ReadFile(starting.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bystander.DatabasePath, library, 0o600); err != nil {
		t.Fatal(err)
	}

	// Starting one profile must not depend on, or read, the other's library.
	report := Validate(context.Background(), registry, "starting")
	if !report.Valid {
		t.Fatalf("starting a profile was refused because of another profile's database: %+v",
			report.Issues)
	}

	// The audit opens everything, and must still find it.
	audit := Validate(context.Background(), registry, "")
	if audit.Valid {
		t.Fatal("a full audit accepted two profiles sharing one replica identity")
	}
	found := false
	for _, issue := range audit.Issues {
		if issue.Code == "duplicate_replica_id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the audit did not report the duplicated replica identity: %+v", audit.Issues)
	}
}

// The audit must keep opening databases, or "startup validates what it starts"
// could be satisfied by never opening anything and the narrowing would have
// removed a check rather than scoped it.
func TestAuditStillOpensEveryDatabase(t *testing.T) {
	isolateRoots(t)
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")

	createRuntimeProfile(t, registry, "one",
		filepath.Join(root, "one.sqlite"), filepath.Join(root, "one-assets"),
		"127.0.0.1:18193", nil)
	other := createRuntimeProfile(t, registry, "two",
		filepath.Join(root, "two.sqlite"), filepath.Join(root, "two-assets"),
		"127.0.0.1:18194", nil)

	if err := os.Remove(other.DatabasePath); err != nil {
		t.Fatal(err)
	}
	audit := Validate(context.Background(), registry, "")
	if audit.Valid {
		t.Fatal("a full audit accepted a registry whose second database is gone")
	}
	found := false
	for _, issue := range audit.Issues {
		if issue.Profile == "two" && issue.Code == "stale_database" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the audit did not report the missing database: %+v", audit.Issues)
	}
}
