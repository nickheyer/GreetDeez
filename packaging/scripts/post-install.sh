#!/bin/sh
set -e

# post install shared by nfpm aur and make install

# system user fallback when no sysusers.d
if ! id -u greetdeez >/dev/null 2>&1; then
    useradd -r -s /usr/bin/nologin -d /var/lib/greetdeez -m greetdeez 2>/dev/null || true
fi

# metal theme needs direct drm and evdev access
for grp in video render input; do
    getent group "$grp" >/dev/null 2>&1 && usermod -aG "$grp" greetdeez 2>/dev/null || true
done

# state dir fallback when no tmpfiles.d
install -d -m 0750 -o greetdeez -g greetdeez /var/cache/greetdeez 2>/dev/null || true

# pam keyring kwallet parity dash prefix skips missing modules
if [ -f /etc/pam.d/greetd ] && ! grep -q 'pam_kwallet5' /etc/pam.d/greetd 2>/dev/null; then
    printf '\n%s\n%s\n%s\n%s\n%s\n' \
        '-auth       optional    pam_gnome_keyring.so' \
        '-auth       optional    pam_kwallet5.so' \
        '-password   optional    pam_gnome_keyring.so    use_authtok' \
        '-session    optional    pam_gnome_keyring.so    auto_start' \
        '-session    optional    pam_kwallet5.so         auto_start' \
        >> /etc/pam.d/greetd
    echo "==> Patched /etc/pam.d/greetd with keyring/kwallet support"
fi

# --- greetd config ---
if [ -f /etc/greetd/config.toml ]; then
    echo "==> Found existing greetd config for backup: /etc/greetd/config.toml"
    mv /etc/greetd/config.toml /etc/greetd/config.bak
    echo "==> Backed up existing greetd config: /etc/greetd/config.bak"
fi

echo "==> Installing packaged greetd config: /etc/greetd/greetd.toml"
mv /etc/greetd/greetd.toml /etc/greetd/config.toml
echo "==> Installed packaged greetd config: /etc/greetd/config.toml"

# --- Enable greetd ---
if command -v systemctl >/dev/null 2>&1; then
    for dm in sddm gdm lightdm lxdm ly emptty; do
        systemctl disable "$dm.service" >/dev/null 2>&1 || true
    done

    systemctl set-default graphical.target >/dev/null 2>&1 || true
    systemctl enable greetd.service >/dev/null 2>&1 || true
    echo "==> Enabled greetd.service"
    echo "    To start GreetDeez Greeter (via greetd service):"
    echo ""
    echo "    Reboot this device"
    echo "    or run the following:"
    echo "    sudo systemctl start greetd.service"
    echo ""
elif command -v rc-update >/dev/null 2>&1; then
    rc-update add greetd default >/dev/null 2>&1 || true
    echo "==> Enabled greetd service (OpenRC)"
fi
