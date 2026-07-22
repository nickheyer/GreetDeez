package metal

import (
	"encoding/binary"
	"log/slog"
	"path/filepath"
	"sort"
	"syscall"
	"unsafe"
)

// evdev pointer reader same philosophy as input.go: no libinput

const (
	evSyn = 0x00
	evRel = 0x02

	relX     = 0x00
	relY     = 0x01
	relWheel = 0x08

	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
)

// MouseEvent is pointer motion
type MouseEvent struct {
	DX, DY float64 // relative motion
	X, Y   float64 // absolute position, valid when Abs
	Abs    bool
	Btn    int // 0 none, 1 left, 2 right, 3 middle
	Down   bool
	Wheel  int // vertical steps, positive is up
}

type Mouse struct {
	fds []int
	Ch  chan MouseEvent
}

// OpenMice scans /dev/input for relative pointer devices
func OpenMice() *Mouse {
	m := &Mouse{Ch: make(chan MouseEvent, 256)}
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return m
	}
	sort.Strings(paths)
	for _, p := range paths {
		fd, err := syscall.Open(p, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		if !isMouse(fd) {
			syscall.Close(fd)
			continue
		}
		slog.Info("input: mouse attached", "dev", p, "name", deviceName(fd))
		m.fds = append(m.fds, fd)
		go m.readLoop(fd, p)
	}
	return m
}

func (m *Mouse) Close() {
	for _, fd := range m.fds {
		syscall.Close(fd)
	}
}

// a mouse advertises EV_REL with REL_X plus BTN_LEFT
func isMouse(fd int) bool {
	var evBits [1]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		eviocgbit(0, unsafe.Sizeof(evBits)), uintptr(unsafe.Pointer(&evBits))); errno != 0 {
		return false
	}
	if !hasBit(evBits[:], evRel) {
		return false
	}
	var relBits [2]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		eviocgbit(evRel, unsafe.Sizeof(relBits)), uintptr(unsafe.Pointer(&relBits))); errno != 0 {
		return false
	}
	var keyBits [96]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		eviocgbit(evKey, unsafe.Sizeof(keyBits)), uintptr(unsafe.Pointer(&keyBits))); errno != 0 {
		return false
	}
	return hasBit(relBits[:], relX) && hasBit(keyBits[:], btnLeft)
}

// readLoop batches REL motion per SYN_REPORT so a 1 kHz mouse does not
// flood the channel; buttons and wheel go out immediately.
func (m *Mouse) readLoop(fd int, path string) {
	buf := make([]byte, inputEventSize*64)
	var dx, dy int32
	for {
		n, err := syscall.Read(fd, buf)
		if err != nil || n <= 0 {
			slog.Debug("input: mouse read loop ended", "dev", path, "error", err)
			return
		}
		for off := 0; off+inputEventSize <= n; off += inputEventSize {
			typ := binary.LittleEndian.Uint16(buf[off+16:])
			code := binary.LittleEndian.Uint16(buf[off+18:])
			value := int32(binary.LittleEndian.Uint32(buf[off+20:]))
			switch typ {
			case evRel:
				switch code {
				case relX:
					dx += value
				case relY:
					dy += value
				case relWheel:
					m.send(MouseEvent{Wheel: int(value)})
				}
			case evKey:
				var btn int
				switch code {
				case btnLeft:
					btn = 1
				case btnRight:
					btn = 2
				case btnMiddle:
					btn = 3
				default:
					continue
				}
				m.send(MouseEvent{Btn: btn, Down: value != 0})
			case evSyn:
				if dx != 0 || dy != 0 {
					m.send(MouseEvent{DX: float64(dx), DY: float64(dy)})
					dx, dy = 0, 0
				}
			}
		}
	}
}

func (m *Mouse) send(ev MouseEvent) {
	select {
	case m.Ch <- ev:
	default: // ui stalled drop rather than block the reader
	}
}
