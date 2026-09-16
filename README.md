# lumen

[![Go Reference](https://pkg.go.dev/badge/github.com/lohitcode/lumen.svg)](https://pkg.go.dev/github.com/lohitcode/lumen)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A keyboard-driven terminal UI for your smart lights — fully local, no cloud, no account, one static binary.

Ships with a **WiZ driver** out of the box, but the light interface is small and generic: if your lamp speaks some other protocol (Hue, TP-Link Kasa, ESPHome, MQTT, …), you can add it with one short driver and the whole UI — discovery, switching, live state, persistence — just works.

```
  LUMEN  Study Lamp

  ● ON  45% • 4900 K

  ╭──────────────────────────────────────────────────────────╮
  │                                                          │
  │  CONTROLS                                                │
  │                                                          │
  │  ›  Brightness      ████████░░░░░░░░░░░░░░░░░       45%  │
  │                                                          │
  │     Temperature     ██████████████░░░░░░░░░░░     4900 K │
  │                                                          │
  ╰──────────────────────────────────────────────────────────╯

  ● live

  ↑↓ select  ←→ adjust  o power  s lights  q quit
```

## Features

- **Automatic discovery** — finds lights on your network via broadcast; no configuration needed
- **Real names** — shows each light's friendly device name, not its IP
- **Live state** — power, brightness, and white temperature refresh every two seconds
- **Silky sliders** — native [bubbles](https://github.com/charmbracelet/bubbles) progress bars animated on springs at 60 fps
- **Multi-light switching** — press `s` to re-scan and jump between lights instantly
- **Inline aliases** — rename any light right in the switcher; names persist per light
- **Remembers you** — reopens the light you used last time automatically
- **Driver architecture** — a small Go interface per protocol; the UI never talks to hardware directly
- **Fully local** — control stays on your LAN; the cloud is never contacted
- **Single binary** — pure Go, no runtime dependencies

## Requirements

- [Go](https://go.dev/dl/) 1.26+ (only to install/build)
- At least one supported light on the same network as your computer
- For the built-in WiZ driver: local communication enabled in the WiZ mobile app — **Settings → Security settings → Allow local communication** (on newer firmware, set local control to **"All controls"**)

## Install

```sh
go install github.com/lohitcode/lumen@latest
```

Make sure `$(go env GOPATH)/bin` is in your `PATH`.

Or build from source:

```sh
git clone https://github.com/lohitcode/lumen
cd lumen
go install .
```

## Usage

```sh
lumen                              # reopen your last light, or discover
lumen --host 192.168.1.42          # target a specific light
lumen --host hue.local --driver hue  # target a light through a specific driver
lumen --name "Reading Lamp"        # name a light (shown instead of its address)
```

Lights show their friendly device name when the driver can read one. WiZ firmware that keeps names cloud-side can be named locally instead — the `--name` label is saved per light and shown from then on.

| Key | Action |
| --- | --- |
| `o` / `space` | Toggle power |
| `↑` `↓` / `k` `j` | Select brightness or temperature |
| `←` `→` / `h` `l` | Adjust the selected value (±2% brightness, ±100 K) |
| `s` | Scan the network and switch to another light |
| `n` (in switcher) | Rename the highlighted light inline |
| `q` / `esc` / `ctrl+c` | Quit |

### Session persistence

The light you open is saved to your user config directory (`~/Library/Application Support/lumen/config.json` on macOS, `~/.config/lumen/config.json` on Linux) and reopened automatically next launch. If that light is unreachable, lumen falls back to discovery. Switching lights with `s` updates the saved choice.

## Supporting your own lights

Everything above is hardware-agnostic. A driver is two small types: a `Light` (one lamp) and a `Driver` (how to find and reconnect to lamps). See `light/light.go` for the full definitions and `wiz/` for a complete, compact implementation.

```go
type Light interface {
    Driver() string              // protocol name, e.g. "wiz"
    Address() string             // reachable address (IP, host, …)
    Label() string               // shown in pickers and the status bar
    Ranges() Ranges              // hardware limits for brightness/temperature
    State() (State, error)       // current power, brightness, temperature
    SetPower(on bool) error
    SetBrightness(percent int) error
    SetTemp(kelvin int) error
}

type Driver interface {
    Name() string
    Discover(timeout time.Duration) ([]Light, error)
    Connect(address string) (Light, error)
}
```

To add a driver:

1. Create a package (e.g. `hue/`) that implements both interfaces.
2. Register it: `func init() { light.Register(Driver{}) }`.
3. Blank-import it in `main.go` next to the WiZ driver.

That's it — discovery, the `s` switcher, last-light persistence, and the dashboard all pick it up automatically. Drivers clamp values to their own hardware ranges, so the UI needs no per-device knowledge.

## How the WiZ driver works

WiZ lights listen on UDP port 38899 for small JSON commands. The driver broadcasts `getPilot` to discover bulbs, reads state with `getPilot`, reads each bulb's friendly name with `getSystemConfig`, writes changes with `setPilot` (falling back to the legacy `setState` on older firmware), and performs the `registration` handshake before writes. Everything stays on your LAN.

## Project layout

```
main.go          entry point: flags, last-light reconnect, discovery picker
light/light.go   the generic Light and Driver interfaces + driver registry
config/config.go session persistence (last light) in your user config dir
ui/tui.go        Bubble Tea model: keys, refresh loop, light switcher
ui/view.go       Lipgloss rendering: dashboard, picker, sliders, colors
wiz/             built-in WiZ driver (client, discovery, registration, light)
```

## Troubleshooting

- **No lights found** — confirm your computer and the light are on the same network and that local control is enabled in the vendor app. Some routers isolate wireless clients from each other (AP isolation); disable it.
- **WiZ commands are rejected** — newer WiZ firmware set to "Only verified controls" blocks unsigned local commands; switch local control to "All controls" in the WiZ app.
- **Discovery is slow or flaky** — check that your firewall allows outbound UDP on your local network (the WiZ protocol uses port 38899).

## Contributing

Issues and pull requests are welcome — new drivers for other protocols are especially appreciated. Keep it small, local, and dependency-light; that's the spirit of the project.

## License

[MIT](LICENSE)
