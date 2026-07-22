// Package gtkwin presents the metal theme inside a GTK window so it can
// run anywhere a compositor already owns the display: fullscreen under
// cage (legacy greetd configs that wrap greetdeez) or windowed on a
// desktop for development. Rendering and input semantics are identical
// to the DRM path; only the presentation differs.
package gtkwin

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <gdk/gdk.h>

extern gboolean greetdeezTick(gpointer data);
extern gboolean greetdeezDraw(GtkWidget *w, cairo_t *cr, gpointer data);
extern gboolean greetdeezKey(GtkWidget *w, GdkEventKey *ev, gpointer data);
extern gboolean greetdeezMotion(GtkWidget *w, GdkEventMotion *ev, gpointer data);
extern gboolean greetdeezButton(GtkWidget *w, GdkEventButton *ev, gpointer data);
extern gboolean greetdeezScroll(GtkWidget *w, GdkEventScroll *ev, gpointer data);
extern gboolean greetdeezDelete(GtkWidget *w, GdkEvent *ev, gpointer data);

static void connect_signals(GtkWidget *win, GtkWidget *area) {
	gtk_widget_add_events(win, GDK_KEY_PRESS_MASK | GDK_KEY_RELEASE_MASK |
		GDK_POINTER_MOTION_MASK | GDK_BUTTON_PRESS_MASK |
		GDK_BUTTON_RELEASE_MASK | GDK_SCROLL_MASK | GDK_SMOOTH_SCROLL_MASK);
	g_signal_connect(win, "key-press-event", G_CALLBACK(greetdeezKey), (gpointer)1);
	g_signal_connect(win, "key-release-event", G_CALLBACK(greetdeezKey), (gpointer)0);
	g_signal_connect(win, "motion-notify-event", G_CALLBACK(greetdeezMotion), NULL);
	g_signal_connect(win, "button-press-event", G_CALLBACK(greetdeezButton), (gpointer)1);
	g_signal_connect(win, "button-release-event", G_CALLBACK(greetdeezButton), (gpointer)0);
	g_signal_connect(win, "scroll-event", G_CALLBACK(greetdeezScroll), NULL);
	g_signal_connect(win, "delete-event", G_CALLBACK(greetdeezDelete), NULL);
	g_signal_connect(area, "draw", G_CALLBACK(greetdeezDraw), NULL);
}

// nearest-neighbor scale keeps the chunky pixels chunky
static void blit(cairo_t *cr, unsigned char *pix, int w, int h, double sx, double sy) {
	cairo_surface_t *s = cairo_image_surface_create_for_data(
		pix, CAIRO_FORMAT_RGB24, w, h, w * 4);
	cairo_scale(cr, sx, sy);
	cairo_set_source_surface(cr, s, 0, 0);
	cairo_pattern_set_filter(cairo_get_source(cr), CAIRO_FILTER_NEAREST);
	cairo_paint(cr);
	cairo_surface_destroy(s);
}

// the theme draws its own cursor so the system one must vanish
static void hide_cursor(GtkWidget *win) {
	GdkWindow *gw = gtk_widget_get_window(win);
	if (!gw) return;
	GdkCursor *cur = gdk_cursor_new_from_name(gdk_window_get_display(gw), "none");
	if (!cur) return;
	gdk_window_set_cursor(gw, cur);
	g_object_unref(cur);
}

static guint add_tick(void) {
	return g_timeout_add(16, (GSourceFunc)greetdeezTick, NULL);
}

static gboolean quit_cb(gpointer data) { gtk_main_quit(); return FALSE; }

// safe from any thread
static void schedule_quit(void) { g_idle_add(quit_cb, NULL); }
*/
import "C"

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/nickheyer/greetdeez/pkg/binds"
	"github.com/nickheyer/greetdeez/pkg/outputs"
	"github.com/nickheyer/greetdeez/ui/metal"
)

// devW/devH is the preview window; fullscreen sizes come from the monitor.
const (
	devW = 1280
	devH = 720
)

// one presenter per process, reachable from the exported C callbacks
var st struct {
	loop      *metal.Loop
	area      *C.GtkWidget
	fw, fh    int
	scrollAcc float64
}

// Run shows the metal theme under the current compositor and blocks
// until login succeeds, the window closes (dev), or a signal arrives.
func Run(socketPath string, timeout time.Duration, dev bool, output string) error {
	runtime.LockOSThread()

	be, err := metal.DialBackend(socketPath, timeout)
	if err != nil {
		return fmt.Errorf("dial rpc socket: %w", err)
	}

	if C.gtk_init_check(nil, nil) == C.FALSE {
		return fmt.Errorf("gtk: no display to connect to")
	}

	win := C.gtk_window_new(C.GTK_WINDOW_TOPLEVEL)
	title := C.CString("GreetDeez metal")
	C.gtk_window_set_title((*C.GtkWindow)(unsafe.Pointer(win)), title)
	C.free(unsafe.Pointer(title))

	st.fw, st.fh = devW, devH
	if dev {
		C.gtk_window_set_default_size((*C.GtkWindow)(unsafe.Pointer(win)), devW, devH)
	} else {
		monitors := binds.EnumerateMonitors()
		outs := make([]outputs.Output, len(monitors))
		for i, m := range monitors {
			outs[i] = outputs.Output{
				Name: m.Connector, Width: m.Width, Height: m.Height,
				WidthMM: m.WidthMM, HeightMM: m.HeightMM,
			}
		}
		if idx := outputs.Pick(outs, output); idx >= 0 {
			st.fw, st.fh = monitors[idx].Width, monitors[idx].Height
			binds.FullscreenOnMonitor(unsafe.Pointer(win), monitors[idx].Index)
		} else {
			binds.Fullscreen(unsafe.Pointer(win))
		}
	}

	st.area = C.gtk_drawing_area_new()
	C.gtk_container_add((*C.GtkContainer)(unsafe.Pointer(win)), st.area)
	C.connect_signals(win, st.area)

	slog.Info("metal: windowed presenter up", "resolution", fmt.Sprintf("%dx%d", st.fw, st.fh), "dev", dev)
	st.loop = metal.NewLoop(be, st.fw, st.fh, dev)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		slog.Info("metal: signal, shutting down", "signal", sig)
		C.schedule_quit()
	}()

	C.gtk_widget_show_all(win)
	C.hide_cursor(win)
	C.add_tick()
	C.gtk_main()

	if st.loop.Success() {
		slog.Info("metal: session start requested, handing off")
	}
	return nil
}

// frameScale maps window coords onto frame coords.
func frameScale() (sx, sy float64) {
	var alloc C.GtkAllocation
	C.gtk_widget_get_allocation(st.area, &alloc)
	if alloc.width <= 0 || alloc.height <= 0 {
		return 1, 1
	}
	return float64(alloc.width) / float64(st.fw), float64(alloc.height) / float64(st.fh)
}

//export greetdeezTick
func greetdeezTick(data C.gpointer) C.gboolean {
	if st.loop.Done() {
		C.gtk_main_quit()
		return C.FALSE
	}
	st.loop.Step()
	C.gtk_widget_queue_draw(st.area)
	return C.TRUE
}

//export greetdeezDraw
func greetdeezDraw(w *C.GtkWidget, cr *C.cairo_t, data C.gpointer) C.gboolean {
	f := st.loop.Frame
	sx, sy := frameScale()
	C.blit(cr, (*C.uchar)(unsafe.Pointer(&f.Pix[0])), C.int(f.W), C.int(f.H), C.double(sx), C.double(sy))
	return C.TRUE
}

//export greetdeezKey
func greetdeezKey(w *C.GtkWidget, ev *C.GdkEventKey, data C.gpointer) C.gboolean {
	// gdk hardware keycodes are evdev codes shifted by 8 on both x11
	// and wayland backends
	code := int(ev.hardware_keycode) - 8
	if code <= 0 {
		return C.TRUE
	}
	st.loop.Key(metal.KeyEvent{Code: uint16(code), Down: uintptr(data) == 1})
	return C.TRUE
}

//export greetdeezMotion
func greetdeezMotion(w *C.GtkWidget, ev *C.GdkEventMotion, data C.gpointer) C.gboolean {
	sx, sy := frameScale()
	st.loop.Mouse(metal.MouseEvent{Abs: true, X: float64(ev.x) / sx, Y: float64(ev.y) / sy})
	return C.TRUE
}

//export greetdeezButton
func greetdeezButton(w *C.GtkWidget, ev *C.GdkEventButton, data C.gpointer) C.gboolean {
	if ev._type == C.GDK_2BUTTON_PRESS || ev._type == C.GDK_3BUTTON_PRESS {
		return C.TRUE // synthetic double/triple events duplicate the press
	}
	sx, sy := frameScale()
	st.loop.Mouse(metal.MouseEvent{
		Abs: true, X: float64(ev.x) / sx, Y: float64(ev.y) / sy,
		Btn: int(ev.button), Down: uintptr(data) == 1,
	})
	return C.TRUE
}

//export greetdeezScroll
func greetdeezScroll(w *C.GtkWidget, ev *C.GdkEventScroll, data C.gpointer) C.gboolean {
	step := 0
	switch ev.direction {
	case C.GDK_SCROLL_UP:
		step = 1
	case C.GDK_SCROLL_DOWN:
		step = -1
	case C.GDK_SCROLL_SMOOTH:
		st.scrollAcc += float64(ev.delta_y)
		for st.scrollAcc >= 1 {
			st.scrollAcc--
			step-- // positive delta scrolls down
		}
		for st.scrollAcc <= -1 {
			st.scrollAcc++
			step++
		}
	}
	if step != 0 {
		st.loop.Mouse(metal.MouseEvent{Wheel: step})
	}
	return C.TRUE
}

//export greetdeezDelete
func greetdeezDelete(w *C.GtkWidget, ev *C.GdkEvent, data C.gpointer) C.gboolean {
	C.gtk_main_quit()
	return C.FALSE
}
