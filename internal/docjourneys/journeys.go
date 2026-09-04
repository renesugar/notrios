// Package docjourneys is the command-line journey catalogue: what a person
// sets out to do, the steps that do it, and how they can tell it worked.
//
// It exists because a command reference answers "what does this flag mean?"
// and never answers "how do I get this done?". The two are different documents
// and only one of them is written here.
//
// Every journey is executed. That is the whole reason the catalogue is data
// rather than prose: a sequence of commands in a Markdown file is a claim, and
// a sequence of commands a test runs against a real library is a fact. The
// distinction matters most for the part that is easiest to get wrong -- not
// whether a command exits zero, but whether the thing the reader wanted
// actually happened afterwards.
package docjourneys

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const Schema = "notrios.docjourneys.cli.v1"

// Step is one command a reader would type, and the sentence explaining why.
type Step struct {
	// Narrative is what the reader is doing and why, in one or two sentences.
	Narrative string `json:"narrative"`
	// Command is the notriosctl argument vector, with {placeholders} the runner
	// substitutes. It never includes the binary name.
	Command []string `json:"command"`
	// AllowFailure marks a step whose non-zero exit is the point, such as a
	// refusal a reader should see rather than avoid.
	AllowFailure bool `json:"allow_failure,omitempty"`
	// Manual marks a step the reader performs outside notriosctl -- writing a
	// file, plugging in a drive, clicking something. Such a step has no command
	// and the runner does not execute it.
	//
	// It is a first-class part of a journey rather than an omission. A
	// catalogue that could only describe steps it can run would quietly leave
	// out the parts a reader is most likely to get wrong, and would read as
	// though a task were shorter than it is.
	Manual bool `json:"manual,omitempty"`
}

// Postcondition is how a reader confirms the journey worked, and it is
// deliberately a separate command rather than the exit status of the last
// step.
//
// A command can exit zero having done nothing, and -- the case that matters --
// it can exit zero having done the opposite of what the page described.
// Checking the observable state afterwards is the only way to tell those
// apart.
type Postcondition struct {
	Narrative string   `json:"narrative"`
	Command   []string `json:"command"`
	// Contains must all appear in the combined output; Absent must not.
	Contains []string `json:"contains,omitempty"`
	Absent   []string `json:"absent,omitempty"`
	// FileExists is a path, after substitution, that must exist afterwards.
	FileExists string `json:"file_exists,omitempty"`
}

type Journey struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Goal  string `json:"goal"`
	// Feature is the capability in docs/docfeatures/FEATURES.json this journey
	// demonstrates. Linking them is what lets a feature with no journey be
	// found, which is a different gap from a surface with no feature.
	Feature       string        `json:"feature"`
	Steps         []Step        `json:"steps"`
	Postcondition Postcondition `json:"postcondition"`
	// Note records something a reader should know that the steps cannot show,
	// most often a capability the command line does not have.
	Note string `json:"note,omitempty"`
	// Guessability says whether this journey's command could be produced from
	// command-line convention alone, without reading anything.
	//
	// It is recorded because it decides whether a journey can measure a page at
	// all. `notriosctl paths` is conventional, and the H14 pilot confirmed it:
	// a model with no documentation produced it correctly, so the task scored
	// nothing and could never have scored anything. A task whose command is
	// guessable measures the model. Only "notrios-specific" journeys are worth
	// putting through the three arms.
	Guessability string `json:"guessability"`
}

type Catalogue struct {
	Schema   string    `json:"schema"`
	Journeys []Journey `json:"journeys"`
}

func Load(path string) (Catalogue, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Catalogue{}, err
	}
	var catalogue Catalogue
	if err := json.Unmarshal(contents, &catalogue); err != nil {
		return Catalogue{}, err
	}
	if catalogue.Schema != Schema {
		return Catalogue{}, fmt.Errorf("unexpected journey schema %q", catalogue.Schema)
	}
	if err := catalogue.validate(); err != nil {
		return Catalogue{}, err
	}
	return catalogue, nil
}

func (c Catalogue) validate() error {
	seen := map[string]bool{}
	for _, journey := range c.Journeys {
		switch {
		case journey.ID == "":
			return fmt.Errorf("a journey has no id")
		case seen[journey.ID]:
			return fmt.Errorf("duplicate journey %q", journey.ID)
		case journey.Title == "" || journey.Goal == "":
			return fmt.Errorf("%s has no title or goal", journey.ID)
		case journey.Feature == "":
			return fmt.Errorf("%s names no feature", journey.ID)
		case journey.Guessability != "conventional" && journey.Guessability != "notrios-specific":
			return fmt.Errorf("%s must record guessability as conventional or notrios-specific", journey.ID)
		case len(journey.Steps) == 0:
			return fmt.Errorf("%s has no steps", journey.ID)
		}
		seen[journey.ID] = true
		for index, step := range journey.Steps {
			if step.Narrative == "" {
				return fmt.Errorf("%s step %d has no narrative", journey.ID, index+1)
			}
			if step.Manual != (len(step.Command) == 0) {
				return fmt.Errorf("%s step %d must have a command or be marked manual, not both or neither",
					journey.ID, index+1)
			}
		}
		// A journey with no way to tell whether it worked is a journey that
		// cannot fail, which is worse than not having it.
		post := journey.Postcondition
		if post.Narrative == "" {
			return fmt.Errorf("%s has no postcondition narrative", journey.ID)
		}
		if len(post.Command) == 0 && post.FileExists == "" {
			return fmt.Errorf("%s has no postcondition to check", journey.ID)
		}
		if len(post.Command) > 0 && len(post.Contains) == 0 && len(post.Absent) == 0 {
			return fmt.Errorf("%s runs a postcondition command but asserts nothing about it", journey.ID)
		}
	}
	return nil
}

// Each task below lists the steps that do it, in order.
//
//notrios:doc user cli-journey-surface
//notrios:help journeys-cli the-journeys
//notrios:enumerates go:github.com/renesugar/notrios/internal/docjourneys#Catalogue
func (c Catalogue) Lines() []string {
	// Steps and their commands are emitted, not just journey titles. A list of
	// titles tells a reader which tasks exist and leaves them no better able to
	// do any of them, which is what this page was for. The commands come from
	// the same catalogue the runner executes, so what is printed here is what
	// was run.
	lines := []string{}
	for _, journey := range c.Journeys {
		heading := fmt.Sprintf("**%s** — %s", journey.Title, journey.Goal)
		if journey.Note != "" {
			heading += " " + journey.Note
		}
		lines = append(lines, heading)
		for _, step := range journey.Steps {
			// Indented, so the steps nest under their task instead of sitting
			// beside it. A flat list gave a reader no way to see where one task
			// ended and the next began.
			if step.Manual {
				lines = append(lines, "  - "+step.Narrative+" *(you do this yourself)*")
				continue
			}
			// A code span on its own line rather than a fenced block. A fence
			// inside a generated section is detected as an executable example
			// and needs a registry entry per step, whose hash would change
			// every time a narrative was reworded -- churn with no reader
			// benefit, since the command is already executed by the journey
			// runner.
			lines = append(lines, fmt.Sprintf("  - %s\n\n    `notriosctl %s`",
				step.Narrative, readableCommand(step.Command)))
		}
	}
	return lines
}

// Substitute expands {placeholders} in a command or path.
func Substitute(values map[string]string, argument string) string {
	for name, value := range values {
		argument = strings.ReplaceAll(argument, "{"+name+"}", value)
	}
	return argument
}

// readableCommand renders a step as a reader would type it.
//
// The catalogue carries the sandbox plumbing a test needs -- an explicit
// database and asset store per run, so journeys cannot touch each other or
// anybody's real library. None of that belongs on a documentation page: a
// reader has one library, configured, and typing --db every time is not how the
// tool is used. Leaving it in was the single clearest way this page read as
// test instructions rather than documentation.
//
// What remains of a placeholder becomes an angle-bracket metavariable, because
// `{note}` is a substitution and `<note-id>` is an instruction.
func readableCommand(command []string) string {
	plumbing := map[string]bool{"--db": true, "--asset-store": true, "--config": true, "--keys": true}
	rendered, skip := []string{}, false
	for _, argument := range command {
		if skip {
			skip = false
			continue
		}
		if plumbing[argument] {
			skip = true
			continue
		}
		if strings.HasPrefix(argument, "{") && strings.HasSuffix(argument, "}") {
			argument = "<" + strings.ReplaceAll(strings.Trim(argument, "{}"), "_", "-") + ">"
		}
		if strings.ContainsAny(argument, " ") {
			argument = strconv.Quote(argument)
		}
		rendered = append(rendered, argument)
	}
	return strings.Join(rendered, " ")
}
