package metal

import (
	"log/slog"
	"syscall"
	"unsafe"
)

// vt handling put the console in graphics mode and mute the kernel
// keyboard so keystrokes never echo to the text console under us

const (
	kdSetMode  = 0x4B3A
	kdTextMode = 0x00
	kdGraphics = 0x01
	kdGKbMode  = 0x4B44
	kdSKbMode  = 0x4B45
	kbModeOff  = 0x04 // K_OFF
)

type vtGuard struct {
	fd    int
	oldKb uintptr
	ok    bool
}

// setupVT is best effort a missing controlling tty (dev runs over ssh)
// only costs console echo not functionality
func setupVT() *vtGuard {
	fd, err := syscall.Open("/dev/tty", syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		slog.Warn("vt: no controlling tty, console left as-is", "error", err)
		return &vtGuard{fd: -1}
	}

	g := &vtGuard{fd: fd}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), kdGKbMode,
		uintptr(unsafe.Pointer(&g.oldKb))); errno != 0 {
		slog.Warn("vt: not a virtual console, skipping graphics mode", "errno", errno)
		syscall.Close(fd)
		return &vtGuard{fd: -1}
	}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), kdSetMode, kdGraphics); errno != 0 {
		slog.Warn("vt: KDSETMODE graphics failed", "errno", errno)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), kdSKbMode, kbModeOff); errno != 0 {
		slog.Warn("vt: KDSKBMODE off failed", "errno", errno)
	}
	g.ok = true
	return g
}

// restore must run before the session starts or the next vt user
// inherits a dead console
func (g *vtGuard) restore() {
	if g.fd < 0 {
		return
	}
	if g.ok {
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(g.fd), kdSKbMode, g.oldKb)
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(g.fd), kdSetMode, kdTextMode)
	}
	syscall.Close(g.fd)
	g.fd = -1
}
