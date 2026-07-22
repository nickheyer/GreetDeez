package metal

import (
	"context"
	"testing"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
)

// canned backend so render benchmarks need no socket
type stubBackend struct{}

func (stubBackend) SystemInfo(context.Context) (*pb.SystemInfo, error) {
	return &pb.SystemInfo{Hostname: "deezbox"}, nil
}

func (stubBackend) Sessions(context.Context) ([]*pb.Session, error) {
	return []*pb.Session{
		{Name: "sway", Cmd: []string{"sway"}, Type: pb.SessionType_SESSION_TYPE_WAYLAND},
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

func TestPlasmaRenderFillsFrame(t *testing.T) {
	f := NewFrame(64, 48)
	p := NewPlasma(32, 24)
	p.Render(f, 1.5)

	distinct := map[uint32]bool{}
	for _, px := range f.Pix {
		distinct[px] = true
	}
	if len(distinct) < 8 {
		t.Errorf("plasma too flat: %d distinct colors", len(distinct))
	}
}

func TestPlasmaLavaGlows(t *testing.T) {
	f := NewFrame(64, 48)
	p := NewPlasma(32, 24)
	p.Render(f, 0)

	total := func() (n int) {
		for _, h := range p.heat {
			if h > 0 {
				n++
			}
		}
		return
	}
	base := total()
	if base == 0 {
		t.Fatal("blobs should heat the field even before any pointer input")
	}

	// stirring the pointer adds its own hot spot
	p.SetPointer(16, 12, 30)
	p.Pulse()
	p.Update(1.0 / 60)
	p.Render(f, 0.1)
	if total() <= base {
		t.Error("pointer energy should add heat")
	}
}

func TestPlasmaBlobsStayOnGrid(t *testing.T) {
	p := NewPlasma(40, 30)
	p.SetPointer(0, 0, 100)
	p.Pulse() // shove everything hard
	for i := 0; i < 600; i++ {
		p.Update(1.0 / 30)
	}
	for i, b := range p.blobs {
		if b.x < 0 || b.x > 40 || b.y < 0 || b.y > 30 {
			t.Errorf("blob %d escaped to (%.1f, %.1f)", i, b.x, b.y)
		}
	}
	// stamping after the abuse must not index out of bounds
	p.stampHeat()
}

func TestStarfieldStaysInBounds(t *testing.T) {
	f := NewFrame(100, 80)
	s := NewStarfield(100, 80, 200)
	for i := 0; i < 300; i++ {
		s.Update(1.0 / 60)
	}
	s.Render(f) // Set/Add clip so this must not panic
	s.SetWarp(1)
	s.Update(1.0 / 60)
	s.Render(f)
}

func TestCRTDarkensNotBrightens(t *testing.T) {
	f := NewFrame(32, 32)
	f.Clear(rgb(200, 200, 200))
	NewCRT(32, 32).Apply(f)
	for i, px := range f.Pix {
		r, g, b := channels(px)
		if r > 200 || g > 200 || b > 200 {
			t.Fatalf("pixel %d brightened: %06x", i, px)
		}
	}
	if f.Pix[0] == f.Pix[16*32+16] {
		t.Error("vignette should darken corners more than center")
	}
}

func TestBlendAndClipHelpers(t *testing.T) {
	f := NewFrame(10, 10)
	f.FillRect(-5, -5, 100, 100, rgb(10, 20, 30))
	if f.Pix[0] != rgb(10, 20, 30) {
		t.Error("fill with overshoot should still cover frame")
	}
	f.BlendRect(0, 0, 10, 10, rgb(255, 255, 255), 256)
	r, g, b := channels(f.Pix[55])
	if r < 250 || g < 250 || b < 250 {
		t.Errorf("alpha 256 should fully replace, got %d %d %d", r, g, b)
	}
	if got := addColor(rgb(200, 200, 200), rgb(100, 100, 100)); got != rgb(255, 255, 255) {
		t.Errorf("addColor should saturate, got %06x", got)
	}
}

func BenchmarkFullFrame1080p(b *testing.B) {
	w, h := 1920, 1080
	ui := NewUI(stubBackend{}, w, h)
	f := NewFrame(w, h)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now := float64(i) / 60
		ui.Update(1.0/60, now)
		ui.Render(f, now)
	}
}
