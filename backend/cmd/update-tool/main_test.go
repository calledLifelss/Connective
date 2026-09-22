package main

import "testing"

func TestSplitSpec(t *testing.T) {
	cases := []struct {
		in              string
		typ, file, from string
	}{
		{"full:bundle.zip", "full", "bundle.zip", ""},
		{"delta:patch.zip:0.2.0", "delta", "patch.zip", "0.2.0"},
		{`full:C:\upd\full.zip`, "full", `C:\upd\full.zip`, ""},
		{`delta:C:\upd\delta.zip:0.2.0`, "delta", `C:\upd\delta.zip`, "0.2.0"},
		{"full:C:/upd/full.zip", "full", "C:/upd/full.zip", ""},
		{"/abs/path/full.zip", "", "", ""}, // no type prefix: error tested below
	}
	for _, c := range cases {
		if c.typ == "" {
			if _, _, _, err := splitSpec(c.in); err == nil {
				t.Errorf("splitSpec(%q) should fail", c.in)
			}
			continue
		}
		typ, file, from, err := splitSpec(c.in)
		if err != nil {
			t.Fatalf("splitSpec(%q): %v", c.in, err)
		}
		if typ != c.typ || file != c.file || from != c.from {
			t.Errorf("splitSpec(%q) = %q %q %q, want %q %q %q",
				c.in, typ, file, from, c.typ, c.file, c.from)
		}
	}
}
