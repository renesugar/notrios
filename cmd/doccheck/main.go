package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/renesugar/notrios/internal/docaudit"
	"github.com/renesugar/notrios/internal/doccheck"
	"github.com/renesugar/notrios/internal/docjourney"
)

type registryFile struct {
	Schema      string                       `json:"schema"`
	Claims      []docaudit.RegisteredClaim   `json:"claims"`
	Executables []docaudit.RegisteredExample `json:"executables"`
	Journeys    []docaudit.RegisteredJourney `json:"journeys"`
}

func loadStrict(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}

func main() {
	root := flag.String("root", ".", "repository root")
	endpoint := flag.String("endpoint", "http://127.0.0.1:8081", "local llama.cpp endpoint")
	calibration := flag.String("calibration", "performance/v0.7-g18a/CALIBRATION.json", "calibration file")
	triage := flag.String("triage", "", "optional triage file")
	output := flag.String("output", "", "output report path (required)")
	repeats := flag.Int("repeats", 2, "model repeats")
	temperature := flag.Float64("temperature", 0.1, "model temperature")
	calibrationOnly := flag.Bool("calibration-only", false, "run only the labelled calibration set")
	flag.Parse()
	if *output == "" {
		fail(errors.New("-output is required"))
	}
	reg, err := loadRegistry(filepath.Join(*root, "docs/docaudit/registry.json"))
	if err != nil {
		fail(err)
	}
	inv := filepath.Join(*root, "performance/v0.7-g18a/INVENTORY.json")
	examples, err := docaudit.DetectExecutableExamples(*root, inv)
	if err != nil {
		fail(err)
	}
	if err := verifyExamples(reg, examples); err != nil {
		fail(err)
	}
	jf, err := docjourney.LoadAndValidate(*root, "performance/v0.7-g18e/JOURNEYS.json")
	if err != nil {
		fail(err)
	}
	catalog := doccheck.FixtureCatalog{Examples: examples, ExampleStates: map[string]string{}, ExampleSurfaces: map[string]string{}}
	for _, e := range reg.Executables {
		catalog.ExampleStates[e.ID] = string(e.State)
		if e.Execution != nil {
			catalog.ExampleSurfaces[e.ID] = e.Execution.Surface
		}
	}
	for _, j := range jf.Journeys {
		catalog.Journeys = append(catalog.Journeys, doccheck.JourneyFixture{ID: j.ID, Label: j.Label, State: string(j.State)})
	}
	client := doccheck.Client{Endpoint: *endpoint, Timeout: 2 * time.Minute}
	if err := client.Health(context.Background()); err != nil {
		fail(err)
	}
	report, err := doccheck.Run(context.Background(), client, doccheck.ReviewOptions{Root: *root, Repeats: *repeats, Temperature: *temperature, CalibrationPath: filepath.Join(*root, *calibration), TriagePath: joinOptional(*root, *triage), Fixtures: catalog, CalibrationOnly: *calibrationOnly})
	if err != nil {
		fail(err)
	}
	if err := writeReportAtomic(*output, report); err != nil {
		fail(err)
	}
}

func writeReportAtomic(path string, report doccheck.AdvisoryReport) (resultErr error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".doccheck-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if resultErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
func joinOptional(root, p string) string {
	if p == "" {
		return ""
	}
	return filepath.Join(root, p)
}
func loadRegistry(path string) (registryFile, error) {
	var r registryFile
	err := loadStrict(path, &r)
	if err == nil && r.Schema != "notrios.docaudit.registry.v2" {
		err = errors.New("unsupported registry schema")
	}
	return r, err
}
func verifyExamples(r registryFile, got []docaudit.ExampleCandidate) error {
	if len(r.Executables) != len(got) {
		return fmt.Errorf("registry/candidate count mismatch")
	}
	by := map[string]docaudit.ExampleCandidate{}
	for _, e := range got {
		by[e.ID] = e
	}
	for _, e := range r.Executables {
		c, ok := by[e.ID]
		if !ok || c.Path != e.Path || c.Section != e.Section || c.Language != e.Language || c.SHA256 != e.SHA256 {
			return fmt.Errorf("registry/candidate mismatch %s", e.ID)
		}
	}
	return nil
}
func fail(err error) { fmt.Fprintln(os.Stderr, "doccheck:", err); os.Exit(1) }
