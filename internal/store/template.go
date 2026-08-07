package store

// Note templates (v0.6 F4).
//
// A template is an ordinary note carrying a fenced ```note-template block. That
// reuses E7's shape deliberately: a line-oriented `key: value` block, parsed
// server-side, with no YAML parser and no expression language.
//
// **Substitution is replacement, never evaluation.** A placeholder is
// `{{name}}` and a name is either declared by the template or drawn from a
// closed vocabulary. There is no arithmetic, no conditionals, no file paths,
// and no way to reach anything outside the note being created. An open
// vocabulary would be an expression language, and an expression language living
// in note content is precisely what E7 spent a slice refusing.

// Template block limits, mirroring the note-query block's.
const (
	MaxTemplateBlockBytes = 4096
	MaxTemplateLines      = 32
	MaxTemplatePrompts    = 20
	// MaxTemplateValueBytes bounds one supplied value. A prompt is a field in a
	// dialog, not a way to paste a note into another note.
	MaxTemplateValueBytes = 4096
)

// Automatic placeholder names. **Closed** — an unknown name is an error naming
// this list, never a silent empty string, because a template that quietly
// produces `{{onwer}}` as literal text has failed in the least visible way.
const (
	TemplateVarDate     = "date"     // 2026-08-07
	TemplateVarTime     = "time"     // 14:03
	TemplateVarDateTime = "datetime" // 2026-08-07 14:03
	TemplateVarTitle    = "title"    // the new note's title
	TemplateVarNotebook = "notebook" // the notebook it is filed in
)

// TemplateVariables lists the automatic names, in documentation order.
func TemplateVariables() []string {
	return []string{TemplateVarDate, TemplateVarTime, TemplateVarDateTime, TemplateVarTitle, TemplateVarNotebook}
}

// TemplatePrompt is one value the template asks for.
type TemplatePrompt struct {
	Name string `json:"name"`
	// Label is the trailing prose on the `prompt:` line, shown to whoever fills
	// it in. Empty is fine; the name is the fallback.
	Label string `json:"label,omitempty"`
}

// Template is a note that can produce other notes.
type Template struct {
	DocumentID  string           `json:"document_id"`
	Title       string           `json:"title"`
	NotebookID  string           `json:"notebook_id,omitempty"`
	Description string           `json:"description,omitempty"`
	Prompts     []TemplatePrompt `json:"prompts"`
	// Variables are the automatic names this template actually uses, so a
	// caller can show what will be filled in without being told to guess.
	Variables []string `json:"variables"`
	// Error is set when the block is malformed. Like a note-query block, a
	// broken template is a value rather than a failure: it must still be
	// listable and readable so its author can see what is wrong.
	Error string `json:"error,omitempty"`
}

// CreateFromTemplateRequest fills a template in.
type CreateFromTemplateRequest struct {
	TemplateID string `json:"template_id"`
	Title      string `json:"title"`
	NotebookID string `json:"notebook_id,omitempty"`
	// Values supplies every declared prompt. A missing one is an error rather
	// than an empty string: silently producing a note with a blank where a
	// value belonged is worse than refusing.
	Values map[string]string `json:"values,omitempty"`
}
