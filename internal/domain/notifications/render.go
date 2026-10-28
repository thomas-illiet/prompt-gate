package notifications

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var markdownSyntax = regexp.MustCompile(`(?m)(^#{1,6}\s+|[*_~` + "`" + `]+|\[([^]]+)\]\([^)]+\)|^>\s*|^[-+]\s+)`)

// Renderer produces safe HTML and a readable plain-text alternative.
type Renderer struct {
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

// NewRenderer constructs the shared secure Markdown renderer.
func NewRenderer() *Renderer {
	return &Renderer{
		markdown: goldmark.New(goldmark.WithExtensions(extension.GFM)),
		policy:   bluemonday.UGCPolicy(),
	}
}

// Render converts Markdown to sanitized HTML and plain text.
func (r *Renderer) Render(markdown string) (string, string, error) {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return "", "", ErrInvalidBody
	}
	var rendered bytes.Buffer
	if err := r.markdown.Convert([]byte(markdown), &rendered); err != nil {
		return "", "", fmt.Errorf("render notification markdown: %w", err)
	}
	htmlBody := strings.TrimSpace(r.policy.Sanitize(rendered.String()))
	plain := markdownSyntax.ReplaceAllString(markdown, "$2")
	plain = html.UnescapeString(strings.TrimSpace(plain))
	return plain, htmlBody, nil
}

// RenderTemplate converts Markdown and places it inside the branded email
// layout associated with the notification kind.
func (r *Renderer) RenderTemplate(kind Kind, recipientName, subject, markdown, frontendURL string) (string, string, error) {
	if kind != KindManual {
		markdown = stripLeadingMarkdownHeading(markdown)
	}
	plainContent, htmlContent, err := r.Render(markdown)
	if err != nil {
		return "", "", err
	}
	template := templateFor(kind, recipientName, subject)
	plainParts := []string{template.title, template.intro, template.supporting, template.details, plainContent}
	if actionURL := publicEmailURL(frontendURL, template.actionPath); actionURL != "" {
		plainParts = append(plainParts, template.action+": "+actionURL)
	}
	return strings.Join(plainParts, "\n\n"), renderEmailTemplate(template, subject, htmlContent, frontendURL), nil
}

func stripLeadingMarkdownHeading(markdown string) string {
	lines := strings.Split(strings.TrimSpace(markdown), "\n")
	if len(lines) == 0 || !strings.HasPrefix(strings.TrimSpace(lines[0]), "# ") {
		return strings.TrimSpace(markdown)
	}
	lines = lines[1:]
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
