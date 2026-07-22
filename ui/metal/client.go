package metal

import (
	"context"
	"time"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
	"github.com/nickheyer/greetdeez/pkg/rpc"
)

// Everything metal needs from greetdeez
type Backend interface {
	SystemInfo(ctx context.Context) (*pb.SystemInfo, error)
	Sessions(ctx context.Context) ([]*pb.Session, error)
	PowerCaps(ctx context.Context) (*pb.PowerCapabilities, error)
	State(ctx context.Context) (*pb.GreeterState, error)
	SaveState(ctx context.Context, user, session string) error
	BeginAuth(ctx context.Context, username string) (*pb.AuthStep, error)
	RespondAuth(ctx context.Context, response string) (*pb.AuthStep, error)
	CancelAuth(ctx context.Context) error
	StartSession(ctx context.Context, sess *pb.Session) error
	Power(ctx context.Context, action pb.PowerAction) error
}

const svc = "greetdeez.v1.GreeterService/"

type socketBackend struct {
	c *rpc.SocketClient
}

func DialBackend(path string, timeout time.Duration) (Backend, error) {
	c, err := rpc.DialSocket(path, timeout)
	if err != nil {
		return nil, err
	}
	return &socketBackend{c: c}, nil
}

func (b *socketBackend) SystemInfo(ctx context.Context) (*pb.SystemInfo, error) {
	var resp pb.GetSystemInfoResponse
	if err := b.c.Call(ctx, svc+"GetSystemInfo", &pb.GetSystemInfoRequest{}, &resp); err != nil {
		return nil, err
	}
	return resp.Info, nil
}

func (b *socketBackend) Sessions(ctx context.Context) ([]*pb.Session, error) {
	var resp pb.ListSessionsResponse
	if err := b.c.Call(ctx, svc+"ListSessions", &pb.ListSessionsRequest{}, &resp); err != nil {
		return nil, err
	}
	return resp.Sessions, nil
}

func (b *socketBackend) PowerCaps(ctx context.Context) (*pb.PowerCapabilities, error) {
	var resp pb.GetPowerCapabilitiesResponse
	if err := b.c.Call(ctx, svc+"GetPowerCapabilities", &pb.GetPowerCapabilitiesRequest{}, &resp); err != nil {
		return nil, err
	}
	return resp.Capabilities, nil
}

func (b *socketBackend) State(ctx context.Context) (*pb.GreeterState, error) {
	var resp pb.GetStateResponse
	if err := b.c.Call(ctx, svc+"GetState", &pb.GetStateRequest{}, &resp); err != nil {
		return nil, err
	}
	return resp.State, nil
}

func (b *socketBackend) SaveState(ctx context.Context, user, session string) error {
	var resp pb.SaveStateResponse
	return b.c.Call(ctx, svc+"SaveState",
		&pb.SaveStateRequest{State: &pb.GreeterState{LastUser: user, LastSession: session}}, &resp)
}

func (b *socketBackend) BeginAuth(ctx context.Context, username string) (*pb.AuthStep, error) {
	var step pb.AuthStep
	if err := b.c.Call(ctx, svc+"BeginAuth", &pb.BeginAuthRequest{Username: username}, &step); err != nil {
		return nil, err
	}
	return &step, nil
}

func (b *socketBackend) RespondAuth(ctx context.Context, response string) (*pb.AuthStep, error) {
	var step pb.AuthStep
	if err := b.c.Call(ctx, svc+"RespondAuth", &pb.RespondAuthRequest{Response: response}, &step); err != nil {
		return nil, err
	}
	return &step, nil
}

func (b *socketBackend) CancelAuth(ctx context.Context) error {
	var resp pb.CancelAuthResponse
	return b.c.Call(ctx, svc+"CancelAuth", &pb.CancelAuthRequest{}, &resp)
}

func (b *socketBackend) StartSession(ctx context.Context, sess *pb.Session) error {
	var resp pb.StartSessionResponse
	if err := b.c.Call(ctx, svc+"StartSession", &pb.StartSessionRequest{Session: sess}, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return errString(resp.Error)
	}
	return nil
}

func (b *socketBackend) Power(ctx context.Context, action pb.PowerAction) error {
	var resp pb.ExecutePowerActionResponse
	if err := b.c.Call(ctx, svc+"ExecutePowerAction", &pb.ExecutePowerActionRequest{Action: action}, &resp); err != nil {
		return err
	}
	if !resp.Ok {
		return errString(resp.Error)
	}
	return nil
}

type errString string

func (e errString) Error() string {
	if e == "" {
		return "unknown error"
	}
	return string(e)
}
