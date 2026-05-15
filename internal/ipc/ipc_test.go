package ipc_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/kpango/unk/internal/ipc"
)

func TestServe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	path := t.TempDir() + "/unk-test.sock"
	ch := make(chan ipc.Cmd, 8)
	go ipc.Serve(ctx, path, ch)

	// Give the server a tick to start listening.
	time.Sleep(20 * time.Millisecond)

	send := func(t *testing.T, cmd ipc.Cmd) map[string]any {
		t.Helper()
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()
		if err := json.NewEncoder(conn).Encode(cmd); err != nil {
			t.Fatalf("encode: %v", err)
		}
		var resp map[string]any
		if err := json.NewDecoder(conn).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp
	}

	t.Run("navigate", func(t *testing.T) {
		resp := send(t, ipc.Cmd{Type: ipc.CmdNavigate, File: "src/main.go", Unk: 2})
		if resp["ok"] != true {
			t.Fatalf("expected ok=true, got %v", resp)
		}
		got := <-ch
		if got.Type != ipc.CmdNavigate || got.File != "src/main.go" || got.Unk != 2 {
			t.Fatalf("unexpected cmd: %+v", got)
		}
	})

	t.Run("reload", func(t *testing.T) {
		resp := send(t, ipc.Cmd{Type: ipc.CmdReload})
		if resp["ok"] != true {
			t.Fatalf("expected ok=true, got %v", resp)
		}
		got := <-ch
		if got.Type != ipc.CmdReload {
			t.Fatalf("unexpected cmd type: %v", got.Type)
		}
	})

	t.Run("ping", func(t *testing.T) {
		resp := send(t, ipc.Cmd{Type: ipc.CmdPing})
		if resp["ok"] != true {
			t.Fatalf("expected ok=true, got %v", resp)
		}
		got := <-ch
		if got.Type != ipc.CmdPing {
			t.Fatalf("unexpected cmd type: %v", got.Type)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		conn, _ := net.Dial("unix", path)
		defer conn.Close()
		conn.Write([]byte("not-json\n"))
		var resp map[string]any
		json.NewDecoder(conn).Decode(&resp)
		if resp["ok"] != false {
			t.Fatalf("expected ok=false for invalid JSON, got %v", resp)
		}
	})

	t.Run("context_cancel_closes_socket", func(t *testing.T) {
		cancel()
		time.Sleep(20 * time.Millisecond)
		_, err := net.Dial("unix", path)
		if err == nil {
			t.Fatal("expected connection to fail after ctx cancel")
		}
	})
}
