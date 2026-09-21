package platform

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Runner executes OS tools (netsh, route, powershell, tasklist, …).
// The OS implementation keeps all parsing honest; FakeRunner gives unit
// tests deterministic fixtures without touching the machine.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// OsRunner runs real processes with a bounded timeout.
type OsRunner struct{ Timeout time.Duration }

func (r OsRunner) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return 20 * time.Second
}

// Run executes name with args and returns combined output.
func (r OsRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	cmd := exec.CommandContext(tctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("%s %v: %s: %w", name, args, buf.String(), err)
	}
	return buf.String(), nil
}

// DefaultRunner is the process-wide OS runner.
var DefaultRunner Runner = OsRunner{}

// FakeRunner replays scripted outputs for tests. Unmatched commands
// fail loudly so tests never silently pass on missing fixtures.
type FakeRunner struct {
	Outputs map[string]fakeResult
	Calls   []string
}

type fakeResult struct {
	Out string
	Err error
}

// On scripts `name + " " + args...` → output.
func (f *FakeRunner) On(cmd, out string, err error) {
	if f.Outputs == nil {
		f.Outputs = map[string]fakeResult{}
	}
	f.Outputs[cmd] = fakeResult{Out: out, Err: err}
}

// Run implements Runner.
func (f *FakeRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	f.Calls = append(f.Calls, key)
	if r, ok := f.Outputs[key]; ok {
		return r.Out, r.Err
	}
	return "", fmt.Errorf("fake runner: unexpected command %q", key)
}
