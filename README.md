# GreetDeez

Hackable display manager for [greetd](https://git.sr.ht/~kennylevinsen/greetd), powered by Go + webkit2gtk.

[![Release](https://github.com/nickheyer/GreetDeez/actions/workflows/release.yml/badge.svg)](https://github.com/nickheyer/GreetDeez/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![AUR](https://img.shields.io/aur/version/greetdeez-bin)](https://aur.archlinux.org/packages/greetdeez-bin)

![minimal](minimal.webp)

> **This is the default theme.** It serves as an example and template for creating a custom front end for your login UI.

## Themes

GreetDeez ships with three built-in themes: `minimal` (default), `cyber`, and `doom`. Set the theme in `/etc/greetd/greetdeez.conf`:

```toml
[ui]
theme = "cyber"   # "minimal" (default), "cyber", or "doom"
```

To use your own custom front end, point `path` to a directory containing an `index.html` (and any JS/CSS it references). When `path` is set, it takes priority over the built-in theme. See [Building a Custom Front End](#building-a-custom-front-end) for details.

```toml
[ui]
path = "/usr/share/greetdeez-themes/my-theme/dist"
```

The `GREETDEEZ_UI_THEME` environment variable can also be used to override the theme.

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

### NixOS (Flake)

Add the flake input and enable the module:

```nix
# flake.nix
inputs.greetdeez.url = "github:nickheyer/GreetDeez";

# configuration.nix
{ inputs, ... }:
{
  imports = [ inputs.greetdeez.nixosModules.default ];

  services.greetdeez.enable = true;
}
```

Optional settings can be passed through as toml:

```nix
services.greetdeez.settings = {
  ui.theme = "cyber";
  power.enabled = true;
};
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
debug = false

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
# x11_wrapper = ["startx", "/usr/bin/env"]

[ui]
theme = "minimal"
# path = "/path/to/your/custom/ui"
```

Environment variables (`GREETDEEZ_*`) override file values. For example: `GREETDEEZ_UI_THEME`, `GREETDEEZ_UI_PATH`, `GREETDEEZ_WINDOW_TITLE`, `GREETDEEZ_AUTH_TIMEOUT_SECONDS`, `GREETDEEZ_POWER_ENABLED`, `GREETDEEZ_DEBUG`.

## Building a Custom Front End

GreetDeez loads your UI in a webview and injects a global RPC function (`window.__greetdeez_rpc__`) that your code uses to talk to the backend. The [`@nickheyer/greetdeez`](https://www.npmjs.com/package/@nickheyer/greetdeez) npm package wraps this into a typed client so you don't need to deal with the transport layer yourself.

### Install

```sh
npm install @nickheyer/greetdeez
```

### Create the client

```ts
import { createGreeterServiceClient } from "@nickheyer/greetdeez";

const client = createGreeterServiceClient();
```

When running inside GreetDeez, the client uses the injected RPC bridge automatically. During local development (outside the webview), it falls back to no-op defaults — or you can pass mock implementations:

```ts
const client = createGreeterServiceClient({
  dev: {
    listSessions: async () => ({
      sessions: [{ name: "sway", cmd: ["sway"], type: 1, desktop: "" }],
    }),
    getSystemInfo: async () => ({ info: { hostname: "dev" } }),
  },
});
```

### Available methods

| Method | Description |
|---|---|
| `listSessions()` | Get available desktop sessions (Wayland/X11) |
| `getSystemInfo()` | Get hostname |
| `getPowerCapabilities()` | Check which power actions are available |
| `getState()` | Load persisted state (last user, last session) |
| `authenticate({ username, password })` | Validate credentials without starting a session |
| `startSession({ session })` | Start the selected desktop session |
| `login({ username, password, session })` | Authenticate + start session in one call |
| `executePowerAction({ action })` | Poweroff, reboot, or suspend |
| `saveState({ state })` | Persist last user / last session |

### Minimal example

```ts
import {
  createGreeterServiceClient,
  PowerAction,
  type Session,
} from "@nickheyer/greetdeez";

const client = createGreeterServiceClient();

// Load initial data
const { sessions } = await client.listSessions();
const { info } = await client.getSystemInfo();
const { capabilities } = await client.getPowerCapabilities();
const { state } = await client.getState();

// Log in
const session: Session = sessions[0];
const auth = await client.authenticate({ username: "nick", password: "..." });
if (auth.success) {
  await client.startSession({ session });
  await client.saveState({ state: { lastUser: "nick", lastSession: session.name } });
}

// Power actions
await client.executePowerAction({ action: PowerAction.REBOOT });
```

Build your project so it outputs an `index.html`, then point GreetDeez at it:

```toml
[ui]
path = "/path/to/your/build/output"
```

Any framework (or none) works — the only requirement* is that your output is a static site that imports `@nickheyer/greetdeez`. This is of course just a client wrapper around the greetdeez protocol, which you could implement yourself, see: `proto/**/*.proto`


## License

[MIT](LICENSE)
