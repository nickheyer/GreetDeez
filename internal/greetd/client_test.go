package greetd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nickheyer/greetdeez/internal/mockgreetd"
)

func startMock(t *testing.T) *mockgreetd.Server {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "greetd.sock")
	srv := mockgreetd.New(sock, map[string]string{
		"alice": "secret",
		"bob":   "password",
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("mock start: %v", err)
	}
	t.Setenv("GREETD_SOCK", sock)
	t.Cleanup(srv.Stop)
	return srv
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestAuthenticate_Success(t *testing.T) {
	startMock(t)
	c := newTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Authenticate(ctx, "alice", "secret"); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	startMock(t)
	c := newTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Authenticate(ctx, "alice", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestAuthenticate_UnknownUser(t *testing.T) {
	startMock(t)
	c := newTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Authenticate(ctx, "nobody", "test")
	if err == nil {
		t.Fatal("expected error for unknown user, got nil")
	}
}

func TestAuthenticate_Timeout(t *testing.T) {
	startMock(t)

	// Connect but use an already-cancelled context
	sock := os.Getenv("GREETD_SOCK")
	t.Setenv("GREETD_SOCK", sock)
	c := newTestClient(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	err := c.Authenticate(ctx, "alice", "secret")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestStartSession(t *testing.T) {
	startMock(t)
	c := newTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Authenticate(ctx, "alice", "secret"); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	resp, err := c.StartSession(ctx, []string{"sway"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if resp.Type != "success" {
		t.Errorf("StartSession response type = %q, want %q", resp.Type, "success")
	}
}

func TestCancelSession(t *testing.T) {
	startMock(t)
	c := newTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a session, then cancel it
	resp, err := c.CreateSession(ctx, "alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if resp.Type != "auth_message" {
		t.Fatalf("CreateSession response type = %q, want %q", resp.Type, "auth_message")
	}

	resp, err = c.CancelSession(ctx)
	if err != nil {
		t.Fatalf("CancelSession: %v", err)
	}
	if resp.Type != "success" {
		t.Errorf("CancelSession response type = %q, want %q", resp.Type, "success")
	}
}
