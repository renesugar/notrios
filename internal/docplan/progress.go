package docplan

import (
	"fmt"
	"sort"
	"strings"
)

// ProgressLines renders the progress log: what is unfinished, and what remains
// in each unfinished thing.
//
// Finished items are one row in a table and nothing more. Their story is in
// their own Outcome block further down the plan, and repeating it here would
// rebuild the problem this log exists to solve -- a document where the work
// still to do is outnumbered two to one by the work already done.
func (l Ledger) ProgressLines() []string {
	counts := map[ItemState]int{}
	for _, item := range l.Items {
		counts[item.State]++
	}
	lines := []string{
		fmt.Sprintf("**%d items: %d complete, %d in progress, %d not started, %d deferred.**",
			len(l.Items), counts[ItemComplete], counts[ItemInProgress], counts[ItemNotStarted], counts[ItemDeferred]),
		"",
		"| Item | State | Slices done | Outstanding |",
		"|---|---|---|---|",
	}
	for _, item := range l.Items {
		done, outstanding := item.counts()
		total := len(item.Slices)
		remaining := "—"
		if outstanding > 0 {
			remaining = fmt.Sprintf("%d", outstanding)
		}
		lines = append(lines, fmt.Sprintf("| %s. %s | %s | %d/%d | %s |",
			item.ID, item.Title, item.State, done, total, remaining))
	}

	started := []Item{}
	for _, item := range l.Items {
		done, outstanding := item.counts()
		if item.State == ItemInProgress || (outstanding > 0 && done > 0) {
			started = append(started, item)
		}
	}
	sort.SliceStable(started, func(i, j int) bool { return started[i].ID < started[j].ID })
	if len(started) == 0 {
		return append(lines, "", "Nothing is half-finished.")
	}

	lines = append(lines, "", "### Started and not finished")
	for _, item := range started {
		lines = append(lines, "", fmt.Sprintf("**%s. %s**", item.ID, item.Title), "")
		for _, slice := range item.Slices {
			if slice.State == SliceDone {
				continue
			}
			line := fmt.Sprintf("- `%s` %s — *%s*", slice.ID, slice.Statement, slice.State)
			if slice.BlockedOn != "" {
				line += fmt.Sprintf(" (blocked on: %s)", slice.BlockedOn)
			}
			lines = append(lines, line)
		}
	}

	waiting := []Item{}
	for _, item := range l.Items {
		if item.State == ItemNotStarted {
			waiting = append(waiting, item)
		}
	}
	if len(waiting) > 0 {
		names := make([]string, 0, len(waiting))
		for _, item := range waiting {
			names = append(names, item.ID)
		}
		lines = append(lines, "", "### Not started",
			"", "Written and not begun: "+strings.Join(names, ", ")+
				". Their slices are listed under each item.")
	}
	return lines
}

func (i Item) counts() (done, outstanding int) {
	for _, slice := range i.Slices {
		if slice.State == SliceDone {
			done++
			continue
		}
		outstanding++
	}
	return done, outstanding
}

// RoadmapStatusLines is the one sentence the roadmap needs about the active
// plan, generated so it cannot rot.
//
// The sentence it replaces said "H0 is complete; H1 is the next separately
// approval-gated item" for a month after fourteen items had finished. A
// roadmap says what a version means; how far along it is belongs to the plan,
// and the roadmap should quote it rather than keep its own copy.
func (l Ledger) RoadmapStatusLines() []string {
	counts := map[ItemState]int{}
	for _, item := range l.Items {
		counts[item.State]++
	}
	next := []string{}
	for _, item := range l.Items {
		if item.State == ItemInProgress {
			next = append(next, item.ID)
		}
	}
	lines := []string{
		fmt.Sprintf("`PLAN.md` holds the active plan derived from this roadmap: %d items, "+
			"%d complete, %d in progress, %d not started, %d deferred.",
			len(l.Items), counts[ItemComplete], counts[ItemInProgress],
			counts[ItemNotStarted], counts[ItemDeferred]),
	}
	if len(next) > 0 {
		lines = append(lines, "",
			"Started and unfinished: "+strings.Join(next, ", ")+
				". What remains in each is in the plan's own Progress section.")
	}
	return lines
}
