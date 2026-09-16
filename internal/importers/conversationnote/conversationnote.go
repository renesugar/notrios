// Package conversationnote renders one exported conversation as Markdown.
//
// ChatGPT and Claude both export more than what was said: code that ran, its
// output, pages the model browsed, and the model's own thinking and tool calls.
// J26's owner decision is that a note carries the conversation and the work —
// messages, code, execution output, browsing results and attachments — and not
// the machinery: thinking, reasoning recaps, and the plumbing of tool calls.
// Those are counted in the import report instead, so what was left out is
// visible rather than silently gone.
package conversationnote

import (
	"fmt"
	"strings"
)

// Counts records what a rendering kept and what it left out, for the report.
type Counts struct {
	Messages    int
	CodeBlocks  int
	Attachments int
	// Machinery counts the parts deliberately not rendered: thinking,
	// reasoning recaps, tool calls and their results.
	Machinery int
	// Empty counts messages with nothing to render.
	Empty int
}

// Builder assembles a conversation note, one message at a time.
type Builder struct {
	body   strings.Builder
	counts Counts
}

// Section starts a message with its speaker and time.
func (b *Builder) Section(heading, when string) {
	if when != "" {
		heading += " — " + when
	}
	b.body.WriteString("## " + heading + "\n\n")
	b.counts.Messages++
}

// Text adds a message's prose.
func (b *Builder) Text(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	b.body.WriteString(text + "\n\n")
}

// Code adds a fenced code block. language may be empty, and label names what
// the block is when it is not the conversation's own code ("output", say).
func (b *Builder) Code(label, language, code string) {
	code = strings.TrimRight(code, "\n")
	if strings.TrimSpace(code) == "" {
		return
	}
	if label != "" {
		b.body.WriteString("**" + label + "**\n\n")
	}
	fence := "```"
	// A block containing a fence needs a longer one, or it ends early.
	for strings.Contains(code, fence) {
		fence += "`"
	}
	b.body.WriteString(fence + language + "\n" + code + "\n" + fence + "\n\n")
	b.counts.CodeBlocks++
}

// Attachment names a file the message carried. uri is a resource:// link when
// the archive held the bytes, and empty when it held only the name.
func (b *Builder) Attachment(filename, uri string) {
	switch {
	case uri != "":
		b.body.WriteString(fmt.Sprintf("![%s](%s)\n\n", filename, uri))
	default:
		b.body.WriteString(fmt.Sprintf("*Attachment: %s (not in the archive)*\n\n", filename))
	}
	b.counts.Attachments++
}

// Machinery counts a part deliberately left out of the note.
func (b *Builder) Machinery(n int) { b.counts.Machinery += n }

// Empty counts a message with nothing to render.
func (b *Builder) Empty() { b.counts.Empty++ }

// Counts reports what was kept and left out.
func (b *Builder) Counts() Counts { return b.counts }

// Body is the finished note.
func (b *Builder) Body() string { return strings.TrimSpace(b.body.String()) + "\n" }

// Heading names a speaker: the display name when the export gives one, else the
// role.
func Heading(role, name string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user", "human":
		return "User"
	case "assistant":
		return "Assistant"
	case "":
		return "Message"
	default:
		return strings.ToUpper(role[:1]) + role[1:]
	}
}
