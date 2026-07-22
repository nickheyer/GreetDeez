package greetd

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nickheyer/greetdeez/internal/greetd/greetdtest"
)

func newTestClient(t *testing.T, timeout time.Duration, handler greetdtest.Handler) *Client {
	t.Helper()
	sock := greetdtest.Start(t, handler)
	t.Setenv("GREETD_SOCK", sock)
	c, err := NewClient(timeout)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func success() map[string]any {
	return map[string]any{"type": "success"}
}

func prompt(kind, msg string) map[string]any {
	return map[string]any{"type": "auth_message", "auth_message_type": kind, "auth_message": msg}
}

func authError(desc string) map[string]any {
	return map[string]any{"type": "error", "error_type": "auth_error", "description": desc}
}

func TestAuthenticateSuccess(t *testing.T) {
	c := newTestClient(t, 0, func(req map[string]any) map[string]any {
		switch req["type"] {
		case "cancel_session":
			return success()
		case "create_session":
			return prompt("secret", "Password:")
		case "post_auth_message_response":
			if req["response"] == "hunter2" {
				return success()
			}
			return authError("pam says no")
		}
		return authError("unexpected request")
	})

	res, err := c.Authenticate(context.Background(), "nick", "hunter2")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if len(res.Messages) != 0 {
		t.Fatalf("unexpected messages: %v", res.Messages)
	}
}

func TestAuthenticateWrongPassword(t *testing.T) {
	c := newTestClient(t, 0, func(req map[string]any) map[string]any {
		switch req["type"] {
		case "cancel_session":
			return success()
		case "create_session":
			return prompt("secret", "Password:")
		}
		return authError("pam_authenticate failed")
	})

	_, err := c.Authenticate(context.Background(), "nick", "wrong")
	if err == nil || !strings.Contains(err.Error(), "auth failed") {
		t.Fatalf("want auth failed error got %v", err)
	}
}

func TestAuthenticateCollectsInfoMessages(t *testing.T) {
	c := newTestClient(t, 0, func(req map[string]any) map[string]any {
		switch req["type"] {
		case "cancel_session":
			return success()
		case "create_session":
			return prompt("info", "maintenance at noon")
		case "post_auth_message_response":
			if req["response"] == nil {
				return prompt("secret", "Password:")
			}
			return success()
		}
		return authError("unexpected request")
	})

	res, err := c.Authenticate(context.Background(), "nick", "hunter2")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if len(res.Messages) != 1 || res.Messages[0] != "maintenance at noon" {
		t.Fatalf("want info message got %v", res.Messages)
	}
}

func TestTimeout(t *testing.T) {
	c := newTestClient(t, 50*time.Millisecond, func(req map[string]any) map[string]any {
		time.Sleep(2 * time.Second)
		return success()
	})

	start := time.Now()
	_, err := c.CreateSession(context.Background(), "nick")
	if err == nil {
		t.Fatal("want timeout error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("timeout took too long: %v", time.Since(start))
	}
}

func TestRecvRejectsHugeMessage(t *testing.T) {
	// raw server sends absurd length prefix
	sock := filepath.Join(t.TempDir(), "greetd.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return
		}
		data := make([]byte, binary.LittleEndian.Uint32(lenBuf))
		if _, err := io.ReadFull(conn, data); err != nil {
			return
		}
		huge := make([]byte, 4)
		binary.LittleEndian.PutUint32(huge, 16<<20)
		conn.Write(huge)
	}()

	t.Setenv("GREETD_SOCK", sock)
	c, err := NewClient(0)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	_, err = c.CreateSession(context.Background(), "nick")
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want too large error got %v", err)
	}
}
