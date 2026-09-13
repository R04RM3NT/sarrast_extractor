package database

import (
	"net/url"
	"path"
	"strings"
	"unicode"
)

// SlugFromName returns the canonical, unique identity of a series derived from
// its human-readable name.
func SlugFromName(name string) string {
	return canonicalName(name)
}

// SlugFromURL returns the stable identity of a series derived from its source
// page URL. If the URL is parseable, the last non-empty path segment is used.
// As a last resort the hostname is used. If nothing useful can be extracted the
// result is "unknown".
func SlugFromURL(pageURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || parsed.Hostname() == "" {
		return "unknown"
	}

	// When the URL follows the canonical structure we use the last meaningful
	// path segment as the identity. "https://example.com/series/name" → "name"
	// "https://example.com/series/name/720" → "name"
	if pp := path.Clean(parsed.Path); pp != "" && pp != "/" {
		parts := strings.Split(strings.Trim(pp, "/"), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return parsed.Hostname()
}

func canonicalName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

func humanizeSlug(value string) string {
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(value))
	for i, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

// DeriveSeriesName extracts a human-readable series name from a URL, using
// the same logic the original scanner used.
func DeriveSeriesName(pageURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(pageURL))
	if err == nil {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i, part := range parts {
			if strings.EqualFold(part, "series") && i+1 < len(parts) && parts[i+1] != "" {
				return humanizeSlug(parts[i+1])
			}
		}
		if parsed.Hostname() != "" {
			return parsed.Hostname()
		}
	}
	return "unknown"
}
