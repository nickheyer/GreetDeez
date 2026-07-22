package metal

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
	"github.com/nickheyer/greetdeez/pkg/rpc"
)

// scripted greeter served over the real unix socket transport so these
// tests cover client framing and the state machine in one go
type scriptedGreeter struct {
	pb.UnimplementedGreeterServiceServer
	mu sync.Mutex

	password    string
	infoMsg     string
	failStart   bool
	begun       string
	beginCalls  int
	startedWith string
	savedUser   string
}

func (s *scriptedGreeter) GetSystemInfo(context.Context, *pb.GetSystemInfoRequest) (*pb.GetSystemInfoResponse, error) {
	return &pb.GetSystemInfoResponse{Info: &pb.SystemInfo{Hostname: "testbox"}}, nil
}

func (s *scriptedGreeter) ListSessions(context.Context, *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	return &pb.ListSessionsResponse{Sessions: []*pb.Session{
		{Name: "sway", Cmd: []string{"sway"}, Type: pb.SessionType_SESSION_TYPE_WAYLAND},
		{Name: "i3", Cmd: []string{"i3"}, Type: pb.SessionType_SESSION_TYPE_X11},
	}}, nil
}

func (s *scriptedGreeter) GetPowerCapabilities(context.Context, *pb.GetPowerCapabilitiesRequest) (*pb.GetPowerCapabilitiesResponse, error) {
	return &pb.GetPowerCapabilitiesResponse{Capabilities: &pb.PowerCapabilities{CanPoweroff: true}}, nil
}

func (s *scriptedGreeter) GetState(context.Context, *pb.GetStateRequest) (*pb.GetStateResponse, error) {
	return &pb.GetStateResponse{State: &pb.GreeterState{LastUser: "prefill", LastSession: "i3"}}, nil
}

func (s *scriptedGreeter) SaveState(_ context.Context, req *pb.SaveStateRequest) (*pb.SaveStateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedUser = req.State.GetLastUser()
	return &pb.SaveStateResponse{Ok: true}, nil
}

func (s *scriptedGreeter) BeginAuth(_ context.Context, req *pb.BeginAuthRequest) (*pb.AuthStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.begun = req.Username
	s.beginCalls++
	step := &pb.AuthStep{Prompt: &pb.AuthPrompt{Type: pb.PromptType_PROMPT_TYPE_SECRET, Message: "Password:"}}
	if s.infoMsg != "" {
		step.Messages = []string{s.infoMsg}
	}
	return step, nil
}

func (s *scriptedGreeter) RespondAuth(_ context.Context, req *pb.RespondAuthRequest) (*pb.AuthStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Response == s.password {
		return &pb.AuthStep{Success: true}, nil
	}
	return &pb.AuthStep{Error: "authentication failed"}, nil
}

func (s *scriptedGreeter) CancelAuth(context.Context, *pb.CancelAuthRequest) (*pb.CancelAuthResponse, error) {
	return &pb.CancelAuthResponse{Ok: true}, nil
}

func (s *scriptedGreeter) StartSession(_ context.Context, req *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failStart {
		return &pb.StartSessionResponse{Success: false, Error: "no seat"}, nil
	}
	s.startedWith = req.Session.GetName()
	return &pb.StartSessionResponse{Success: true}, nil
}

func newTestUI(t *testing.T, g *scriptedGreeter) *UI {
	t.Helper()
	d := rpc.NewDispatcher(false)
	pb.RegisterGreeterServiceServer(d, g)
	sockPath := filepath.Join(t.TempDir(), "ui.sock")
	srv, err := rpc.ServeSocket(sockPath, d)
	if err != nil {
		t.Fatalf("ServeSocket: %v", err)
	}
	t.Cleanup(func() { srv.Close() })

	be, err := DialBackend(sockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("DialBackend: %v", err)
	}
	return NewUI(be, 320, 240)
}

// press every key needed to type s
func typeString(t *testing.T, u *UI, s string, now float64) {
	t.Helper()
	rev := map[rune]uint16{}
	for code, ks := range usKeymap {
		if _, seen := rev[ks.base]; !seen {
			rev[ks.base] = code
		}
	}
	for _, r := range strings.ToLower(s) {
		code, ok := rev[r]
		if !ok {
			t.Fatalf("no key for %q", r)
		}
		u.HandleKey(KeyEvent{Code: code, Down: true}, now)
		u.HandleKey(KeyEvent{Code: code, Down: false}, now)
	}
}

// pump update until cond or timeout rpc goroutines need real time
func waitFor(t *testing.T, u *UI, now *float64, cond func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		u.Update(1.0/60, *now)
		*now += 1.0 / 60
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition never met (phase=%d err=%q)", u.phase, u.errMsg)
}

func TestUILoadsBackendData(t *testing.T) {
	u := newTestUI(t, &scriptedGreeter{})
	if u.hostname != "testbox" {
		t.Errorf("hostname = %q", u.hostname)
	}
	if string(u.username) != "prefill" {
		t.Errorf("username prefill = %q", string(u.username))
	}
	if u.session().GetName() != "i3" {
		t.Errorf("session prefill = %q, want i3 from saved state", u.session().GetName())
	}
	if !u.caps.GetCanPoweroff() {
		t.Error("power caps not loaded")
	}
}

func TestUIFullLoginFlow(t *testing.T) {
	g := &scriptedGreeter{password: "hunter2", infoMsg: "Welcome back"}
	u := newTestUI(t, g)
	now := 1.0

	// clear the prefilled username and type a new one
	u.HandleKey(KeyEvent{Code: keyLeftCtrl, Down: true}, now)
	u.HandleKey(KeyEvent{Code: keyBackspace, Down: true}, now)
	u.HandleKey(KeyEvent{Code: keyLeftCtrl, Down: false}, now)
	typeString(t, u, "nick", now)
	if string(u.username) != "nick" {
		t.Fatalf("username = %q", string(u.username))
	}

	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phasePrompt })
	if g.begun != "nick" {
		t.Errorf("BeginAuth saw %q", g.begun)
	}
	if len(u.msgs) == 0 || u.msgs[0] != "Welcome back" {
		t.Errorf("info messages = %v", u.msgs)
	}

	typeString(t, u, "hunter2", now)
	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phaseWarp })

	// warp finishes then the greeter reports done
	waitFor(t, u, &now, func() bool { return u.Done })
	if !u.Success {
		t.Error("expected success")
	}
	if g.startedWith != "i3" {
		t.Errorf("StartSession got %q, want i3", g.startedWith)
	}
	if g.savedUser != "nick" {
		t.Errorf("SaveState got %q, want nick", g.savedUser)
	}
}

func TestUIWrongPassword(t *testing.T) {
	g := &scriptedGreeter{password: "right"}
	u := newTestUI(t, g)
	now := 1.0

	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phasePrompt })

	typeString(t, u, "wrong", now)
	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phaseUser && u.errMsg != "" })

	if u.errMsg != "AUTHENTICATION FAILED" {
		t.Errorf("errMsg = %q", u.errMsg)
	}
	if len(u.input) != 0 {
		t.Error("input should clear on failure")
	}
	if u.Done {
		t.Error("must not exit on failure")
	}
}

func TestUISessionCycleAndEscape(t *testing.T) {
	u := newTestUI(t, &scriptedGreeter{password: "x"})
	now := 1.0

	if u.session().GetName() != "i3" {
		t.Fatalf("start session = %q", u.session().GetName())
	}
	// arrows cycle the session, tab must not
	u.HandleKey(KeyEvent{Code: keyRight, Down: true}, now)
	if u.session().GetName() != "sway" {
		t.Errorf("after right = %q, want sway", u.session().GetName())
	}
	u.HandleKey(KeyEvent{Code: keyLeft, Down: true}, now)
	if u.session().GetName() != "i3" {
		t.Errorf("after left = %q, want i3", u.session().GetName())
	}

	// esc during prompt returns to user phase
	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phasePrompt })
	u.HandleKey(KeyEvent{Code: keyEsc, Down: true}, now)
	if u.phase != phaseUser {
		t.Errorf("esc should return to user phase, got %d", u.phase)
	}

	// arrows still switch session while the prompt is up
	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phasePrompt })
	u.HandleKey(KeyEvent{Code: keyDown, Down: true}, now)
	if u.session().GetName() != "sway" {
		t.Errorf("prompt phase down arrow = %q, want sway", u.session().GetName())
	}
}

func TestUITabMovesBetweenFields(t *testing.T) {
	g := &scriptedGreeter{password: "x"}
	u := newTestUI(t, g)
	now := 1.0

	// tab with a username advances into the pam prompt (the next field)
	u.HandleKey(KeyEvent{Code: keyTab, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phasePrompt })
	if g.begun != "prefill" {
		t.Errorf("tab should begin auth for %q, got %q", "prefill", g.begun)
	}

	// tab from the prompt wraps back to the login field
	u.HandleKey(KeyEvent{Code: keyTab, Down: true}, now)
	if u.phase != phaseUser {
		t.Errorf("tab in prompt should return to user phase, got %d", u.phase)
	}
	if string(u.username) != "prefill" {
		t.Errorf("username must survive the round trip, got %q", string(u.username))
	}

	// tab with an empty username has no next field to go to
	u.HandleKey(KeyEvent{Code: keyLeftCtrl, Down: true}, now)
	u.HandleKey(KeyEvent{Code: keyBackspace, Down: true}, now)
	u.HandleKey(KeyEvent{Code: keyLeftCtrl, Down: false}, now)
	g.mu.Lock()
	g.begun = ""
	g.mu.Unlock()
	u.HandleKey(KeyEvent{Code: keyTab, Down: true}, now)
	if u.phase != phaseUser {
		t.Errorf("tab on empty username should stay put, got phase %d", u.phase)
	}
}

func TestUIEnterSubmitsEvenWhenEmpty(t *testing.T) {
	g := &scriptedGreeter{password: "x"}
	u := newTestUI(t, g)
	now := 1.0

	// clear the prefill then submit anyway: the backend decides
	u.HandleKey(KeyEvent{Code: keyLeftCtrl, Down: true}, now)
	u.HandleKey(KeyEvent{Code: keyBackspace, Down: true}, now)
	u.HandleKey(KeyEvent{Code: keyLeftCtrl, Down: false}, now)
	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase != phaseBusy })
	g.mu.Lock()
	begun, calls := g.begun, g.beginCalls
	g.mu.Unlock()
	if calls != 1 || begun != "" {
		t.Errorf("BeginAuth calls = %d with user %q, want 1 call with empty user", calls, begun)
	}
}

func TestUIMouseClicksAndWheel(t *testing.T) {
	u := newTestUI(t, &scriptedGreeter{password: "x"})
	now := 1.0
	f := NewFrame(u.w, u.h)
	u.Render(f, now) // capture hit regions

	// wheel walks the session list
	if u.session().GetName() != "i3" {
		t.Fatalf("start session = %q", u.session().GetName())
	}
	u.HandleMouse(MouseEvent{Wheel: -1}, now)
	if u.session().GetName() != "sway" {
		t.Errorf("wheel down = %q, want sway", u.session().GetName())
	}

	// click the right half of the session row cycles forward
	sx := u.hitSess.x + u.hitSess.w - 2
	sy := u.hitSess.y + u.hitSess.h/2
	u.HandleMouse(MouseEvent{Abs: true, X: float64(sx), Y: float64(sy)}, now)
	u.HandleMouse(MouseEvent{Abs: true, X: float64(sx), Y: float64(sy), Btn: 1, Down: true}, now)
	if u.session().GetName() != "i3" {
		t.Errorf("session click = %q, want i3", u.session().GetName())
	}

	// clicking the ghost password field with a username begins auth
	px := u.hitPrompt.x + u.hitPrompt.w/2
	py := u.hitPrompt.y + u.hitPrompt.h/2
	u.HandleMouse(MouseEvent{Abs: true, X: float64(px), Y: float64(py)}, now)
	u.HandleMouse(MouseEvent{Abs: true, X: float64(px), Y: float64(py), Btn: 1, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phasePrompt })

	// clicking the login field cancels back out of the prompt
	u.Render(f, now)
	ux := u.hitUser.x + u.hitUser.w/2
	uy := u.hitUser.y + u.hitUser.h/2
	u.HandleMouse(MouseEvent{Abs: true, X: float64(ux), Y: float64(uy)}, now)
	u.HandleMouse(MouseEvent{Abs: true, X: float64(ux), Y: float64(uy), Btn: 1, Down: true}, now)
	if u.phase != phaseUser {
		t.Errorf("login field click should refocus login, got phase %d", u.phase)
	}
}

func TestUICursorRendersAfterMouseSeen(t *testing.T) {
	u := newTestUI(t, &scriptedGreeter{})
	f := NewFrame(u.w, u.h)

	u.Render(f, 1.0)
	if u.mSeen {
		t.Fatal("no mouse event yet")
	}
	u.HandleMouse(MouseEvent{DX: 10, DY: 5}, 1.0)
	if !u.mSeen {
		t.Fatal("mouse should be tracked after motion")
	}
	u.Render(f, 1.1) // must not panic with trail and cursor active
	u.HandleMouse(MouseEvent{Abs: true, X: 5, Y: 5, Btn: 1, Down: true}, 1.2)
	u.Render(f, 1.3)
}

func TestUIStartSessionFailureRecovers(t *testing.T) {
	g := &scriptedGreeter{password: "pw", failStart: true}
	u := newTestUI(t, g)
	now := 1.0

	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phasePrompt })
	typeString(t, u, "pw", now)
	u.HandleKey(KeyEvent{Code: keyEnter, Down: true}, now)
	waitFor(t, u, &now, func() bool { return u.phase == phaseUser && u.errMsg != "" })

	if u.Done {
		t.Error("session start failure must not exit the greeter")
	}
	if !strings.Contains(u.errMsg, "NO SEAT") {
		t.Errorf("errMsg = %q, want the server error surfaced", u.errMsg)
	}
}

func TestUIPowerConfirm(t *testing.T) {
	u := newTestUI(t, &scriptedGreeter{})
	now := 1.0

	// single press only arms
	u.HandleKey(KeyEvent{Code: keyF10, Down: true}, now)
	if u.phase != phaseUser {
		t.Fatal("single power press must not trigger")
	}
	if u.powerArm != pb.PowerAction_POWER_ACTION_POWEROFF {
		t.Fatal("power should be armed")
	}
	// F11 disabled by caps so nothing happens
	u.HandleKey(KeyEvent{Code: keyF11, Down: true}, now)
	if u.powerArm != pb.PowerAction_POWER_ACTION_POWEROFF {
		t.Error("disabled action must not rearm")
	}
	// second press within the window fires
	u.HandleKey(KeyEvent{Code: keyF10, Down: true}, now+1)
	if u.phase != phaseBusy {
		t.Error("confirmed power press should go busy")
	}
}
