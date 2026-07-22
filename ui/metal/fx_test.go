package metal

import "testing"

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
