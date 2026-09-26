package update

import "strings"

// psQuote single-quotes one PowerShell argument (a PowerShell string
// literal: embedded quotes are doubled, nothing else is escaped).
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// winQuote quotes one argument for a Windows command line using
// CommandLineToArgvW rules: double quotes wrap it, backslashes that
// precede a quote (or the closing quote) are doubled, everything else
// is literal. Unquoted is used only when it is provably safe (no
// whitespace or quote).
//
// This matters because Start-Process -ArgumentList joins array
// elements with spaces and does NOT re-quote them: a bare
// C:\Program Files\Connective would arrive as two arguments and the
// updater would never see --version (it parsed nothing and exited
// before writing result.json, so the update silently did nothing).
func winQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	var slashes int
	flush := func(n int) { b.WriteString(strings.Repeat(`\`, n)) }
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			slashes++
		case '"':
			flush(slashes*2 + 1)
			b.WriteByte('"')
			slashes = 0
		default:
			flush(slashes)
			slashes = 0
			b.WriteByte(s[i])
		}
	}
	flush(slashes * 2)
	b.WriteByte('"')
	return b.String()
}
