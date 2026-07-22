package server

import (
	"context"
	"sync/atomic"
	"testing"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
	"github.com/nickheyer/greetdeez/internal/config"
	"github.com/nickheyer/greetdeez/internal/greetd"
	"github.com/nickheyer/greetdeez/internal/greetd/greetdtest"
)

func newTestServer(t *testing.T, handler greetdtest.Handler) *Server {
	t.Helper()
	sock := greetdtest.Start(t, handler)
	t.Setenv("GREETD_SOCK", sock)
	client, err := greetd.NewClient(0)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return New(client, &config.Config{})
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

// password then otp like a real mfa stack
func mfaHandler(req map[string]any) map[string]any {
	switch req["type"] {
	case "cancel_session":
		return success()
	case "create_session":
		return prompt("secret", "Password:")
	case "post_auth_message_response":
		switch req["response"] {
		case "hunter2":
			return prompt("secret", "OTP:")
		case "123456":
			return success()
		}
		return authError("bad credential")
	}
	return authError("unexpected request")
}

func TestBeginAuthMFAFlow(t *testing.T) {
	s := newTestServer(t, mfaHandler)
	ctx := context.Background()

	step, err := s.BeginAuth(ctx, &pb.BeginAuthRequest{Username: "nick"})
	if err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}
	if step.Prompt == nil || step.Prompt.Type != pb.PromptType_PROMPT_TYPE_SECRET || step.Prompt.Message != "Password:" {
		t.Fatalf("want password prompt got %+v", step)
	}

	step, err = s.RespondAuth(ctx, &pb.RespondAuthRequest{Response: "hunter2"})
	if err != nil {
		t.Fatalf("RespondAuth: %v", err)
	}
	if step.Prompt == nil || step.Prompt.Message != "OTP:" {
		t.Fatalf("want otp prompt got %+v", step)
	}

	step, err = s.RespondAuth(ctx, &pb.RespondAuthRequest{Response: "123456"})
	if err != nil {
		t.Fatalf("RespondAuth: %v", err)
	}
	if !step.Success {
		t.Fatalf("want success got %+v", step)
	}
}

func TestBeginAuthCollectsInfo(t *testing.T) {
	s := newTestServer(t, func(req map[string]any) map[string]any {
		switch req["type"] {
		case "cancel_session":
			return success()
		case "create_session":
			return prompt("info", "welcome to the machine")
		case "post_auth_message_response":
			if req["response"] == nil {
				return prompt("secret", "Password:")
			}
			return success()
		}
		return authError("unexpected request")
	})

	step, err := s.BeginAuth(context.Background(), &pb.BeginAuthRequest{Username: "nick"})
	if err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}
	if len(step.Messages) != 1 || step.Messages[0] != "welcome to the machine" {
		t.Fatalf("want info message got %+v", step)
	}
	if step.Prompt == nil || step.Prompt.Type != pb.PromptType_PROMPT_TYPE_SECRET {
		t.Fatalf("want secret prompt got %+v", step)
	}
}

func TestAuthFailureIsUniform(t *testing.T) {
	s := newTestServer(t, mfaHandler)
	ctx := context.Background()

	if _, err := s.BeginAuth(ctx, &pb.BeginAuthRequest{Username: "nick"}); err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}
	step, err := s.RespondAuth(ctx, &pb.RespondAuthRequest{Response: "wrong"})
	if err != nil {
		t.Fatalf("RespondAuth: %v", err)
	}
	// pam detail stays in logs not ui
	if step.Error != authFailedMsg {
		t.Fatalf("want uniform error got %q", step.Error)
	}

	resp, err := s.Authenticate(ctx, &pb.AuthenticateRequest{Username: "nick", Password: "wrong"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if resp.Success || resp.Error != authFailedMsg {
		t.Fatalf("want uniform error got %+v", resp)
	}
}

func TestCancelAuth(t *testing.T) {
	var cancels atomic.Int64
	s := newTestServer(t, func(req map[string]any) map[string]any {
		switch req["type"] {
		case "cancel_session":
			cancels.Add(1)
			return success()
		case "create_session":
			return prompt("secret", "Password:")
		}
		return authError("unexpected request")
	})
	ctx := context.Background()

	if _, err := s.BeginAuth(ctx, &pb.BeginAuthRequest{Username: "nick"}); err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}
	before := cancels.Load()
	resp, err := s.CancelAuth(ctx, &pb.CancelAuthRequest{})
	if err != nil || !resp.Ok {
		t.Fatalf("CancelAuth: %v %+v", err, resp)
	}
	if cancels.Load() != before+1 {
		t.Fatal("cancel_session never reached greetd")
	}
}

func TestDevModeAuthSucceeds(t *testing.T) {
	s := New(nil, &config.Config{})
	step, err := s.BeginAuth(context.Background(), &pb.BeginAuthRequest{Username: "nick"})
	if err != nil || !step.Success {
		t.Fatalf("dev BeginAuth: %v %+v", err, step)
	}
}

func TestBuildSessionEnv(t *testing.T) {
	env := buildSessionEnv(pb.SessionType_SESSION_TYPE_WAYLAND, "GNOME;GNOME-Flashback")
	want := map[string]bool{
		"XDG_SESSION_TYPE=wayland":                  true,
		"XDG_SESSION_DESKTOP=GNOME:GNOME-Flashback": true,
		"XDG_CURRENT_DESKTOP=GNOME:GNOME-Flashback": true,
		"DESKTOP_SESSION=GNOME:GNOME-Flashback":     true,
	}
	if len(env) != len(want) {
		t.Fatalf("env size mismatch: %v", env)
	}
	for _, e := range env {
		if !want[e] {
			t.Fatalf("unexpected env entry %q in %v", e, env)
		}
	}
}
