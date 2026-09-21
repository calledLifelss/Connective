package dns

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"connective/backend/internal/platform"
)

// ResolverSnapshot hashes the effective Windows resolver configuration
// so the daemon can prove it neither changed system DNS on connect nor
// left modifications behind on disconnect. Same role as ResolvConfHash
// on Linux: `netsh interface ip show dnsservers` output, normalized
// (sorted, blank-insensitive) so adapter enumeration order never fakes
// a change.
func ResolverSnapshot() (string, error) {
	out, err := platform.DefaultRunner.Run(context.Background(),
		"netsh", "interface", "ip", "show", "dnsservers")
	if err != nil {
		return "", fmt.Errorf("dns: snapshot: %w", err)
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			lines = append(lines, t)
		}
	}
	sort.Strings(lines)
	return HashBytes([]byte(strings.Join(lines, "\n"))), nil
}
