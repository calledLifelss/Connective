//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// runCore replaces the helper with the core binary (exact argv, no shell),
// so signals and exit codes propagate and no extra process lingers.
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
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("run-core: %w", err)
	}
	armDeathSig()
	return syscall.Exec(bin, argv, os.Environ())
}

// killswitch manages table inet connective. ON: default-reject egress
// with narrow allows (loopback, LAN, established, TUN device, explicit
// VPN server endpoints). OFF: delete the table (absent = success).
func killswitch(on bool, args []string) error {
	nft, err := exec.LookPath("nft")
	if err != nil {
		return fmt.Errorf("killswitch: nft not found: %w", err)
	}
	if !on {
		out, err := exec.Command(nft, "delete", "table", "inet", "connective").CombinedOutput()
		if err != nil && !strings.Contains(string(out), "No such file") {
			return fmt.Errorf("killswitch off: %s: %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	var tun string
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
			// Accepted for CLI parity with Windows (which snapshots
			// the previous firewall policy there). nftables needs no
			// snapshot: OFF deletes the whole table.
			if i+1 >= len(args) {
				return fmt.Errorf("killswitch-on: --state needs a value")
			}
			i++
		default:
			return fmt.Errorf("killswitch-on: unknown flag %q", args[i])
		}
	}
	if tun == "" {
		return fmt.Errorf("killswitch-on: --tun is required")
	}
	var rules []string
	for _, a := range allows {
		ip, port, ok := splitIPPort(a)
		if !ok {
			return fmt.Errorf("killswitch-on: bad --allow %q (want ip:port)", a)
		}
		rules = append(rules, fmt.Sprintf(
			"add rule inet connective output ip daddr %s tcp dport %s ct state new accept", ip, port))
		rules = append(rules, fmt.Sprintf(
			"add rule inet connective output ip daddr %s udp dport %s accept", ip, port))
	}
	script := []string{
		"add table inet connective",
		"add chain inet connective output { type filter hook output priority 0; policy accept; }",
		"add rule inet connective output ct state established,related accept",
		"add rule inet connective output oifname \"lo\" accept",
		"add rule inet connective output ip daddr { 127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } accept",
		"add rule inet connective output ip6 daddr { ::1, fe80::/10 } accept",
		fmt.Sprintf("add rule inet connective output oifname \"%s\" accept", tun),
	}
	script = append(script, rules...)
	script = append(script,
		"add rule inet connective output udp dport 67 accept", // DHCP stays alive
		"add rule inet connective output reject",
	)
	// Two-step apply: drop any previous table (ignoring absence), then
	// build fresh. `nft -f` aborts on first error, so delete and create
	// must not share a batch.
	_ = exec.Command(nft, "delete", "table", "inet", "connective").Run()
	cmd := exec.Command(nft, "-f", "-")
	cmd.Stdin = strings.NewReader(strings.Join(script, "\n") + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("killswitch on: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// tunCleanup deletes a stale TUN device (missing device = success).
// SAFETY: only Connective's own device may ever be deleted. Any other
// name — including another VPN's TUN — is refused so a caller bug can
// never destroy foreign networking state.
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
	if name != "connective0" {
		return fmt.Errorf("tun-cleanup: refusing to touch foreign interface %q (only connective0 is managed)", name)
	}
	ip, err := exec.LookPath("ip")
	if err != nil {
		return fmt.Errorf("tun-cleanup: ip not found: %w", err)
	}
	out, err := exec.Command(ip, "link", "del", name).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "Cannot find device") {
		return fmt.Errorf("tun-cleanup: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// stopCore terminates an elevated core the unprivileged daemon cannot
// signal itself (root-owned child: daemon signals fail with EPERM and
// core.Manager.Stop would otherwise wait forever). Already-gone PID =
// success. SAFETY: only a process that still looks like our sing-box
// core may be killed — PID reuse must never let us shoot an innocent
// process, so the cmdline is verified first.
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
	if _, err := fmt.Sscanf(pidStr, "%d", &pid); err != nil || pid <= 1 || fmt.Sprint(pid) != pidStr {
		return fmt.Errorf("stop-core: bad --pid %q", pidStr)
	}
	if pid == os.Getpid() {
		return fmt.Errorf("stop-core: refusing to kill the helper itself")
	}
	if gone(pid) {
		return nil
	}
	if !looksLikeCore(pid) {
		return fmt.Errorf("stop-core: pid %d does not look like a connective core, refusing", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("stop-core: %w", err)
	}
	_ = proc.Signal(syscall.SIGTERM)
	if waitGone(pid, 5*time.Second) {
		return nil
	}
	_ = proc.Signal(syscall.SIGKILL)
	if waitGone(pid, 3*time.Second) {
		return nil
	}
	return fmt.Errorf("stop-core: pid %d would not die", pid)
}

// looksLikeCore verifies pid's cmdline still references sing-box or the
// connective core config before we may signal it (PID-reuse guard).
func looksLikeCore(pid int) bool {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	cmd := string(raw)
	return strings.Contains(cmd, "sing-box") || strings.Contains(cmd, "core.json")
}
