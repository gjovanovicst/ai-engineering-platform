package domain

import (
	"strings"
	"unicode"
)

// Slugify turns a human name into a URL-safe slug. It is deterministic and
// keeps Unicode letters so that non-English names stay readable.
func Slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) < 3 {
		slug += "-org"
	}
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	return slug
}

// UniqueSlug appends -2, -3, ... while taken reports true.
func UniqueSlug(base string, taken func(string) bool) string {
	slug := base
	for i := 2; i <= 9 && taken(slug); i++ {
		trimmed := base
		if len(trimmed) > 36 {
			trimmed = trimmed[:36]
		}
		slug = trimmed + "-" + itoa(i)
	}
	return slug
}

func itoa(n int) string {
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
