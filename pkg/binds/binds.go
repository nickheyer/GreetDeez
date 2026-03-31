package binds

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
*/
import "C"
import (
	"fmt"
	"log/slog"
	"math"
	"unsafe"
)

// Fullscreens and chops browser deco
func Fullscreen(gtkWindow unsafe.Pointer) {
	win := (*C.GtkWindow)(gtkWindow)
	C.gtk_window_set_decorated(win, C.FALSE)
	C.gtk_window_fullscreen(win)
}

// SetZoomLevel sets the page zoom on the WebKitWebView.
// scale=2.0 means everything renders at 2× size.
func SetZoomLevel(webkitWebView unsafe.Pointer, scale float64) {
	if scale <= 1.0 {
		return
	}
	wv := (*C.WebKitWebView)(webkitWebView)
	C.webkit_web_view_set_zoom_level(wv, C.double(scale))
}

// DetectMonitorScale queries GDK for the scale factor and physical dimensions
// of the monitor the window is currently on, and returns an appropriate scale.
// Must be called from the GTK main thread after the window is mapped.
func DetectMonitorScale(gtkWindow unsafe.Pointer) float64 {
	const baseDPI = 96.0

	win := (*C.GtkWidget)(gtkWindow)
	gdkWin := C.gtk_widget_get_window(win)
	if gdkWin == nil {
		slog.Warn("scale detect: gtk_widget_get_window returned nil")
		return 1.0
	}
	display := C.gdk_display_get_default()
	if display == nil {
		slog.Warn("scale detect: gdk_display_get_default returned nil")
		return 1.0
	}
	monitor := C.gdk_display_get_monitor_at_window(display, gdkWin)
	if monitor == nil {
		slog.Warn("scale detect: gdk_display_get_monitor_at_window returned nil")
		return 1.0
	}

	// First check the GDK scale factor — this is authoritative when the
	// compositor (e.g. cage) actually reports one.
	gdkScale := int(C.gdk_monitor_get_scale_factor(monitor))
	slog.Info("scale detect: gdk scale factor", "gdk_scale", gdkScale)
	if gdkScale > 1 {
		return float64(gdkScale)
	}

	// Fallback: compute from physical dimensions + pixel geometry.
	var geo C.GdkRectangle
	C.gdk_monitor_get_geometry(monitor, &geo)
	widthMm := int(C.gdk_monitor_get_width_mm(monitor))

	slog.Info("scale detect: monitor geometry",
		"width_px", int(geo.width), "height_px", int(geo.height),
		"width_mm", widthMm)

	if widthMm <= 0 || int(geo.width) <= 0 {
		slog.Warn("scale detect: physical dimensions unavailable from compositor")
		return 1.0
	}

	dpi := float64(geo.width) / (float64(widthMm) / 25.4)
	ratio := dpi / baseDPI
	slog.Info("scale detect: computed DPI", "dpi", fmt.Sprintf("%.1f", dpi), "ratio", fmt.Sprintf("%.2f", ratio))

	if ratio >= 1.2 {
		return math.Ceil(ratio)
	}
	return 1.0
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
