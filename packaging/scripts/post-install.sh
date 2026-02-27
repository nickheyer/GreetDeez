#!/bin/bash

# Create greetdeez system user if it doesn't exist
if ! id -u greetdeez >/dev/null 2>&1; then
    useradd -r -s /usr/bin/nologin -d /var/lib/greetdeez -m greetdeez 2>/dev/null || true
    echo "Created system user: greetdeez"
fi

# Install greetd config only if no config.toml exists at all.
# If one exists, print instructions instead of overwriting.
CONFIG="/etc/greetd/config.toml"
TEMPLATE="/etc/greetd/greetd.toml"

if [ ! -f "$CONFIG" ]; then
    if [ -f "$TEMPLATE" ]; then
        cp "$TEMPLATE" "$CONFIG"
        echo "Installed greetdeez config to $CONFIG"
    fi
elif ! grep -q "greetdeez" "$CONFIG"; then
    echo ""
    echo "========================================="
    echo " greetd config.toml exists but does not"
    echo " reference greetdeez. To use greetdeez,"
    echo " update your [default_session] in:"
    echo "   $CONFIG"
    echo ""
    echo " Example:"
    echo "   [default_session]"
    echo "   command = \"cage -s -- /usr/bin/greetdeez\""
    echo "   user = \"greetdeez\""
    echo "========================================="
    echo ""
else
    echo "greetd config already configured for greetdeez, skipping."
fi
