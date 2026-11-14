package design

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var tokenSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug returns a CSS-safe token name.
func Slug(name string) string {
	slug := tokenSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	return strings.Trim(slug, "-")
}

func sortedTokens(tokens []Token) []Token {
	out := append([]Token(nil), tokens...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CSS renders design tokens as custom properties.
func CSS(tokens []Token) string {
	var b strings.Builder
	b.WriteString(":root {\n")
	for _, token := range sortedTokens(tokens) {
		slug := Slug(token.Name)
		if slug == "" {
			continue
		}
		_, _ = fmt.Fprintf(&b, "  --%s: %s;\n", slug, token.Value)
	}
	b.WriteString("}\n")
	return b.String()
}

// SCSS renders design tokens as variables.
func SCSS(tokens []Token) string {
	var b strings.Builder
	for _, token := range sortedTokens(tokens) {
		slug := Slug(token.Name)
		if slug == "" {
			continue
		}
		_, _ = fmt.Fprintf(&b, "$%s: %s;\n", slug, token.Value)
	}
	return b.String()
}

// TypeScript renders design tokens as a typed const object.
func TypeScript(tokens []Token) string {
	var b strings.Builder
	b.WriteString("// Generated from the design token registry.\n")
	b.WriteString("export const tokens = {\n")
	for _, token := range sortedTokens(tokens) {
		slug := Slug(token.Name)
		if slug == "" {
			continue
		}
		_, _ = fmt.Fprintf(&b, "  %q: %q,\n", slug, token.Value)
	}
	b.WriteString("} as const;\n\nexport type TokenName = keyof typeof tokens;\n")
	return b.String()
}

// Render picks a format: css, scss or ts.
func Render(format string, tokens []Token) (string, string, error) {
	switch strings.ToLower(format) {
	case "css":
		return CSS(tokens), "text/css; charset=utf-8", nil
	case "scss":
		return SCSS(tokens), "text/x-scss; charset=utf-8", nil
	case "ts", "typescript":
		return TypeScript(tokens), "text/typescript; charset=utf-8", nil
	default:
		return "", "", fmt.Errorf("format must be css, scss or ts")
	}
}
