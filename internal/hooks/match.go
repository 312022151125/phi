package hooks

import (
	"regexp"
	"strings"
)

// matchesPattern implements matcher semantics (empty/*, exact|pipe, regex).
func matchesPattern(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return true
	}
	if isSimplePattern(pattern) {
		for part := range strings.SplitSeq(pattern, "|") {
			if part == value {
				return true
			}
		}
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

func isSimplePattern(s string) bool {
	for _, r := range s {
		if r == '|' {
			continue
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
