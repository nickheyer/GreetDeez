# GreetDeez

Hackable display manager greeter for [greetd](https://git.sr.ht/~kennylevinsen/greetd), powered by Go + webkit2gtk.

[![CI](https://github.com/nickheyer/greetdeez/actions/workflows/ci.yml/badge.svg)](https://github.com/nickheyer/greetdeez/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![AUR](https://img.shields.io/aur/version/greetdeez-git)](https://aur.archlinux.org/packages/greetdeez-git)


## Features

- **Native webview** via webkit2gtk-4.1 — no Electron, no browser
- **Svelte 5** frontend with reactive state and Tailwind CSS
- **greetd IPC** — speaks the greetd protocol directly over Unix sockets
- **Multi-session** — discovers Wayland and X11 sessions from `.desktop` files
- **Hackable** — swap the entire UI by replacing the embedded Svelte app

## Installation

### Arch Linux (AUR)

```sh
yay -S greetdeez-git
```

### Debian / Ubuntu

Download the `.deb` from [Releases](https://github.com/nickheyer/greetdeez/releases):

```sh
sudo dpkg -i greetdeez_*.deb
```

### Fedora

Download the `.rpm` from [Releases](https://github.com/nickheyer/greetdeez/releases):

```sh
sudo rpm -i greetdeez-*.rpm
```

### Alpine

Download the `.apk` from [Releases](https://github.com/nickheyer/greetdeez/releases):

```sh
sudo apk add --allow-untrusted greetdeez-*.apk
```

### NixOS

Add the flake to your inputs and enable the module:

```nix
{
  inputs.greetdeez.url = "github:nickheyer/greetdeez";

  # In your NixOS configuration:
  programs.greetdeez.enable = true;
}
```

Or build directly:

```sh
nix build github:nickheyer/greetdeez
```

### From source

```sh
make build
sudo make install
```

Requires: Go 1.25+, Node.js 20+, `libwebkit2gtk-4.1-dev`, `pkg-config`

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
# Commands are auto-detected: loginctl > systemctl > raw POSIX.
# poweroff_cmd = ["loginctl", "poweroff"]
# reboot_cmd   = ["loginctl", "reboot"]
# suspend_cmd  = ["loginctl", "suspend"]

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
