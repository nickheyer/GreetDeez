package webview

/*
#cgo CFLAGS: -I${SRCDIR}/libs/webview/include
#cgo CXXFLAGS: -I${SRCDIR}/libs/webview/include -DWEBVIEW_STATIC -DWEBVIEW_GTK -std=c++11
#cgo LDFLAGS: -ldl
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1

#include "webview.h"

#include <stdlib.h>
#include <stdint.h>

void CgoWebViewDispatch(webview_t w, uintptr_t arg);
void CgoWebViewBind(webview_t w, const char *name, uintptr_t index);
void CgoDisableContextMenu(void *widget);
*/
import "C"
import (
	"encoding/json"
	"runtime"
	"sync"
	"unsafe"

	_ "github.com/nickheyer/greetdeez/pkg/webview/libs/webview"
	_ "github.com/nickheyer/greetdeez/pkg/webview/libs/webview/include"
)

func init() {
	// gtk wants main.main pinned to main thread
	runtime.LockOSThread()
}

// window sizing hints for SetSize
type Hint int

const (
	HintNone  = C.WEBVIEW_HINT_NONE
	HintFixed = C.WEBVIEW_HINT_FIXED
	HintMin   = C.WEBVIEW_HINT_MIN
	HintMax   = C.WEBVIEW_HINT_MAX
)

type WebView interface {
	// Run spins main loop until Terminate
	Run()

	// Terminate stops main loop safe from any thread
	Terminate()

	// Dispatch runs f on main thread
	Dispatch(f func())

	// Destroy tears down webview and window
	Destroy()

	// Window returns GtkWindow pointer
	Window() unsafe.Pointer

	// Widget returns WebKitWebView pointer
	Widget() unsafe.Pointer

	// SetTitle sets native window title
	SetTitle(title string)

	// SetSize sets native window size
	SetSize(w int, h int, hint Hint)

	// Navigate points webview at url
	Navigate(url string)

	// Bind exposes fn as global js function runs off main thread
	Bind(name string, fn func(req string) string)

	// DisableContextMenu kills browser right click menu
	DisableContextMenu()
}

type webview struct {
	w C.webview_t
}

var (
	m        sync.Mutex
	index    uintptr
	dispatch = map[uintptr]func(){}
	bindings = map[uintptr]func(string) string{}
)

func boolToInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

func New(debug bool) WebView {
	w := &webview{}
	w.w = C.webview_create(boolToInt(debug), nil)
	return w
}

func (w *webview) Destroy() {
	C.webview_destroy(w.w)
}

func (w *webview) Run() {
	C.webview_run(w.w)
}

func (w *webview) Terminate() {
	C.webview_terminate(w.w)
}

func (w *webview) Window() unsafe.Pointer {
	return C.webview_get_window(w.w)
}

func (w *webview) Widget() unsafe.Pointer {
	return C.webview_get_native_handle(w.w, C.WEBVIEW_NATIVE_HANDLE_KIND_UI_WIDGET)
}

func (w *webview) Navigate(url string) {
	s := C.CString(url)
	defer C.free(unsafe.Pointer(s))
	C.webview_navigate(w.w, s)
}

func (w *webview) SetTitle(title string) {
	s := C.CString(title)
	defer C.free(unsafe.Pointer(s))
	C.webview_set_title(w.w, s)
}

func (w *webview) SetSize(width int, height int, hint Hint) {
	C.webview_set_size(w.w, C.int(width), C.int(height), C.webview_hint_t(hint))
}

func dispatchOn(w C.webview_t, f func()) {
	m.Lock()
	for ; dispatch[index] != nil; index++ {
	}
	dispatch[index] = f
	m.Unlock()
	C.CgoWebViewDispatch(w, C.uintptr_t(index))
}

func (w *webview) Dispatch(f func()) {
	dispatchOn(w.w, f)
}

//export _webviewDispatchGoCallback
func _webviewDispatchGoCallback(index unsafe.Pointer) {
	m.Lock()
	f := dispatch[uintptr(index)]
	delete(dispatch, uintptr(index))
	m.Unlock()
	f()
}

//export _webviewBindingGoCallback
func _webviewBindingGoCallback(w C.webview_t, id *C.char, req *C.char, index uintptr) {
	m.Lock()
	fn := bindings[uintptr(index)]
	m.Unlock()

	callID := C.GoString(id)
	request := C.GoString(req)

	// handler off main loop pam delays must not freeze ui
	go func() {
		// js args arrive as json array take first
		var args []string
		arg := ""
		if err := json.Unmarshal([]byte(request), &args); err == nil && len(args) > 0 {
			arg = args[0]
		}
		res, _ := json.Marshal(fn(arg))

		// webview_return only from main loop
		dispatchOn(w, func() {
			cid := C.CString(callID)
			cres := C.CString(string(res))
			defer C.free(unsafe.Pointer(cid))
			defer C.free(unsafe.Pointer(cres))
			C.webview_return(w, cid, 0, cres)
		})
	}()
}

func (w *webview) Bind(name string, fn func(req string) string) {
	m.Lock()
	for ; bindings[index] != nil; index++ {
	}
	bindings[index] = fn
	m.Unlock()
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.CgoWebViewBind(w.w, cname, C.uintptr_t(index))
}

func (w *webview) DisableContextMenu() {
	C.CgoDisableContextMenu(w.Widget())
}
