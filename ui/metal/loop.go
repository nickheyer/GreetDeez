package metal

import "time"

// Loop owns the UI plus frame timing so every presenter (bare DRM, a
// window under a compositor) drives the greeter the same way: feed
// events, call Step once per display refresh, blit the returned frame.
type Loop struct {
	UI    *UI
	Frame *Frame
	start time.Time
	last  time.Time
}

func NewLoop(be Backend, w, h int, dev bool) *Loop {
	ui := NewUI(be, w, h)
	ui.Dev = dev
	now := time.Now()
	return &Loop{UI: ui, Frame: NewFrame(w, h), start: now, last: now}
}

func (l *Loop) now() float64 { return time.Since(l.start).Seconds() }

func (l *Loop) Key(ev KeyEvent)     { l.UI.HandleKey(ev, l.now()) }
func (l *Loop) Mouse(ev MouseEvent) { l.UI.HandleMouse(ev, l.now()) }

// Step advances the simulation and renders one frame.
func (l *Loop) Step() *Frame {
	now := l.now()
	dt := time.Since(l.last).Seconds()
	l.last = time.Now()
	if dt > 0.1 {
		dt = 0.1
	}
	l.UI.Update(dt, now)
	l.UI.Render(l.Frame, now)
	return l.Frame
}

func (l *Loop) Done() bool    { return l.UI.Done }
func (l *Loop) Success() bool { return l.UI.Success }
