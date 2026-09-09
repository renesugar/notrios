package clispec

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// noteIndent is where a command's notes line up under its usage line.
const noteIndent = "                                                 # "

// Help writes help for a path: the whole program when the path is empty, a
// group's subcommands when it names a group, and one command otherwise.
//
// Every level answers in the same shape and the caller exits 0. Asking for
// help was a usage error at group level before this -- `notriosctl notes
// --help` printed `unknown notes subcommand "--help"` and exited 2 -- which
// tells a person looking for help that they have made a mistake.
func (r Registry) Help(out io.Writer, path ...string) bool {
	switch {
	case len(path) == 0:
		r.writeProgramHelp(out)
		return true
	case len(path) == 1:
		if command, ok := r.Lookup(path...); ok && len(r.Children(path...)) == 0 {
			r.writeCommandHelp(out, command)
			return true
		}
		if _, ok := r.Group(path[0]); ok {
			r.writeGroupHelp(out, path[0])
			return true
		}
		return false
	default:
		if command, ok := r.Lookup(path...); ok {
			r.writeCommandHelp(out, command)
			return true
		}
		if len(r.Children(path...)) > 0 {
			r.writeGroupHelp(out, path...)
			return true
		}
		return false
	}
}

func (r Registry) writeProgramHelp(out io.Writer) {
	fmt.Fprintf(out, "%s - %s\n\nUsage:\n", r.Program, r.Tagline)
	for _, command := range r.Offered() {
		fmt.Fprintf(out, "  %s\n", command.Form(r.Program))
		for _, line := range command.explanation() {
			fmt.Fprintf(out, "%s%s\n", noteIndent, line)
		}
	}
	fmt.Fprintf(out, "\nRun `%s help <command>` or `%s <command> --help` for one command.\n", r.Program, r.Program)
}

func (r Registry) writeGroupHelp(out io.Writer, path ...string) {
	name := strings.Join(path, " ")
	if group, ok := r.Group(path[0]); ok && len(path) == 1 {
		fmt.Fprintf(out, "%s %s - %s\n\n", r.Program, name, group.Purpose)
	} else {
		fmt.Fprintf(out, "%s %s\n\n", r.Program, name)
	}
	fmt.Fprintln(out, "Usage:")
	for _, command := range r.Children(path...) {
		fmt.Fprintf(out, "  %s\n", command.Form(r.Program))
		if command.Purpose != "" {
			fmt.Fprintf(out, "%s%s\n", noteIndent, command.Purpose)
		}
	}
}

func (r Registry) writeCommandHelp(out io.Writer, command Command) {
	fmt.Fprintf(out, "%s\n", command.Form(r.Program))
	if command.Purpose != "" {
		fmt.Fprintf(out, "\n%s\n", command.Purpose)
	}
	for _, note := range command.Notes {
		fmt.Fprintf(out, "%s\n", note)
	}
}

// IsHelpRequest reports whether an argument asks for help.
//
// `help` is included because `notriosctl notes help` is a thing people type,
// and answering it costs nothing.
func IsHelpRequest(argument string) bool {
	switch argument {
	case "-h", "--help", "help":
		return true
	}
	return false
}

// HelpRequested reports whether any argument before a `--` separator asks for
// help. A command's own flags are parsed later; this runs first so that every
// level answers the same way instead of leaving it to Go's flag package, whose
// dump names flags in a form the documentation does not use and never mentions
// positional arguments at all.
func HelpRequested(args []string) bool {
	for _, argument := range args {
		if argument == "--" {
			return false
		}
		if IsHelpRequest(argument) {
			return true
		}
	}
	return false
}

// explanation is a command's purpose followed by its notes, which is what a
// reader scanning the whole command line needs and what the old help text
// showed as one run of comment lines.
func (c Command) explanation() []string {
	lines := []string{}
	if strings.TrimSpace(c.Purpose) != "" {
		lines = append(lines, c.Purpose)
	}
	return append(lines, c.Notes...)
}

// JSON writes the command line as data, for a tool that needs to discover what
// this program offers rather than read about it.
//
// The documentation generator reads the registry directly because it is in the
// same repository. Anything else -- a shell completion, a wrapper, an agent --
// has only the binary, and asking a program what it can do should not mean
// parsing its prose.
func (r Registry) JSON(out io.Writer, path ...string) error {
	view := Registry{
		Schema:  r.Schema,
		Program: r.Program,
		Tagline: r.Tagline,
		Groups:  r.Groups,
	}
	prefix := strings.Join(path, " ")
	for _, command := range r.Offered() {
		if prefix != "" && command.Name() != prefix && !strings.HasPrefix(command.Name(), prefix+" ") {
			continue
		}
		view.Commands = append(view.Commands, command)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(view)
}
