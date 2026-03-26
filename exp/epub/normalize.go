package epub

import (
	"regexp"
	"strings"
)

var articlePrefixRe = regexp.MustCompile(`(?i)^(the|a|an)\s+`)

// normalizeForSort strips leading articles and lowercases the string
// for use in sort-friendly metadata fields.
func normalizeForSort(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	return articlePrefixRe.ReplaceAllString(lower, "")
}
