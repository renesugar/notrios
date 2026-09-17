package installreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J11: the report is the record of what an installation occupies, and the
// oracle for whether a purge removed it.

func write(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// installation builds a small tree shaped like a real one and returns its roots.
func installation(t *testing.T) (string, map[string]string) {
	t.Helper()
	home := t.TempDir()
	roots := map[string]string{
		"config":  filepath.Join(home, "config/notrios"),
		"data":    filepath.Join(home, "data/notrios"),
		"state":   filepath.Join(home, "state/notrios"),
		"cache":   filepath.Join(home, "cache/notrios"),
		"runtime": filepath.Join(home, "runtime/notrios"),
	}
	write(t, filepath.Join(roots["config"], "config.yaml"), "server:\n  listen_addr: 127.0.0.1:8080\n")
	write(t, filepath.Join(roots["data"], "notes.sqlite"), "sqlite")
	write(t, filepath.Join(roots["data"], "assets/ab/cd/blob"), "bytes")
	write(t, filepath.Join(roots["state"], "quarantine/.keep"), "")
	// The cache root exists as an empty directory; the runtime root does not
	// exist at all, which a fresh installation is entitled to.
	if err := os.MkdirAll(roots["cache"], 0o755); err != nil {
		t.Fatal(err)
	}
	return home, roots
}

func paths(report Report, kind string) []string {
	out := []string{}
	source := report.Manifest
	if kind == KindDirectory {
		source = report.Structure
	}
	for _, entry := range source {
		out = append(out, entry.Path)
	}
	return out
}

func TestJ11AReportsEveryDirectoryAndFile(t *testing.T) {
	_, roots := installation(t)
	report, err := Build(Options{Roots: roots, Mode: "installed"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != Schema || report.Mode != "installed" {
		t.Fatalf("schema=%s mode=%s", report.Schema, report.Mode)
	}
	for _, want := range []string{
		filepath.Join(roots["config"], "config.yaml"),
		filepath.Join(roots["data"], "notes.sqlite"),
		filepath.Join(roots["data"], "assets/ab/cd/blob"),
		filepath.Join(roots["state"], "quarantine/.keep"),
	} {
		if !contains(paths(report, KindFile), want) {
			t.Errorf("the manifest does not list %s", want)
		}
	}
	for _, want := range []string{
		roots["config"], roots["data"], filepath.Join(roots["data"], "assets/ab/cd"), roots["cache"],
	} {
		if !contains(paths(report, KindDirectory), want) {
			t.Errorf("the structure does not list %s", want)
		}
	}
	if report.Counts.Files != 4 || report.Counts.Directories != 8 {
		t.Errorf("counts = %+v; want 4 files and 8 directories", report.Counts)
	}
	if report.Counts.Bytes == 0 {
		t.Error("the report should total the bytes it found")
	}
	// A root that does not exist is said plainly rather than omitted.
	if !strings.Contains(strings.Join(report.Notes, "\n"), "runtime root") {
		t.Errorf("notes = %v; want the missing runtime root named", report.Notes)
	}
	// Sorted, so two runs of one installation compare.
	if !sorted(paths(report, KindFile)) || !sorted(paths(report, KindDirectory)) {
		t.Error("entries must be sorted by path")
	}
}

func TestJ11AProfilesOutsideTheRootsAreListedAndNotOwned(t *testing.T) {
	home, roots := installation(t)
	outside := filepath.Join(home, "elsewhere/library")
	write(t, filepath.Join(outside, "notes.sqlite"), "sqlite")
	registry := write(t, filepath.Join(home, "profiles.json"), `{"version":2,"profiles":[
      {"name":"inside","database_id":"db_inside","database_path":"`+filepath.Join(roots["data"], "notes.sqlite")+`",
       "registered_at":"2026-09-18T00:00:00Z"},
      {"name":"on the NAS","database_id":"db_nas","database_path":"`+filepath.Join(outside, "notes.sqlite")+`",
       "asset_store":"`+filepath.Join(outside, "assets")+`","registered_at":"2026-09-18T00:00:00Z"}
    ]}`)

	report, err := Build(Options{Roots: roots, RegistryPath: registry})
	if err != nil {
		t.Fatal(err)
	}
	external := map[string]Entry{}
	for _, entry := range report.Manifest {
		if !entry.Owned {
			external[entry.Path] = entry
		}
	}
	if len(external) != 2 || report.Counts.External != 2 || report.Counts.Profiles != 2 {
		t.Fatalf("external=%v counts=%+v", external, report.Counts)
	}
	database := external[filepath.Join(outside, "notes.sqlite")]
	if database.Profile != "on the NAS" || database.Field != "database" || database.Kind != KindFile {
		t.Errorf("the external database entry is %+v", database)
	}
	// An external path that does not exist is still listed: the check needs to
	// know it was expected, and its absence is the interesting case.
	if missing := external[filepath.Join(outside, "assets")]; missing.Kind != KindMissing {
		t.Errorf("a missing external path should be listed as missing, got %+v", missing)
	}
	// The profile inside the roots is not repeated as external; the walk has it.
	for path, entry := range external {
		if strings.HasPrefix(path, roots["data"]) {
			t.Errorf("a profile inside the roots must not be listed as external: %+v", entry)
		}
	}
	if !strings.Contains(strings.Join(report.Notes, "\n"), "a purge keeps them") {
		t.Errorf("notes = %v; want the kept-by-purge note", report.Notes)
	}
}

func TestJ11AProgramFilesComeFromTheInstallManifest(t *testing.T) {
	home, roots := installation(t)
	manifest := write(t, filepath.Join(home, "MANIFEST.json"), `{"schema":"notrios.install-manifest/1",
      "version":"1.0.0","installed_at":"2026-09-18T00:00:00Z","prefix":"/usr/local",
      "entries":[{"path":"/usr/local/bin/notriosctl"},{"path":"/usr/local/bin/notriosd"}]}`)
	report, err := Build(Options{Roots: roots, InstallManifest: manifest})
	if err != nil {
		t.Fatal(err)
	}
	if report.Program == nil || report.Program.Entries != 2 || report.Program.Version != "1.0.0" || report.Program.Prefix != "/usr/local" {
		t.Fatalf("program = %+v", report.Program)
	}

	// A source checkout has no install manifest, and says so rather than
	// implying nothing was installed.
	report, err = Build(Options{Roots: roots, InstallManifest: filepath.Join(home, "absent.json")})
	if err != nil {
		t.Fatal(err)
	}
	if report.Program != nil || !strings.Contains(strings.Join(report.Notes, "\n"), "no install manifest") {
		t.Fatalf("program=%+v notes=%v", report.Program, report.Notes)
	}
}

func TestJ11APathsOutputMarksOwnership(t *testing.T) {
	home, roots := installation(t)
	outside := filepath.Join(home, "elsewhere/library")
	write(t, filepath.Join(outside, "notes.sqlite"), "sqlite")
	registry := write(t, filepath.Join(home, "profiles.json"), `{"version":2,"profiles":[
      {"name":"nas","database_id":"db_nas","database_path":"`+filepath.Join(outside, "notes.sqlite")+`",
       "registered_at":"2026-09-18T00:00:00Z"}]}`)
	report, err := Build(Options{Roots: roots, RegistryPath: registry})
	if err != nil {
		t.Fatal(err)
	}
	lines := report.Paths()
	if len(lines) != len(report.Structure)+len(report.Manifest) {
		t.Fatalf("%d lines for %d entries", len(lines), len(report.Structure)+len(report.Manifest))
	}
	owned, external := 0, 0
	for _, line := range lines {
		state, path, found := strings.Cut(line, "\t")
		if !found || !filepath.IsAbs(path) {
			t.Fatalf("each line is a state and an absolute path, got %q", line)
		}
		switch state {
		case "owned":
			owned++
		case "external":
			external++
		default:
			t.Fatalf("unknown state %q", state)
		}
	}
	if external != 1 || owned == 0 {
		t.Errorf("owned=%d external=%d", owned, external)
	}
}

func TestJ11AReportIsJSONSerialisable(t *testing.T) {
	_, roots := installation(t)
	report, err := Build(Options{Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var round Report
	if err := json.Unmarshal(encoded, &round); err != nil {
		t.Fatal(err)
	}
	if round.Counts != report.Counts || len(round.Manifest) != len(report.Manifest) {
		t.Errorf("a round trip lost content: %+v", round.Counts)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sorted(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			return false
		}
	}
	return true
}

func TestJ11AEveryPathIsAbsolute(t *testing.T) {
	// A source checkout resolves relative roots, and a manifest of relative
	// paths would make the post-purge check test whatever sat beside its caller.
	home := t.TempDir()
	write(t, filepath.Join(home, "data/notes.sqlite"), "sqlite")
	write(t, filepath.Join(home, "elsewhere/notes.sqlite"), "sqlite")
	registry := write(t, filepath.Join(home, "profiles.json"), `{"version":2,"profiles":[
      {"name":"relative","database_id":"db_rel","database_path":"elsewhere/notes.sqlite",
       "registered_at":"2026-09-18T00:00:00Z"}]}`)

	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(home); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(working) })

	report, err := Build(Options{Roots: map[string]string{"data": "data"}, RegistryPath: registry})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range append(append([]Entry{}, report.Structure...), report.Manifest...) {
		if !filepath.IsAbs(entry.Path) {
			t.Errorf("%s is not absolute", entry.Path)
		}
	}
	for name, root := range report.Roots {
		if !filepath.IsAbs(root) {
			t.Errorf("the %s root %s is not absolute", name, root)
		}
	}
	for _, line := range report.Paths() {
		_, path, _ := strings.Cut(line, "\t")
		if !filepath.IsAbs(path) {
			t.Errorf("%q is not an absolute path", line)
		}
	}
}

func TestJ11AOverlappingRootsAreListedOnce(t *testing.T) {
	// An installed layout puts the program's assets under the data root, so two
	// root names resolve to one directory. Listing it twice would inflate every
	// count and make the post-purge check ask about each path twice.
	home := t.TempDir()
	shared := filepath.Join(home, "share/notrios")
	write(t, filepath.Join(shared, "notes.sqlite"), "sqlite")
	write(t, filepath.Join(shared, "assets/blob"), "bytes")

	report, err := Build(Options{Roots: map[string]string{"data": shared, "program_assets": shared}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, entry := range append(append([]Entry{}, report.Structure...), report.Manifest...) {
		seen[entry.Path]++
	}
	for path, count := range seen {
		if count != 1 {
			t.Errorf("%s is listed %d times", path, count)
		}
	}
	if report.Counts.Files != 2 {
		t.Errorf("files = %d, want 2", report.Counts.Files)
	}
	if !strings.Contains(strings.Join(report.Notes, "\n"), "is the data root") {
		t.Errorf("notes = %v; want the overlap named", report.Notes)
	}
}
