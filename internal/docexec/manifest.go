package docexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/docaudit"
)

// Manifest is the immutable, bidirectional view of the documentation registry
// and the literal fences detected in the repository.
type Manifest struct {
	Root    string
	Entries []ManifestEntry
}

type ManifestEntry struct {
	Candidate  docaudit.ExampleCandidate
	Registered docaudit.RegisteredExample
}

// LoadManifest reads a strict registry and verifies it against every detected
// literal executable fence. No registry entry or fence may be orphaned.
func LoadManifest(root, inventoryPath, registryPath string) (Manifest, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, err
	}
	var registry docaudit.Registry
	if err := readStrict(filepath.Join(abs, registryPath), &registry); err != nil {
		return Manifest{}, fmt.Errorf("registry: %w", err)
	}
	if registry.Schema != docaudit.RegistrySchema {
		return Manifest{}, fmt.Errorf("registry schema = %q, want %q", registry.Schema, docaudit.RegistrySchema)
	}
	candidates, err := docaudit.DetectExecutableExamples(abs, inventoryPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("detect examples: %w", err)
	}
	registered := make(map[string]docaudit.RegisteredExample, len(registry.Executables))
	for _, e := range registry.Executables {
		if e.ID == "" || registered[e.ID].ID != "" {
			return Manifest{}, fmt.Errorf("duplicate or empty executable id %q", e.ID)
		}
		if e.Path == "" || e.Section == "" || e.Language == "" || e.SHA256 == "" {
			return Manifest{}, fmt.Errorf("executable %q has incomplete identity", e.ID)
		}
		if e.State != docaudit.GradeExecuted && e.State != docaudit.GradeUnverified {
			return Manifest{}, fmt.Errorf("executable %q has invalid state %q", e.ID, e.State)
		}
		if e.State == docaudit.GradeExecuted {
			if e.Execution == nil || e.Unrun != nil || e.Check == "" {
				return Manifest{}, fmt.Errorf("executed example %q has invalid contract", e.ID)
			}
		} else if e.Execution != nil || e.Unrun == nil || e.Check != "" {
			return Manifest{}, fmt.Errorf("unverified example %q has invalid contract", e.ID)
		}
		registered[e.ID] = e
	}
	entries := make([]ManifestEntry, 0, len(candidates))
	for _, c := range candidates {
		e, ok := registered[c.ID]
		if !ok {
			return Manifest{}, fmt.Errorf("unaccounted executable example %q", c.ID)
		}
		if e.Path != c.Path || e.Section != c.Section || e.Language != c.Language || e.SHA256 != c.SHA256 {
			return Manifest{}, fmt.Errorf("executable %q identity/hash mismatch", c.ID)
		}
		delete(registered, c.ID)
		entries = append(entries, ManifestEntry{Candidate: c, Registered: e})
	}
	if len(registered) != 0 {
		ids := make([]string, 0, len(registered))
		for id := range registered {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return Manifest{}, fmt.Errorf("orphan registered executable %q", ids[0])
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Candidate.ID < entries[j].Candidate.ID })
	return Manifest{Root: abs, Entries: entries}, nil
}

// Invocation is the only input supplied to an adapter. Adapters are closed
// code-owned cases; the runner never interprets the fence as a shell command.
type Invocation struct {
	Entry ManifestEntry
	Body  string
}

type AdapterResult struct {
	Kind            string
	Status          int
	PostconditionOK bool
	Detail          string
}
type Adapter func(context.Context, Invocation) (AdapterResult, error)

type RunOptions struct {
	Adapters           map[string]Adapter
	SubstitutionValues map[string]string // keyed by declared Source
	// ResolveSubstitution supplies per-entry fixture values. It takes
	// precedence over SubstitutionValues so every example can own fresh state.
	ResolveSubstitution func(ManifestEntry, string) (string, bool)
}

const RunReportSchema = "notrios.docexec.report.v1"

type RunReport struct {
	Schema        string         `json:"schema"`
	Entries       int            `json:"entries"`
	Executed      int            `json:"executed"`
	Unverified    int            `json:"unverified"`
	UnrunReasons  map[string]int `json:"unrun_reasons"`
	RuntimeMillis int64          `json:"runtime_ms"`
	Topics        []TopicRun     `json:"topics"`
}
type TopicRun struct {
	Topic         string         `json:"topic"`
	Counts        map[string]int `json:"counts"`
	RuntimeMillis int64          `json:"runtime_ms"`
}

// Run dispatches executed entries through exactly the supplied adapter cases.
// Unverified entries remain visible in the report and are never dispatched.
func (m Manifest) Run(ctx context.Context, options RunOptions) (RunReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	used := make(map[string]bool)
	byTopic := make(map[string]*TopicRun)
	for _, entry := range m.Entries {
		topic := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(entry.Candidate.Path), "docs/"), ".md")
		topic = strings.ReplaceAll(topic, "/", "-")
		tr := byTopic[topic]
		if tr == nil {
			tr = &TopicRun{Topic: topic, Counts: map[string]int{}}
			byTopic[topic] = tr
		}
		tr.Counts[string(entry.Registered.State)]++
		if entry.Registered.State != docaudit.GradeExecuted {
			continue
		}
		x := entry.Registered.Execution
		if x == nil {
			return RunReport{}, fmt.Errorf("%s: missing execution", entry.Candidate.ID)
		}
		adapter, ok := options.Adapters[x.Case]
		if !ok {
			return RunReport{}, fmt.Errorf("%s: missing adapter case %q", entry.Candidate.ID, x.Case)
		}
		used[x.Case] = true
		resolve := func(source string) (string, bool) {
			if options.ResolveSubstitution != nil {
				if value, ok := options.ResolveSubstitution(entry, source); ok {
					return value, true
				}
			}
			value, ok := options.SubstitutionValues[source]
			return value, ok
		}
		body, err := substitute(entry.Candidate.Body, x.Substitutions, resolve)
		if err != nil {
			return RunReport{}, fmt.Errorf("%s: %w", entry.Candidate.ID, err)
		}
		caseStart := time.Now()
		result, err := adapter(ctx, Invocation{Entry: entry, Body: body})
		tr.RuntimeMillis += time.Since(caseStart).Round(time.Millisecond).Milliseconds()
		if err != nil {
			return RunReport{}, fmt.Errorf("%s: adapter: %w", entry.Candidate.ID, err)
		}
		if result.Kind != x.Expected.Kind || result.Status != x.Expected.Status {
			return RunReport{}, fmt.Errorf("%s: result kind/status %s/%d, expected %s/%d", entry.Candidate.ID, result.Kind, result.Status, x.Expected.Kind, x.Expected.Status)
		}
		if !result.PostconditionOK {
			return RunReport{}, fmt.Errorf("%s: semantic postcondition failed: %s", entry.Candidate.ID, result.Detail)
		}
	}
	for key := range options.Adapters {
		if !used[key] {
			return RunReport{}, fmt.Errorf("unused adapter case %q", key)
		}
	}
	result := RunReport{Schema: RunReportSchema, Entries: len(m.Entries), UnrunReasons: make(map[string]int), RuntimeMillis: time.Since(start).Round(time.Millisecond).Milliseconds()}
	for _, e := range m.Entries {
		if e.Registered.State == docaudit.GradeExecuted {
			result.Executed++
		} else {
			result.Unverified++
			result.UnrunReasons[e.Registered.Unrun.Code]++
		}
	}
	keys := make([]string, 0, len(byTopic))
	for k := range byTopic {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result.Topics = append(result.Topics, *byTopic[k])
	}
	return result, nil
}

func substitute(body string, declarations []docaudit.ExampleSubstitution, resolve func(string) (string, bool)) (string, error) {
	for _, sub := range declarations {
		if !strings.Contains(body, sub.Token) {
			return "", fmt.Errorf("declared substitution token %q is absent", sub.Token)
		}
		value, ok := resolve(sub.Source)
		if !ok {
			return "", fmt.Errorf("unresolved substitution %q (source %q)", sub.Token, sub.Source)
		}
		body = strings.ReplaceAll(body, sub.Token, value)
	}
	return body, nil
}

func readStrict(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON values")
		}
		return err
	}
	return nil
}

// HashFence is provided for adapters/tests that need to independently verify
// the identity digest without reimplementing the registry's algorithm.
func HashFence(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
