package outputs

import "testing"

func TestPick(t *testing.T) {
	outs := []Output{
		{Name: "DP-1", Width: 1080, Height: 1920, WidthMM: 336, HeightMM: 598}, // vertical secondary
		{Name: "DP-2", Width: 3840, Height: 2160, WidthMM: 697, HeightMM: 392},
		{Name: "HDMI-A-1", Width: 1920, Height: 1080, WidthMM: 521, HeightMM: 293},
	}

	cases := []struct {
		name string
		outs []Output
		want string
		idx  int
	}{
		{"auto picks most pixels", outs, "", 1},
		{"explicit connector wins over resolution", outs, "HDMI-A-1", 2},
		{"match is case-insensitive", outs, "dp-1", 0},
		{"unknown connector falls back to auto", outs, "DP-9", 1},
		{"empty list", nil, "DP-1", -1},
		{"single output", outs[:1], "", 0},
		{
			"equal pixels tie-break on dpi",
			[]Output{
				{Name: "HDMI-A-1", Width: 2560, Height: 1440, WidthMM: 697}, // ~93 dpi
				{Name: "eDP-1", Width: 2560, Height: 1440, WidthMM: 302},    // ~215 dpi
			},
			"", 1,
		},
		{
			"missing physical size never beats a known dpi",
			[]Output{
				{Name: "Virtual-1", Width: 1920, Height: 1080},
				{Name: "HDMI-A-1", Width: 1920, Height: 1080, WidthMM: 521},
			},
			"", 1,
		},
	}
	for _, c := range cases {
		if got := Pick(c.outs, c.want); got != c.idx {
			t.Errorf("%s: Pick(want=%q) = %d, want %d", c.name, c.want, got, c.idx)
		}
	}
}

func TestDPI(t *testing.T) {
	if got := (Output{Width: 3840, WidthMM: 697}).DPI(); got < 139 || got > 141 {
		t.Errorf("DPI() = %f, want ~140", got)
	}
	if got := (Output{Width: 1920}).DPI(); got != 0 {
		t.Errorf("DPI() with no physical size = %f, want 0", got)
	}
}
