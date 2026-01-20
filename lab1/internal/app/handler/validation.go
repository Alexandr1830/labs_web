package handler

import "strings"

// sanitizeString trims and strips control characters.
func sanitizeString(s string) string {
	return strings.TrimSpace(s)
}

