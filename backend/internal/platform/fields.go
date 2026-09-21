package platform

import "strings"

// splitFields splits on spaces (CONNECTIVE_HELPER_RUNNER override).
func splitFields(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
