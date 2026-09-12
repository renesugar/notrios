package purge

import (
	"archive/tar"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A library, and the roots a purge would act on.
func fixture(t *testing.T) (map[string]string, string) {
	t.Helper()
	home := t.TempDir()
	roots := map[string]string{
		"config":         filepath.Join(home, ".config", "notrios"),
		"data":           filepath.Join(home, ".local", "share", "notrios"),
		"state":          filepath.Join(home, ".local", "state", "notrios"),
		"cache":          filepath.Join(home, ".cache", "notrios"),
		"runtime":        filepath.Join(home, ".local", "state", "notrios", "runtime"),
		"program_assets": filepath.Join(home, ".local", "share", "notrios"),
	}
	for _, path := range roots {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(roots["data"], "notes.sqlite"), "a library")
	write(filepath.Join(roots["cache"], "index"), "rebuildable")
	return roots, home
}

func TestBackupHoldsTheDataAndVerifies(t *testing.T) {
	roots, home := fixture(t)
	steps := Plan(roots, Environment{Home: home})
	destination := filepath.Join(t.TempDir(), "backup")

	if _, err := CreateBackup(steps, destination); err != nil {
		t.Fatalf("backup: %v", err)
	}
	ok, detail := VerifyBackup(destination)
	if !ok {
		t.Fatalf("a freshly written backup did not verify: %s", detail)
	}
	if !strings.Contains(detail, "verified") {
		t.Errorf("verification said %q", detail)
	}
	// The library has to actually be in there. A backup that verifies its own
	// emptiness is the failure this is guarding against.
	if !archiveHas(t, destination, "data/notes.sqlite") {
		t.Error("the backup does not contain the library")
	}
}

func TestBackupExcludesSyncKeyMaterialAndSaysSo(t *testing.T) {
	roots, home := fixture(t)
	if err := os.WriteFile(filepath.Join(roots["state"], "sync-keys.json"),
		[]byte(`{"k":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roots["state"], "sync-keys-sealed.json"),
		[]byte(`{"sealed":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	steps := Plan(roots, Environment{Home: home})
	destination := filepath.Join(t.TempDir(), "backup")
	manifest, err := CreateBackup(steps, destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.ExcludedKeyMaterial) != 2 {
		t.Fatalf("excluded %v, want both key files", manifest.ExcludedKeyMaterial)
	}
	if archiveHas(t, destination, "state/sync-keys.json") ||
		archiveHas(t, destination, "state/sync-keys-sealed.json") {
		t.Fatal("sync key material reached the backup")
	}
	if ok, detail := VerifyBackup(destination); !ok {
		t.Fatalf("verification failed: %s", detail)
	}
}

// The exclusion is checked against the archive rather than trusted, so plant
// key material in an archive and confirm verification refuses it.
func TestVerificationRefusesAnArchiveHoldingKeyMaterial(t *testing.T) {
	roots, home := fixture(t)
	steps := Plan(roots, Environment{Home: home})
	destination := filepath.Join(t.TempDir(), "backup")
	if _, err := CreateBackup(steps, destination); err != nil {
		t.Fatal(err)
	}
	// Rebuild the archive with every recorded member still present *plus* a
	// planted key file, and re-record its hash. Both of the earlier checks --
	// the archive hash, and every recorded entry being a member -- therefore
	// pass, so the only thing left to catch this is the exclusion check itself.
	// A first version of this test simply replaced the archive, and the
	// "recorded but not in the archive" rule caught it first, which proved a
	// different rule than the one under test.
	manifest := readManifest(t, destination)
	archivePath := filepath.Join(destination, "backup.tar")
	handle, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(handle)
	for _, entry := range manifest.Entries {
		content, err := os.ReadFile(entry.Source)
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.WriteHeader(&tar.Header{Name: entry.Member, Mode: 0o600,
			Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	body := []byte(`{"k":1}`)
	if err := writer.WriteHeader(&tar.Header{Name: "state/sync-keys.json",
		Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(body); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	handle.Close()
	rehash(t, destination)

	ok, detail := VerifyBackup(destination)
	if ok {
		t.Fatal("verification accepted an archive containing sync key material")
	}
	if !strings.Contains(detail, "sync key material") {
		t.Errorf("refused for the wrong reason: %s", detail)
	}
}

func TestVerificationRefusesATamperedArchive(t *testing.T) {
	roots, home := fixture(t)
	steps := Plan(roots, Environment{Home: home})
	destination := filepath.Join(t.TempDir(), "backup")
	if _, err := CreateBackup(steps, destination); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(destination, "backup.tar")
	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 1
	if err := os.WriteFile(archivePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, detail := VerifyBackup(destination); ok {
		t.Fatal("verification accepted a tampered archive")
	} else if !strings.Contains(detail, "hash") {
		t.Errorf("refused for the wrong reason: %s", detail)
	}
}

func TestVerificationRefusesAnIncompleteBackup(t *testing.T) {
	destination := t.TempDir()
	if ok, detail := VerifyBackup(destination); ok || !strings.Contains(detail, "incomplete") {
		t.Fatalf("an empty destination verified: ok=%v %s", ok, detail)
	}
}

// The backup must not land where the same run would delete it.
func TestBackupDestinationIsOutsideEveryRootThePurgeRemoves(t *testing.T) {
	roots, home := fixture(t)
	destination, err := BackupDestination(roots, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	steps := Plan(roots, Environment{Home: home})
	for _, step := range steps {
		if step.Action != "backup_then_delete" && step.Action != "dispose" {
			continue
		}
		if destination == step.Path || strings.HasPrefix(destination, step.Path+"/") {
			t.Fatalf("the backup %s is inside %s, which this purge deletes", destination, step.Path)
		}
	}
	// And the oracle itself must refuse it as a target.
	decision := Decide(destination, Environment{Home: home,
		OwnedRoots: []string{roots["data"], roots["state"], roots["cache"], roots["config"]}})
	if decision.Verdict == Allow {
		t.Fatalf("the oracle would allow deleting the backup destination: %s", decision)
	}
}

func TestPlanRefusesASymlinkedRoot(t *testing.T) {
	roots, home := fixture(t)
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(roots["config"]); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, roots["config"]); err != nil {
		t.Fatal(err)
	}
	for _, step := range Plan(roots, Environment{Home: home}) {
		if step.Category == "config" {
			if step.Action != "refuse" {
				t.Fatalf("a symlinked root was planned as %q, not refused", step.Action)
			}
			return
		}
	}
	t.Fatal("no config step was planned")
}

func TestRemoveDeletesOnlyWhatThePlanAllows(t *testing.T) {
	roots, home := fixture(t)
	steps := Plan(roots, Environment{Home: home})
	if _, err := Remove(steps); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(roots["data"], "notes.sqlite")); err == nil {
		t.Error("the library survived the purge")
	}
	// The home directory is not a purge target and must still be there.
	if _, err := os.Lstat(home); err != nil {
		t.Errorf("the home directory was removed: %v", err)
	}
}

func archiveHas(t *testing.T, destination, member string) bool {
	t.Helper()
	handle, err := os.Open(filepath.Join(destination, "backup.tar"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	reader := tar.NewReader(handle)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return false
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == member {
			return true
		}
	}
}

func readManifest(t *testing.T, destination string) BackupManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(destination, "MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func rehash(t *testing.T, destination string) {
	t.Helper()
	digest, err := sha256File(filepath.Join(destination, "backup.tar"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(destination, "MANIFEST.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := replaceHash(string(raw), digest)
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
}

func replaceHash(document, digest string) string {
	start := strings.Index(document, `"archive_sha256": "`)
	if start < 0 {
		return document
	}
	start += len(`"archive_sha256": "`)
	end := strings.Index(document[start:], `"`)
	return document[:start] + digest + document[start+end:]
}
