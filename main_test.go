package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/its-the-vibe/NodeWorld/sim"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "nodeworld")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func TestServeInvalidCombinations(t *testing.T) {
	bin := buildBinary(t)
	for _, args := range [][]string{
		{"--serve"},
		{"--serve", "--ws", "localhost:0", "--days", "3"},
		{"--serve", "--ws", "localhost:0", "--output", "x.json"},
	} {
		cmd := exec.Command(bin, append([]string{"--config", "examples/sample.yaml"}, args...)...)
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Errorf("%v: expected failure, got %s", args, out)
		}
	}
}

func TestServeLifecycle(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--config", "examples/sample.yaml", "--serve", "--ws", "127.0.0.1:0")
	cmd.Stdin = strings.NewReader("") // immediate EOF
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	defer func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
		}
	}()
	reader := bufio.NewReader(stderr)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	addr := strings.TrimSpace(strings.TrimPrefix(line, "WebSocket listening on"))
	go func() { _, _ = reader.WriteTo(os.Stderr) }()

	time.Sleep(300 * time.Millisecond) // stdin EOF must not stop the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	read := func(c *websocket.Conn) sim.Event {
		var e sim.Event
		if err := wsjson.Read(ctx, c, &e); err != nil {
			t.Fatal(err)
		}
		return e
	}
	conn, _, err := websocket.Dial(ctx, "ws://"+addr+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	if e := read(conn); e.Type != "snapshot" || e.Snapshot.SimDay != 0 {
		t.Fatalf("snapshot: %+v", e)
	}
	wsjson.Write(ctx, conn, map[string]any{"type": "step", "days": 2})
	for e := read(conn); !(e.Type == "day_tick" && e.SimDay == 2); e = read(conn) {
	}
	wsjson.Write(ctx, conn, map[string]any{"type": "reset"})
	for e := read(conn); !(e.Type == "snapshot" && e.Snapshot.SimDay == 0); e = read(conn) {
	}
	conn.CloseNow()

	conn, _, err = websocket.Dial(ctx, "ws://"+addr+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if e := read(conn); e.Type != "snapshot" {
		t.Fatalf("reconnect snapshot: %+v", e)
	}

	cmd.Process.Signal(syscall.SIGTERM)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not exit on SIGTERM")
	}
}
