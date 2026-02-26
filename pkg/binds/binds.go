package binds

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
*/
import "C"
import "unsafe"

// Fullscreen removes window decorations and fullscreens a GtkWindow.
func Fullscreen(gtkWindow unsafe.Pointer) {
	win := (*C.GtkWindow)(gtkWindow)
	C.gtk_window_set_decorated(win, C.FALSE)
	C.gtk_window_fullscreen(win)
}
