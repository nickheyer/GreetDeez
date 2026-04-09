#include <wayland-client.h>
#include "wlr_output_management_client.h"
#include "libs/wlr/wlr_output_management_protocol.c"
#include <math.h>
#include <stdlib.h>
#include <string.h>

#define MAX_HEADS 8
#define MAX_MODES 64

struct mode_info {
    struct zwlr_output_mode_v1 *wl_mode;
    int width, height;
    int preferred;
};

struct head_info {
    struct zwlr_output_head_v1 *wl_head;
    char name[64];
    int phys_width_mm, phys_height_mm;
    int enabled;
    struct mode_info modes[MAX_MODES];
    int n_modes;
    struct zwlr_output_mode_v1 *current_mode;
};

static struct {
    struct wl_display *display;
    struct wl_registry *registry;
    struct zwlr_output_manager_v1 *manager;
    uint32_t serial;
    struct head_info heads[MAX_HEADS];
    int n_heads;
    int config_result; /* 0=pending, 1=ok, -1=fail */
} S;

/* ---- mode listeners ---- */
static void ml_size(void *d, struct zwlr_output_mode_v1 *m, int w, int h) {
    struct mode_info *mi = d; mi->width = w; mi->height = h;
}
static void ml_refresh(void *d, struct zwlr_output_mode_v1 *m, int r) { (void)d;(void)m;(void)r; }
static void ml_preferred(void *d, struct zwlr_output_mode_v1 *m) {
    ((struct mode_info *)d)->preferred = 1;
}
static void ml_finished(void *d, struct zwlr_output_mode_v1 *m) { (void)d;(void)m; }
static const struct zwlr_output_mode_v1_listener mode_listener = {
    .size = ml_size, .refresh = ml_refresh, .preferred = ml_preferred, .finished = ml_finished,
};

/* ---- head listeners ---- */
static void hl_name(void *d, struct zwlr_output_head_v1 *h, const char *n) {
    strncpy(((struct head_info*)d)->name, n, 63);
}
static void hl_desc(void *d, struct zwlr_output_head_v1 *h, const char *s) { (void)d;(void)h;(void)s; }
static void hl_phys(void *d, struct zwlr_output_head_v1 *h, int w, int hmm) {
    struct head_info *hi = d; hi->phys_width_mm = w; hi->phys_height_mm = hmm;
}
static void hl_mode(void *d, struct zwlr_output_head_v1 *h, struct zwlr_output_mode_v1 *m) {
    struct head_info *hi = d;
    if (hi->n_modes >= MAX_MODES) return;
    struct mode_info *mi = &hi->modes[hi->n_modes++];
    memset(mi, 0, sizeof(*mi));
    mi->wl_mode = m;
    zwlr_output_mode_v1_add_listener(m, &mode_listener, mi);
}
static void hl_enabled(void *d, struct zwlr_output_head_v1 *h, int e) {
    ((struct head_info*)d)->enabled = e;
}
static void hl_cur_mode(void *d, struct zwlr_output_head_v1 *h, struct zwlr_output_mode_v1 *m) {
    ((struct head_info*)d)->current_mode = m;
}
static void hl_pos(void *d, struct zwlr_output_head_v1 *h, int x, int y) { (void)d;(void)h;(void)x;(void)y; }
static void hl_transform(void *d, struct zwlr_output_head_v1 *h, int t) { (void)d;(void)h;(void)t; }
static void hl_scale(void *d, struct zwlr_output_head_v1 *h, wl_fixed_t s) { (void)d;(void)h;(void)s; }
static void hl_finished(void *d, struct zwlr_output_head_v1 *h) { (void)d;(void)h; }
static void hl_make(void *d, struct zwlr_output_head_v1 *h, const char *s) { (void)d;(void)h;(void)s; }
static void hl_model(void *d, struct zwlr_output_head_v1 *h, const char *s) { (void)d;(void)h;(void)s; }
static void hl_serial(void *d, struct zwlr_output_head_v1 *h, const char *s) { (void)d;(void)h;(void)s; }
static void hl_adaptive(void *d, struct zwlr_output_head_v1 *h, uint32_t a) { (void)d;(void)h;(void)a; }
static const struct zwlr_output_head_v1_listener head_listener = {
    .name = hl_name, .description = hl_desc, .physical_size = hl_phys,
    .mode = hl_mode, .enabled = hl_enabled, .current_mode = hl_cur_mode,
    .position = hl_pos, .transform = hl_transform, .scale = hl_scale,
    .finished = hl_finished, .make = hl_make, .model = hl_model,
    .serial_number = hl_serial, .adaptive_sync = hl_adaptive,
};

/* ---- manager listeners ---- */
static void mgr_head(void *d, struct zwlr_output_manager_v1 *mgr, struct zwlr_output_head_v1 *head) {
    if (S.n_heads >= MAX_HEADS) return;
    struct head_info *hi = &S.heads[S.n_heads++];
    memset(hi, 0, sizeof(*hi));
    hi->wl_head = head;
    zwlr_output_head_v1_add_listener(head, &head_listener, hi);
}
static void mgr_done(void *d, struct zwlr_output_manager_v1 *mgr, uint32_t serial) {
    S.serial = serial;
}
static void mgr_finished(void *d, struct zwlr_output_manager_v1 *mgr) { (void)d;(void)mgr; }
static const struct zwlr_output_manager_v1_listener mgr_listener = {
    .head = mgr_head, .done = mgr_done, .finished = mgr_finished,
};

/* ---- config listeners ---- */
static void cfg_ok(void *d, struct zwlr_output_configuration_v1 *c) { S.config_result = 1; }
static void cfg_fail(void *d, struct zwlr_output_configuration_v1 *c) { S.config_result = -1; }
static void cfg_cancel(void *d, struct zwlr_output_configuration_v1 *c) { S.config_result = -1; }
static const struct zwlr_output_configuration_v1_listener cfg_listener = {
    .succeeded = cfg_ok, .failed = cfg_fail, .cancelled = cfg_cancel,
};

/* ---- registry ---- */
static void reg_global(void *d, struct wl_registry *r, uint32_t name, const char *iface, uint32_t ver) {
    if (strcmp(iface, zwlr_output_manager_v1_interface.name) == 0) {
        uint32_t v = ver < 4 ? ver : 4;
        S.manager = wl_registry_bind(r, name, &zwlr_output_manager_v1_interface, v);
        zwlr_output_manager_v1_add_listener(S.manager, &mgr_listener, NULL);
    }
}
static void reg_remove(void *d, struct wl_registry *r, uint32_t n) { (void)d;(void)r;(void)n; }
static const struct wl_registry_listener reg_listener = {
    .global = reg_global, .global_remove = reg_remove,
};

/*
 * configure_output_scale: connect to the compositor via wlr-output-management
 * protocol and set the given scale on all enabled outputs.
 * Returns 1 on success, 0 if nothing to do, negative on error.
 */
int configure_output_scale(double scale) {
    if (scale <= 0.0) return 0;

    memset(&S, 0, sizeof(S));

    S.display = wl_display_connect(NULL);
    if (!S.display) return -1;

    S.registry = wl_display_get_registry(S.display);
    wl_registry_add_listener(S.registry, &reg_listener, NULL);
    wl_display_roundtrip(S.display);

    if (!S.manager) {
        wl_registry_destroy(S.registry);
        wl_display_disconnect(S.display);
        return -2;
    }

    wl_display_roundtrip(S.display);

    if (S.serial == 0 || S.n_heads == 0) {
        zwlr_output_manager_v1_destroy(S.manager);
        wl_registry_destroy(S.registry);
        wl_display_disconnect(S.display);
        return 0;
    }

    struct zwlr_output_configuration_v1 *cfg =
        zwlr_output_manager_v1_create_configuration(S.manager, S.serial);
    zwlr_output_configuration_v1_add_listener(cfg, &cfg_listener, NULL);

    for (int i = 0; i < S.n_heads; i++) {
        struct head_info *hi = &S.heads[i];
        if (!hi->enabled) {
            zwlr_output_configuration_v1_disable_head(cfg, hi->wl_head);
            continue;
        }

        struct zwlr_output_configuration_head_v1 *ch =
            zwlr_output_configuration_v1_enable_head(cfg, hi->wl_head);

        if (hi->current_mode)
            zwlr_output_configuration_head_v1_set_mode(ch, hi->current_mode);

        zwlr_output_configuration_head_v1_set_scale(ch, wl_fixed_from_double(scale));
    }

    zwlr_output_configuration_v1_apply(cfg);
    wl_display_roundtrip(S.display);

    int result = S.config_result;

    zwlr_output_configuration_v1_destroy(cfg);
    zwlr_output_manager_v1_destroy(S.manager);
    wl_registry_destroy(S.registry);
    wl_display_disconnect(S.display);
    return result;
}
