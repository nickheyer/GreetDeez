package metal

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
)

// snapshot mode renders frames offscreen and writes pngs no drm no
// greetd needed used for theme development and screenshots

type stubBackend struct{}

func (stubBackend) SystemInfo(context.Context) (*pb.SystemInfo, error) {
	return &pb.SystemInfo{Hostname: "deezbox"}, nil
}

func (stubBackend) Sessions(context.Context) ([]*pb.Session, error) {
	return []*pb.Session{
		{Name: "sway", Cmd: []string{"sway"}, Type: pb.SessionType_SESSION_TYPE_WAYLAND},
		{Name: "plasma", Cmd: []string{"startplasma-wayland"}, Type: pb.SessionType_SESSION_TYPE_WAYLAND},
		{Name: "i3", Cmd: []string{"i3"}, Type: pb.SessionType_SESSION_TYPE_X11},
	}, nil
}

func (stubBackend) PowerCaps(context.Context) (*pb.PowerCapabilities, error) {
	return &pb.PowerCapabilities{CanPoweroff: true, CanReboot: true, CanSuspend: true}, nil
}

func (stubBackend) State(context.Context) (*pb.GreeterState, error) {
	return &pb.GreeterState{LastUser: "nick", LastSession: "sway"}, nil
}

func (stubBackend) SaveState(context.Context, string, string) error { return nil }

func (stubBackend) BeginAuth(context.Context, string) (*pb.AuthStep, error) {
	return &pb.AuthStep{Prompt: &pb.AuthPrompt{Type: pb.PromptType_PROMPT_TYPE_SECRET, Message: "Password:"}}, nil
}

func (stubBackend) RespondAuth(context.Context, string) (*pb.AuthStep, error) {
	return &pb.AuthStep{Success: true}, nil
}

func (stubBackend) CancelAuth(context.Context) error                { return nil }
func (stubBackend) StartSession(context.Context, *pb.Session) error { return nil }
func (stubBackend) Power(context.Context, pb.PowerAction) error     { return nil }

// Snapshot renders the login screen at fixed timestamps into dir.
func Snapshot(dir string, w, h int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	ui := NewUI(stubBackend{}, w, h)
	ui.Clock = func() time.Time {
		return time.Date(2026, 7, 21, 21, 34, 0, 0, time.UTC)
	}
	frame := NewFrame(w, h)

	shots := []struct {
		name  string
		t     float64
		setup func()
	}{
		{"boot", 0.35, nil},
		{"idle", 2.4, nil},
		{"typing", 5.1, func() {
			ui.phase = phasePrompt
			ui.prompt = &pb.AuthPrompt{Type: pb.PromptType_PROMPT_TYPE_SECRET, Message: "Password:"}
			ui.input = []rune("hunter22")
		}},
		{"busy", 6.2, func() {
			ui.phase = phaseBusy
			ui.busyMsg = txtAuthenticating
		}},
		{"denied", 7.4, func() {
			ui.phase = phaseUser
			ui.prompt = nil
			ui.errMsg = txtAccessDenied
			ui.errAt = 6.9
		}},
		{"warp", 8.3, func() {
			ui.errMsg = ""
			ui.phase = phaseWarp
			ui.warpAt = 8.0
			ui.stars.SetWarp(0.4)
		}},
	}

	prev := 0.0
	for _, shot := range shots {
		if shot.setup != nil {
			shot.setup()
		}
		// advance effect state to the target time in small steps
		for t := prev; t < shot.t; t += 1.0 / 60 {
			ui.stars.Update(1.0 / 60)
			ui.scroller.Update(1.0/60, w)
		}
		prev = shot.t

		ui.Render(frame, shot.t)
		if err := writePNG(filepath.Join(dir, fmt.Sprintf("metal-%s.png", shot.name)), frame); err != nil {
			return err
		}
	}
	return nil
}

func writePNG(path string, f *Frame) error {
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			r, g, b := channels(f.Pix[y*f.W+x])
			img.SetRGBA(x, y, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
		}
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, img)
}
