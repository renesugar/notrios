// Package docplan holds the plan's slice ledger: what each item is made of,
// which parts of it are done, and what a done part is evidenced by.
//
// PLAN.md is a journal. It records what happened, in the order it happened,
// and it does that well -- but the question somebody actually asks of a plan is
// "what is left?", and answering it meant reading four thousand lines written
// in nine different status notations and then checking the answer against the
// code, because the plan could not be trusted to know. Two thirds of the text
// under items that are still open describes work already finished.
//
// So the remaining work is declared here instead, as slices, and the progress
// log in PLAN.md is generated from it. The gates matter more than the format:
// this repository keeps finding hand-maintained prose that went stale while
// every check was green -- a page that said tagging was unreachable months
// after it was built, a count of fifteen gaps when there were five, a pinned
// fifty-eight controls when the interface had fifty-nine. A ledger nobody
// checks would rot the same way, so a finished slice must name evidence that
// exists, and an item's state must agree with what its slices say.
package docplan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/docaudit"
)

const Schema = "notrios.docplan.slices.v1"

// SliceState is where one slice of an item stands.
type SliceState string

const (
	// SliceDone is finished and evidenced. Nothing else counts as done.
	SliceDone SliceState = "done"
	// SliceInProgress has been started and is not finished. It is the state
	// that most needs saying out loud, because it is invisible otherwise.
	SliceInProgress SliceState = "in-progress"
	SliceNotStarted SliceState = "not-started"
	// SliceBlocked cannot proceed for a reason outside the work itself --
	// missing hardware, an authorisation, a decision somebody else owns. It
	// must say what on.
	SliceBlocked SliceState = "blocked"
)

// ItemState is where a whole plan item stands.
type ItemState string

const (
	ItemComplete   ItemState = "complete"
	ItemInProgress ItemState = "in-progress"
	ItemNotStarted ItemState = "not-started"
	ItemDeferred   ItemState = "deferred"
)

// Slice is one implementable piece of an item: small enough to finish in a
// sitting, large enough to be worth naming, and stated as an outcome rather
// than as an activity.
type Slice struct {
	ID        string     `json:"id"`
	Statement string     `json:"statement"`
	State     SliceState `json:"state"`
	// Evidence is required when the slice is done and refused otherwise. A
	// `go:` anchor resolves through the same resolver the documentation gates
	// use; `file:` names a path that must exist; `test:` names a Go test
	// function that must be defined somewhere in the tree.
	Evidence string `json:"evidence,omitempty"`
	// BlockedOn is required when the slice is blocked, and says what would
	// unblock it. "Blocked" without that is just "not started" with a better
	// excuse.
	BlockedOn string `json:"blocked_on,omitempty"`
}

// Item mirrors one `## H…` heading in PLAN.md.
type Item struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	State    ItemState `json:"state"`
	Archived string    `json:"archived,omitempty"`
	Slices   []Slice   `json:"slices"`
}

type Ledger struct {
	Schema string `json:"schema"`
	Items  []Item `json:"items"`
}

func Load(path string) (Ledger, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Ledger{}, err
	}
	var ledger Ledger
	if err := json.Unmarshal(contents, &ledger); err != nil {
		return Ledger{}, err
	}
	if ledger.Schema != Schema {
		return Ledger{}, fmt.Errorf("unexpected plan ledger schema %q", ledger.Schema)
	}
	return ledger, nil
}

var planHeading = regexp.MustCompile(`(?m)^## (H[0-9a-z]+)\. (.+?)\s*$`)

// PlanHeadings reads the item headings out of PLAN.md, with the completion
// marker each one carries in its title.
func PlanHeadings(planPath string) (map[string]string, error) {
	contents, err := os.ReadFile(planPath)
	if err != nil {
		return nil, err
	}
	found := map[string]string{}
	for _, match := range planHeading.FindAllStringSubmatch(string(contents), -1) {
		if _, seen := found[match[1]]; seen {
			return nil, fmt.Errorf("duplicate plan item heading %s", match[1])
		}
		found[match[1]] = match[2]
	}
	return found, nil
}

// Check reports every disagreement between the ledger, PLAN.md and the
// repository. It reports all of them rather than the first, because a ledger
// corrected one error per run is a ledger nobody finishes correcting.
func Check(root string, ledger Ledger, headings map[string]string) []string {
	problems := []string{}
	seen := map[string]bool{}

	for _, item := range ledger.Items {
		if seen[item.ID] {
			problems = append(problems, item.ID+": listed twice in the ledger")
			continue
		}
		seen[item.ID] = true

		title, ok := headings[item.ID]
		if !ok {
			problems = append(problems, item.ID+": in the ledger and not in PLAN.md")
			continue
		}
		// The heading is where a reader looks first, so it may not disagree
		// with the ledger about whether an item is finished.
		headingSaysDone := strings.Contains(title, "— complete") || strings.Contains(title, "deferred")
		ledgerSaysDone := item.State == ItemComplete || item.State == ItemDeferred
		if headingSaysDone != ledgerSaysDone {
			problems = append(problems, fmt.Sprintf(
				"%s: PLAN.md heading says %q and the ledger says %q", item.ID, title, item.State))
		}

		done, outstanding := 0, 0
		sliceIDs := map[string]bool{}
		for _, slice := range item.Slices {
			switch {
			case slice.ID == "" || slice.Statement == "":
				problems = append(problems, item.ID+": a slice has no id or no statement")
				continue
			case sliceIDs[slice.ID]:
				problems = append(problems, slice.ID+": listed twice")
				continue
			case !strings.HasPrefix(slice.ID, item.ID+"-"):
				problems = append(problems, slice.ID+": does not belong to "+item.ID+" by its id")
			}
			sliceIDs[slice.ID] = true

			switch slice.State {
			case SliceDone:
				done++
				if slice.Evidence == "" {
					problems = append(problems, slice.ID+": done and names no evidence")
					break
				}
				if err := resolveEvidence(root, slice.Evidence); err != nil {
					problems = append(problems, fmt.Sprintf("%s: %v", slice.ID, err))
				}
			case SliceInProgress, SliceNotStarted, SliceBlocked:
				outstanding++
				if slice.Evidence != "" {
					// Evidence for unfinished work is how a plan starts
					// describing intentions as achievements.
					problems = append(problems, slice.ID+": names evidence but is not done")
				}
				if slice.State == SliceBlocked && slice.BlockedOn == "" {
					problems = append(problems, slice.ID+": blocked without saying what on")
				}
			default:
				problems = append(problems, fmt.Sprintf("%s: unknown slice state %q", slice.ID, slice.State))
			}
		}

		switch item.State {
		case ItemComplete:
			if outstanding > 0 {
				problems = append(problems, fmt.Sprintf("%s: complete with %d slices outstanding", item.ID, outstanding))
			}
		case ItemNotStarted:
			if done > 0 {
				problems = append(problems, fmt.Sprintf("%s: not started with %d slices done", item.ID, done))
			}
		case ItemInProgress:
			if outstanding == 0 && len(item.Slices) > 0 {
				problems = append(problems, item.ID+": every slice is done, so it is complete rather than in progress")
			}
		case ItemDeferred:
		default:
			problems = append(problems, fmt.Sprintf("%s: unknown item state %q", item.ID, item.State))
		}
	}

	missing := []string{}
	for id := range headings {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		// The direction that catches a new item written straight into the
		// journal, which is how the plan grew a status notation per author.
		problems = append(problems, id+": in PLAN.md and not in the ledger")
	}
	return problems
}

func resolveEvidence(root, evidence string) error {
	switch {
	case strings.HasPrefix(evidence, "go:"):
		if err := docaudit.ResolveGoAnchor(root, evidence); err != nil {
			return fmt.Errorf("evidence %q does not resolve: %w", evidence, err)
		}
	case strings.HasPrefix(evidence, "file:"):
		path := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(evidence, "file:")))
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("evidence %q names a path that is not there", evidence)
		}
	case strings.HasPrefix(evidence, "test:"):
		name := strings.TrimPrefix(evidence, "test:")
		found, err := testExists(root, name)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("evidence %q names a test that is not defined", evidence)
		}
	default:
		return fmt.Errorf("evidence %q must begin with go:, file: or test:", evidence)
	}
	return nil
}

func testExists(root, name string) (bool, error) {
	needle := []byte("func " + name + "(")
	found := false
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(contents), string(needle)) {
			found = true
		}
		return nil
	})
	return found, err
}

// CheckPlanPointer reports whether the plan tells its reader where the rules
// that govern it live.
//
// The rules are in AGENTS.md because they outlive any one plan; a plan that
// does not name them leaves the next author with a generated block, a ledger
// and a build gate that nobody explained. This is checked rather than
// instructed for the same reason the ledger is: the instruction to write the
// pointer would otherwise live only in the document the pointer is in, and
// would go when it goes -- which is exactly how it came to be missing.
func CheckPlanPointer(planPath string) []string {
	contents, err := os.ReadFile(planPath)
	if err != nil {
		return []string{fmt.Sprintf("cannot read %s: %v", planPath, err)}
	}
	body := string(contents)
	const heading = "## Progress"
	const marker = "<!-- notrios:generated:plan:progress:begin -->"
	start, generated := strings.Index(body, heading), strings.Index(body, marker)
	switch {
	case start < 0:
		return []string{"PLAN.md has no `## Progress` section"}
	case generated < 0:
		return []string{"PLAN.md has no progress-log markers"}
	case generated < start:
		return []string{"PLAN.md's progress markers are not inside its Progress section"}
	}
	if !strings.Contains(body[start:generated], "AGENTS.md") {
		return []string{"PLAN.md's Progress section does not point at AGENTS.md, " +
			"so the next reader is left with a generated block and no rules"}
	}
	return nil
}
