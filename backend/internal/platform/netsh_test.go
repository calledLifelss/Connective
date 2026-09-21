package platform

import "testing"

func TestParseNetshInterfaceLine(t *testing.T) {
	state, name := parseNetshInterfaceLine("Enabled        Connected      Dedicated        Ethernet")
	if state != "connected" || name != "Ethernet" {
		t.Fatalf("got %q/%q", state, name)
	}
	state, name = parseNetshInterfaceLine("Enabled        Disconnected   Dedicated        Wi-Fi")
	if state != "disconnected" || name != "Wi-Fi" {
		t.Fatalf("got %q/%q", state, name)
	}
	_, name = parseNetshInterfaceLine("Enabled        Connected      Dedicated        Local Area Connection* 2")
	if name != "Local Area Connection* 2" {
		t.Fatalf("multiword name: got %q", name)
	}
	if s, n := parseNetshInterfaceLine("Admin State  State  Type  Interface"); s != "" || n != "" {
		t.Fatalf("header must not parse: %q/%q", s, n)
	}
	if s, n := parseNetshInterfaceLine(""); s != "" || n != "" {
		t.Fatalf("empty must not parse")
	}
}
