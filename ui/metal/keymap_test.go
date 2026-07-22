package metal

import "testing"

func TestKeymapBasics(t *testing.T) {
	var m Mods
	if r := m.Rune(30); r != 'a' {
		t.Errorf("KEY_A = %q, want a", r)
	}
	if r := m.Rune(2); r != '1' {
		t.Errorf("KEY_1 = %q, want 1", r)
	}
	if r := m.Rune(keyEnter); r != 0 {
		t.Errorf("enter should not map to a rune, got %q", r)
	}
}

func TestKeymapShift(t *testing.T) {
	var m Mods
	m.Track(keyLeftShift, true)
	if r := m.Rune(30); r != 'A' {
		t.Errorf("shift+a = %q, want A", r)
	}
	if r := m.Rune(2); r != '!' {
		t.Errorf("shift+1 = %q, want !", r)
	}
	m.Track(keyLeftShift, false)
	if r := m.Rune(30); r != 'a' {
		t.Errorf("released shift, got %q, want a", r)
	}
}

func TestKeymapCaps(t *testing.T) {
	var m Mods
	m.Track(keyCapsLock, true)
	m.Track(keyCapsLock, false) // caps toggles on press only
	if r := m.Rune(30); r != 'A' {
		t.Errorf("caps+a = %q, want A", r)
	}
	// caps does not shift digits
	if r := m.Rune(2); r != '1' {
		t.Errorf("caps+1 = %q, want 1", r)
	}
	// shift undoes caps for letters
	m.Track(keyRightShift, true)
	if r := m.Rune(30); r != 'a' {
		t.Errorf("caps+shift+a = %q, want a", r)
	}
}

func TestModifierTracking(t *testing.T) {
	var m Mods
	if !m.Track(keyLeftCtrl, true) {
		t.Error("ctrl should be tracked as modifier")
	}
	if !m.Ctrl() {
		t.Error("ctrl should be down")
	}
	if m.Track(30, true) {
		t.Error("letter keys are not modifiers")
	}
}
