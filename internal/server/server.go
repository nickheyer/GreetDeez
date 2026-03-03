package server

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
	"github.com/nickheyer/greetdeez/internal/config"
	"github.com/nickheyer/greetdeez/internal/greetd"
	"github.com/nickheyer/greetdeez/internal/sessions"
	"github.com/nickheyer/greetdeez/internal/state"
)

type Server struct {
	pb.UnimplementedGreeterServiceServer
	client *greetd.Client
	cfg    *config.Config
}

func New(client *greetd.Client, cfg *config.Config) *Server {
	return &Server{client: client, cfg: cfg}
}

func (s *Server) Authenticate(ctx context.Context, req *pb.AuthenticateRequest) (*pb.AuthenticateResponse, error) {
	if s.client == nil {
		slog.Debug("dev: authenticate", "username", req.Username)
		return &pb.AuthenticateResponse{Success: true}, nil
	}

	_, err := s.client.Authenticate(ctx, req.Username, req.Password)
	if err != nil {
		return &pb.AuthenticateResponse{Success: false, Error: err.Error()}, nil
	}
	return &pb.AuthenticateResponse{Success: true}, nil
}

func (s *Server) StartSession(ctx context.Context, req *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	sess := req.Session
	if sess == nil {
		return &pb.StartSessionResponse{Success: false, Error: "session is required"}, nil
	}

	cmd := sess.Cmd
	env := buildSessionEnv(sess.Type, sess.Desktop)

	if sess.Type == pb.SessionType_SESSION_TYPE_X11 && len(s.cfg.Sessions.X11Wrapper) > 0 {
		cmd = append(append([]string{}, s.cfg.Sessions.X11Wrapper...), cmd...)
	}

	if s.client == nil {
		slog.Debug("dev: startSession (no-op)", "cmd", cmd, "env", env)
		return &pb.StartSessionResponse{Success: true}, nil
	}

	resp, err := s.client.StartSession(ctx, cmd, env)
	if err != nil {
		slog.Error("session start failed", "cmd", cmd, "error", err)
		return &pb.StartSessionResponse{Success: false, Error: err.Error()}, nil
	}
	if resp.Type != "success" {
		slog.Error("session start rejected", "cmd", cmd, "type", resp.Type)
		return &pb.StartSessionResponse{Success: false, Error: "session start failed"}, nil
	}

	slog.Info("session started", "cmd", cmd, "env", env)
	return &pb.StartSessionResponse{Success: true}, nil
}

func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if s.client == nil {
		slog.Debug("dev: login", "username", req.Username)
		if req.Session != nil {
			state.Save(state.State{
				LastUser:    req.Username,
				LastSession: req.Session.Name,
			})
		}
		return &pb.LoginResponse{Success: true}, nil
	}

	// Step 1: Authenticate
	_, err := s.client.Authenticate(ctx, req.Username, req.Password)
	if err != nil {
		return &pb.LoginResponse{Success: false, Error: err.Error()}, nil
	}

	// Step 2: Start session
	startResp, err := s.StartSession(ctx, &pb.StartSessionRequest{Session: req.Session})
	if err != nil {
		return &pb.LoginResponse{Success: false, Error: err.Error()}, nil
	}
	if !startResp.Success {
		return &pb.LoginResponse{Success: false, Error: startResp.Error}, nil
	}

	// Save state on success
	if req.Session != nil {
		state.Save(state.State{
			LastUser:    req.Username,
			LastSession: req.Session.Name,
		})
	}

	return &pb.LoginResponse{Success: true}, nil
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
