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

	// the kernel keyboard is off so ctrl+alt+fn is ours to implement:
	// VT_PROCESS mode makes switches a signal handshake with us
	vtCh := make(chan os.Signal, 4)
	signal.Notify(vtCh, syscall.SIGUSR1, syscall.SIGUSR2)
	vt.takeControl(syscall.SIGUSR1, syscall.SIGUSR2)

	w, h := surf.Size()
	slog.Info("metal: up", "resolution", fmt.Sprintf("%dx%d", w, h))
	loop := NewLoop(be, w, h, dev)

	var chord chordState
	away := false

	for !loop.Done() {
		select {
		case sig := <-sigCh:
			slog.Info("metal: signal, shutting down", "signal", sig)
			return nil
		case sig := <-vtCh:
			away = handleVTSignal(sig, surf, vt, loop, away)
			continue
		default:
		}

		if away {
			// parked on another vt: keep answering signals but swallow
			// input, keystrokes typed over there must never reach the
			// form. Modifier tracking stays live so the chord state is
			// right when we come back.
			select {
			case sig := <-sigCh:
				slog.Info("metal: signal, shutting down", "signal", sig)
				return nil
			case sig := <-vtCh:
				away = handleVTSignal(sig, surf, vt, loop, away)
			case ev := <-kb.Ch:
				chord.consume(ev)
			case <-mice.Ch:
			}
			continue
		}

	events:
		for {
			select {
			case ev := <-kb.Ch:
				if n, ok := chord.consume(ev); ok {
					slog.Info("metal: vt switch", "vt", n)
					vt.switchTo(n)
					continue
				}
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

// handleVTSignal runs the VT_PROCESS handshake: relsig means drop the
// gpu and approve the switch, acqsig means ack and take everything back.
// Returns whether we are now parked on another vt.
func handleVTSignal(sig os.Signal, surf *DRMSurface, vt *vtGuard, loop *Loop, away bool) bool {
	switch sig {
	case syscall.SIGUSR1:
		if !away {
			slog.Info("metal: vt released")
			surf.Pause()
			vt.ackRelease()
		}
		return true
	case syscall.SIGUSR2:
		vt.ackAcquire()
		surf.Resume()
		loop.UI.ResetMods() // releases were swallowed while away
		slog.Info("metal: vt reacquired")
		return false
	}
	return away
}

// chordState watches raw key events for ctrl+alt+fn
type chordState struct {
	ctrlL, ctrlR, altL, altR bool
}

// consume tracks modifiers and reports the target vt when an event
// completes a switch chord.
func (c *chordState) consume(ev KeyEvent) (int, bool) {
	switch ev.Code {
	case keyLeftCtrl:
		c.ctrlL = ev.Down
	case keyRightCtrl:
		c.ctrlR = ev.Down
	case keyLeftAlt:
		c.altL = ev.Down
	case keyRightAlt:
		c.altR = ev.Down
	default:
		if ev.Down && !ev.Repeat && (c.ctrlL || c.ctrlR) && (c.altL || c.altR) {
			if n := fkeyNum(ev.Code); n > 0 {
				return n, true
			}
		}
	}
	return 0, false
}

// f1..f10 are contiguous evdev codes, f11 and f12 are not
func fkeyNum(code uint16) int {
	switch {
	case code >= keyF1 && code <= keyF10:
		return int(code-keyF1) + 1
	case code == keyF11:
		return 11
	case code == keyF12:
		return 12
	}
	return 0
}
