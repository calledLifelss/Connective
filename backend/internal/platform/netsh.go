package platform

import "strings"

// parseNetshInterfaceLine parses one `netsh interface show interface`
// row: "Enabled  Connected  Dedicated  <name>". Portable pure logic;
// the netsh invocation itself stays in tuncheck_windows.go.
func parseNetshInterfaceLine(line string) (state, name string) {
	f := strings.Fields(line)
	if len(f) < 4 {
		return "", ""
	}
	if !strings.EqualFold(f[0], "Enabled") && !strings.EqualFold(f[0], "Disabled") {
		return "", ""
	}
	state = strings.ToLower(f[1])
	name = strings.Join(f[3:], " ")
	return state, name
}
