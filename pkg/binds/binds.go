package binds

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 wayland-client
#cgo CFLAGS: -I${SRCDIR}/libs/wlr/include
#cgo LDFLAGS: -lm
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
#include <string.h>
#include <dlfcn.h>

extern int configure_output_scale(double scale);

// header guards can hide gdk_monitor_get_connector so resolve at runtime
typedef const char* (*gdk_monitor_get_connector_fn)(GdkMonitor*);
static const char* try_get_connector(GdkMonitor *mon) {
    static gdk_monitor_get_connector_fn fn = NULL;
    static gboolean resolved = FALSE;
    if (!resolved) {
        fn = (gdk_monitor_get_connector_fn)dlsym(RTLD_DEFAULT,
                                                  "gdk_monitor_get_connector");
        resolved = TRUE;
    }
    return fn ? fn(mon) : NULL;
}

static int get_n_monitors(void) {
    GdkDisplay *d = gdk_display_get_default();
    return d ? gdk_display_get_n_monitors(d) : 0;
}

static void get_monitor_info(int idx, int *width, int *height,
                             int *width_mm, int *height_mm,
                             char *connector_out, int buf_size) {
    connector_out[0] = '\0';
    *width = 0; *height = 0; *width_mm = 0; *height_mm = 0;
    GdkDisplay *d = gdk_display_get_default();
    if (!d) return;
    int n = gdk_display_get_n_monitors(d);
    if (idx < 0 || idx >= n) return;
    GdkMonitor *mon = gdk_display_get_monitor(d, idx);
    GdkRectangle geo;
    gdk_monitor_get_geometry(mon, &geo);
    *width = geo.width;
    *height = geo.height;
    *width_mm = gdk_monitor_get_width_mm(mon);
    *height_mm = gdk_monitor_get_height_mm(mon);
    const char *conn = try_get_connector(mon);
    if (conn) {
        strncpy(connector_out, conn, buf_size - 1);
        connector_out[buf_size - 1] = '\0';
    }
}

static void fullscreen_on_monitor_idx(GtkWindow *win, int idx) {
    gtk_window_set_decorated(win, FALSE);
    GdkScreen *screen = gtk_window_get_screen(win);
    gtk_window_fullscreen_on_monitor(win, screen, idx);
}
*/
import "C"
import (
	"log/slog"
	"unsafe"
)

// MonitorInfo holds information about a GDK monitor from the running compositor.
type MonitorInfo struct {
	Index     int
	Width     int
	Height    int
	WidthMM   int
	HeightMM  int
	Connector string
}

// EnumerateMonitors lists gdk monitors call after webview.New
func EnumerateMonitors() []MonitorInfo {
	n := int(C.get_n_monitors())
	monitors := make([]MonitorInfo, 0, n)
	for i := 0; i < n; i++ {
		var w, h, wmm, hmm C.int
		var conn [64]C.char
		C.get_monitor_info(C.int(i), &w, &h, &wmm, &hmm, &conn[0], 64)
		monitors = append(monitors, MonitorInfo{
			Index:     i,
			Width:     int(w),
			Height:    int(h),
			WidthMM:   int(wmm),
			HeightMM:  int(hmm),
			Connector: C.GoString(&conn[0]),
		})
	}
	return monitors
}

// FullscreenOnMonitor fullscreens the window on the GDK monitor at the given index.
func FullscreenOnMonitor(gtkWindow unsafe.Pointer, idx int) {
	win := (*C.GtkWindow)(gtkWindow)
	C.fullscreen_on_monitor_idx(win, C.int(idx))
}

// Fullscreen fullscreens the window on the default monitor.
func Fullscreen(gtkWindow unsafe.Pointer) {
	win := (*C.GtkWindow)(gtkWindow)
	C.gtk_window_set_decorated(win, C.FALSE)
	C.gtk_window_fullscreen(win)
}

// Undecorate strips the CSD title bar without fullscreening
func Undecorate(gtkWindow unsafe.Pointer) {
	win := (*C.GtkWindow)(gtkWindow)
	C.gtk_window_set_decorated(win, C.FALSE)
}

// ConfigureOutputScale sets wlr output scale call before webview.New
func ConfigureOutputScale(scale float64) {
	if scale <= 0 {
		slog.Info("no output scale configured, skipping wlr-output-management")
		return
	}
	rc := int(C.configure_output_scale(C.double(scale)))
	switch rc {
	case 1:
		slog.Info("output scale configured via wlr-output-management", "scale", scale)
	case 0:
		slog.Info("no output scale change needed")
	case -1:
		slog.Warn("could not connect to wayland display for scale configuration")
	case -2:
		slog.Warn("compositor does not support wlr-output-management protocol")
	default:
		slog.Error("output scale configuration failed", "rc", rc)
	}
}

// Banning browser features bad for greeter
func HardenWebView(webkitWebView unsafe.Pointer) {
	wv := (*C.WebKitWebView)(webkitWebView)
	settings := C.webkit_web_view_get_settings(wv)
	C.webkit_settings_set_enable_developer_extras(settings, C.FALSE)

	css := C.CString(`*, *::before, *::after {
  -webkit-user-select: none;
  user-select: none;
}
input, textarea, [contenteditable="true"] {
  -webkit-user-select: text;
  user-select: text;
}`)
	defer C.free(unsafe.Pointer(css))
	sheet := C.webkit_user_style_sheet_new(
		css,
		C.WEBKIT_USER_CONTENT_INJECT_ALL_FRAMES,
		C.WEBKIT_USER_STYLE_LEVEL_USER,
		nil, nil,
	)
	manager := C.webkit_web_view_get_user_content_manager(wv)
	C.webkit_user_content_manager_add_style_sheet(manager, sheet)
	C.webkit_user_style_sheet_unref(sheet)
}
