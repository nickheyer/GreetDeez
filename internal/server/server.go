package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
	"github.com/nickheyer/greetdeez/internal/config"
	"github.com/nickheyer/greetdeez/internal/greetd"
	"github.com/nickheyer/greetdeez/internal/sessions"
	"github.com/nickheyer/greetdeez/internal/state"
	"github.com/nickheyer/greetdeez/pkg/logs"
)

type Server struct {
	pb.UnimplementedGreeterServiceServer
	client *greetd.Client
	cfg    *config.Config
	logs   *logs.LogCapture
}

func New(client *greetd.Client, cfg *config.Config, logs *logs.LogCapture) *Server {
	return &Server{client: client, cfg: cfg, logs: logs}
}

func (s *Server) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	if s.client == nil {
		slog.Debug("dev: createSession", "username", req.Username)
		return &pb.CreateSessionResponse{Outcome: pb.AuthOutcome_AUTH_OUTCOME_SUCCESS}, nil
	}

	// Cancel any stale session first
	s.client.CancelSession(ctx)

	resp, err := s.client.CreateSession(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("create_session: %w", err)
	}
	return mapGreetdResponse(resp), nil
}

func (s *Server) PostAuth(ctx context.Context, req *pb.PostAuthRequest) (*pb.PostAuthResponse, error) {
	if s.client == nil {
		slog.Debug("dev: postAuth")
		return &pb.PostAuthResponse{Outcome: pb.AuthOutcome_AUTH_OUTCOME_SUCCESS}, nil
	}

	var response *string
	if req.Response != nil {
		response = req.Response
	}

	resp, err := s.client.PostAuthResponse(ctx, response)
	if err != nil {
		return nil, fmt.Errorf("post_auth: %w", err)
	}

	out := mapGreetdResponse(resp)
	return &pb.PostAuthResponse{
		Outcome:     out.Outcome,
		AuthMessage: out.AuthMessage,
		Success:     out.Success,
		Failure:     out.Failure,
	}, nil
}

func (s *Server) StartSession(ctx context.Context, req *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	cmd := req.Cmd
	env := buildSessionEnv(req.Type, req.Desktop)

	// Wrap X11 sessions with the configured wrapper command.
	if req.Type == pb.SessionType_SESSION_TYPE_X11 && len(s.cfg.Sessions.X11Wrapper) > 0 {
		cmd = append(append([]string{}, s.cfg.Sessions.X11Wrapper...), cmd...)
	}

	if s.client == nil {
		slog.Debug("dev: startSession (no-op)", "cmd", cmd, "env", env)
		return &pb.StartSessionResponse{Ok: true}, nil
	}

	resp, err := s.client.StartSession(ctx, cmd, env)
	if err != nil {
		slog.Error("session start failed", "cmd", cmd, "error", err)
		return &pb.StartSessionResponse{Ok: false, Error: err.Error()}, nil
	}
	if resp.Type != "success" {
		slog.Error("session start rejected", "cmd", cmd, "type", resp.Type)
		return &pb.StartSessionResponse{Ok: false, Error: "session start failed"}, nil
	}

	slog.Info("session started", "cmd", cmd, "env", env)
	return &pb.StartSessionResponse{Ok: true}, nil
}

func buildSessionEnv(sessType pb.SessionType, desktop string) []string {
	typeStr := "wayland"
	if sessType == pb.SessionType_SESSION_TYPE_X11 {
		typeStr = "x11"
	}
	env := []string{"XDG_SESSION_TYPE=" + typeStr}

	d := strings.ReplaceAll(desktop, ";", ":")
	d = strings.TrimRight(d, ":")
	if d == "" {
		d = typeStr
	}

	env = append(env,
		"XDG_SESSION_DESKTOP="+d,
		"XDG_CURRENT_DESKTOP="+d,
		"DESKTOP_SESSION="+d,
	)
	return env
}

func (s *Server) CancelSession(ctx context.Context, _ *pb.CancelSessionRequest) (*pb.CancelSessionResponse, error) {
	if s.client == nil {
		slog.Debug("dev: cancelSession (no-op)")
		return &pb.CancelSessionResponse{}, nil
	}
	_, err := s.client.CancelSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("cancel_session: %w", err)
	}
	return &pb.CancelSessionResponse{}, nil
}

func (s *Server) ListSessions(_ context.Context, _ *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	list := sessions.List(s.cfg.Sessions.Dirs)
	pbSessions := make([]*pb.Session, len(list))
	for i, sess := range list {
		pbSessions[i] = &pb.Session{
			Name:    sess.Name,
			Cmd:     sess.Cmd,
			Type:    mapSessionType(sess.Type),
			Desktop: sess.Desktop,
		}
	}
	return &pb.ListSessionsResponse{Sessions: pbSessions}, nil
}

func (s *Server) GetSystemInfo(_ context.Context, _ *pb.GetSystemInfoRequest) (*pb.GetSystemInfoResponse, error) {
	hostname, _ := os.Hostname()
	return &pb.GetSystemInfoResponse{
		Info: &pb.SystemInfo{Hostname: hostname},
	}, nil
}

func (s *Server) GetPowerCapabilities(_ context.Context, _ *pb.GetPowerCapabilitiesRequest) (*pb.GetPowerCapabilitiesResponse, error) {
	caps := &pb.PowerCapabilities{}
	if s.cfg.Power.Enabled {
		caps.CanPoweroff = len(s.cfg.Power.PoweroffCmd) > 0
		caps.CanReboot = len(s.cfg.Power.RebootCmd) > 0
		caps.CanSuspend = len(s.cfg.Power.SuspendCmd) > 0
	}
	return &pb.GetPowerCapabilitiesResponse{Capabilities: caps}, nil
}

func (s *Server) ExecutePowerAction(_ context.Context, req *pb.ExecutePowerActionRequest) (*pb.ExecutePowerActionResponse, error) {
	if !s.cfg.Power.Enabled {
		return &pb.ExecutePowerActionResponse{Ok: false, Error: "power actions disabled"}, nil
	}

	var args []string
	switch req.Action {
	case pb.PowerAction_POWER_ACTION_POWEROFF:
		args = s.cfg.Power.PoweroffCmd
	case pb.PowerAction_POWER_ACTION_REBOOT:
		args = s.cfg.Power.RebootCmd
	case pb.PowerAction_POWER_ACTION_SUSPEND:
		args = s.cfg.Power.SuspendCmd
	default:
		return &pb.ExecutePowerActionResponse{Ok: false, Error: "unknown power action"}, nil
	}

	if len(args) == 0 {
		return &pb.ExecutePowerActionResponse{Ok: false, Error: "no command configured for action"}, nil
	}

	slog.Info("executing power action", "action", req.Action, "cmd", args)
	if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
		slog.Error("power action failed", "action", req.Action, "error", err)
		return &pb.ExecutePowerActionResponse{Ok: false, Error: err.Error()}, nil
	}
	return &pb.ExecutePowerActionResponse{Ok: true}, nil
}

func (s *Server) GetState(_ context.Context, _ *pb.GetStateRequest) (*pb.GetStateResponse, error) {
	st := state.Load()
	return &pb.GetStateResponse{
		State: &pb.GreeterState{
			LastUser:    st.LastUser,
			LastSession: st.LastSession,
		},
	}, nil
}

func (s *Server) SaveState(_ context.Context, req *pb.SaveStateRequest) (*pb.SaveStateResponse, error) {
	st := state.State{
		LastUser:    req.State.GetLastUser(),
		LastSession: req.State.GetLastSession(),
	}
	if err := state.Save(st); err != nil {
		slog.Warn("failed to save state", "error", err)
		return &pb.SaveStateResponse{Ok: false, Error: err.Error()}, nil
	}
	return &pb.SaveStateResponse{Ok: true}, nil
}

func (s *Server) GetLogs(_ context.Context, _ *pb.GetLogsRequest) (*pb.GetLogsResponse, error) {
	return &pb.GetLogsResponse{Lines: s.logs.Lines()}, nil
}

// mapGreetdResponse maps a greetd IPC response to a proto CreateSessionResponse.
func mapGreetdResponse(resp *greetd.Response) *pb.CreateSessionResponse {
	switch resp.Type {
	case "auth_message":
		msgType := pb.AuthMessageType_AUTH_MESSAGE_TYPE_UNSPECIFIED
		if resp.AuthMessageType != nil {
			switch *resp.AuthMessageType {
			case "visible":
				msgType = pb.AuthMessageType_AUTH_MESSAGE_TYPE_VISIBLE
			case "secret":
				msgType = pb.AuthMessageType_AUTH_MESSAGE_TYPE_SECRET
			case "info":
				msgType = pb.AuthMessageType_AUTH_MESSAGE_TYPE_INFO
			case "error":
				msgType = pb.AuthMessageType_AUTH_MESSAGE_TYPE_ERROR
			}
		}
		msg := ""
		if resp.AuthMessage != nil {
			msg = *resp.AuthMessage
		}
		return &pb.CreateSessionResponse{
			Outcome: pb.AuthOutcome_AUTH_OUTCOME_AUTH_MESSAGE,
			AuthMessage: &pb.AuthMessage{
				Type:    msgType,
				Message: msg,
			},
		}

	case "success":
		return &pb.CreateSessionResponse{
			Outcome: pb.AuthOutcome_AUTH_OUTCOME_SUCCESS,
			Success: &pb.AuthSuccess{},
		}

	case "error":
		errType := pb.ErrorType_ERROR_TYPE_ERROR
		if resp.ErrorType != nil && *resp.ErrorType == "auth_error" {
			errType = pb.ErrorType_ERROR_TYPE_AUTH_ERROR
		}
		desc := ""
		if resp.Description != nil {
			desc = *resp.Description
		}
		return &pb.CreateSessionResponse{
			Outcome: pb.AuthOutcome_AUTH_OUTCOME_FAILURE,
			Failure: &pb.AuthFailure{
				ErrorType:   errType,
				Description: desc,
			},
		}

	default:
		return &pb.CreateSessionResponse{
			Outcome: pb.AuthOutcome_AUTH_OUTCOME_FAILURE,
			Failure: &pb.AuthFailure{
				ErrorType:   pb.ErrorType_ERROR_TYPE_ERROR,
				Description: "unexpected response: " + resp.Type,
			},
		}
	}
}

func mapSessionType(t string) pb.SessionType {
	switch strings.ToLower(t) {
	case "wayland":
		return pb.SessionType_SESSION_TYPE_WAYLAND
	case "x11":
		return pb.SessionType_SESSION_TYPE_X11
	default:
		return pb.SessionType_SESSION_TYPE_UNSPECIFIED
	}
}
