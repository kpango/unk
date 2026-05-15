// Package ipc provides a lightweight Unix-domain-socket server that lets
// external tools control a running unk TUI. Each connection sends
// newline-delimited JSON commands; the server responds with a single JSON
// line per command and forwards the decoded Cmd onto a Go channel consumed
// by the TUI's BubbleTea update loop.
//
// Protocol (newline-delimited JSON):
//
//	→  {"type":"navigate","file":"src/main.go","unk":2}
//	←  {"ok":true}
//
//	→  {"type":"reload"}
//	←  {"ok":true}
//
//	→  {"type":"ping"}
//	←  {"ok":true}
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// CmdType identifies the requested action.
type CmdType string

const (
	CmdNavigate CmdType = "navigate"
	CmdReload   CmdType = "reload"
	CmdPing     CmdType = "ping"
)

// Cmd is a single decoded IPC request. Fields beyond Type are action-specific.
type Cmd struct {
	Type CmdType `json:"type"`

	// navigate: jump to a specific file and unk or line.
	File    string `json:"file,omitempty"`
	Unk    int    `json:"unk,omitempty"`    // 1-indexed; 0 means first unk
	OldLine int    `json:"oldLine,omitempty"` // alternative to Unk
	NewLine int    `json:"newLine,omitempty"` // alternative to Unk
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// SocketPath returns the default Unix socket path for a given PID.
func SocketPath(pid int) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("unk-%d.sock", pid))
}

// Serve listens on path and forwards decoded Cmds onto ch. It removes a stale
// socket file on entry and again on exit. Serve blocks until ctx is cancelled
// or an accept error occurs; run it in a goroutine.
func Serve(ctx context.Context, path string, ch chan<- Cmd) {
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return
	}
	defer os.Remove(path)
	defer ln.Close()

	// Close the listener when the context is cancelled so Accept() unblocks.
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleConn(conn, ch)
	}
}

func handleConn(conn net.Conn, ch chan<- Cmd) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	enc := json.NewEncoder(conn)

	for sc.Scan() {
		var cmd Cmd
		if err := json.Unmarshal(sc.Bytes(), &cmd); err != nil {
			_ = enc.Encode(response{OK: false, Error: "invalid json"})
			continue
		}
		if cmd.Type == "" {
			_ = enc.Encode(response{OK: false, Error: "missing type"})
			continue
		}
		// Non-blocking send: if the TUI channel is full we reject the command
		// rather than blocking the connection handler.
		select {
		case ch <- cmd:
			_ = enc.Encode(response{OK: true})
		default:
			_ = enc.Encode(response{OK: false, Error: "busy"})
		}
	}
}
