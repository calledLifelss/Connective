// Command connective-helper performs the small set of privileged
// operations the unprivileged daemon may never do itself: running the
// core with TUN (needs CAP_NET_ADMIN), managing the nftables kill-switch
// table, and cleaning stray TUN devices.
//
// Invocation is always explicit and auditable, one action per process:
//
//	connective-helper run-core -- <sing-box> <args...>
//	connective-helper killswitch-on --tun connective0 --allow 1.2.3.4:443 [--allow ...]
//	connective-helper killswitch-off
//	connective-helper tun-cleanup --if connective0
//	connective-helper stop-core --pid 1234
//
// The daemon launches it via pkexec (interactive polkit auth, one prompt
// per connect) or directly when already root (tests). Every subcommand is
// idempotent: re-running and cleaning a clean state both succeed.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"connective/backend/internal/platform"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "connective-helper:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: connective-helper <run-core|killswitch-on|killswitch-off|tun-cleanup|stop-core> ...")
	}
	switch args[0] {
	case "run-core":
		return runCore(args[1:])
	case "killswitch-on":
		return killswitch(true, args[1:])
	case "killswitch-off":
		return killswitch(false, args[1:])
	case "tun-cleanup":
		return tunCleanup(args[1:])
	case "stop-core":
		return stopCore(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// gone reports whether pid no longer exists (platform probe; any
// probe error fails safe to "alive" so we never skip a needed kill).
func gone(pid int) bool {
	return platform.ProcessGone(pid) == nil
}

func splitIPPort(a string) (ip, port string, ok bool) {
	i := strings.LastIndex(a, ":")
	if i <= 0 || i == len(a)-1 {
		return "", "", false
	}
	return a[:i], a[i+1:], true
}

// waitGone polls until pid disappears or the timeout elapses.
func waitGone(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if gone(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}
