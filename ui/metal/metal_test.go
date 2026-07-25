package metal

import "testing"

func TestChordStateVTSwitch(t *testing.T) {
	var c chordState

	press := func(code uint16) (int, bool) {
		return c.consume(KeyEvent{Code: code, Down: true})
	}
	release := func(code uint16) {
		c.consume(KeyEvent{Code: code, Down: false})
	}

	// bare f-keys are not a chord (f10 is the power key)
	if _, ok := press(keyF10); ok {
		t.Error("bare F10 must not switch vts")
	}

	// ctrl alone is not enough
	press(keyLeftCtrl)
	if _, ok := press(keyF2); ok {
		t.Error("ctrl+F2 must not switch vts")
	}

	// ctrl+alt+f2 is
	press(keyLeftAlt)
	n, ok := press(keyF2)
	if !ok || n != 2 {
		t.Errorf("ctrl+alt+F2 = (%d, %v), want (2, true)", n, ok)
	}

	// autorepeat of the held chord must not fire again
	if _, ok := c.consume(KeyEvent{Code: keyF2, Down: true, Repeat: true}); ok {
		t.Error("chord autorepeat must not re-fire")
	}

	// releasing a modifier disarms the chord
	release(keyLeftAlt)
	if _, ok := press(keyF2); ok {
		t.Error("chord must disarm when alt is released")
	}

	// right-hand modifiers work too and f11/f12 map past the gap
	press(keyRightAlt)
	if n, ok := press(keyF11); !ok || n != 11 {
		t.Errorf("ctrl+alt+F11 = (%d, %v), want (11, true)", n, ok)
	}
	if n, ok := press(keyF12); !ok || n != 12 {
		t.Errorf("ctrl+alt+F12 = (%d, %v), want (12, true)", n, ok)
	}
	if n, ok := press(keyF1); !ok || n != 1 {
		t.Errorf("ctrl+alt+F1 = (%d, %v), want (1, true)", n, ok)
	}
}
