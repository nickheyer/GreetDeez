package metal

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run drives the bare metal greeter until login succeeds or we are
// told to die. It must be called with the rpc socket already serving.
// output is the configured connector name, empty for auto. In dev mode
// esc at the login screen quits.
func Run(socketPath string, timeout time.Duration, dev bool, output string) error {
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != "" {
		return fmt.Errorf("metal DRM path cannot run under a compositor")
	}

	be, err := DialBackend(socketPath, timeout)
	if err != nil {
		return fmt.Errorf("dial rpc socket: %w", err)
	}

	// drm before vt so a headless failure leaves the console alone
	surf, err := OpenDRM(output)
	if err != nil {
		return fmt.Errorf("drm: %w", err)
	}

	kb, err := OpenKeyboards()
	if err != nil {
		surf.Close()
		return fmt.Errorf("input: %w", err)
	}

	mice := OpenMice() // optional keyboard-only boxes are fine

	vt := setupVT()
	defer func() {
		// order matters restore text mode after the last flip
		mice.Close()
		kb.Close()
		surf.Close()
		vt.restore()
	}()

	// even on panic the vt must come back
	defer func() {
		if r := recover(); r != nil {
			vt.restore()
			panic(r)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	w, h := surf.Size()
	slog.Info("metal: up", "resolution", fmt.Sprintf("%dx%d", w, h))
	loop := NewLoop(be, w, h, dev)

	for !loop.Done() {
		select {
		case sig := <-sigCh:
			slog.Info("metal: signal, shutting down", "signal", sig)
			return nil
		default:
		}

	events:
		for {
			select {
			case ev := <-kb.Ch:
				loop.Key(ev)
			case ev := <-mice.Ch:
				loop.Mouse(ev)
			default:
				break events
			}
		}

		if err := surf.Present(loop.Step()); err != nil {
			return fmt.Errorf("present: %w", err)
		}
	}

	if loop.Success() {
		slog.Info("metal: session start requested, handing off")
	}
	return nil
}
