package ipc

import (
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := NewServer()
	s.Handle(MethodPing, func(p json.RawMessage) (any, error) {
		return map[string]string{"pong": "ok"}, nil
	})
	s.Handle("fail", func(p json.RawMessage) (any, error) {
		return nil, errors.New("boom")
	})
	path := filepath.Join(t.TempDir(), "ipc.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(l)
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestPing(t *testing.T) {
	_, path := testServer(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var out map[string]string
	if err := c.Call(MethodPing, nil, &out); err != nil {
		t.Fatal(err)
	}
	if out["pong"] != "ok" {
		t.Fatalf("bad pong: %v", out)
	}
}

func TestErrors(t *testing.T) {
	_, path := testServer(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Call("fail", nil, nil); err == nil {
		t.Errorf("expected handler error")
	}
	if err := c.Call("nope", nil, nil); err == nil {
		t.Errorf("expected unknown-method error")
	}
}

func TestBroadcast(t *testing.T) {
	s, path := testServer(t)
	rawConn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer rawConn.Close()
	// Give the server a moment to register the connection.
	time.Sleep(100 * time.Millisecond)
	s.Broadcast(EventStats, map[string]int{"up": 1})
	rawConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, err := rawConn.Read(buf)
	if err != nil {
		t.Fatalf("no event received: %v", err)
	}
	var m Message
	if err := json.Unmarshal(buf[:n], &m); err != nil {
		t.Fatal(err)
	}
	if m.V != Version || m.Type != EventStats {
		t.Fatalf("bad event frame: %+v", m)
	}
}
