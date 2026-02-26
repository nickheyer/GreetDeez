#!/bin/bash

# Create greetdeez system user if it doesn't exist
if ! id -u greetdeez >/dev/null 2>&1; then
    useradd -r -s /usr/bin/nologin -d /var/lib/greetdeez -m greetdeez 2>/dev/null || true
    echo "Created system user: greetdeez"
fi

# Install greetd config if it hasn't been customized
CONFIG="/etc/greetd/config.toml"
if [ -f "$CONFIG" ]; then
    if grep -q "greetdeez" "$CONFIG"; then
        echo "greetd config already configured for greetdeez, skipping."
    else
        cp "$CONFIG" "$CONFIG.bak"
        cp /etc/greetd/greetdeez.toml "$CONFIG"
        echo "Installed greetdeez config to $CONFIG (backup: $CONFIG.bak)"
    fi
else
    cp /etc/greetd/greetdeez.toml "$CONFIG"
    echo "Installed greetdeez config to $CONFIG"
fi
