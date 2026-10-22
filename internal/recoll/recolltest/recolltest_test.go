package recolltest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryLiveRecollTestIsolates is the guard on the rule this package exists
// for. A test file that runs a real Recoll binary — it names recollindex and
// either looks the binary up or asks a sidecar whether it is available — must
// call Isolate, or it can read the owner's own data through an external indexer
// the way TestLiveSidecarStartupRepairsDamageAndReportsStatus once did.
func TestEveryLiveRecollTestIsolates(t *testing.T) {
	root := repositoryRoot(t)
	var live, missing []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "dist", "_site", "data":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") || strings.HasPrefix(path, filepath.Join(root, "internal", "recoll", "recolltest")) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		runsRecoll := strings.Contains(text, `"recollindex"`) &&
			(strings.Contains(text, ".Available()") || strings.Contains(text, `exec.LookPath("recollindex")`))
		if !runsRecoll {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		live = append(live, relative)
		if !strings.Contains(text, "recolltest.Isolate(") {
			missing = append(missing, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(live) == 0 {
		t.Fatal("found no test that runs Recoll; the detection has drifted from how the tests are written")
	}
	if len(missing) > 0 {
		t.Fatalf("these tests run a real Recoll binary without recolltest.Isolate, so an external indexer can reach the owner's data: %v", missing)
	}
}

func TestIsolateMovesHomeAndEveryXDGDirectory(t *testing.T) {
	realHome := os.Getenv("HOME")
	root := Isolate(t)
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
		"XDG_CACHE_HOME", "XDG_RUNTIME_DIR", "XDG_CONFIG_DIRS", "XDG_DATA_DIRS"} {
		value := os.Getenv(name)
		if !strings.HasPrefix(value, root) {
			t.Errorf("%s = %q, want a path under %s", name, value, root)
		}
	}
	if os.Getenv("HOME") == realHome && realHome != "" {
		t.Error("HOME was not moved")
	}
	if info, err := os.Stat(os.Getenv("XDG_RUNTIME_DIR")); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the runtime directory must exist and be private: %v %v", info, err)
	}
}

func TestAssertNoExternalIndexersReadsTheFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "backends"), []byte("# none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	AssertNoExternalIndexers(t, dir) // must not fail
	if !failsWith(func(tb testing.TB) { AssertNoExternalIndexers(tb, t.TempDir()) }) {
		t.Error("a missing backends file must fail")
	}
	declared := t.TempDir()
	if err := os.WriteFile(filepath.Join(declared, "backends"), []byte("[JOPLIN]\nindex = rcljoplin.py index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !failsWith(func(tb testing.TB) { AssertNoExternalIndexers(tb, declared) }) {
		t.Error("a declared external indexer must fail")
	}
}

// failsWith reports whether check called Fatalf, without failing this test.
func failsWith(check func(testing.TB)) bool {
	recorder := &fatalRecorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		check(recorder)
	}()
	<-done
	return recorder.failed
}

type fatalRecorder struct {
	testing.TB
	failed bool
}

func (r *fatalRecorder) Helper() {}
func (r *fatalRecorder) Fatalf(string, ...any) {
	r.failed = true
	panic("fatal")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test")
		}
		dir = parent
	}
}
