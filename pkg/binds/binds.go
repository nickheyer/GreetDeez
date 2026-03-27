package binds

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
*/
import "C"
import "unsafe"

// Fullscreens and chops browser deco
func Fullscreen(gtkWindow unsafe.Pointer) {
	win := (*C.GtkWindow)(gtkWindow)
	C.gtk_window_set_decorated(win, C.FALSE)
	C.gtk_window_fullscreen(win)
}

// SetZoomLevel sets the page zoom on the WebKitWebView.
// scale=2 means everything renders at 2× size.
func SetZoomLevel(webkitWebView unsafe.Pointer, scale int) {
	if scale <= 1 {
		return
	}
	wv := (*C.WebKitWebView)(webkitWebView)
	C.webkit_web_view_set_zoom_level(wv, C.double(scale))
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
