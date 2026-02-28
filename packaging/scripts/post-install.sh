#!/bin/sh
set -e

# Single source of truth for post-install setup.
# Called by: nFPM postinstall, AUR .install, and `make install`.

# --- System user ---
# sysusers.d handles this on systemd distros; this is the fallback.
if ! id -u greetdeez >/dev/null 2>&1; then
    useradd -r -s /usr/bin/nologin -d /var/lib/greetdeez -m greetdeez 2>/dev/null || true
fi

# --- State directory ---
# tmpfiles.d handles this on systemd distros; this is the fallback.
install -d -m 0750 -o greetdeez -g greetdeez /var/lib/greetdeez 2>/dev/null || true

# --- greetd config ---
if [ ! -f /etc/greetd/config.toml ]; then
    # No existing config — install ours as the active config.
    if [ -f /etc/greetd/greetd.toml ]; then
        cp /etc/greetd/greetd.toml /etc/greetd/config.toml
        echo "==> Installed greetd config: /etc/greetd/config.toml"
    fi
elif ! grep -q greetdeez /etc/greetd/config.toml 2>/dev/null; then
    echo "==> greetd config exists but does not reference greetdeez."
    echo "    A reference config is at /etc/greetd/greetd.toml"
    echo "    To activate, set [default_session] in /etc/greetd/config.toml:"
    echo ""
    echo "      [default_session]"
    echo "      command = \"cage -s -- /usr/bin/greetdeez\""
    echo "      user = \"greetdeez\""
    echo ""
fi

# --- Enable greetd ---
if command -v systemctl >/dev/null 2>&1; then
    # Detect any currently enabled display manager.
    current_dm=""
    for dm in sddm gdm lightdm lxdm ly emptty; do
        if systemctl is-enabled "$dm.service" >/dev/null 2>&1; then
            current_dm="$dm"
            break
        fi
    done

    if [ -z "$current_dm" ]; then
        # Nothing else enabled — safe to auto-enable.
        systemctl enable greetd.service >/dev/null 2>&1 || true
        echo "==> Enabled greetd.service (starts on next boot)."
    elif [ "$current_dm" = "greetd" ] || systemctl is-enabled greetd.service >/dev/null 2>&1; then
        # greetd already enabled (possibly alongside another DM the user manages).
        true
    else
        echo "==> ${current_dm}.service is currently enabled."
        echo "    To switch: sudo systemctl disable --now ${current_dm} && sudo systemctl enable greetd"
    fi
fi
