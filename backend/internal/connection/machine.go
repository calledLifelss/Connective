// Package connection implements Connective's centralized connection
// state machine. The UI must always reflect actual backend state, so every
// lifecycle transition (connect, failover, network change, disconnect)
// flows through this machine and emits events for the IPC layer.
package connection

import (
	"fmt"
	"sync"
	"time"
)

// State is a connection lifecycle state.
type State string

const (
	StDisconnected    State = "disconnected"
	StTesting         State = "testing"
	StSelecting       State = "selecting"
	StConnecting      State = "connecting"
	StStartingCore    State = "starting-core"
	StInitializingTUN State = "initializing-tun"
	StApplyingRouting State = "applying-routing"
	StConnected       State = "connected"
	StDegraded        State = "degraded"
	StReconnecting    State = "reconnecting"
	StSwitchingServer State = "switching-server"
	StDisconnecting   State = "disconnecting"
	StError           State = "error"
)

// Event describes a state transition for observers (UI, logs).
type Event struct {
	From   State
	To     State
	At     time.Time
	Reason string
	Server string // active server ID, if any
}

// allowed transitions of the machine.
var allowed = map[State][]State{
	StDisconnected:    {StTesting, StSelecting, StConnecting},
	StTesting:         {StSelecting, StDisconnected, StError},
	StSelecting:       {StConnecting, StDisconnected, StError},
	StConnecting:      {StStartingCore, StDisconnected, StError},
	StStartingCore:    {StInitializingTUN, StReconnecting, StDisconnecting, StError},
	StInitializingTUN: {StApplyingRouting, StReconnecting, StDisconnecting, StError},
	StApplyingRouting: {StConnected, StReconnecting, StDisconnecting, StError},
	StConnected:       {StDegraded, StSwitchingServer, StReconnecting, StDisconnecting, StError},
	StDegraded:        {StConnected, StSwitchingServer, StReconnecting, StDisconnecting, StError},
	StReconnecting:    {StSelecting, StConnecting, StConnected, StDisconnecting, StError},
	StSwitchingServer: {StConnecting, StConnected, StDegraded, StDisconnecting, StError},
	StDisconnecting:   {StDisconnected, StError},
	StError:           {StDisconnected, StSelecting, StReconnecting},
}

// Machine is a goroutine-safe connection state machine.
type Machine struct {
	mu        sync.Mutex
	state     State
	serverID  string
	listeners []func(Event)
	history   []Event
}

// NewMachine starts in StDisconnected.
func NewMachine() *Machine { return &Machine{state: StDisconnected} }

// State returns the current state.
func (m *Machine) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// ServerID returns the active server ID (may be empty).
func (m *Machine) ServerID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.serverID
}

// Subscribe registers an event listener (called synchronously).
func (m *Machine) Subscribe(fn func(Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, fn)
}

// Transition moves to a new state if the edge is legal.
func (m *Machine) Transition(to State, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, next := range allowed[m.state] {
		if next == to {
			ev := Event{From: m.state, To: to, At: time.Now(), Reason: reason, Server: m.serverID}
			m.state = to
			m.history = append(m.history, ev)
			for _, fn := range m.listeners {
				fn(ev)
			}
			return nil
		}
	}
	return fmt.Errorf("illegal connection transition %q -> %q", m.state, to)
}

// SetServer records the active server ID.
func (m *Machine) SetServer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serverID = id
}

// Reset forces the machine to StDisconnected, emitting an event. It is
// an operator safety valve for wedged states (e.g. a start interrupted
// by shutdown); normal flows must use Transition.
func (m *Machine) Reset(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StDisconnected {
		return
	}
	ev := Event{From: m.state, To: StDisconnected, At: time.Now(), Reason: reason, Server: m.serverID}
	m.state = StDisconnected
	m.history = append(m.history, ev)
	for _, fn := range m.listeners {
		fn(ev)
	}
}

// Connected reports whether traffic is flowing (connected or degraded).
func (m *Machine) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == StConnected || m.state == StDegraded
}

// History returns a copy of the transition log.
func (m *Machine) History() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Event(nil), m.history...)
}
