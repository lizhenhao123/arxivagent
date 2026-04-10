package service

import (
	"regexp"
	"strings"
)

func stringPtrIfPresent(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	s := strings.TrimSpace(v)
	return &s
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func buildSlug(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.ReplaceAll(s, "—", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "draft"
	}
	return s
}

func renderMarkdown(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var b strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "### "):
			b.WriteString("<h3>" + escapeHTML(strings.TrimPrefix(trimmed, "### ")) + "</h3>\n")
		case strings.HasPrefix(trimmed, "## "):
			b.WriteString("<h2>" + escapeHTML(strings.TrimPrefix(trimmed, "## ")) + "</h2>\n")
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString("<h1>" + escapeHTML(strings.TrimPrefix(trimmed, "# ")) + "</h1>\n")
		case trimmed == "":
			b.WriteString("\n")
		default:
			b.WriteString("<p>" + escapeHTML(trimmed) + "</p>\n")
		}
	}
	return b.String()
}

func escapeHTML(v string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(v)
}
