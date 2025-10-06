package util

import (
	"strings"
)

// HasPrefixFold tests whether the string s begins with prefix,
// without regard to case.
func HasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}
