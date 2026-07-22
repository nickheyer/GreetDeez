package rpc_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
	"github.com/nickheyer/greetdeez/pkg/rpc"
)

type fakeGreeter struct {
	pb.UnimplementedGreeterServiceServer
	lastUsername string
}

func (f *fakeGreeter) GetSystemInfo(context.Context, *pb.GetSystemInfoRequest) (*pb.GetSystemInfoResponse, error) {
	return &pb.GetSystemInfoResponse{Info: &pb.SystemInfo{Hostname: "sockethost"}}, nil
}

func (f *fakeGreeter) BeginAuth(_ context.Context, req *pb.BeginAuthRequest) (*pb.AuthStep, error) {
	f.lastUsername = req.Username
	return &pb.AuthStep{Prompt: &pb.AuthPrompt{Type: pb.PromptType_PROMPT_TYPE_SECRET, Message: "Password:"}}, nil
}

func startServer(t *testing.T, srv pb.GreeterServiceServer) string {
	t.Helper()
	d := rpc.NewDispatcher(false)
	pb.RegisterGreeterServiceServer(d, srv)

	sockPath := filepath.Join(t.TempDir(), "rpc.sock")
	s, err := rpc.ServeSocket(sockPath, d)
	if err != nil {
		t.Fatalf("ServeSocket: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return sockPath
}

func TestSocketRoundTrip(t *testing.T) {
	fake := &fakeGreeter{}
	sockPath := startServer(t, fake)

	c, err := rpc.DialSocket(sockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("DialSocket: %v", err)
	}
	defer c.Close()

	var info pb.GetSystemInfoResponse
	if err := c.Call(context.Background(), "greetdeez.v1.GreeterService/GetSystemInfo",
		&pb.GetSystemInfoRequest{}, &info); err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	if info.Info.GetHostname() != "sockethost" {
		t.Errorf("hostname = %q, want sockethost", info.Info.GetHostname())
	}

	var step pb.AuthStep
	if err := c.Call(context.Background(), "greetdeez.v1.GreeterService/BeginAuth",
		&pb.BeginAuthRequest{Username: "nick"}, &step); err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}
	if fake.lastUsername != "nick" {
		t.Errorf("server saw username %q, want nick", fake.lastUsername)
	}
	if step.Prompt.GetType() != pb.PromptType_PROMPT_TYPE_SECRET {
		t.Errorf("prompt type = %v, want SECRET", step.Prompt.GetType())
	}
}

func TestSocketSequentialCalls(t *testing.T) {
	sockPath := startServer(t, &fakeGreeter{})

	c, err := rpc.DialSocket(sockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("DialSocket: %v", err)
	}
	defer c.Close()

	for i := 0; i < 50; i++ {
		var info pb.GetSystemInfoResponse
		if err := c.Call(context.Background(), "greetdeez.v1.GreeterService/GetSystemInfo",
			&pb.GetSystemInfoRequest{}, &info); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestSocketUnknownMethod(t *testing.T) {
	sockPath := startServer(t, &fakeGreeter{})

	c, err := rpc.DialSocket(sockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("DialSocket: %v", err)
	}
	defer c.Close()

	var info pb.GetSystemInfoResponse
	err = c.Call(context.Background(), "greetdeez.v1.GreeterService/Nope",
		&pb.GetSystemInfoRequest{}, &info)
	if err == nil || !strings.Contains(err.Error(), "unknown method") {
		t.Fatalf("err = %v, want unknown method", err)
	}
}

func TestSocketErrorPropagates(t *testing.T) {
	// UnimplementedGreeterServiceServer errors on everything not overridden
	sockPath := startServer(t, &fakeGreeter{})

	c, err := rpc.DialSocket(sockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("DialSocket: %v", err)
	}
	defer c.Close()

	var resp pb.LoginResponse
	err = c.Call(context.Background(), "greetdeez.v1.GreeterService/Login", &pb.LoginRequest{}, &resp)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("err = %v, want not implemented", err)
	}
}

func TestSocketConcurrentClients(t *testing.T) {
	sockPath := startServer(t, &fakeGreeter{})

	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			c, err := rpc.DialSocket(sockPath, 5*time.Second)
			if err != nil {
				errCh <- err
				return
			}
			defer c.Close()
			for j := 0; j < 20; j++ {
				var info pb.GetSystemInfoResponse
				if err := c.Call(context.Background(), "greetdeez.v1.GreeterService/GetSystemInfo",
					&pb.GetSystemInfoRequest{}, &info); err != nil {
					errCh <- fmt.Errorf("call %d: %w", j, err)
					return
				}
			}
			errCh <- nil
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("client failed: %v", err)
		}
	}
}
