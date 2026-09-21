package update

import "testing"

func TestParseCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.0", "0.2.0", 0},
		{"0.2.0", "0.2.1", -1},
		{"0.2.1", "0.2.0", 1},
		// Numeric, never lexicographic: 10 > 9.
		{"0.2.1", "0.2.10", -1},
		{"0.2.10", "0.2.9", 1},
		{"0.10.0", "0.9.9", 1},
		{"1.0.0", "0.99.99", 1},
		{"0.2", "0.2.0", 0},
		{"1.0.0-beta.1", "1.0.0", -1},
	}
	for _, c := range cases {
		a, err := ParseVersion(c.a)
		if err != nil {
			t.Fatalf("parse %q: %v", c.a, err)
		}
		b, err := ParseVersion(c.b)
		if err != nil {
			t.Fatalf("parse %q: %v", c.b, err)
		}
		if got := a.Compare(b); got != c.want {
			t.Errorf("compare %q vs %q = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseMalformed(t *testing.T) {
	for _, s := range []string{"", "x.y", "1.2.3.4", "1.-2.3", "1..3", "  ", "v1.2.3"} {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("ParseVersion(%q) should fail", s)
		}
	}
}

func TestAcceptable(t *testing.T) {
	if err := Acceptable("0.2.0", "0.2.1", "0.2.0"); err != nil {
		t.Fatalf("upgrade should be acceptable: %v", err)
	}
	// Same version: not an upgrade.
	if err := Acceptable("0.2.1", "0.2.1", ""); err == nil {
		t.Error("same version must be rejected")
	}
	// Downgrade: never.
	if err := Acceptable("0.2.1", "0.2.0", ""); err == nil {
		t.Error("downgrade must be rejected")
	}
	// Below minimum: not eligible.
	if err := Acceptable("0.1.9", "0.2.1", "0.2.0"); err == nil {
		t.Error("below-minimum current must be rejected")
	}
	// Bad minimum is an error, not a silent pass.
	if err := Acceptable("0.2.0", "0.2.1", "bogus"); err == nil {
		t.Error("bad minimum must be rejected")
	}
}
