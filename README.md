# GreetDeez

Hackable display manager greeter for [greetd](https://git.sr.ht/~kennylevinsen/greetd), powered by Go + webkit2gtk.

[![Release](https://github.com/nickheyer/GreetDeez/actions/workflows/release.yml/badge.svg)](https://github.com/nickheyer/GreetDeez/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![AUR](https://img.shields.io/aur/version/greetdeez-bin)](https://aur.archlinux.org/packages/greetdeez-bin)



## Installation

### Arch Linux (AUR)

```sh
yay -S greetdeez-bin
```

### Debian / Ubuntu

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/nickheyer/greetdeez/setup.deb.sh' | sudo bash
sudo apt install greetdeez
```

### Fedora / RHEL

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/nickheyer/greetdeez/setup.rpm.sh' | sudo bash
sudo dnf install greetdeez
```

### Alpine

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/nickheyer/greetdeez/setup.alpine.sh' | sudo bash
sudo apk add greetdeez
```

### From source

Requires: Go 1.26+, Node.js 20+, `libwebkit2gtk-4.1-dev`, `pkg-config`

Runtime dependencies: `greetd`, `cage`, `webkit2gtk-4.1`

```sh
make build
sudo make install
```

After installing from source, create the system user and configure greetd manually:

```sh
sudo useradd -r -s /usr/bin/nologin -d /var/lib/greetdeez -m greetdeez
sudo cp /etc/greetd/greetd.toml /etc/greetd/config.toml
```

Package installs handle both of these automatically.

## Configuration

### greetd

GreetDeez runs inside [cage](https://github.com/cage-compositor/cage) (a single-window Wayland compositor). The greetd config at `/etc/greetd/config.toml` should look like:

```toml
[terminal]
vt = 7

[default_session]
command = "cage -s -- /usr/bin/greetdeez"
user = "greetdeez"
```

This is installed automatically by the packages.

### greetdeez.conf

All options with their defaults (`/etc/greetd/greetdeez.conf`):

```toml
[window]
title = "GreetDeez"
# width and height are auto-detected from DRM. Under cage these are irrelevant.
# width  = 1920
# height = 1080

[auth]
timeout_seconds = 30

[power]
enabled = true
# poweroff and reboot use POSIX shutdown(8). Only suspend is auto-detected.
# poweroff_cmd = ["shutdown", "-h", "now"]
# reboot_cmd   = ["shutdown", "-r", "now"]
# suspend_cmd  = ["systemctl", "suspend"]

[sessions]
# dirs = [
#     { path = "/usr/share/wayland-sessions", type = "wayland" },
#     { path = "/usr/share/xsessions", type = "x11" },
# ]

[theme]
accent_color = ""
aurora_speed = 1.0
```

Environment variables (`GREETDEEZ_WINDOW_TITLE`, `GREETDEEZ_AUTH_TIMEOUT_SECONDS`, etc.) override file values.

## License

[MIT](LICENSE)
