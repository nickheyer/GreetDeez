package metal

// evdev key codes from linux input-event-codes.h the ones we handle
const (
	keyEsc        = 1
	keyBackspace  = 14
	keyTab        = 15
	keyEnter      = 28
	keyLeftCtrl   = 29
	keyLeftShift  = 42
	keyRightShift = 54
	keySpace      = 57
	keyCapsLock   = 58
	keyF1         = 59
	keyF2         = 60
	keyF10        = 68
	keyF11        = 87
	keyF12        = 88
	keyKpEnter    = 96
	keyRightCtrl  = 97
	keyUp         = 103
	keyLeft       = 105
	keyRight      = 106
	keyDown       = 108
	keyDelete     = 111
)

type keysym struct {
	base, shift rune
}

// us qwerty v1 keymap is built in see readme for the caveat
var usKeymap = map[uint16]keysym{
	2: {'1', '!'}, 3: {'2', '@'}, 4: {'3', '#'}, 5: {'4', '$'},
	6: {'5', '%'}, 7: {'6', '^'}, 8: {'7', '&'}, 9: {'8', '*'},
	10: {'9', '('}, 11: {'0', ')'}, 12: {'-', '_'}, 13: {'=', '+'},
	16: {'q', 'Q'}, 17: {'w', 'W'}, 18: {'e', 'E'}, 19: {'r', 'R'},
	20: {'t', 'T'}, 21: {'y', 'Y'}, 22: {'u', 'U'}, 23: {'i', 'I'},
	24: {'o', 'O'}, 25: {'p', 'P'}, 26: {'[', '{'}, 27: {']', '}'},
	30: {'a', 'A'}, 31: {'s', 'S'}, 32: {'d', 'D'}, 33: {'f', 'F'},
	34: {'g', 'G'}, 35: {'h', 'H'}, 36: {'j', 'J'}, 37: {'k', 'K'},
	38: {'l', 'L'}, 39: {';', ':'}, 40: {'\'', '"'}, 41: {'`', '~'},
	43: {'\\', '|'},
	44: {'z', 'Z'}, 45: {'x', 'X'}, 46: {'c', 'C'}, 47: {'v', 'V'},
	48: {'b', 'B'}, 49: {'n', 'N'}, 50: {'m', 'M'}, 51: {',', '<'},
	52: {'.', '>'}, 53: {'/', '?'},
	keySpace: {' ', ' '},
	// keypad digits
	71: {'7', '7'}, 72: {'8', '8'}, 73: {'9', '9'},
	75: {'4', '4'}, 76: {'5', '5'}, 77: {'6', '6'},
	79: {'1', '1'}, 80: {'2', '2'}, 81: {'3', '3'},
	82: {'0', '0'}, 83: {'.', '.'},
	98: {'/', '/'}, 55: {'*', '*'}, 74: {'-', '-'}, 78: {'+', '+'},
}

// Mods tracks modifier state from raw key events.
type Mods struct {
	shiftL, shiftR bool
	caps           bool
	ctrlL, ctrlR   bool
}

// Track updates modifier state returns true when the event was a modifier
func (m *Mods) Track(code uint16, down bool) bool {
	switch code {
	case keyLeftShift:
		m.shiftL = down
	case keyRightShift:
		m.shiftR = down
	case keyLeftCtrl:
		m.ctrlL = down
	case keyRightCtrl:
		m.ctrlR = down
	case keyCapsLock:
		if down {
			m.caps = !m.caps
		}
	default:
		return false
	}
	return true
}

func (m *Mods) Shift() bool { return m.shiftL || m.shiftR }
func (m *Mods) Ctrl() bool  { return m.ctrlL || m.ctrlR }

// Rune translates a key code to a printable rune or 0.
func (m *Mods) Rune(code uint16) rune {
	ks, ok := usKeymap[code]
	if !ok {
		return 0
	}
	shift := m.Shift()
	// caps only flips letters
	if m.caps && ks.base >= 'a' && ks.base <= 'z' {
		shift = !shift
	}
	if shift {
		return ks.shift
	}
	return ks.base
}
