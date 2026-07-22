package metal

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

// evdev keyboard reader no libinput no xkb

const (
	evKey = 0x01

	// input_event on 64-bit: timeval (16) + type u16 + code u16 + value s32
	inputEventSize = 24
)

// KeyEvent is a key press from any attached keyboard.
type KeyEvent struct {
	Code   uint16
	Down   bool // true for press and autorepeat
	Repeat bool
}

type Keyboard struct {
	fds []int
	Ch  chan KeyEvent
}

func eviocgbit(ev, length uintptr) uintptr {
	// _IOC(READ, 'E', 0x20 + ev, len)
	return 2<<30 | length<<16 | 'E'<<8 | (0x20 + ev)
}

func eviocgname(length uintptr) uintptr {
	return 2<<30 | length<<16 | 'E'<<8 | 0x06
}

func hasBit(bits []byte, n uint) bool {
	return int(n/8) < len(bits) && bits[n/8]&(1<<(n%8)) != 0
}

// OpenKeyboards scans /dev/input for devices that look like keyboards.
func OpenKeyboards() (*Keyboard, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(paths) == 0 {
		return nil, fmt.Errorf("no input devices found")
	}
	sort.Strings(paths)

	kb := &Keyboard{Ch: make(chan KeyEvent, 64)}
	for _, p := range paths {
		fd, err := syscall.Open(p, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			slog.Debug("input: open failed", "dev", p, "error", err)
			continue
		}
		if !isKeyboard(fd) {
			syscall.Close(fd)
			continue
		}
		name := deviceName(fd)
		slog.Info("input: keyboard attached", "dev", p, "name", name)
		kb.fds = append(kb.fds, fd)
		go kb.readLoop(fd, p)
	}

	if len(kb.fds) == 0 {
		return nil, fmt.Errorf("no keyboards found under /dev/input (is the greeter user in the input group?)")
	}
	return kb, nil
}

func (k *Keyboard) Close() {
	for _, fd := range k.fds {
		syscall.Close(fd)
	}
}

// a keyboard advertises EV_KEY with letter and enter keys
func isKeyboard(fd int) bool {
	var evBits [1]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		eviocgbit(0, unsafe.Sizeof(evBits)), uintptr(unsafe.Pointer(&evBits))); errno != 0 {
		return false
	}
	if !hasBit(evBits[:], evKey) {
		return false
	}
	var keyBits [96]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		eviocgbit(evKey, unsafe.Sizeof(keyBits)), uintptr(unsafe.Pointer(&keyBits))); errno != 0 {
		return false
	}
	return hasBit(keyBits[:], keyEnter) && hasBit(keyBits[:], 30 /* KEY_A */)
}

func deviceName(fd int) string {
	var buf [256]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		eviocgname(unsafe.Sizeof(buf)), uintptr(unsafe.Pointer(&buf))); errno != 0 {
		return "?"
	}
	return strings.TrimRight(string(buf[:]), "\x00")
}

func (k *Keyboard) readLoop(fd int, path string) {
	buf := make([]byte, inputEventSize*64)
	for {
		n, err := syscall.Read(fd, buf)
		if err != nil || n <= 0 {
			slog.Debug("input: read loop ended", "dev", path, "error", err)
			return
		}
		for off := 0; off+inputEventSize <= n; off += inputEventSize {
			typ := binary.LittleEndian.Uint16(buf[off+16:])
			code := binary.LittleEndian.Uint16(buf[off+18:])
			value := int32(binary.LittleEndian.Uint32(buf[off+20:]))
			if typ != evKey {
				continue
			}
			ev := KeyEvent{Code: code, Down: value != 0, Repeat: value == 2}
			select {
			case k.Ch <- ev:
			default: // ui stalled drop rather than block the reader
			}
		}
	}
}
