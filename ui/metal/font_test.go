package metal

import "testing"

func TestFontCoversPrintableASCII(t *testing.T) {
	for ch := rune(33); ch <= 126; ch++ {
		g := glyph(ch)
		empty := true
		for _, row := range g {
			if row != 0 {
				empty = false
				break
			}
		}
		if empty {
			t.Errorf("glyph %q is empty", ch)
		}
	}
}

func TestGlyphFallback(t *testing.T) {
	if glyph('é') != glyph('?') {
		t.Error("non-ascii should fall back to '?'")
	}
	if glyph(31) != glyph('?') {
		t.Error("control chars should fall back to '?'")
	}
	if glyph(' ') == glyph('?') {
		t.Error("space is printable and must not fall back")
	}
}

func TestTextWidth(t *testing.T) {
	if w := TextWidth("abcd", 2); w != 4*8*2 {
		t.Errorf("TextWidth = %d, want %d", w, 4*8*2)
	}
	// runes not bytes
	if w := TextWidth("éé", 1); w != 2*8 {
		t.Errorf("TextWidth utf8 = %d, want %d", w, 2*8)
	}
}

func TestDrawTextClipsAtEdges(t *testing.T) {
	f := NewFrame(20, 20)
	// none of these may panic or write out of bounds
	f.DrawText(-100, -100, "hello", 3, colText)
	f.DrawText(15, 15, "world", 3, colText)
	f.DrawTextGrad(-5, 18, "grad", 2, colText, colAccent)
}
