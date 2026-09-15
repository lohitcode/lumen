# Study Light

[![Go Reference](https://pkg.go.dev/badge/github.com/lohitcode/study-light.svg)](https://pkg.go.dev/github.com/lohitcode/study-light)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A polished terminal UI for controlling [WiZ](https://www.wizconnected.com/) lights on your local network — direct UDP, no cloud round-trips, no account, no daemon. Just one static binary.

```
  STUDY LIGHT  WiZ · 192.168.1.42

  ●  ON     70%  •  5000 K

  ╭─ CONTROLS ─────────────────────────────────────────────╮
  │                                                        │
  │  › Brightness      ━━━━━━━━━━━━━━━━━━━━━          70%  │
  │                                                    │
  │    Temperature     ━━━━━━━━━━━━━━━━━━            5000 K │
  │                                                        │
  ╰────────────────────────────────────────────────────────╯

  Live status · refreshed just now

  ↑↓ select  ←→ adjust  o toggle power  q quit
```

## Features

- **Automatic discovery** — finds WiZ lights on your network via UDP broadcast; no configuration needed
- **Live state** — power, brightness, and white temperature refresh every two seconds
- **Keyboard-driven** — vim-style keys, instant feedback, clamped ranges
- **Fully local** — speaks WiZ's LAN protocol (UDP port 38899) directly; works even when your internet is down
- **Single binary** — pure Go, no runtime dependencies

## Requirements

- [Go](https://go.dev/dl/) 1.26+ (only to install/build)
- A WiZ light connected to the same network as your computer
- Local communication enabled in the WiZ mobile app: **Settings → Security settings → Allow local communication** (on newer firmware, set local control to **"All controls"**)

## Install

```sh
go install github.com/lohitcode/study-light@latest
```

Make sure `$(go env GOPATH)/bin` is in your `PATH`.

Or build from source:

```sh
git clone https://github.com/lohitcode/study-light
cd study-light
go install .
```

## Usage

```sh
study-light                     # discover lights; pick one if several are found
study-light --host 192.168.1.42 # skip discovery and target a specific light
```

### Keys

| Key | Action |
| --- | --- |
| `o` / `space` | Toggle power |
| `↑` `↓` / `k` `j` | Select brightness or temperature |
| `←` `→` / `h` `l` | Adjust the selected value (±5% brightness, ±100 K) |
| `q` / `esc` / `ctrl+c` | Quit |

## How it works

WiZ lights listen on UDP port 38899 for small JSON commands. Study Light:

1. broadcasts `getPilot` to every network interface's broadcast address to discover lights,
2. reads state with `getPilot`,
3. writes changes with `setPilot` (falling back to the legacy `setState` on older firmware), performing the `registration` handshake first.

Everything stays on your LAN — the WiZ cloud is never contacted.

## Project layout

```
main.go            entry point: flags, discovery/picker, launches the TUI
wiz/client.go      UDP JSON protocol: get/set with setPilot→setState fallback
wiz/discovery.go   UDP broadcast discovery
wiz/registration.go  registration handshake required before writes
ui/tui.go          Bubble Tea model: key handling and refresh loop
ui/view.go         Lipgloss rendering: dashboard, meters, colors
```

## Troubleshooting

- **No lights found** — confirm your computer and the light are on the same network and that local communication is enabled in the WiZ app. Some routers isolate wireless clients from each other (AP isolation); disable it.
- **Commands are rejected** — newer WiZ firmware set to "Only verified controls" blocks unsigned local commands; switch local control to "All controls" in the WiZ app.
- **Discovery is slow or flaky** — check that your firewall allows outbound UDP to port 38899 on your local network.

## Contributing

Issues and pull requests are welcome. Keep it small, local, and dependency-light — that's the spirit of the project.

## License

[MIT](LICENSE)
