// Package update implements Connective's self-update foundation:
// version model, signed manifests, provider abstraction, download,
// verification, staging, installation and rollback. The UpdateManager
// never knows how GitHub works; see provider.go.
package update

import (
	"fmt"
	"strconv"
	"strings"
)

// CurrentVersion is the running application's version. Release tooling
// must keep this in lockstep with the Flutter pubspec and RPM spec.
const CurrentVersion = "0.5.1"

// Channels supported by update queries. The UI exposes the setting;
// the backend already filters on it.
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
	ChannelDev    = "dev"
)

// ValidChannel reports whether c is a known update channel.
func ValidChannel(c string) bool {
	switch c {
	case ChannelStable, ChannelBeta, ChannelDev:
		return true
	default:
		return false
	}
}

// Version is a parsed dotted version: 0.2.10, never string-compared.
type Version struct {
	Major int
	Minor int
	Patch int
	Pre   string // optional prerelease tag ("beta.1"); empty = final
}

// ParseVersion parses "M[.m[.p]][-pre]" ("0.2", "0.2.1", "1.0.0-beta.1").
// Empty components default to 0; anything non-numeric is an error.
func ParseVersion(s string) (Version, error) {
	var v Version
	s = strings.TrimSpace(s)
	if s == "" {
		return v, fmt.Errorf("update: empty version")
	}
	if i := strings.Index(s, "-"); i >= 0 {
		v.Pre = s[i+1:]
		s = s[:i]
		if v.Pre == "" {
			return v, fmt.Errorf("update: bad version %q", s)
		}
	}
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return v, fmt.Errorf("update: bad version %q", s)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		if p == "" {
			return v, fmt.Errorf("update: bad version %q", s)
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, fmt.Errorf("update: bad version %q", s)
		}
		nums[i] = n
	}
	v.Major, v.Minor, v.Patch = nums[0], nums[1], nums[2]
	return v, nil
}

// Compare returns -1/0/+1 of v against o, numerically per component. A
// final release outranks any prerelease of the same core.
func (v Version) Compare(o Version) int {
	if v.Major != o.Major {
		return cmpInt(v.Major, o.Major)
	}
	if v.Minor != o.Minor {
		return cmpInt(v.Minor, o.Minor)
	}
	if v.Patch != o.Patch {
		return cmpInt(v.Patch, o.Patch)
	}
	switch {
	case v.Pre == o.Pre:
		return 0
	case v.Pre == "":
		return 1
	case o.Pre == "":
		return -1
	default:
		return strings.Compare(v.Pre, o.Pre)
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// String renders the canonical form.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// IsNewerThan reports whether the candidate is an upgrade over current.
func IsNewerThan(current, candidate string) (bool, error) {
	c, err := ParseVersion(current)
	if err != nil {
		return false, err
	}
	n, err := ParseVersion(candidate)
	if err != nil {
		return false, err
	}
	return n.Compare(c) > 0, nil
}

// Acceptable enforces the safety policy: the target must parse, must be
// strictly newer (no downgrade, no reinstall), and must satisfy the
// manifest's minimum supported version.
func Acceptable(current, target, minimum string) error {
	c, err := ParseVersion(current)
	if err != nil {
		return fmt.Errorf("update: bad current version: %w", err)
	}
	t, err := ParseVersion(target)
	if err != nil {
		return fmt.Errorf("update: bad target version: %w", err)
	}
	if t.Compare(c) <= 0 {
		return fmt.Errorf("update: %s is not newer than %s",
			t.String(), c.String())
	}
	if minimum != "" {
		m, err := ParseVersion(minimum)
		if err != nil {
			return fmt.Errorf("update: bad minimum version: %w", err)
		}
		if c.Compare(m) < 0 {
			return fmt.Errorf("update: current %s below minimum %s",
				c.String(), m.String())
		}
	}
	return nil
}
