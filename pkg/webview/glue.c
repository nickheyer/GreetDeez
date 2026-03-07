#include "webview.h"

#include <stdlib.h>
#include <stdint.h>
#include <webkit2/webkit2.h>

struct binding_context {
    webview_t w;
    uintptr_t index;
};

void _webviewDispatchGoCallback(void *);
void _webviewBindingGoCallback(webview_t, char *, char *, uintptr_t);

static void _webview_dispatch_cb(webview_t w, void *arg) {
    _webviewDispatchGoCallback(arg);
}

static void _webview_binding_cb(const char *id, const char *req, void *arg) {
    struct binding_context *ctx = (struct binding_context *) arg;
    _webviewBindingGoCallback(ctx->w, (char *)id, (char *)req, ctx->index);
}

void CgoWebViewDispatch(webview_t w, uintptr_t arg) {
    webview_dispatch(w, _webview_dispatch_cb, (void *)arg);
}

void CgoWebViewBind(webview_t w, const char *name, uintptr_t index) {
    struct binding_context *ctx = calloc(1, sizeof(struct binding_context));
    ctx->w = w;
    ctx->index = index;
    webview_bind(w, name, _webview_binding_cb, (void *)ctx);
}

void CgoWebViewUnbind(webview_t w, const char *name) {
    webview_unbind(w, name);
}

static gboolean _suppress_context_menu(
    WebKitWebView *wv, WebKitContextMenu *menu,
    GdkEvent *event, WebKitHitTestResult *hit, gpointer data) {
    (void)wv; (void)menu; (void)event; (void)hit; (void)data;
    return TRUE;
}

void CgoDisableContextMenu(void *widget) {
    g_signal_connect(widget, "context-menu",
        G_CALLBACK(_suppress_context_menu), NULL);
}
