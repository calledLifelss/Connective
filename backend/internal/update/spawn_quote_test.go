package update

import "testing"

// parseWinArgs splits a Windows command line's argument portion using
// the CommandLineToArgvW algorithm (backslash-escape + double-quote
// rules) — the same rules the child applies to its argv.
func parseWinArgs(s string) []string {
	var args []string
	var cur []byte
	inQuotes := false
	started := false // token seen (even an empty quoted one)
	slashes := 0
	flush := func() {
		args = append(args, string(cur))
		cur = cur[:0]
		started = false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			slashes++
			continue
		case '"':
			started = true
			cur = append(cur, bytesRepeat('\\', slashes/2)...)
			if slashes%2 == 1 {
				cur = append(cur, '"')
			} else if inQuotes && i+1 < len(s) && s[i+1] == '"' {
				cur = append(cur, '"')
				i++
			} else {
				inQuotes = !inQuotes
			}
			slashes = 0
			continue
		default:
			if n := slashes; n > 0 {
				cur = append(cur, bytesRepeat('\\', n)...)
				started = true
			}
			slashes = 0
		}
		if !inQuotes && (c == ' ' || c == '\t') {
			if started {
				flush()
			}
			continue
		}
		started = true
		cur = append(cur, c)
	}
	if n := slashes; n > 0 {
		cur = append(cur, bytesRepeat('\\', n)...)
		started = true
	}
	if started || inQuotes {
		flush()
	}
	return args
}

func bytesRepeat(b byte, n int) []byte {
	if n <= 0 {
		return nil
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

// The default Windows install root contains a space; if it is not
// quoted correctly the updater sees --install-root C:\Program and dies
// during flag parsing, writing no result.json (a silently skipped
// update). This is the round-trip the production argv must survive.
func TestWinQuoteRoundTrip(t *testing.T) {
	cases := [][]string{
		{"apply", "--install-root", `C:\Program Files\Connective`, "--version", "0.5.2"},
		{"--result-file", `C:\Users\John Doe\AppData\Local\connective\updates\result.json`},
		{"--staged-dir", `C:\dir with "quotes"\tree`},
		{`trailing\`, "--wait-timeout", "15m0s"},
		{`back\\slashes`, `end\"quote`},
		{"plain", "--flag", "value"},
		{""},
	}
	for _, args := range cases {
		joined := ""
		for i, a := range args {
			if i > 0 {
				joined += " "
			}
			joined += winQuote(a)
		}
		got := parseWinArgs(joined)
		if len(got) != len(args) {
			t.Fatalf("round trip %q → %d args, want %d (%q)", args, len(got), len(args), joined)
		}
		for i := range args {
			if got[i] != args[i] {
				t.Errorf("round trip arg %d = %q, want %q (cmdline %q)", i, got[i], args[i], joined)
			}
		}
	}
}

// A path without spaces or quotes must stay bare (minimal quoting
// keeps the command line readable in logs).
func TestWinQuoteLeavesSafeArgsBare(t *testing.T) {
	if got := winQuote("apply"); got != "apply" {
		t.Fatalf("winQuote(apply) = %q", got)
	}
	if got := winQuote(`C:\opt\connective`); got != `C:\opt\connective` {
		t.Fatalf("winQuote path = %q", got)
	}
}

// psQuote must be a faithful PowerShell single-quoted literal.
func TestPsQuoteRoundTrip(t *testing.T) {
	for _, s := range []string{"apply", `C:\Program Files\Connective`, "it's"} {
		q := psQuote(s)
		if len(q) < 2 || q[0] != '\'' || q[len(q)-1] != '\'' {
			t.Fatalf("psQuote(%q) = %q, not quoted", s, q)
		}
		inner := q[1 : len(q)-1]
		unquoted := ""
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\'' {
				if i+1 < len(inner) && inner[i+1] == '\'' {
					unquoted += "'"
					i++
					continue
				}
			}
			unquoted += string(inner[i])
		}
		if unquoted != s {
			t.Errorf("psQuote round trip = %q, want %q", unquoted, s)
		}
	}
}
