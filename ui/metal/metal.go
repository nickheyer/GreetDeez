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
// In dev mode esc at the login screen quits.
func Run(socketPath string, timeout time.Duration, dev bool) error {
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != "" {
		return fmt.Errorf("metal theme drives DRM directly and cannot run under a compositor; " +
			`use command = "/usr/bin/greetdeez" (no cage) in /etc/greetd/config.toml`)
	}

	be, err := DialBackend(socketPath, timeout)
	if err != nil {
		return fmt.Errorf("dial rpc socket: %w", err)
	}

	// drm before vt so a headless failure leaves the console alone
	surf, err := OpenDRM()
	if err != nil {
		return fmt.Errorf("drm: %w", err)
	}

	kb, err := OpenKeyboards()
	if err != nil {
		surf.Close()
		return fmt.Errorf("input: %w", err)
	}

	vt := setupVT()
	defer func() {
		// order matters restore text mode after the last flip
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
	ui := NewUI(be, w, h)
	ui.Dev = dev
	frame := NewFrame(w, h)

	start := time.Now()
	last := start
	for !ui.Done {
		select {
		case sig := <-sigCh:
			slog.Info("metal: signal, shutting down", "signal", sig)
			return nil
		default:
		}

		now := time.Since(start).Seconds()
		dt := time.Since(last).Seconds()
		last = time.Now()
		if dt > 0.1 {
			dt = 0.1
		}

	keys:
		for {
			select {
			case ev := <-kb.Ch:
				ui.HandleKey(ev, now)
			default:
				break keys
			}
		}

		ui.Update(dt, now)
		ui.Render(frame, now)
		if err := surf.Present(frame); err != nil {
			return fmt.Errorf("present: %w", err)
		}
	}

	if ui.Success {
		slog.Info("metal: session start requested, handing off")
	}
	return nil
}
