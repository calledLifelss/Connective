package platform

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// errUnreachable marks a PID that is alive. Windows has no signal
// probing; liveness comes from tasklist. Anything surviving Stop needs
// the elevated helper (taskkill), same division as unix.
var errUnreachable = errors.New("process alive but not signalable")

// ErrUnreachable reports whether ProcessGone found a live process.
func ErrUnreachable(err error) bool { return err == errUnreachable }

// ProcessGone probes pid via tasklist: nil when reaped. Only stdlib
// process tools are used (no x/sys dependency); output parsing is
// strict so a confused parser fails safe (reports alive).
func ProcessGone(pid int) error {
	out, err := DefaultRunner.Run(context.Background(),
		"tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH")
	if err != nil {
		// tasklist itself failing must not read as "gone": fail safe.
		return errUnreachable
	}
	if strings.Contains(out, "INFO: No tasks") {
		return nil
	}
	// Any listed row means a live process with that PID. PID reuse is
	// guarded by the callers' image-name checks (helper stop-core).
	if strings.Contains(out, strconv.Itoa(pid)) {
		return errUnreachable
	}
	return errUnreachable
}
