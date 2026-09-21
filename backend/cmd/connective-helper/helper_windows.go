package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"connective/backend/internal/firewall"
	"connective/backend/internal/platform"
)

// Windows privilege model: this binary performs only the actions below
// and is launched elevated through a UAC prompt (Start-Process -Verb
// RunAs) for the exact validated argv. No generic command execution.
//
//	connective-helper.exe run-core -- <sing-box.exe> <args...>
//	connective-helper.exe killswitch-on --tun connective0 --allow 1.2.3.4:443 [--allow ...] --state <file>
//	connective-helper.exe killswitch-off --state <file>
//	connective-helper.exe tun-cleanup --if connective0
//	connective-helper.exe stop-core --pid 1234
//
// Every subcommand is idempotent. Timeouts are bounded per external
// tool call (platform runner, 20s default).

var winRunner platform.Runner = platform.OsRunner{}

// runCore launches the core binary with inherited stdio and waits,
// forwarding its exit code. (Linux uses syscall.Exec; Windows has no
// equivalent in stdlib, so the helper supervises one extra parent
// process. The daemon tracks the core by PID either way.)
func runCore(args []string) error {
	dash := -1
	for i, a := range args {
		if a == "--" {
			dash = i
			break
		}
	}
	argv := args
	if dash >= 0 {
		argv = args[dash+1:]
	}
	if len(argv) == 0 {
		return fmt.Errorf("run-core: no core binary given")
	}
	if err := platform.TrustedBinary(argv[0]); err != nil {
		return fmt.Errorf("run-core: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Start (not Run): startup is async; readiness is the daemon's job
	// via the core manager, same as Linux.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("run-core: %w", err)
	}
	// Detach: the daemon supervises by PID (stop-core when needed).
	return cmd.Process.Release()
}

// killswitch delegates to the firewall package (single source of truth
// for the owned rule model). CLI parsing stays here so the helper's
// auditable surface is explicit: --tun kept for parity with Linux,
// --state snapshots/restores the previous policy.
func killswitch(on bool, args []string) error {
	var tun, state string
	var allows []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tun":
			if i+1 >= len(args) {
				return fmt.Errorf("killswitch-on: --tun needs a value")
			}
			i++
			tun = args[i]
		case "--allow":
			if i+1 >= len(args) {
				return fmt.Errorf("killswitch-on: --allow needs ip:port")
			}
			i++
			allows = append(allows, args[i])
		case "--state":
			if i+1 >= len(args) {
				return fmt.Errorf("killswitch: --state needs a file")
			}
			i++
			state = args[i]
		default:
			return fmt.Errorf("killswitch: unknown flag %q", args[i])
		}
	}
	_ = tun // endpoint rules cover the tunnel; kept for CLI parity
	mgr := &firewall.NetshManager{Runner: winRunner, Allows: allows}
	if !on {
		return killswitchOff(mgr, state)
	}
	if state == "" {
		return fmt.Errorf("killswitch-on: --state is required")
	}
	prev, err := firewall.ReadPolicy(winRunner)
	if err != nil {
		return fmt.Errorf("killswitch on: read policy: %w", err)
	}
	if err := os.WriteFile(state, []byte(prev), 0o600); err != nil {
		return fmt.Errorf("killswitch on: snapshot policy: %w", err)
	}
	if err := mgr.Enable(); err != nil {
		return fmt.Errorf("killswitch on: %w", err)
	}
	return nil
}

func killswitchOff(mgr *firewall.NetshManager, state string) error {
	if state != "" {
		if raw, err := os.ReadFile(state); err == nil {
			if p := strings.TrimSpace(string(raw)); p != "" {
				mgr.RestorePolicy = p
			}
		}
	}
	// Absent/foreign state still converges: owned rules delete first;
	// report policy errors but do not wedge callers.
	if err := mgr.Disable(); err != nil {
		return fmt.Errorf("killswitch off: %w", err)
	}
	return nil
}

// tunCleanup verifies Connective's own interface state and removes
// Connective-owned stale routes. SAFETY: only OurTunName may be named;
// anything else is refused. wintun interfaces live and die with the
// core process (driver unloads on last handle close), so unlike Linux
// there is usually nothing to delete: a present-but-unowned interface
// is disabled, never force-removed, and foreign state is untouched.
func tunCleanup(args []string) error {
	var name string
	for i := 0; i < len(args); i++ {
		if args[i] == "--if" && i+1 < len(args) {
			name = args[i+1]
			i++
		}
	}
	if name == "" {
		return fmt.Errorf("tun-cleanup: --if is required")
	}
	if !strings.EqualFold(name, platform.OwnTunName) {
		return fmt.Errorf("tun-cleanup: refusing to touch foreign interface %q (only %s is managed)", name, platform.OwnTunName)
	}
	ctx := context.Background()
	out, err := winRunner.Run(ctx, "netsh", "interface", "show", "interface", "name="+strconvQuote(name))
	if err != nil || strings.Contains(out, "There is no such interface") ||
		strings.Contains(out, "The following command was not found") {
		return nil // already gone: success
	}
	// Present: disable so it cannot carry traffic outside supervision.
	if out2, err := winRunner.Run(ctx, "netsh", "interface", "set", "interface", "name="+strconvQuote(name), "admin=disable"); err != nil {
		return fmt.Errorf("tun-cleanup: disable %q: %s: %w", name, oneLine(out2), err)
	}
	return nil
}

// stopCore terminates an elevated core the unprivileged daemon cannot
// touch. Already-gone PID = success. SAFETY mirrors Linux: only a
// process whose image/cmdline still looks like our sing-box core may
// be killed (tasklist + wmic/CIM verification before taskkill), PID
// reuse must never hit an innocent process, and the helper itself is
// never a target.
func stopCore(args []string) error {
	var pidStr string
	for i := 0; i < len(args); i++ {
		if args[i] == "--pid" && i+1 < len(args) {
			pidStr = args[i+1]
			i++
		}
	}
	if pidStr == "" {
		return fmt.Errorf("stop-core: --pid is required")
	}
	var pid int
	if _, err := fmt.Sscanf(pidStr, "%d", &pid); err != nil || pid <= 4 || fmt.Sprint(pid) != pidStr {
		return fmt.Errorf("stop-core: bad --pid %q", pidStr)
	}
	if pid == os.Getpid() {
		return fmt.Errorf("stop-core: refusing to kill the helper itself")
	}
	if gone(pid) {
		return nil
	}
	ok, err := looksLikeCore(pid)
	if err != nil {
		return fmt.Errorf("stop-core: verify pid %d: %w", pid, err)
	}
	if !ok {
		return fmt.Errorf("stop-core: pid %d does not look like a connective core, refusing", pid)
	}
	ctx := context.Background()
	if out, err := winRunner.Run(ctx, "taskkill", "/PID", pidStr); err != nil {
		if gone(pid) {
			return nil
		}
		return fmt.Errorf("stop-core: taskkill: %s: %w", oneLine(out), err)
	}
	if waitGone(pid, 5*time.Second) {
		return nil
	}
	if out, err := winRunner.Run(ctx, "taskkill", "/F", "/PID", pidStr); err != nil {
		if gone(pid) {
			return nil
		}
		return fmt.Errorf("stop-core: taskkill /F: %s: %w", oneLine(out), err)
	}
	if waitGone(pid, 3*time.Second) {
		return nil
	}
	return fmt.Errorf("stop-core: pid %d would not die", pid)
}

// looksLikeCore verifies the PID's image/cmdline still references our
// core before signaling. wmic first (fast, local), CIM fallback.
func looksLikeCore(pid int) (bool, error) {
	ctx := context.Background()
	id := fmt.Sprint(pid)
	if out, err := winRunner.Run(ctx, "wmic", "process", "where", "ProcessId="+id, "get", "ExecutablePath,CommandLine", "/format:csv"); err == nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "sing-box") || strings.Contains(lower, "core.json") || strings.Contains(lower, "connective") {
			return true, nil
		}
		return false, nil
	}
	out, err := winRunner.Run(ctx, "powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_Process -Filter 'ProcessId="+id+"').CommandLine")
	if err != nil {
		return false, fmt.Errorf("identify pid %d: %w", pid, err)
	}
	lower := strings.ToLower(out)
	return strings.Contains(lower, "sing-box") || strings.Contains(lower, "core.json") || strings.Contains(lower, "connective"), nil
}

func strconvQuote(s string) string { return "\"" + s + "\"" }

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
