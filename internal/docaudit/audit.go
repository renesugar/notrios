package docaudit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const RegistrySchema = "notrios.docaudit.registry.v1"

func Audit(options Options) (Report, error) {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return Report{}, err
	}
	var inventory Inventory
	if err := readJSON(filepath.Join(root, options.InventoryPath), &inventory, false); err != nil {
		return Report{}, fmt.Errorf("inventory: %w", err)
	}
	var registry Registry
	if err := readJSON(filepath.Join(root, options.RegistryPath), &registry, true); err != nil {
		return Report{}, fmt.Errorf("registry: %w", err)
	}
	if registry.Schema != RegistrySchema {
		return Report{}, fmt.Errorf("registry schema = %q, want %q", registry.Schema, RegistrySchema)
	}
	fragments, err := scanGoFragments(root)
	if err != nil {
		return Report{}, err
	}
	templates := make(map[string]map[string]bool)
	for _, document := range inventory.Documents {
		topic := topicForPath(document.Path)
		if templates[topic] != nil {
			return Report{}, fmt.Errorf("duplicate inventory topic %q", topic)
		}
		templates[topic] = make(map[string]bool)
		for _, section := range document.Sections {
			templates[topic][section.ID] = true
		}
	}
	claims := make(map[string]RegisteredClaim)
	for _, claim := range registry.Claims {
		if !idPattern.MatchString(claim.ID) || claims[claim.ID].ID != "" {
			return Report{}, fmt.Errorf("duplicate or invalid registered claim id %q", claim.ID)
		}
		claims[claim.ID] = claim
	}
	fragmentIDs := make(map[string]bool)
	usedClaims := make(map[string]bool)
	var productionAnchors, checkAnchors []string
	for _, fragment := range fragments {
		if fragmentIDs[fragment.ID] {
			return Report{}, fmt.Errorf("duplicate fragment id %q", fragment.ID)
		}
		fragmentIDs[fragment.ID] = true
		productionAnchors = append(productionAnchors, fragment.Anchor)
		if fragment.Topic == "" || !templates[fragment.Topic][fragment.Section] {
			return Report{}, fmt.Errorf("fragment %q names missing topic/section %s/%s", fragment.ID, fragment.Topic, fragment.Section)
		}
		if fragment.ClaimID != "" {
			registered, ok := claims[fragment.ClaimID]
			if !ok {
				return Report{}, fmt.Errorf("dangling claim %q on fragment %q", fragment.ClaimID, fragment.ID)
			}
			if registered.CheckAnchor != fragment.ClaimCheck {
				return Report{}, fmt.Errorf("claim %q check mismatch: directive %q registry %q", fragment.ClaimID, fragment.ClaimCheck, registered.CheckAnchor)
			}
			if usedClaims[fragment.ClaimID] {
				return Report{}, fmt.Errorf("duplicate claim id %q", fragment.ClaimID)
			}
			usedClaims[fragment.ClaimID] = true
			checkAnchors = append(checkAnchors, fragment.ClaimCheck)
		}
		if fragment.Enumeration != "" {
			productionAnchors = append(productionAnchors, fragment.Enumeration)
		}
	}
	for id := range claims {
		if !usedClaims[id] {
			return Report{}, fmt.Errorf("orphan registered check %q", id)
		}
	}

	detected, err := scanExecutableExamples(root, inventory)
	if err != nil {
		return Report{}, err
	}
	registeredExamples := make(map[string]RegisteredExample)
	for _, example := range registry.Executables {
		if registeredExamples[example.ID].ID != "" {
			return Report{}, fmt.Errorf("duplicate registered executable %q", example.ID)
		}
		registeredExamples[example.ID] = example
		if example.State != GradeUnverified && example.State != GradeExecuted {
			return Report{}, fmt.Errorf("executable %q has invalid state %q", example.ID, example.State)
		}
		if example.State == GradeExecuted {
			if example.Check == "" {
				return Report{}, fmt.Errorf("executed example %q has no check", example.ID)
			}
			checkAnchors = append(checkAnchors, example.Check)
		}
		if !templates[topicForPath(example.Path)][example.Section] {
			return Report{}, fmt.Errorf("executable %q names missing topic/section", example.ID)
		}
	}
	for _, candidate := range detected {
		registered, ok := registeredExamples[candidate.ID]
		if !ok || registered.Path != candidate.Path || registered.Section != candidate.Section || registered.Language != candidate.Language || registered.SHA256 != candidate.SHA256 {
			return Report{}, fmt.Errorf("unaccounted executable example %q", candidate.ID)
		}
		delete(registeredExamples, candidate.ID)
	}
	if len(registeredExamples) != 0 {
		return Report{}, fmt.Errorf("orphan registered executable %q", firstKey(registeredExamples))
	}

	inventoryJourneys := make(map[string]InventoryJourney)
	for _, surface := range inventory.Surfaces {
		if surface.ID == "gui_journeys" {
			for _, journey := range surface.Journeys {
				inventoryJourneys[journey.ID] = journey
			}
		}
	}
	registeredJourneys := make(map[string]RegisteredJourney)
	for _, journey := range registry.Journeys {
		if registeredJourneys[journey.ID].ID != "" {
			return Report{}, fmt.Errorf("duplicate registered journey %q", journey.ID)
		}
		registeredJourneys[journey.ID] = journey
		productionAnchors = append(productionAnchors, journey.Owner)
		if !templates[topicForPath(journey.Path)][journey.Section] {
			return Report{}, fmt.Errorf("journey %q names missing topic/section", journey.ID)
		}
		if journey.State != GradeUnverified && journey.State != GradeExecuted {
			return Report{}, fmt.Errorf("journey %q has invalid state %q", journey.ID, journey.State)
		}
		if journey.State == GradeExecuted {
			if journey.Check == "" {
				return Report{}, fmt.Errorf("executed journey %q has no check", journey.ID)
			}
			checkAnchors = append(checkAnchors, journey.Check)
		}
	}
	for id, expected := range inventoryJourneys {
		actual, ok := registeredJourneys[id]
		if !ok || actual.Owner != expected.Owner || actual.ProposedActions != expected.ProposedActions {
			return Report{}, fmt.Errorf("unaccounted executable journey %q", id)
		}
		delete(registeredJourneys, id)
	}
	if len(registeredJourneys) != 0 {
		return Report{}, fmt.Errorf("orphan registered journey %q", firstKey(registeredJourneys))
	}

	if err := resolveAnchors(root, productionAnchors, false, options.TSResolver); err != nil {
		return Report{}, err
	}
	if err := resolveAnchors(root, checkAnchors, true, options.TSResolver); err != nil {
		return Report{}, err
	}
	report := buildReport(inventory, fragments, registry)
	if report.ManualSections != inventory.GradeBaseline.Denominator {
		return Report{}, fmt.Errorf("manual-section denominator %d does not match frozen inventory %d", report.ManualSections, inventory.GradeBaseline.Denominator)
	}
	return report, nil
}

func resolveAnchors(root string, anchors []string, includeTests bool, tsResolver func(string, []string) error) error {
	goAnchors, tsAnchors, err := splitAnchorsByLanguage(anchors)
	if err != nil {
		return err
	}
	for _, anchor := range goAnchors {
		if err := resolveGoAnchor(root, anchor, includeTests); err != nil {
			return err
		}
	}
	if len(tsAnchors) > 0 {
		if tsResolver == nil {
			return fmt.Errorf("TypeScript anchors require the compiler-API resolver")
		}
		if err := tsResolver(root, tsAnchors); err != nil {
			return err
		}
	}
	return nil
}

func readJSON(path string, target any, strict bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON values")
		}
		return err
	}
	return nil
}

func firstKey[T any](values map[string]T) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys[0]
}

func buildReport(inventory Inventory, fragments []Fragment, registry Registry) Report {
	report := Report{Schema: "notrios.docaudit.report.v1", Counts: map[Grade]int{GradeExecuted: 0, GradeGenerated: 0, GradeClaimed: 0, GradeUnverified: 0}, Fragments: len(fragments), Claims: len(registry.Claims), Executables: len(registry.Executables), Journeys: len(registry.Journeys)}
	for _, surface := range inventory.Surfaces {
		report.Surfaces = append(report.Surfaces, SurfaceReport{
			ID: surface.ID, Count: surface.Count, Unit: surface.Unit,
			OperationIDs: surface.OperationIDs, Owner: surface.Owner,
			Finding: surface.Finding, MeasurementState: surface.MeasurementState,
		})
	}
	topicIndex := make(map[string]int)
	for _, document := range inventory.Documents {
		topic := topicForPath(document.Path)
		topicReport := TopicReport{Topic: topic, Path: document.Path, Counts: map[Grade]int{GradeExecuted: 0, GradeGenerated: 0, GradeClaimed: 0, GradeUnverified: 0}}
		for _, section := range document.Sections {
			topicReport.Sections = append(topicReport.Sections, SectionGrade{ID: section.ID, Grade: GradeUnverified})
			topicReport.Counts[GradeUnverified]++
			report.Counts[GradeUnverified]++
			report.Denominator++
			report.ManualSections++
		}
		topicIndex[topic] = len(report.TopicReports)
		report.TopicReports = append(report.TopicReports, topicReport)
	}
	add := func(topic string, unit UnitGrade, kind string) {
		index := topicIndex[topic]
		switch kind {
		case "fragment":
			report.TopicReports[index].Fragments = append(report.TopicReports[index].Fragments, unit)
		case "example":
			report.TopicReports[index].Examples = append(report.TopicReports[index].Examples, unit)
		case "journey":
			report.TopicReports[index].Journeys = append(report.TopicReports[index].Journeys, unit)
		}
		report.TopicReports[index].Counts[unit.Grade]++
		report.Counts[unit.Grade]++
		report.Denominator++
	}
	for _, fragment := range fragments {
		grade := GradeUnverified
		if fragment.ClaimID != "" {
			grade = GradeClaimed
		}
		if fragment.Enumeration != "" {
			grade = GradeGenerated
		}
		add(fragment.Topic, UnitGrade{ID: fragment.ID, Section: fragment.Section, Grade: grade}, "fragment")
	}
	for _, example := range registry.Executables {
		add(topicForPath(example.Path), UnitGrade{ID: example.ID, Section: example.Section, Grade: example.State}, "example")
	}
	for _, journey := range registry.Journeys {
		add(topicForPath(journey.Path), UnitGrade{ID: journey.ID, Section: journey.Section, Grade: journey.State}, "journey")
	}
	return report
}

func DetectExecutableExamples(root, inventoryPath string) ([]ExampleCandidate, error) {
	var inventory Inventory
	if err := readJSON(filepath.Join(root, inventoryPath), &inventory, false); err != nil {
		return nil, err
	}
	return scanExecutableExamples(root, inventory)
}
