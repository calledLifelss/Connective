// Command connectived is the Connective backend daemon: it owns the
// connection state machine, subscriptions, servers, core supervision,
// health monitoring, failover and the IPC endpoint the Flutter UI talks
// to. The UI and daemon run unprivileged; TUN/firewall syscalls execute
// in connective-helper via pkexec (one auth prompt per connect).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"connective/backend/internal/configgen"
	"connective/backend/internal/connection"
	"connective/backend/internal/core"
	"connective/backend/internal/dns"
	"connective/backend/internal/ipc"
	"connective/backend/internal/logging"
	"connective/backend/internal/monitor"
	"connective/backend/internal/persistence"
	"connective/backend/internal/platform"
	"connective/backend/internal/routing"
	"connective/backend/internal/selector"
	"connective/backend/internal/servers"
	"connective/backend/internal/settings"
	"connective/backend/internal/stats"
	"connective/backend/internal/subscriptions"
	"connective/backend/internal/tester"
	"connective/backend/internal/update"
)

// selection persists AUTO vs manual choice.
type selection struct {
	Auto     bool   `json:"auto"`
	ServerID string `json:"serverId,omitempty"`
}

// Daemon aggregates backend state. connMu serializes connect/disconnect
// so duplicate simultaneous connections are impossible.
type Daemon struct {
	log     *logging.Logger
	store   *persistence.Store
	dataDir string
	machine *connection.Machine

	mu         sync.Mutex
	settings   settings.Settings
	subs       []*subscriptions.Subscription
	serverList []*servers.Server
	sel        selection
	upd        *update.Manager

	connMu  sync.Mutex
	coreMgr *core.Manager
	clash   *stats.Client
	tracker *stats.Tracker
	// Monitors are generational: stopMonitors never blocks the caller
	// (a health tick may itself trigger the recovery that stops them;
	// waiting would self-deadlock). Stale loops observe a closed stop
	// channel or a superseded generation and exit quietly.
	monMu       sync.Mutex
	mons        *monSet
	monGen      int
	sessionUp   int64
	sessionDown int64
	connectAt   time.Time
	failCount   int
	flapCount   int
	resolvHash  string
	// lastRecovery gates network-change reconnects (cooldown).
	lastRecovery time.Time
	// lastTargets preserves group-config ordering so failover can map a
	// server to its positional outbound tag (proxy-N).
	lastTargets []*servers.Server

	ipc *ipc.Server
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "connectived:", err)
		os.Exit(1)
	}
}

func run() error {
	// DEBUG so supervised-core output (piped at DEBUG) is retained;
	// the UI filters by level.
	log := logging.New(logging.DEBUG, 2000)
	dataDir, err := platform.DataDir()
	if err != nil {
		return err
	}
	store, err := persistence.New(dataDir)
	if err != nil {
		return err
	}
	d := &Daemon{
		log:      log,
		store:    store,
		dataDir:  dataDir,
		machine:  connection.NewMachine(),
		settings: settings.Defaults(),
		sel:      selection{Auto: true},
	}
	d.loadState()
	d.initUpdates()

	d.ipc = ipc.NewServer()
	d.registerHandlers()

	sockPath, err := platform.SocketPath()
	if err != nil {
		return err
	}
	os.Remove(sockPath)
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", sockPath, err)
	}
	os.Chmod(sockPath, 0o600)
	go d.ipc.Serve(l)
	log.Info("connectived listening on %s", sockPath)
	d.machine.Subscribe(func(e connection.Event) {
		d.ipc.Broadcast(ipc.EventStateChanged, map[string]any{
			"from": string(e.From), "to": string(e.To),
			"reason": e.Reason, "server": e.Server,
		})
	})

	if d.settings.UpdateOnStart {
		go d.startupUpdate()
	}
	if m := d.updateManager(); m != nil {
		go m.MaybeCheckOnStartup()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("connectived shutting down")
	if d.machine.Connected() {
		_ = d.disconnect("shutdown")
	}
	return d.ipc.Close()
}

func (d *Daemon) loadState() {
	if ok, err := d.store.Load(persistence.DocSettings, &d.settings); err != nil {
		d.log.Warn("settings load failed, using defaults: %v", err)
		d.settings = settings.Defaults()
	} else if !ok {
		d.settings = settings.Defaults()
	}
	d.settings.Validate()
	if ok, err := d.store.Load(persistence.DocSubscriptions, &d.subs); err != nil {
		d.log.Warn("subscriptions load failed: %v", err)
	} else if !ok {
		d.subs = nil
	}
	if ok, err := d.store.Load(persistence.DocServers, &d.serverList); err != nil {
		d.log.Warn("servers load failed: %v", err)
	} else if !ok {
		d.serverList = nil
	}
	var sel selection
	if ok, err := d.store.Load(persistence.DocSelection, &sel); err == nil && ok {
		d.sel = sel
	}
	d.settings.AutoMode = d.sel.Auto
}

func (d *Daemon) persistServers() {
	_ = d.store.Save(persistence.DocServers, &d.serverList)
	_ = d.store.Save(persistence.DocSubscriptions, &d.subs)
	_ = d.store.Save(persistence.DocSelection, &d.sel)
}

func (d *Daemon) startupUpdate() {
	time.Sleep(3 * time.Second)
	d.mu.Lock()
	subs := d.subs
	enabled := d.settings.UpdateOnStart
	d.mu.Unlock()
	if !enabled || len(subs) == 0 {
		return
	}
	if _, err := d.refreshSubscriptions(""); err != nil {
		d.log.Warn("startup subscription update: %v", err)
	}
}

// --- core/sing-box resolution ---

func (d *Daemon) singBoxPath() (string, error) {
	if d.settings.CorePath != "" {
		return d.settings.CorePath, nil
	}
	// Bundled layout (e.g. /opt/connective): prefer the sing-box next
	// to the daemon binary, mirroring helperPath. Without this a fresh
	// install can never connect until the user hand-configures a path,
	// because sing-box is not on PATH.
	if exe, err := os.Executable(); err == nil {
		if candidate := siblingBinary(exe, platform.CoreExeName()); candidate != "" {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath(platform.CoreExeName()); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("sing-box not found: set corePath in settings or install sing-box")
}

func (d *Daemon) helperPath() (string, error) {
	resolve := func() (string, error) {
		if d.settings.HelperPath != "" {
			return d.settings.HelperPath, nil
		}
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		candidate := filepath.Join(filepath.Dir(exe), platform.HelperExeName())
		if _, err := os.Stat(candidate); err == nil {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
		if p, err := exec.LookPath(platform.HelperExeName()); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("connective-helper not found next to daemon or on PATH")
	}
	p, err := resolve()
	if err != nil {
		return "", err
	}
	// The helper runs elevated: never execute an untrusted path.
	if err := platform.TrustedBinary(p); err != nil {
		return "", fmt.Errorf("untrusted helper: %w", err)
	}
	return p, nil
}

// siblingBinary returns the bundled binary next to exePath when present
// (fresh installs where the core is not on PATH), or "" otherwise.
func siblingBinary(exePath, name string) string {
	candidate := filepath.Join(filepath.Dir(exePath), name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// Elevation lives in internal/platform now (pkexec on Linux, UAC
// PowerShell wrapper on Windows); tests override via
// CONNECTIVE_HELPER_RUNNER. See ElevatedCommand.

// --- connect / disconnect ---

// Connect establishes the connection for serverID ("" = AUTO).
func (d *Daemon) Connect(serverID string) error {
	d.connMu.Lock()
	defer d.connMu.Unlock()
	if d.machine.Connected() {
		return fmt.Errorf("already connected")
	}
	st := d.machine.State()
	if st != connection.StDisconnected && st != connection.StError {
		return fmt.Errorf("busy: %s", st)
	}
	d.noteRecovery()
	return d.connectLocked(serverID, "user")
}

func (d *Daemon) connectLocked(serverID, reason string) error {
	d.mu.Lock()
	st := d.settings
	all := usableServers(d.serverList)
	d.mu.Unlock()

	if len(all) == 0 {
		_ = d.machine.Transition(connection.StError, "no servers available")
		return fmt.Errorf("no servers available: add a subscription first")
	}
	// Foreign-TUN safety: never start a TUN core into a routing conflict
	// with another VPN. Fail clearly before any state change beyond
	// selecting; proxy-only mode (TUN off) is unaffected and may coexist.
	if st.TunEnabled {
		if err := platform.CheckSafeForTun(); err != nil {
			d.log.Warn("connect refused: %v", err)
			_ = d.machine.Transition(connection.StError, err.Error())
			return err
		}
	}
	_ = d.machine.Transition(connection.StSelecting, reason)

	d.quickTest(all)
	d.mu.Lock()
	all = usableServers(d.serverList)
	d.mu.Unlock()

	var targets []*servers.Server
	if serverID != "" && serverID != "auto" {
		// Manual selection honors the explicit choice even when the
		// server is currently marked unhealthy: the user asked for
		// THIS server (it may have recovered). Recovery still applies
		// if the path proves dead.
		for _, s := range d.serverList {
			if s.ID == serverID {
				targets = []*servers.Server{s}
				break
			}
		}
		if targets == nil {
			_ = d.machine.Transition(connection.StError, "server not found")
			return fmt.Errorf("server %q not found", serverID)
		}
	} else {
		targets = all
	}

	hist := historiesOf(all)
	pick := selector.Select(targets, hist, d.machine.ServerID(), selector.Defaults())
	if pick == nil {
		_ = d.machine.Transition(connection.StError, "no usable server")
		return fmt.Errorf("no usable server")
	}
	_ = d.machine.Transition(connection.StConnecting, pick.DisplayName())
	d.machine.SetServer(pick.ID)

	cfgBytes, clashSecret, err := d.renderConfig(targets, pick.ID)
	if err != nil {
		_ = d.machine.Transition(connection.StError, err.Error())
		return err
	}
	cfgPath := filepath.Join(d.dataDir, "core.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o600); err != nil {
		_ = d.machine.Transition(connection.StError, err.Error())
		return err
	}
	if h, err := dns.ResolverSnapshot(); err == nil {
		d.resolvHash = h
	}

	if err := d.bringUp(targets, pick, cfgPath, clashSecret, st); err != nil {
		return err
	}
	d.startMonitors(st)
	d.broadcastServers()
	return nil
}

func usableServers(list []*servers.Server) []*servers.Server {
	var out []*servers.Server
	for _, s := range list {
		if s.Health != servers.HealthUnhealthy {
			out = append(out, s)
		}
	}
	if len(out) > 0 {
		return out
	}
	return list
}

func historiesOf(list []*servers.Server) map[string][]int64 {
	h := map[string][]int64{}
	for _, s := range list {
		if s.Tested() {
			h[s.ID] = []int64{s.LatencyMs}
		}
	}
	return h
}

func (d *Daemon) renderConfig(targets []*servers.Server, activeID string) ([]byte, string, error) {
	d.mu.Lock()
	st := d.settings
	d.mu.Unlock()
	mode := configgen.ModeGlobal
	if st.RoutingMode == "rules" {
		mode = configgen.ModeRules
	}
	secret := randomSecret()
	excludes := tunExcludes(targets)
	opts := configgen.Options{
		MixedPort:       st.MixedPort,
		TunEnabled:      st.TunEnabled,
		Mode:            mode,
		ClashPort:       st.ClashAPIPort,
		ClashSecret:     secret,
		URLTestURL:      "https://www.gstatic.com/generate_204",
		URLTestInterval: time.Duration(st.URLTestIntervalMin) * time.Minute,
		MTU:             st.MTU,
		ExcludeAddrs:    excludes,
	}
	d.mu.Lock()
	d.mu.Unlock()
	if len(targets) == 1 {
		cfg, err := configgen.Generate(targets[0], opts)
		return cfg, secret, err
	}
	cfg, err := configgen.GenerateGroup(targets, activeID, opts)
	return cfg, secret, err
}

func randomSecret() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "connective-local"
	}
	return hex.EncodeToString(b[:])
}

func (d *Daemon) startCore(cfgPath, clashSecret string) (*core.Manager, int, error) {
	d.mu.Lock()
	st := d.settings
	d.mu.Unlock()
	sb, err := d.singBoxPath()
	if err != nil {
		return nil, 0, err
	}
	ready := func() bool {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", st.MixedPort), 300*time.Millisecond)
		if err != nil {
			return false
		}
		c.Close()
		return true
	}
	var mgr *core.Manager
	if st.TunEnabled || st.KillSwitch {
		if _, err := d.helperPath(); err != nil {
			return nil, 0, err
		}
		// Windows TUN rides on wintun.dll beside sing-box.exe.
		if err := platform.RequireWintun(sb); err != nil && st.TunEnabled {
			return nil, 0, err
		}
		// Elevated launch: the core binary must be trusted too.
		if err := platform.TrustedBinary(sb); err != nil {
			return nil, 0, fmt.Errorf("untrusted core binary: %w", err)
		}
		bin, args, via := d.elevatedCoreCommand(cfgPath)
		d.log.Info("starting core %s for TUN", via)
		mgr = core.New(core.Config{
			Binary: bin, Args: args,
			StartupTimeout: 20 * time.Second, ShutdownTimeout: 5 * time.Second,
			Ready: ready, OnLog: func(l string) { d.log.Debug("core: %s", l) },
		})
	} else {
		mgr = core.New(core.Config{
			Binary: sb, Args: []string{"run", "-c", cfgPath},
			StartupTimeout: 20 * time.Second, ShutdownTimeout: 5 * time.Second,
			Ready: ready, OnLog: func(l string) { d.log.Debug("core: %s", l) },
		})
	}
	if err := mgr.Start(); err != nil {
		return nil, 0, err
	}
	return mgr, st.ClashAPIPort, nil
}

func (d *Daemon) verifyTUN() error {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if present, _ := platform.TunExists(platform.OwnTunName); present {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("TUN device connective0 did not appear")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (d *Daemon) verifyRouting(tun bool) error {
	// Route application lags core start; poll briefly instead of
	// checking once (observed async on sing-box 1.14).
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := routing.VerifyTUNDefault("connective0", tun)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// errUnreachable marks a core PID that is alive but beyond the
// daemon's own signals (root-owned elevated core: EPERM, or still
// alive despite SIGKILL). Only the privileged helper can end it.
var errUnreachable = errors.New("process alive but not signalable")

// processGone probes pid: nil when reaped, errUnreachable when alive
// (signalable or not — anything surviving Stop needs the helper),
// other errors pass through (treated as gone by the caller).
// The probe itself is platform-specific (see internal/platform/proc_*).
func processGone(pid int) error {
	if err := platform.ProcessGone(pid); err != nil {
		if platform.ErrUnreachable(err) {
			return errUnreachable
		}
		return err
	}
	return nil
}

func (d *Daemon) stopCore() {
	if d.coreMgr == nil {
		return
	}
	pid := d.coreMgr.Pid()
	_ = d.coreMgr.Stop()
	d.coreMgr = nil
	d.ensureCoreGone(pid)
}

// ensureCoreGone finishes a core that outlived the unprivileged stop:
// an elevated (root-owned) core ignores daemon signals, so Stop can
// only bound its wait and return. If the PID is still alive and beyond
// our reach, the privileged helper ends it (cmdline-verified, never a
// blind kill). Already gone = success. This keeps disconnect, failover
// and reconnect from wedging on "Disconnecting..." forever.
func (d *Daemon) ensureCoreGone(pid int) {
	if pid <= 0 {
		return
	}
	if err := processGone(pid); err == nil {
		return // reaped by Stop
	} else if err != errUnreachable {
		d.log.Warn("core pid %d already gone: %v", pid, err)
		return
	}
	d.log.Warn("core pid %d survives unprivileged stop, using helper", pid)
	if err := d.runHelper("stop-core", "--pid", fmt.Sprint(pid)); err != nil {
		d.log.Error("helper stop-core pid %d: %v", pid, err)
	}
}

// Disconnect tears everything down and restores system state.
func (d *Daemon) Disconnect(reason string) error {
	d.connMu.Lock()
	defer d.connMu.Unlock()
	return d.disconnect(reason)
}

func (d *Daemon) disconnect(reason string) error {
	st := d.machine.State()
	if st == connection.StDisconnected {
		return fmt.Errorf("not connected")
	}
	// StError and wedged transients have no disconnecting edge; they
	// still need the same cleanup, then a forced reset.
	if st == connection.StError || d.recoveryUnderway() {
		d.log.Warn("disconnect from %s: forcing cleanup", st)
		d.stopMonitors()
		d.stopCore()
		d.mu.Lock()
		kill := d.settings.KillSwitch
		tun := d.settings.TunEnabled
		d.mu.Unlock()
		if kill {
			_ = d.runHelper("killswitch-off", "--state", filepath.Join(d.dataDir, "killswitch.state"))
		}
		if tun {
			_ = d.runHelper("tun-cleanup", "--if", "connective0")
		}
		d.machine.Reset(reason)
		return nil
	}
	_ = d.machine.Transition(connection.StDisconnecting, reason)
	d.stopMonitors()
	d.stopCore()
	d.mu.Lock()
	kill := d.settings.KillSwitch
	tun := d.settings.TunEnabled
	d.mu.Unlock()
	if kill {
		_ = d.runHelper("killswitch-off", "--state", filepath.Join(d.dataDir, "killswitch.state"))
	}
	if tun {
		_ = d.runHelper("tun-cleanup", "--if", "connective0")
	}
	if h, err := dns.ResolverSnapshot(); err == nil && d.resolvHash != "" && h != d.resolvHash {
		d.log.Warn("system resolver changed during session (before=%s after=%s)", d.resolvHash, h)
	}
	_ = d.machine.Transition(connection.StDisconnected, reason)
	d.log.Info("disconnected")
	return nil
}

// bringUp starts the core for an already-written config and enforces the
// full post-start verification — TUN device, effective routing, kill
// switch — before declaring the connection. Shared by initial connect
// and every reconnect so both offer identical guarantees. The caller
// must hold connMu with the machine in connecting state.
func (d *Daemon) bringUp(targets []*servers.Server, pick *servers.Server, cfgPath, clashSecret string, st settings.Settings) error {
	_ = d.machine.Transition(connection.StStartingCore, pick.DisplayName())
	mgr, clashPort, err := d.startCore(cfgPath, clashSecret)
	if err != nil {
		_ = d.machine.Transition(connection.StError, err.Error())
		return fmt.Errorf("could not connect to %s: %w", pick.DisplayName(), err)
	}
	d.coreMgr = mgr
	d.clash = stats.New(clashPort, clashSecret)
	d.tracker = stats.NewTracker(d.clash)
	d.lastTargets = targets

	_ = d.machine.Transition(connection.StInitializingTUN, "")
	if st.TunEnabled {
		if err := d.verifyTUN(); err != nil {
			d.stopCore()
			_ = d.machine.Transition(connection.StError, err.Error())
			return err
		}
	}
	_ = d.machine.Transition(connection.StApplyingRouting, "")
	if err := d.verifyRouting(st.TunEnabled); err != nil {
		d.stopCore()
		_ = d.machine.Transition(connection.StError, err.Error())
		return err
	}
	if st.KillSwitch {
		if err := d.applyKillSwitch(targets); err != nil {
			d.stopCore()
			_ = d.machine.Transition(connection.StError, err.Error())
			return fmt.Errorf("kill switch: %w", err)
		}
	}
	_ = d.machine.Transition(connection.StConnected, pick.DisplayName())
	d.log.Info("connected via %s (%s)", pick.DisplayName(), pick.ID)
	d.connectAt = time.Now()
	d.failCount = 0
	d.flapCount = 0
	if tot, err := d.clash.Totals(context.Background()); err == nil {
		d.sessionUp, d.sessionDown = tot.Up, tot.Down
	}
	return nil
}

// elevatedCoreCommand builds the helper-mediated core launch. When the
// daemon itself is privileged (tests, netns lab) the helper runs
// directly; otherwise it goes through the elevation runner (pkexec).
func (d *Daemon) elevatedCoreCommand(cfgPath string) (bin string, args []string, via string) {
	sb, _ := d.singBoxPath()
	helper, _ := d.helperPath()
	coreArgs := []string{"run-core", "--", sb, "run", "-c", cfgPath}
	if platform.IsElevated() {
		return helper, coreArgs, "via helper (already privileged)"
	}
	bin, full := platform.ElevatedCommand(helper, coreArgs)
	return bin, full, fmt.Sprintf("elevated (%s)", bin)
}

// applyKillSwitch enforces egress lockdown: only the tunnel device,
// loopback/LAN/established flows and the VPN server endpoints may pass.
func (d *Daemon) applyKillSwitch(targets []*servers.Server) error {
	seen := map[string]bool{}
	// --state snapshots/restores the previous firewall policy. The
	// Linux helper accepts and ignores it (nft needs no snapshot);
	// Windows requires it.
	args := []string{
		"--tun", "connective0",
		"--state", filepath.Join(d.dataDir, "killswitch.state"),
	}
	for _, s := range targets {
		ips, err := net.LookupIP(s.Address)
		if err != nil || len(ips) == 0 {
			// Unresolvable names are skipped: without an IP there is
			// nothing to allowlist, and failing closed (no rule) is
			// safer than failing open. The connection itself still
			// works because established + TUN traffic is allowed.
			d.log.Warn("killswitch: cannot resolve %q, no allow rule", s.Address)
			continue
		}
		for _, ip := range ips {
			if ip.To4() == nil {
				continue // nft set is IPv4-egress focused (see helper)
			}
			key := ip.String() + ":" + fmt.Sprint(s.Port)
			if !seen[key] {
				seen[key] = true
				args = append(args, "--allow", key)
			}
		}
	}
	return d.runHelper(append([]string{"killswitch-on"}, args...)...)
}

func (d *Daemon) runHelper(args ...string) error {
	helper, err := d.helperPath()
	if err != nil {
		return err
	}
	// Every helper invocation is bounded (20s): an unbounded pkexec wait
	// once wedged disconnect forever on cancel/rejection. run-core is
	// never launched here (it goes through core.Manager with its own
	// startup timeout); these are all short syscalls.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	bin, full := platform.ElevatedCommand(helper, args)
	cmd := exec.CommandContext(ctx, bin, full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("helper %v: %s: %w", args, string(out), err)
	}
	return nil
}

// monSet is one generation of monitor loops.
type monSet struct {
	gen  int
	stop chan struct{}
	wg   sync.WaitGroup
}

// monActive reports whether m is still the current generation.
func (d *Daemon) monActive(m *monSet) bool {
	d.monMu.Lock()
	defer d.monMu.Unlock()
	return d.mons == m
}

func (d *Daemon) startMonitors(st settings.Settings) {
	d.monMu.Lock()
	d.monGen++
	m := &monSet{gen: d.monGen, stop: make(chan struct{})}
	d.mons = m
	d.monMu.Unlock()
	m.wg.Add(3)
	go d.statsLoop(m)
	go d.healthLoop(m, st)
	w := &monitor.Watcher{
		Interval: 5 * time.Second,
		Confirm:  2, // change must persist across polls: noisy hosts flap once
		OnChange: func(before, after string) {
			d.log.Warn("network change detected: %s => %s", trunc(before, 160), trunc(after, 160))
			d.onNetworkChange()
		},
	}
	w.Start()
	go func() {
		defer m.wg.Done()
		<-m.stop
		w.Stop()
	}()
}

// stopMonitors signals the current generation to exit and reaps it in
// the background. It never blocks: recovery paths (failover,
// reconnect) run on the very goroutine a monitor tick arrived on.
func (d *Daemon) stopMonitors() {
	d.monMu.Lock()
	m := d.mons
	d.mons = nil
	d.monMu.Unlock()
	if m != nil {
		close(m.stop)
		go m.wg.Wait()
	}
}

func (d *Daemon) statsLoop(m *monSet) {
	defer m.wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-tick.C:
			if !d.monActive(m) {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			snap, err := d.tracker.Sample(ctx)
			cancel()
			if err != nil {
				continue
			}
			d.ipc.Broadcast(ipc.EventStats, map[string]any{
				"upRate": snap.UpRate, "downRate": snap.DownRate,
				"upTotal": snap.Up - d.sessionUp, "downTotal": snap.Down - d.sessionDown,
				"durationMs": time.Since(d.connectAt).Milliseconds(),
			})
		}
	}
}

func (d *Daemon) healthLoop(m *monSet, st settings.Settings) {
	defer m.wg.Done()
	interval := time.Duration(st.HealthIntervalSec) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-tick.C:
			if !d.monActive(m) {
				return
			}
			d.checkHealth(st)
		}
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// noteRecovery records a recovery start for the network-change cooldown.
func (d *Daemon) noteRecovery() {
	d.mu.Lock()
	d.lastRecovery = time.Now()
	d.mu.Unlock()
}

func (d *Daemon) proxyURL() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.settings.MixedPort <= 0 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", d.settings.MixedPort)
}

func (d *Daemon) checkHealth(st settings.Settings) {
	if !d.machine.Connected() {
		return
	}
	healthy := true
	if d.coreMgr == nil || !d.coreMgr.Running() {
		healthy = false
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		err := monitor.ProbeURLs(ctx, d.proxyURL(), d.probeURLs(), 10*time.Second)
		cancel()
		if err != nil {
			d.log.Warn("health probe failed: %v", err)
			healthy = false
		}
	}
	if healthy {
		d.failCount = 0
		d.flapCount = 0
		if d.machine.State() == connection.StDegraded {
			_ = d.machine.Transition(connection.StConnected, "recovered")
		}
		return
	}
	d.failCount++
	d.log.Warn("unhealthy (%d/2)", d.failCount)
	if d.machine.State() == connection.StConnected {
		_ = d.machine.Transition(connection.StDegraded, "probe failed")
	}
	if d.failCount >= 2 {
		// The current path is confirmed dead: record it so selection
		// never picks it back until a fresh test proves otherwise.
		// Without this, failover ping-pongs between stale "healthy"
		// records.
		d.markCurrentUnhealthy()
		d.failover()
	}
}

// markCurrentUnhealthy records the active server as unhealthy in the
// store (persisted + broadcast) after consecutive probe failures.
func (d *Daemon) markCurrentUnhealthy() {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := d.machine.ServerID()
	for _, s := range d.serverList {
		if s.ID == id {
			s.Health = servers.HealthUnhealthy
			s.LastTest = time.Now()
			d.ipc.Broadcast(ipc.EventHealth, map[string]any{
				"serverId": s.ID, "latencyMs": s.LatencyMs,
				"health": string(s.Health),
			})
			break
		}
	}
	d.persistServers()
}

// recoveryUnderway reports whether a connect/reconnect flow already owns
// the machine. Recovery triggers must stand down while it is true:
// double recovery kills the new core out from under the first flow and
// wedges the state machine.
func (d *Daemon) recoveryUnderway() bool {
	switch d.machine.State() {
	case connection.StTesting, connection.StSelecting, connection.StConnecting,
		connection.StStartingCore, connection.StInitializingTUN,
		connection.StApplyingRouting, connection.StReconnecting,
		connection.StSwitchingServer, connection.StDisconnecting:
		return true
	}
	return false
}

// failover picks the best surviving server and switches to it.
func (d *Daemon) failover() {
	d.connMu.Lock()
	defer d.connMu.Unlock()
	if !d.machine.Connected() {
		d.log.Info("failover: standing down, machine is %s", d.machine.State())
		d.failCount = 0
		return
	}
	d.noteRecovery()
	d.mu.Lock()
	all := usableServers(d.serverList)
	d.mu.Unlock()
	current := d.machine.ServerID()
	var candidates []*servers.Server
	for _, s := range all {
		if s.ID != current {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		if d.flapCount >= 2 {
			d.log.Warn("failover: no alternative and current failing, holding")
			d.failCount = 0
			return
		}
		d.flapCount++
		d.log.Error("failover: no alternative server, retrying current")
		d.failCount = 0
		_ = d.machine.Transition(connection.StReconnecting, "retry current")
		d.reconnectLocked()
		return
	}
	best := selector.Select(candidates, historiesOf(candidates), "", selector.Defaults())
	if best == nil {
		return
	}
	if d.flapCount >= 2 {
		// Every alternative has failed too. Before holding, re-test:
		// a dead server may have come back (or the outage was
		// transient). If anything is alive, resume switching.
		if d.reviveCheck() {
			d.log.Info("failover: revival detected, resuming")
			d.flapCount = 0
		} else {
			// Stop hopping, stay degraded and keep probing. Hopping
			// between dead servers helps nobody.
			d.log.Warn("failover: all alternatives failing, holding %s", d.machine.ServerID())
			d.failCount = 0
			return
		}
	}
	d.flapCount++
	d.log.Info("failover: %s -> %s", current, best.DisplayName())
	_ = d.machine.Transition(connection.StSwitchingServer, best.DisplayName())
	// Fast path: switch inside the running group config without restart.
	switched := false
	if d.clash != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := d.clash.SwitchProxy(ctx, configgen.ProxyTag, d.groupTagFor(best))
		cancel()
		if err == nil {
			switched = true
		} else {
			d.log.Warn("in-place switch failed, restarting core: %v", err)
		}
	}
	if !switched {
		d.log.Info("failover: restarting core on %s", best.DisplayName())
		d.stopMonitors()
		d.stopCore()
		d.machine.SetServer(best.ID)
		serverID := best.ID
		_ = d.machine.Transition(connection.StReconnecting, best.DisplayName())
		if err := d.reconnectWith(serverID); err != nil {
			d.log.Error("failover reconnect failed: %v", err)
			return
		}
	} else {
		d.log.Info("failover: switched in place to %s", best.DisplayName())
		d.machine.SetServer(best.ID)
		d.failCount = 0
		// Never declare victory without proof: probe the new path
		// now. Success → connected; failure → stay degraded so the
		// next tick keeps recovering instead of lying to the UI.
		vctx, vcancel := context.WithTimeout(context.Background(), 25*time.Second)
		verr := monitor.ProbeURLs(vctx, d.proxyURL(), d.probeURLs(), 10*time.Second)
		vcancel()
		if verr == nil {
			d.flapCount = 0
			_ = d.machine.Transition(connection.StConnected, best.DisplayName())
		} else {
			d.log.Warn("failover: new path probe failed: %v", verr)
			d.failCount = 1
			_ = d.machine.Transition(connection.StDegraded, "new path bad")
		}
	}
	d.broadcastServers()
}

// settingsSnapshot returns a copy of current settings.
func (d *Daemon) settingsSnapshot() settings.Settings {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.settings
}

// probeURLs returns the health-probe targets: the configured URL plus a
// well-known dual-stack fallback (see monitor.ProbeURLs).
func (d *Daemon) probeURLs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	primary := d.settings.ConnectionTestURL
	fallback := "http://www.gstatic.com/generate_204"
	if primary == "" || primary == fallback {
		return []string{fallback}
	}
	return []string{primary, fallback}
}

// reviveCheck quickly re-tests all servers after repeated failover
// failures. It returns true when at least one server answers, in which
// case records are updated (revived servers un-marked) and failover may
// resume. Bounded: single 3s-timeout round, never blocks the UI.
func (d *Daemon) reviveCheck() bool {
	d.mu.Lock()
	list := append([]*servers.Server(nil), d.serverList...)
	d.mu.Unlock()
	if len(list) == 0 {
		return false
	}
	d.quickTest(list)
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.serverList {
		if s.Health == servers.HealthHealthy || s.Health == servers.HealthDegraded {
			return true
		}
	}
	return false
}

// quickTest runs one fast TCP round over candidates so reconnect and
// failover decide on fresh data, not stale records.
func (d *Daemon) quickTest(list []*servers.Server) {
	if len(list) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	opts := tester.Options{Timeout: 3 * time.Second, Concurrency: 5, Retries: 0}
	for r := range tester.RunBulk(ctx, list, opts, tester.TCPTestFunc(opts)) {
		d.mu.Lock()
		for _, s := range d.serverList {
			if s.ID == r.ServerID {
				tester.Outcome(s, r)
				break
			}
		}
		d.mu.Unlock()
	}
	d.mu.Lock()
	d.persistServers()
	d.mu.Unlock()
	d.broadcastServers()
}

// tunExcludes resolves VPN server endpoints to CIDR prefixes for TUN
// exclusion. Literals pass through (/32 appended); hostnames resolve
// best-effort (unresolvable names are skipped — the kill-switch layer
// documents that case).
func tunExcludes(targets []*servers.Server) []string {
	seen := map[string]bool{}
	var out []string
	add := func(cidr string) {
		if !seen[cidr] {
			seen[cidr] = true
			out = append(out, cidr)
		}
	}
	for _, s := range targets {
		if ip := net.ParseIP(s.Address); ip != nil {
			if ip.To4() != nil {
				add(ip.String() + "/32")
			} else {
				add(ip.String() + "/128")
			}
			continue
		}
		ips, err := net.LookupIP(s.Address)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil {
				add(v4.String() + "/32")
			}
		}
	}
	return out
}

// groupTagFor maps a server to its positional outbound tag (proxy-N) in
// the last rendered group config. Falls back to the urltest group, which
// re-picks the best member on its own.
func (d *Daemon) groupTagFor(best *servers.Server) string {
	for i, s := range d.lastTargets {
		if s.ID == best.ID {
			return fmt.Sprintf("proxy-%d", i)
		}
	}
	return configgen.ProxyAutoTag
}

func (d *Daemon) reconnectLocked() {
	id := d.machine.ServerID()
	d.stopMonitors()
	d.stopCore()
	_ = d.reconnectWith(id)
}

func (d *Daemon) reconnectWith(serverID string) error {
	// Re-enter the connect path below the connected-state guard.
	// NOTE: the full candidate set is used (not just the previous
	// server): reconnecting to a dead server in a loop helps nobody.
	// serverID is a preference (selector default/incumbent), honored
	// when it is alive.
	d.mu.Lock()
	all := usableServers(d.serverList)
	d.mu.Unlock()
	d.quickTest(all)
	incumbent := ""
	if serverID != "" {
		for _, s := range all {
			if s.ID == serverID {
				incumbent = serverID
				break
			}
		}
	}
	targets := all
	pick := selector.Select(targets, historiesOf(targets), incumbent, selector.Defaults())
	if pick == nil {
		_ = d.machine.Transition(connection.StError, "no usable server")
		return fmt.Errorf("no usable server")
	}
	_ = d.machine.Transition(connection.StConnecting, pick.DisplayName())
	d.machine.SetServer(pick.ID)
	cfgBytes, clashSecret, err := d.renderConfig(targets, pick.ID)
	if err != nil {
		_ = d.machine.Transition(connection.StError, err.Error())
		return err
	}
	cfgPath := filepath.Join(d.dataDir, "core.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o600); err != nil {
		_ = d.machine.Transition(connection.StError, err.Error())
		return err
	}
	if err := d.bringUp(targets, pick, cfgPath, clashSecret, d.settingsSnapshot()); err != nil {
		return err
	}
	d.mu.Lock()
	st := d.settings
	d.mu.Unlock()
	d.startMonitors(st)
	return nil
}

func (d *Daemon) onNetworkChange() {
	if !d.machine.Connected() || d.recoveryUnderway() {
		d.log.Info("network change: standing down, machine is %s", d.machine.State())
		return
	}
	// Cooldown: never launch recoveries more often than this; noisy
	// hosts (DHCP churn, VPN peer activity) flap snapshots.
	d.mu.Lock()
	cool := time.Since(d.lastRecovery) < 90*time.Second
	d.mu.Unlock()
	if cool {
		d.log.Info("network change: within cooldown, re-verifying only")
	}
	// Double probe: a single failure can be transient; reconnect only
	// on two consecutive failures 5s apart.
	bad := 0
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		err := monitor.ProbeURLs(ctx, d.proxyURL(), d.probeURLs(), 10*time.Second)
		cancel()
		if err != nil {
			d.log.Warn("post-change probe %d failed: %v", i+1, err)
			bad++
			time.Sleep(5 * time.Second)
			continue
		}
		d.log.Info("post-change probe ok, keeping connection")
		return
	}
	if bad == 2 {
		if cool {
			d.log.Warn("post-change probes failed inside cooldown: holding")
			return
		}
		d.log.Warn("post-change probes failed, reconnecting")
		d.mu.Lock()
		d.lastRecovery = time.Now()
		d.mu.Unlock()
		d.connMu.Lock()
		defer d.connMu.Unlock()
		_ = d.machine.Transition(connection.StReconnecting, "network change")
		d.reconnectLocked()
	}
}

// --- subscriptions refresh ---

func (d *Daemon) refreshSubscriptions(onlyID string) (int, error) {
	d.mu.Lock()
	subs := d.subs
	// NOTE: do not call proxyURL() here — it takes d.mu, which we hold.
	// Inline the read instead (machine has its own lock).
	proxy := ""
	if d.machine.Connected() && d.settings.MixedPort > 0 {
		proxy = fmt.Sprintf("http://127.0.0.1:%d", d.settings.MixedPort)
	}
	timeout := time.Duration(d.settings.TestTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	d.mu.Unlock()

	f := subscriptions.NewHTTPFetcher(timeout)
	if proxy != "" {
		f.TryProxy, f.ProxyURL = true, proxy
	}
	var targets []*subscriptions.Subscription
	for _, s := range subs {
		if onlyID == "" || s.ID == onlyID {
			targets = append(targets, s)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	fresh, updated := subscriptions.UpdateAll(ctx, f, targets)

	d.mu.Lock()
	byID := map[string]*subscriptions.Subscription{}
	for _, s := range d.subs {
		byID[s.ID] = s
	}
	for _, u := range updated {
		byID[u.ID] = u
	}
	d.subs = d.subs[:0]
	for _, s := range byID {
		d.subs = append(d.subs, s)
	}
	// Membership: locals always survive; servers of successfully
	// refreshed subs are replaced by the merge; everything else
	// (untouched or failed subs) is preserved stale.
	refreshed := map[string]bool{}
	for _, u := range updated {
		if u.LastError == "" {
			refreshed[u.ID] = true
		}
	}
	merged := subscriptions.Merge(d.serverList, fresh)
	var next []*servers.Server
	for _, s := range d.serverList {
		if s.SubscriptionID == "" {
			next = append(next, s) // local
		} else if !refreshed[s.SubscriptionID] {
			next = append(next, s) // stale but preserved
		}
	}
	d.serverList = append(next, merged...)
	d.persistServers()
	d.mu.Unlock()
	d.broadcastServers()
	d.broadcastSubs()
	n := 0
	for _, u := range updated {
		if u.LastError == "" {
			n++
		}
	}
	if onlyID != "" && n == 0 {
		return 0, fmt.Errorf("subscription update failed")
	}
	return n, nil
}

func (d *Daemon) broadcastServers() {
	d.mu.Lock()
	list := d.serverList
	d.mu.Unlock()
	d.ipc.Broadcast(ipc.EventServers, list)
}

func (d *Daemon) broadcastSubs() {
	d.mu.Lock()
	subs := d.subs
	d.mu.Unlock()
	d.ipc.Broadcast(ipc.EventSubscriptions, subs)
}

// --- IPC handlers ---

func (d *Daemon) registerHandlers() {
	d.ipc.Handle(ipc.MethodPing, d.hPing)
	d.ipc.Handle(ipc.MethodGetState, d.hGetState)
	d.ipc.Handle(ipc.MethodGetSettings, d.hGetSettings)
	d.ipc.Handle(ipc.MethodUpdateSettings, d.hUpdateSettings)
	d.ipc.Handle(ipc.MethodGetLogs, d.hGetLogs)
	d.ipc.Handle(ipc.MethodClearLogs, d.hClearLogs)
	d.ipc.Handle(ipc.MethodConnect, d.hConnect)
	d.ipc.Handle(ipc.MethodDisconnect, d.hDisconnect)
	d.ipc.Handle(ipc.MethodListServers, d.hListServers)
	d.ipc.Handle(ipc.MethodSelectServer, d.hSelectServer)
	d.ipc.Handle(ipc.MethodTestServer, d.hTestServer)
	d.ipc.Handle(ipc.MethodAddServer, d.hAddServer)
	d.ipc.Handle(ipc.MethodUpdateServer, d.hUpdateServer)
	d.ipc.Handle(ipc.MethodRemoveServer, d.hRemoveServer)
	d.ipc.Handle(ipc.MethodDuplicateServer, d.hDuplicateServer)
	d.ipc.Handle(ipc.MethodExportServer, d.hExportServer)
	d.ipc.Handle(ipc.MethodAddSubscription, d.hAddSubscription)
	d.ipc.Handle(ipc.MethodListSubscriptions, d.hListSubscriptions)
	d.ipc.Handle(ipc.MethodEditSubscription, d.hEditSubscription)
	d.ipc.Handle(ipc.MethodUpdateSubscription, d.hUpdateSubscription)
	d.ipc.Handle(ipc.MethodRemoveSubscription, d.hRemoveSubscription)
	d.ipc.Handle(ipc.MethodGetStats, d.hGetStats)
	d.ipc.Handle(ipc.MethodUIGet, d.hUIGet)
	d.ipc.Handle(ipc.MethodUIUpdate, d.hUIUpdate)
	d.ipc.Handle(ipc.MethodUpdateCheck, d.hUpdateCheck)
	d.ipc.Handle(ipc.MethodUpdateStatus, d.hUpdateStatus)
	d.ipc.Handle(ipc.MethodUpdateDownload, d.hUpdateDownload)
	d.ipc.Handle(ipc.MethodUpdateCancel, d.hUpdateCancel)
	d.ipc.Handle(ipc.MethodUpdateInstall, d.hUpdateInstall)
	d.ipc.Handle(ipc.MethodUpdateDismiss, d.hUpdateDismiss)
}

func (d *Daemon) hPing(p json.RawMessage) (any, error) {
	return map[string]string{"version": ipc.Version}, nil
}

func (d *Daemon) hGetState(p json.RawMessage) (any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]any{
		"state":  string(d.machine.State()),
		"server": d.machine.ServerID(),
		"auto":   d.sel.Auto,
		// Manual selection, distinct from the connected server above.
		// Lets the Dashboard show "connected X" vs "selected Y".
		// Additive: older UIs ignore it.
		"selectedServer": d.sel.ServerID,
		"settings":       d.settings,
		"foreignTun":     platform.OtherTunInterfaces(),
	}, nil
}

func (d *Daemon) hGetSettings(p json.RawMessage) (any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.settings, nil
}

func (d *Daemon) hUpdateSettings(p json.RawMessage) (any, error) {
	var s settings.Settings
	if err := json.Unmarshal(p, &s); err != nil {
		return nil, fmt.Errorf("bad settings: %w", err)
	}
	s.Validate()
	d.mu.Lock()
	d.settings = s
	d.mu.Unlock()
	if err := d.store.Save(persistence.DocSettings, &s); err != nil {
		return nil, err
	}
	return s, nil
}

func (d *Daemon) hGetLogs(p json.RawMessage) (any, error) {
	return d.log.Recent(200, logging.DEBUG), nil
}

func (d *Daemon) hClearLogs(p json.RawMessage) (any, error) {
	d.log.Clear()
	return map[string]bool{"cleared": true}, nil
}

func (d *Daemon) hConnect(p json.RawMessage) (any, error) {
	var req struct {
		ServerID string `json:"serverId"`
	}
	if len(p) > 0 {
		if err := json.Unmarshal(p, &req); err != nil {
			return nil, fmt.Errorf("bad connect request: %w", err)
		}
	}
	if err := d.Connect(req.ServerID); err != nil {
		return nil, err
	}
	return map[string]string{"state": string(d.machine.State())}, nil
}

func (d *Daemon) hDisconnect(p json.RawMessage) (any, error) {
	if err := d.Disconnect("user"); err != nil {
		return nil, err
	}
	return map[string]string{"state": string(d.machine.State())}, nil
}

func (d *Daemon) hListServers(p json.RawMessage) (any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.serverList == nil {
		return []*servers.Server{}, nil
	}
	return d.serverList, nil
}

func (d *Daemon) hSelectServer(p json.RawMessage) (any, error) {
	var req struct {
		ServerID string `json:"serverId"`
	}
	if err := json.Unmarshal(p, &req); err != nil {
		return nil, fmt.Errorf("bad select request: %w", err)
	}
	d.mu.Lock()
	if req.ServerID == "" || req.ServerID == "auto" {
		d.sel = selection{Auto: true}
	} else {
		found := false
		for _, s := range d.serverList {
			if s.ID == req.ServerID {
				found = true
				break
			}
		}
		if !found {
			d.mu.Unlock()
			return nil, fmt.Errorf("server %q not found", req.ServerID)
		}
		d.sel = selection{Auto: false, ServerID: req.ServerID}
	}
	d.settings.AutoMode = d.sel.Auto
	d.persistServers()
	sel := d.sel
	d.mu.Unlock()
	return sel, nil
}

func (d *Daemon) hTestServer(p json.RawMessage) (any, error) {
	var req struct {
		ServerIDs []string `json:"serverIds"`
	}
	if err := json.Unmarshal(p, &req); err != nil {
		return nil, fmt.Errorf("bad test request: %w", err)
	}
	go d.testServers(req.ServerIDs)
	return map[string]bool{"started": true}, nil
}

// hAddServer imports one server from a share link ({link}) or stores a
// full record ({server}, ID assigned when empty).
func (d *Daemon) hAddServer(p json.RawMessage) (any, error) {
	var req struct {
		Link   string          `json:"link"`
		Server *servers.Server `json:"server"`
	}
	if err := json.Unmarshal(p, &req); err != nil {
		return nil, fmt.Errorf("bad add request: %w", err)
	}
	var s *servers.Server
	if req.Link != "" {
		parsed, err := servers.ParseShareLink(req.Link)
		if err != nil {
			return nil, fmt.Errorf("cannot import server: %w", err)
		}
		s = parsed
	} else if req.Server != nil {
		s = req.Server
	} else {
		return nil, fmt.Errorf("add needs a link or a server object")
	}
	s.Normalize()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.serverList = append(d.serverList, s)
	d.persistServers()
	d.mu.Unlock()
	d.broadcastServers()
	return s, nil
}

// hUpdateServer replaces the stored record with a new one (same ID).
// Test results survive the edit; subscription-owned servers keep their
// subscription tag.
func (d *Daemon) hUpdateServer(p json.RawMessage) (any, error) {
	var req struct {
		Server *servers.Server `json:"server"`
	}
	if err := json.Unmarshal(p, &req); err != nil || req.Server == nil {
		return nil, fmt.Errorf("bad update request")
	}
	upd := req.Server
	if upd.ID == "" {
		return nil, fmt.Errorf("update needs a server id")
	}
	upd.Normalize()
	if err := upd.Validate(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	for i, s := range d.serverList {
		if s.ID == upd.ID {
			upd.LatencyMs, upd.LastTest, upd.Health = s.LatencyMs, s.LastTest, s.Health
			if upd.SubscriptionID == "" {
				upd.SubscriptionID = s.SubscriptionID
			}
			d.serverList[i] = upd
			d.persistServers()
			d.mu.Unlock()
			d.broadcastServers()
			return upd, nil
		}
	}
	d.mu.Unlock()
	return nil, fmt.Errorf("server %q not found", upd.ID)
}

// hRemoveServer deletes a server. The active server and members of
// subscriptions are protected with friendly errors.
func (d *Daemon) hRemoveServer(p json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(p, &req); err != nil || req.ID == "" {
		return nil, fmt.Errorf("bad remove request")
	}
	if d.machine.Connected() && d.machine.ServerID() == req.ID {
		return nil, fmt.Errorf("disconnect first: %s is the active connection", "this server")
	}
	d.mu.Lock()
	for i, s := range d.serverList {
		if s.ID == req.ID {
			if s.SubscriptionID != "" {
				d.mu.Unlock()
				return nil, fmt.Errorf("this server belongs to a subscription and returns on next update; remove the subscription instead")
			}
			d.serverList = append(d.serverList[:i], d.serverList[i+1:]...)
			if d.sel.ServerID == req.ID {
				d.sel = selection{Auto: true}
			}
			d.persistServers()
			d.mu.Unlock()
			d.broadcastServers()
			return map[string]bool{"removed": true}, nil
		}
	}
	d.mu.Unlock()
	return nil, fmt.Errorf("server %q not found", req.ID)
}

// hDuplicateServer copies a server under a new ID.
func (d *Daemon) hDuplicateServer(p json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(p, &req); err != nil || req.ID == "" {
		return nil, fmt.Errorf("bad duplicate request")
	}
	d.mu.Lock()
	for _, s := range d.serverList {
		if s.ID == req.ID {
			cp := *s
			cp.ID = servers.NewID()
			cp.Name = s.DisplayName() + " (copy)"
			cp.CustomName = ""
			cp.SubscriptionID = ""
			cp.LatencyMs, cp.Health = -1, servers.HealthUnknown
			d.serverList = append(d.serverList, &cp)
			d.persistServers()
			d.mu.Unlock()
			d.broadcastServers()
			return &cp, nil
		}
	}
	d.mu.Unlock()
	return nil, fmt.Errorf("server %q not found", req.ID)
}

// hExportServer renders a server back to its share link.
func (d *Daemon) hExportServer(p json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(p, &req); err != nil || req.ID == "" {
		return nil, fmt.Errorf("bad export request")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.serverList {
		if s.ID == req.ID {
			link, err := servers.ExportShareLink(s)
			if err != nil {
				return nil, err
			}
			return map[string]string{"link": link}, nil
		}
	}
	return nil, fmt.Errorf("server %q not found", req.ID)
}

func (d *Daemon) hAddSubscription(p json.RawMessage) (any, error) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(p, &req); err != nil {
		return nil, fmt.Errorf("bad subscription request: %w", err)
	}
	sub := &subscriptions.Subscription{
		ID:      servers.NewID(),
		Name:    req.Name,
		URL:     req.URL,
		Enabled: true,
	}
	if sub.Name == "" {
		sub.Name = sub.URL
	}
	if err := subscriptions.Validate(sub); err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.subs = append(d.subs, sub)
	d.persistServers()
	d.mu.Unlock()
	d.broadcastSubs()
	return sub, nil
}

func (d *Daemon) hListSubscriptions(p json.RawMessage) (any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.subs == nil {
		return []*subscriptions.Subscription{}, nil
	}
	return d.subs, nil
}

// hEditSubscription updates subscription settings (rename, URL,
// enabled flag, auto-update interval in minutes, user agent).
func (d *Daemon) hEditSubscription(p json.RawMessage) (any, error) {
	var req struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		URL            string `json:"url"`
		Enabled        *bool  `json:"enabled"`
		UpdateInterval int    `json:"updateIntervalMin"`
		UserAgent      string `json:"userAgent"`
	}
	if err := json.Unmarshal(p, &req); err != nil || req.ID == "" {
		return nil, fmt.Errorf("bad edit request")
	}
	d.mu.Lock()
	for _, s := range d.subs {
		if s.ID == req.ID {
			if req.Name != "" {
				s.Name = req.Name
			}
			if req.URL != "" {
				s.URL = req.URL
				if err := subscriptions.Validate(s); err != nil {
					d.mu.Unlock()
					return nil, err
				}
			}
			if req.Enabled != nil {
				s.Enabled = *req.Enabled
			}
			if req.UpdateInterval >= 0 {
				s.UpdateInterval = time.Duration(req.UpdateInterval) * time.Minute
			}
			if req.UserAgent != "" {
				s.UserAgent = req.UserAgent
			}
			d.persistServers()
			d.mu.Unlock()
			d.broadcastSubs()
			return s, nil
		}
	}
	d.mu.Unlock()
	return nil, fmt.Errorf("subscription %q not found", req.ID)
}

func (d *Daemon) hUpdateSubscription(p json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if len(p) > 0 {
		_ = json.Unmarshal(p, &req)
	}
	n, err := d.refreshSubscriptions(req.ID)
	if err != nil {
		return nil, err
	}
	return map[string]int{"updated": n}, nil
}

func (d *Daemon) hRemoveSubscription(p json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(p, &req); err != nil {
		return nil, fmt.Errorf("bad remove request: %w", err)
	}
	d.mu.Lock()
	var subs []*subscriptions.Subscription
	for _, s := range d.subs {
		if s.ID != req.ID {
			subs = append(subs, s)
		}
	}
	var list []*servers.Server
	for _, s := range d.serverList {
		if s.SubscriptionID != req.ID {
			list = append(list, s)
		}
	}
	d.subs, d.serverList = subs, list
	d.persistServers()
	d.mu.Unlock()
	d.broadcastServers()
	d.broadcastSubs()
	return map[string]bool{"removed": true}, nil
}

func (d *Daemon) hGetStats(p json.RawMessage) (any, error) {
	if !d.machine.Connected() || d.tracker == nil {
		return map[string]any{"connected": false}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	snap, err := d.tracker.Sample(ctx)
	cancel()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"connected":  true,
		"upRate":     snap.UpRate,
		"downRate":   snap.DownRate,
		"upTotal":    snap.Up - d.sessionUp,
		"downTotal":  snap.Down - d.sessionDown,
		"durationMs": time.Since(d.connectAt).Milliseconds(),
	}, nil
}

// hUIGet returns persisted UI state (collapse maps etc.).
func (d *Daemon) hUIGet(p json.RawMessage) (any, error) {
	var state map[string]any
	if ok, err := d.store.Load(persistence.DocUIState, &state); err != nil {
		return nil, err
	} else if !ok {
		return map[string]any{}, nil
	}
	return state, nil
}

// hUIUpdate persists UI state.
func (d *Daemon) hUIUpdate(p json.RawMessage) (any, error) {
	var state map[string]any
	if err := json.Unmarshal(p, &state); err != nil {
		return nil, fmt.Errorf("bad ui state: %w", err)
	}
	if err := d.store.Save(persistence.DocUIState, &state); err != nil {
		return nil, err
	}
	return map[string]bool{"saved": true}, nil
}

func (d *Daemon) testServers(ids []string) {
	d.mu.Lock()
	var targets []*servers.Server
	if len(ids) == 0 {
		targets = d.serverList
	} else {
		want := map[string]bool{}
		for _, id := range ids {
			want[id] = true
		}
		for _, s := range d.serverList {
			if want[s.ID] {
				targets = append(targets, s)
			}
		}
	}
	timeout := time.Duration(d.settings.TestTimeoutMs) * time.Millisecond
	concurrency := d.settings.TestConcurrency
	d.mu.Unlock()

	opts := tester.Options{Timeout: timeout, Concurrency: concurrency, Retries: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for r := range tester.RunBulk(ctx, targets, opts, tester.TCPTestFunc(opts)) {
		d.mu.Lock()
		for _, s := range d.serverList {
			if s.ID == r.ServerID {
				tester.Outcome(s, r)
				d.ipc.Broadcast(ipc.EventHealth, map[string]any{
					"serverId": s.ID, "latencyMs": s.LatencyMs,
					"health": string(s.Health),
				})
				break
			}
		}
		d.persistServers()
		d.mu.Unlock()
	}
	d.broadcastServers()
}
