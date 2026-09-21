//go:build !windows

package dns

// ResolverSnapshot is the portable resolver-state hash: resolv.conf
// on unix, adapter DNS table on Windows (see dns_windows.go).
func ResolverSnapshot() (string, error) { return ResolvConfHash() }
