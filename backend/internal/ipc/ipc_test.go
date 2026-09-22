package ipc

import (
	"bufio"
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

// A slow handler (connect, subscription refresh) must not stall fast
// calls behind it: ping during a blocked request answers immediately.
// Regression test for the "Backend unavailable on every connect" bug,
// where serial handling pushed every slow call past the UI timeout.
//
// A raw connection is used (not Client.Call, which serializes calls
// on its own mutex): both frames go down one connection, and the ping
// response must arrive while the slow handler is still blocked.
func TestSlowHandlerDoesNotBlockPing(t *testing.T) {
	s, path := testServer(t)
	release := make(chan struct{})
	s.Handle("slow", func(p json.RawMessage) (any, error) {
		<-release
		return map[string]string{"done": "yes"}, nil
	})
	nc, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	send := func(id, method string) {
		t.Helper()
		frame, _ := json.Marshal(Message{V: Version, ID: id, Type: method})
		nc.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if _, err := nc.Write(append(frame, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	send("1", "slow")
	send("2", MethodPing)

	nc.SetReadDeadline(time.Now().Add(5 * time.Second))
	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	sawPing, sawSlow := false, false
	pingFirst := false
	for !(sawPing && sawSlow) {
		if !sc.Scan() {
			t.Fatalf("stalled: ping=%v slow=%v err=%v", sawPing, sawSlow, sc.Err())
		}
		var m Message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		switch m.ID {
		case "2":
			sawPing = true
			if !sawSlow {
				pingFirst = true
			}
			// Let the slow call finish now that ping proved itself.
			select {
			case <-release:
			default:
				close(release)
			}
		case "1":
			sawSlow = true
			if m.Error != "" {
				t.Fatalf("slow call failed: %s", m.Error)
			}
		}
	}
	if !pingFirst {
		t.Fatal("ping answered only after the slow handler finished")
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
