// Package ipc is the versioned communication boundary between the
// Flutter UI and the Go backend (spec §5).
//
// Transport: newline-delimited JSON frames over a unix-domain socket
// (Linux) — reliable, credential-friendly and dependency-free. Every
// frame carries a protocol version so mismatched UI/backend pairs fail
// loudly instead of misbehaving:
//
//	{"v":"1","id":"...","type":"<method|event>","payload":{...}}
//
// The UI never parses shell output; all state arrives as structured
// messages defined here.
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Version is the IPC protocol version. Bump on any breaking change.
const Version = "1"

// Message is one protocol frame.
type Message struct {
	V       string          `json:"v"`
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Method names (requests from UI to backend).
const (
	MethodPing               = "ping"
	MethodGetState           = "state.get"
	MethodConnect            = "connection.connect"
	MethodDisconnect         = "connection.disconnect"
	MethodSelectServer       = "servers.select"
	MethodListServers        = "servers.list"
	MethodTestServer         = "servers.test"
	MethodAddServer          = "servers.add"
	MethodUpdateServer       = "servers.update"
	MethodRemoveServer       = "servers.remove"
	MethodDuplicateServer    = "servers.duplicate"
	MethodExportServer       = "servers.export"
	MethodListSubscriptions  = "subscriptions.list"
	MethodEditSubscription   = "subscriptions.edit"
	MethodAddSubscription    = "subscriptions.add"
	MethodUpdateSubscription = "subscriptions.update"
	MethodRemoveSubscription = "subscriptions.remove"
	MethodGetSettings        = "settings.get"
	MethodUpdateSettings     = "settings.update"
	MethodGetLogs            = "logs.get"
	MethodClearLogs          = "logs.clear"
	MethodGetStats           = "stats.get"
	MethodUIGet              = "ui.get"
	MethodUIUpdate           = "ui.update"
	MethodUpdateCheck        = "update.check"
	MethodUpdateStatus       = "update.status"
	MethodUpdateDownload     = "update.download"
	MethodUpdateCancel       = "update.cancel"
	MethodUpdateInstall      = "update.install"
	MethodUpdateDismiss      = "update.dismiss"
)

// Event names (backend to UI broadcasts).
const (
	EventStateChanged  = "event.state"
	EventServers       = "event.servers"
	EventStats         = "event.stats"
	EventLog           = "event.log"
	EventHealth        = "event.health"
	EventSubscriptions = "event.subscriptions"
	EventUpdate        = "event.update"
)

// Handler answers one request. The returned value is marshaled as the
// response payload; a non-nil error becomes an error frame.
type Handler func(payload json.RawMessage) (any, error)

// Server is a unix-socket JSON-RPC-style endpoint with event broadcast.
//
// Handlers run concurrently (one goroutine per request) so a slow call
// such as connection.connect (core startup + TUN/routing verification,
// tens of seconds) or subscriptions.update (minutes) never blocks fast
// calls like ping or state.get behind it. The UI times a stalled call
// out and reports "Backend unavailable" — serial handling therefore
// turned every connect into a false backend-death report. Responses
// funnel through the single per-connection writer, so frames never
// interleave; clients match responses by id and already tolerate
// out-of-order arrival.
type Server struct {
	mu       sync.Mutex
	handlers map[string]Handler
	subs     map[*conn]struct{}

	listener net.Listener
	quit     chan struct{}
	closeMu  sync.Mutex
	closed   bool
	wg       sync.WaitGroup
}

// NewServer creates an unbound server.
func NewServer() *Server {
	return &Server{handlers: map[string]Handler{}, subs: map[*conn]struct{}{}}
}

// Handle registers a method handler.
func (s *Server) Handle(method string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

// Serve accepts connections on l until Close.
func (s *Server) Serve(l net.Listener) {
	s.closeMu.Lock()
	s.listener = l
	s.quit = make(chan struct{})
	quit := s.quit
	s.closeMu.Unlock()
	for {
		c, err := l.Accept()
		if err != nil {
			select {
			case <-quit:
				return
			default:
				continue
			}
		}
		s.wg.Add(1)
		go s.serveConn(c)
	}
}

// Close stops the server and all connections. It is idempotent: the
// daemon calls it once on shutdown, and tests may defer it alongside
// error paths.
func (s *Server) Close() error {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	quit := s.quit
	l := s.listener
	s.closeMu.Unlock()

	if quit != nil {
		close(quit)
	}
	var err error
	if l != nil {
		err = l.Close()
	}
	s.mu.Lock()
	for c := range s.subs {
		c.net.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return err
}

// Broadcast sends an event to every connected client. Slow clients are
// dropped rather than allowed to stall the backend.
func (s *Server) Broadcast(event string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.subs {
		select {
		case c.send <- Message{V: Version, Type: event, Payload: raw}:
		default:
			c.net.Close()
			delete(s.subs, c)
		}
	}
}

type conn struct {
	net  net.Conn
	send chan Message
	// done closes when the connection tears down; in-flight handler
	// goroutines stop waiting on a dead writer via done instead of
	// leaking. wg tracks those goroutines so teardown closes send
	// only after the last response was queued (never send-on-closed).
	done chan struct{}
	wg   sync.WaitGroup
}

func (s *Server) serveConn(nc net.Conn) {
	defer s.wg.Done()
	c := &conn{net: nc, send: make(chan Message, 256), done: make(chan struct{})}
	s.mu.Lock()
	s.subs[c] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, c)
		s.mu.Unlock()
		close(c.done)
		c.wg.Wait()
		close(c.send)
		nc.Close()
	}()

	go func() {
		w := bufio.NewWriter(nc)
		for m := range c.send {
			raw, err := json.Marshal(m)
			if err != nil {
				continue
			}
			raw = append(raw, '\n')
			nc.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := w.Write(raw); err != nil {
				return
			}
			w.Flush()
		}
	}()

	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var req Message
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			c.send <- Message{V: Version, ID: req.ID, Error: "bad frame"}
			continue
		}
		if req.V != Version {
			c.send <- Message{V: Version, ID: req.ID, Error: fmt.Sprintf("ipc version mismatch: got %q want %q", req.V, Version)}
			continue
		}
		s.mu.Lock()
		h := s.handlers[req.Type]
		s.mu.Unlock()
		if h == nil {
			c.send <- Message{V: Version, ID: req.ID, Type: req.Type, Error: "unknown method " + req.Type}
			continue
		}
		// Slow handlers must not stall the read loop: ping/state.get
		// arriving during a connect or subscription refresh are
		// answered by their own goroutine. Responses carry the
		// request id, so out-of-order arrival is fine. Lifetime is
		// scoped to this connection (c.wg): teardown waits for
		// in-flight handlers before closing send, so a response can
		// never land on a closed channel.
		c.wg.Add(1)
		go func(req Message, h Handler) {
			defer c.wg.Done()
			out, err := h(req.Payload)
			resp := Message{V: Version, ID: req.ID, Type: req.Type}
			if err != nil {
				resp.Error = err.Error()
			} else if out != nil {
				raw, merr := json.Marshal(out)
				if merr != nil {
					resp.Error = merr.Error()
				} else {
					resp.Payload = raw
				}
			}
			select {
			case c.send <- resp:
			case <-c.done:
			}
		}(req, h)
	}
}

// Client is a blocking unix-socket client for tests and the daemon CLI.
type Client struct {
	mu   sync.Mutex
	conn net.Conn
	rd   *bufio.Scanner
	next int
}

// Dial connects to path.
func Dial(path string) (*Client, error) {
	c, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	return &Client{conn: c, rd: sc}, nil
}

// Close the client connection.
func (c *Client) Close() error { return c.conn.Close() }

// Call performs one request/response round trip.
func (c *Client) Call(method string, payload, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		raw = b
	}
	c.next++
	req := Message{V: Version, ID: fmt.Sprint(c.next), Type: method, Payload: raw}
	frame, err := json.Marshal(req)
	if err != nil {
		return err
	}
	c.conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := c.conn.Write(append(frame, '\n')); err != nil {
		return err
	}
	for c.rd.Scan() {
		var resp Message
		if err := json.Unmarshal(c.rd.Bytes(), &resp); err != nil {
			return err
		}
		if resp.ID != req.ID {
			// An event frame raced the response; ignore and keep waiting.
			continue
		}
		if resp.Error != "" {
			return fmt.Errorf("ipc %s: %s", method, resp.Error)
		}
		if result != nil && len(resp.Payload) > 0 {
			return json.Unmarshal(resp.Payload, result)
		}
		return nil
	}
	return fmt.Errorf("ipc %s: connection closed", method)
}
