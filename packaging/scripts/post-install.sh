#!/bin/sh
set -e

# Create greetdeez system user (for non-Arch distros without sysusers.d hooks).
# On Arch, sysusers.d/greetdeez.conf handles this automatically.
if ! id -u greetdeez >/dev/null 2>&1; then
    useradd -r -s /usr/bin/nologin -d /var/lib/greetdeez -m greetdeez 2>/dev/null || true
    echo "Created system user: greetdeez"
fi

# Ensure state directory exists
install -d -m 0750 -o greetdeez -g greetdeez /var/lib/greetdeez 2>/dev/null || true

# Install greetd config if none exists
if [ ! -f /etc/greetd/config.toml ]; then
    if [ -f /etc/greetd/greetd.toml ]; then
        cp /etc/greetd/greetd.toml /etc/greetd/config.toml
        echo "==> Installed greetd config: /etc/greetd/config.toml"
    fi
elif ! grep -q greetdeez /etc/greetd/config.toml; then
    echo ""
    echo "==> greetd config exists but does not use greetdeez."
    echo "    To activate greetdeez, update [default_session] in /etc/greetd/config.toml:"
    echo ""
    echo "      [default_session]"
    echo "      command = \"cage -s -- /usr/bin/greetdeez\""
    echo "      user = \"greetdeez\""
    echo ""
fi

echo ""
echo "==> To switch to greetdeez from your current display manager:"
echo "    1. sudo systemctl disable --now <current-dm>   (e.g. sddm, gdm, lightdm)"
echo "    2. sudo systemctl enable greetd"
echo "    3. reboot"
echo ""
