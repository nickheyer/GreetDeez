#!/bin/bash
set -euo pipefail

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/1000}"
mkdir -p "$XDG_RUNTIME_DIR"

# Force software rendering everywhere — no GPU in container
export LIBGL_ALWAYS_SOFTWARE=1
export GALLIUM_DRIVER=llvmpipe
export WEBKIT_DISABLE_COMPOSITING_MODE=1
export WEBKIT_DISABLE_DMABUF_RENDERER=1

# Start sway in headless mode
echo "[dev] Starting headless sway..."
WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1 \
    sway &
SWAY_PID=$!

# Wait for Wayland socket
echo "[dev] Waiting for Wayland socket..."
for _ in $(seq 1 30); do
    if find "$XDG_RUNTIME_DIR" -maxdepth 1 -name 'wayland-*' -type s 2>/dev/null | grep -q .; then
        break
    fi
    sleep 0.5
done

WAYLAND_DISPLAY=$(find "$XDG_RUNTIME_DIR" -maxdepth 1 -name 'wayland-*' -type s -printf '%f\n' 2>/dev/null | head -1)
export WAYLAND_DISPLAY="${WAYLAND_DISPLAY:-wayland-0}"
echo "[dev] Wayland socket: $WAYLAND_DISPLAY"

# Start wayvnc
echo "[dev] Starting wayvnc on 0.0.0.0:5910..."
wayvnc --output=HEADLESS-1 0.0.0.0 5910 &
WAYVNC_PID=$!
sleep 1

# Start greetd-stub (real greetd IPC protocol, no VT/PAM needed)
SOCK="/tmp/greetd.sock"
echo "[dev] Starting greetd-stub on $SOCK..."
greetd-stub -s "$SOCK" --user test:test &
STUB_PID=$!
sleep 0.5

# Launch greeter as a Wayland client inside sway
export GREETD_SOCK="$SOCK"
export GREETDEEZ_SESSION_DIRS="${GREETDEEZ_SESSION_DIRS:-wayland=/app/testdata/sessions/wayland:x11=/app/testdata/sessions/x11}"
echo "[dev] Starting greetdeez..."
/app/greetdeez -dev &
GREETER_PID=$!

echo "[dev] Ready! Connect VNC to localhost:5910"
echo "[dev] Test user: test:test"

# Wait for any process to exit
wait -n $SWAY_PID $WAYVNC_PID $STUB_PID $GREETER_PID 2>/dev/null || true

echo "[dev] Shutting down..."
kill $GREETER_PID $STUB_PID $WAYVNC_PID $SWAY_PID 2>/dev/null || true
wait
