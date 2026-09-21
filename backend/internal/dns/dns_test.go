package dns

import "testing"

func TestResolvConfHashStable(t *testing.T) {
	a, err := ResolvConfHash()
	if err != nil {
		t.Skipf("no resolv.conf: %v", err)
	}
	b, err := ResolvConfHash()
	if err != nil || a != b || len(a) != 64 {
		t.Fatalf("hash unstable: %q %q %v", a, b, err)
	}
}
