// Package outputs picks which display a greeter should appear on. It is
// backend-agnostic: the GDK webview path and the metal DRM path both map
// their monitors into []Output and share one selection policy.
package outputs

import "strings"

// Output describes one connected display.
type Output struct {
	Name     string // connector name, e.g. "DP-1", "HDMI-A-1", "eDP-1"
	Width    int    // pixels
	Height   int
	WidthMM  int // physical size, 0 when the display doesn't report it
	HeightMM int
}

// DPI estimates dots per inch from the physical width, 0 when unknown.
func (o Output) DPI() float64 {
	if o.WidthMM <= 0 || o.Width <= 0 {
		return 0
	}
	return float64(o.Width) / (float64(o.WidthMM) / 25.4)
}

// Pick returns the index of the output to use, or -1 if outs is empty.
// A non-empty want selects that connector by name (case-insensitive);
// when want is empty or absent the output with the most pixels wins,
// with DPI breaking ties.
func Pick(outs []Output, want string) int {
	if len(outs) == 0 {
		return -1
	}
	if want != "" {
		for i := range outs {
			if strings.EqualFold(outs[i].Name, want) {
				return i
			}
		}
	}
	best := 0
	for i := 1; i < len(outs); i++ {
		bp, ip := outs[best].Width*outs[best].Height, outs[i].Width*outs[i].Height
		if ip > bp || (ip == bp && outs[i].DPI() > outs[best].DPI()) {
			best = i
		}
	}
	return best
}
