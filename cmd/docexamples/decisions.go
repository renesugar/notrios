package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Every published example is either generated from a tracked set or left as a
// hand-written fence with a recorded reason (v1.0 J15-B).
//
// A document left alone with a reason is a finished decision. A document left
// alone silently is the thing J15 exists to end, so this refuses an example
// that is in neither list -- which is what makes the record a gate rather than
// a description somebody has to remember to update.

const decisionsSchema = "notrios.docexamples.decisions.v1"

type decisions struct {
	Schema    string            `json:"schema"`
	Reasons   map[string]string `json:"reasons"`
	Documents []struct {
		Document string `json:"document"`
		Tracked  int    `json:"tracked"`
		Left     []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"left"`
	} `json:"documents"`
}

// registryIDs is every example the execution registry knows about: the set this
// check has to account for completely.
func registryIDs(root string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "docs/docaudit/registry.json"))
	if err != nil {
		return nil, err
	}
	var registry struct {
		Executables []struct {
			ID string `json:"id"`
		} `json:"executables"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(registry.Executables))
	for _, entry := range registry.Executables {
		ids = append(ids, entry.ID)
	}
	sort.Strings(ids)
	return ids, nil
}

var reasonPattern = regexp.MustCompile(`^[a-z][a-z-]+$`)

// checkDecisions accounts for every published example exactly once.
func checkDecisions(root string, tracked map[string]bool) []error {
	problems := []error{}
	raw, err := os.ReadFile(filepath.Join(root, "docs/docexamples/DECISIONS.json"))
	if err != nil {
		return []error{fmt.Errorf("read the decisions record: %w", err)}
	}
	var record decisions
	if err := json.Unmarshal(raw, &record); err != nil {
		return []error{fmt.Errorf("read the decisions record: %w", err)}
	}
	if record.Schema != decisionsSchema {
		return []error{fmt.Errorf("the decisions record has schema %q, want %q", record.Schema, decisionsSchema)}
	}

	explained := map[string]string{}
	for _, document := range record.Documents {
		for _, left := range document.Left {
			if !reasonPattern.MatchString(left.Reason) {
				problems = append(problems, fmt.Errorf("%s is left for %q, which is not one of the recorded reasons",
					left.ID, left.Reason))
				continue
			}
			if _, ok := record.Reasons[left.Reason]; !ok {
				problems = append(problems, fmt.Errorf("%s is left for %q, which the record does not explain",
					left.ID, left.Reason))
				continue
			}
			explained[left.ID] = left.Reason
		}
	}

	ids, err := registryIDs(root)
	if err != nil {
		return append(problems, fmt.Errorf("read the execution registry: %w", err))
	}
	for _, id := range ids {
		_, isTracked := tracked[id]
		_, isExplained := explained[id]
		switch {
		case isTracked && isExplained:
			problems = append(problems, fmt.Errorf("%s is both generated and recorded as left", id))
		case !isTracked && !isExplained:
			problems = append(problems, fmt.Errorf("%s is neither generated nor recorded as left; "+
				"run python3 performance/v1.0-j15/record_decisions.py", id))
		}
	}
	for id := range explained {
		if !contains(ids, id) {
			problems = append(problems, fmt.Errorf("%s is recorded as left and is not a published example", id))
		}
	}
	return problems
}

func contains(values []string, want string) bool {
	index := sort.SearchStrings(values, want)
	return index < len(values) && values[index] == want
}

// trackedIDs is every example the sets generate.
func trackedIDs(sets []Set) map[string]bool {
	ids := map[string]bool{}
	for _, set := range sets {
		for _, example := range set.Examples {
			ids[example.ID(set.Document)] = true
		}
	}
	return ids
}

// summariseDecisions is what the tool prints, so a reader sees the split.
func summariseDecisions(tracked map[string]bool, root string) string {
	ids, err := registryIDs(root)
	if err != nil {
		return ""
	}
	left := 0
	for _, id := range ids {
		if !tracked[id] {
			left++
		}
	}
	return fmt.Sprintf("%d generated, %d left with a recorded reason",
		len(tracked), left)
}
