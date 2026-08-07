package store

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	// templateFenceRE finds the ```note-template block. Same shape as the
	// note-query fence E7 established, so a reader who has met one has met both.
	templateFenceRE = regexp.MustCompile("(?s)```[ \t]*note-template[ \t]*\r?\n(.*?)```")
	// placeholderRE matches `{{name}}`. Names are a closed character set: no
	// dots, no pipes, no parentheses, so a placeholder can never look like a
	// call or a path even by accident.
	placeholderRE = regexp.MustCompile(`\{\{[ \t]*([A-Za-z][A-Za-z0-9_]{0,63})[ \t]*\}\}`)
)

// ListTemplates returns every note carrying a template block.
//
// Malformed templates are listed with their error rather than hidden. A
// template that fails to parse is exactly the one its author needs to find.
func (s *SQLiteStore) ListTemplates(ctx context.Context, collectionID string) ([]Template, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(collectionID) == "" {
		collectionID = "default"
	}
	docs, err := s.documentsContainingLocked(ctx, collectionID, "%```note-template%")
	if err != nil {
		return nil, err
	}
	templates := make([]Template, 0, len(docs))
	for _, doc := range docs {
		templates = append(templates, templateFromDocument(doc))
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Title < templates[j].Title })
	return templates, nil
}

// GetTemplate reads one template.
func (s *SQLiteStore) GetTemplate(ctx context.Context, documentID string) (Template, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Template{}, err
	}
	doc, err := s.GetDocument(ctx, documentID)
	if err != nil {
		return Template{}, err
	}
	template := templateFromDocument(doc)
	if !templateFenceRE.MatchString(doc.Body) {
		return Template{}, fmt.Errorf("%w: note %q carries no note-template block", ErrInvalidInput, documentID)
	}
	return template, nil
}

// CreateFromTemplate fills a template in and stores the result as a new note.
//
// The created note is an ordinary note: no marker, no link back, nothing that
// makes it a second-class citizen of the library. A template is a starting
// point, not a parent.
func (s *SQLiteStore) CreateFromTemplate(ctx context.Context, req CreateFromTemplateRequest) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return Document{}, fmt.Errorf("%w: a title is required", ErrInvalidInput)
	}
	source, err := s.GetDocument(ctx, strings.TrimSpace(req.TemplateID))
	if err != nil {
		return Document{}, err
	}
	template := templateFromDocument(source)
	if template.Error != "" {
		return Document{}, fmt.Errorf("%w: template %q is malformed: %s", ErrInvalidInput, req.TemplateID, template.Error)
	}

	notebookID := strings.TrimSpace(req.NotebookID)
	notebookName := notebookID
	if notebookID != "" {
		nb, err := s.GetNotebook(ctx, notebookID)
		if err != nil {
			return Document{}, err
		}
		notebookName = nb.Name
	} else if nb, err := s.GetNotebook(ctx, DefaultNotebookID); err == nil {
		notebookName = nb.Name
	}

	body, err := renderTemplate(template, stripTemplateBlock(source.Body), req.Values, title, notebookName, time.Now())
	if err != nil {
		return Document{}, err
	}
	// The title is rendered too, so `{{date}} standup` works as a title.
	renderedTitle, err := renderTemplate(template, title, req.Values, title, notebookName, time.Now())
	if err != nil {
		return Document{}, err
	}
	return s.CreateDocument(ctx, CreateDocumentRequest{
		CollectionID: source.CollectionID,
		NotebookID:   notebookID,
		Title:        renderedTitle,
		Body:         body,
		Message:      "created from template " + source.ID,
	})
}

// templateFromDocument parses a note's template block.
func templateFromDocument(doc Document) Template {
	template := Template{
		DocumentID: doc.ID,
		Title:      doc.Title,
		NotebookID: doc.NotebookID,
		Prompts:    []TemplatePrompt{},
		Variables:  []string{},
	}
	match := templateFenceRE.FindStringSubmatch(doc.Body)
	if match == nil {
		template.Error = "no note-template block"
		return template
	}
	prompts, description, err := parseTemplateBlock(match[1])
	if err != nil {
		template.Error = err.Error()
		return template
	}
	template.Prompts = prompts
	template.Description = description

	// Report the automatic names actually used, and refuse a placeholder that
	// is neither declared nor automatic — a template with a typo in a
	// placeholder is broken, and finding out at creation time is too late.
	declared := map[string]bool{}
	for _, prompt := range prompts {
		declared[prompt.Name] = true
	}
	automatic := map[string]bool{}
	for _, name := range TemplateVariables() {
		automatic[name] = true
	}
	usedAutomatic := map[string]bool{}
	for _, found := range placeholderRE.FindAllStringSubmatch(stripTemplateBlock(doc.Body)+" "+doc.Title, -1) {
		name := found[1]
		switch {
		case declared[name]:
		case automatic[name]:
			usedAutomatic[name] = true
		default:
			template.Error = fmt.Sprintf("unknown placeholder {{%s}}: declare it with `prompt: %s`, or use one of %s",
				name, name, strings.Join(TemplateVariables(), ", "))
			return template
		}
	}
	for _, name := range TemplateVariables() {
		if usedAutomatic[name] {
			template.Variables = append(template.Variables, name)
		}
	}
	return template
}

// parseTemplateBlock reads the `key: value` lines.
func parseTemplateBlock(block string) ([]TemplatePrompt, string, error) {
	if len(block) > MaxTemplateBlockBytes {
		return nil, "", fmt.Errorf("block is longer than %d bytes", MaxTemplateBlockBytes)
	}
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	if len(lines) > MaxTemplateLines {
		return nil, "", fmt.Errorf("block has more than %d lines", MaxTemplateLines)
	}
	prompts := []TemplatePrompt{}
	seen := map[string]bool{}
	description := ""
	for number, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			return nil, "", fmt.Errorf("line %d is not `key: value`", number+1)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "prompt":
			name, label, _ := strings.Cut(value, " ")
			name = strings.TrimSpace(name)
			if !placeholderRE.MatchString("{{" + name + "}}") {
				return nil, "", fmt.Errorf("line %d: %q is not a usable placeholder name (letters, digits and underscore, starting with a letter)", number+1, name)
			}
			if seen[name] {
				return nil, "", fmt.Errorf("line %d: prompt %q is declared twice", number+1, name)
			}
			for _, reserved := range TemplateVariables() {
				if name == reserved {
					return nil, "", fmt.Errorf("line %d: %q is an automatic name and cannot be prompted for", number+1, name)
				}
			}
			seen[name] = true
			prompts = append(prompts, TemplatePrompt{Name: name, Label: strings.TrimSpace(strings.TrimLeft(label, "—-"))})
			if len(prompts) > MaxTemplatePrompts {
				return nil, "", fmt.Errorf("more than %d prompts", MaxTemplatePrompts)
			}
		case "description":
			description = value
		default:
			return nil, "", fmt.Errorf("line %d: unknown key %q (use prompt, description)", number+1, key)
		}
	}
	return prompts, description, nil
}

// stripTemplateBlock removes the declaration from what gets copied.
func stripTemplateBlock(body string) string {
	return strings.TrimLeft(templateFenceRE.ReplaceAllString(body, ""), "\n")
}

// renderTemplate substitutes placeholders in **one pass**.
//
// One pass is the security property, not an optimisation. A supplied value
// containing `{{date}}` is inserted literally and never re-scanned, so a value
// cannot introduce a placeholder, cannot reach an automatic name it was not
// given, and cannot recurse. Everything a caller supplies is data.
func renderTemplate(template Template, text string, values map[string]string, title, notebookName string, now time.Time) (string, error) {
	resolved := map[string]string{
		TemplateVarDate:     now.Format("2006-01-02"),
		TemplateVarTime:     now.Format("15:04"),
		TemplateVarDateTime: now.Format("2006-01-02 15:04"),
		TemplateVarTitle:    title,
		TemplateVarNotebook: notebookName,
	}
	for _, prompt := range template.Prompts {
		value, given := values[prompt.Name]
		if !given {
			return "", fmt.Errorf("%w: template needs a value for %q", ErrInvalidInput, prompt.Name)
		}
		if len(value) > MaxTemplateValueBytes {
			return "", fmt.Errorf("%w: value for %q is longer than %d bytes", ErrInvalidInput, prompt.Name, MaxTemplateValueBytes)
		}
		resolved[prompt.Name] = value
	}
	for name := range values {
		if _, known := resolved[name]; !known {
			return "", fmt.Errorf("%w: template declares no prompt %q", ErrInvalidInput, name)
		}
	}

	var failure error
	rendered := placeholderRE.ReplaceAllStringFunc(text, func(match string) string {
		name := placeholderRE.FindStringSubmatch(match)[1]
		value, known := resolved[name]
		if !known {
			failure = fmt.Errorf("%w: unknown placeholder {{%s}}", ErrInvalidInput, name)
			return match
		}
		return value
	})
	return rendered, failure
}

// documentsContainingLocked finds notes whose body matches a LIKE pattern.
//
// A LIKE scan rather than FTS: the fence is punctuation, which the tokenizer
// discards, so full-text search cannot find it. The scan is bounded by the
// collection and by the caller's expectation that templates are few.
func (s *SQLiteStore) documentsContainingLocked(ctx context.Context, collectionID, pattern string) ([]Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.documentsMatchingBodyLocked(collectionID, pattern)
}
