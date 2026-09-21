// Package core supervises the sing-box child process: start, stop,
// restart, startup timeout, unexpected-exit detection, log capture and
// guaranteed cleanup. No orphan processes are left behind: Stop always
// SIGTERMs and, after a grace period, SIGKILLs and reaps the child.
package core

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// Status describes the supervised process.
type Status string

const (
	StatusStopped Status = "stopped"
	StatusRunning Status = "running"
	StatusFailed  Status = "failed"
)

// Config tunes one managed core process.
type Config struct {
	Binary          string        // core executable (sing-box)
	Args            []string      // e.g. {"run", "-c", configPath}
	StartupTimeout  time.Duration // max wait for Ready
	ShutdownTimeout time.Duration // grace between SIGTERM and SIGKILL
	Ready           func() bool   // readiness probe, polled until true
	OnLog           func(line string)
}

// Manager supervises at most one core process at a time.
type Manager struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan error
	status Status
	Config Config
}

// New creates a Manager.
func New(cfg Config) *Manager {
	if cfg.StartupTimeout <= 0 {
		cfg.StartupTimeout = 15 * time.Second
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}
	return &Manager{Config: cfg, status: StatusStopped}
}

// Running reports whether the child is up.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.status == StatusRunning
}

// Status returns the current status.
func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
}

// Start launches the core and waits for readiness. A second Start while
// running is a no-op returning nil (duplicate prevention).
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.cmd != nil && m.status == StatusRunning {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.Config.Binary, m.Config.Args...)
	if attr := deathSignal(); attr != nil {
		cmd.SysProcAttr = attr
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("core: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("core: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("core start %q: %w", m.Config.Binary, err)
	}

	m.mu.Lock()
	m.cmd, m.cancel, m.status = cmd, cancel, StatusRunning
	m.done = make(chan error, 1)
	m.mu.Unlock()

	go drainLogs(stdout, m.Config.OnLog)
	go drainLogs(stderr, m.Config.OnLog)
	go func() {
		m.done <- cmd.Wait()
		m.mu.Lock()
		if m.status == StatusRunning {
			m.status = StatusFailed
		}
		m.mu.Unlock()
	}()

	deadline := time.Now().Add(m.Config.StartupTimeout)
	for {
		if m.Config.Ready == nil || m.Config.Ready() {
			return nil
		}
		select {
		case err := <-m.done:
			m.kill()
			return fmt.Errorf("core exited during startup: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			m.kill()
			return fmt.Errorf("core failed to become ready within %s", m.Config.StartupTimeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Stop terminates the child gracefully, then forcefully. It succeeds even
// when nothing is running (idempotent cleanup). It NEVER blocks forever:
// an elevated core (root, started via the helper) cannot be signaled by
// the unprivileged daemon, so after a bounded grace the caller proceeds
// and finishes privileged cleanup via the helper instead of wedging.
func (m *Manager) Stop() error {
	m.mu.Lock()
	cmd, cancel := m.cmd, m.cancel
	m.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(terminatedSignal())
	select {
	case <-m.done:
	case <-time.After(m.Config.ShutdownTimeout):
		_ = cmd.Process.Kill()
		select {
		case <-m.done:
		case <-time.After(5 * time.Second):
			// Unreaped (e.g. EPERM on a root-owned core): the
			// daemon-level stopCore() follows up via the
			// privileged helper. Hanging here wedged disconnect
			// forever with the UI stuck on "Disconnecting...".
		}
	}
	cancel()
	m.mu.Lock()
	m.cmd, m.cancel, m.status = nil, nil, StatusStopped
	m.mu.Unlock()
	return nil
}

// Pid returns the supervised child PID, or 0 when nothing is running.
// Used by the daemon to finish an unprivileged-unreachable (elevated)
// core via the helper.
func (m *Manager) Pid() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd == nil || m.cmd.Process == nil {
		return 0
	}
	return m.cmd.Process.Pid
}

// Restart is Stop followed by Start.
func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
}

func (m *Manager) kill() {
	m.mu.Lock()
	cmd, cancel := m.cmd, m.cancel
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if cancel != nil {
		cancel()
	}
	select {
	case <-m.done:
	case <-time.After(2 * time.Second):
	}
	m.mu.Lock()
	m.cmd, m.cancel, m.status = nil, nil, StatusFailed
	m.mu.Unlock()
}

func drainLogs(r io.Reader, onLog func(string)) {
	if onLog == nil {
		io.Copy(io.Discard, r)
		return
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	for sc.Scan() {
		onLog(sc.Text())
	}
}
