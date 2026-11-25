# Stream Remote

Stream Remote lets you control a Windows PC from your phone’s browser. Run the portable app on your PC, scan the pairing QR code, and use your phone to open streaming services, move the pointer, type, or control playback.

## Features

| Tab | What you can do |
| --- | --- |
| Streaming | Open services such as Netflix, YouTube, and Prime Video in your PC’s default browser. |
| Mouse | Move the pointer, tap to click, scroll with two fingers, and drag. |
| Keyboard | Send text to the active window, type live, or use common keys and shortcuts. |
| Remote | Control playback and volume, navigate with arrow keys, toggle browser fullscreen, or show the desktop. |

Playback keys depend on the active player’s support for Windows media keys.

## Quick start

1. Extract `Stream Remote for Windows.zip` and run the executable for your PC. Find your system type in **Settings → System → About**.
2. Connect your phone and PC to the same home network. If Windows Firewall prompts you, allow Stream Remote on your private network.
3. Scan the QR code shown in Stream Remote with your phone’s camera. You can also open the displayed `http://` address in Safari or Chrome and enter the six-digit pairing code.
4. Keep Stream Remote open and your PC awake and unlocked while using the remote.

| Windows version | System type | Executable |
| --- | --- | --- |
| 64-bit Windows 10 or 11 | Intel or AMD x64 processor | `Stream Remote (x64).exe` |
| 64-bit Windows 10 or 11 | ARM64 processor | `Stream Remote (ARM64).exe` |

The executables are unsigned, so Windows may show a publisher prompt. See `Start-here.txt` for more setup details.

## Build from source

Install Go 1.25 or newer. Open PowerShell in the folder containing `go.mod`. [Get Go](https://go.dev/dl/)

```powershell
$env:CGO_ENABLED = '0'
go build -mod=vendor -trimpath '-ldflags=-s -w -H=windowsgui' -o 'Stream Remote.exe' .
```

This builds for the architecture of your Go installation on Windows. The phone interface, icons, and font are embedded in the executable. The source includes the Go dependencies and Windows resources for an offline build once Go is installed.

To cross-compile, set `$env:GOOS = 'windows'` and set `$env:GOARCH` to `'amd64'` for x64 or `'arm64'` for ARM64 before running the build command.

## Troubleshooting

- **The phone cannot connect.** Check that both devices are on the same network. Try another address from the app’s **Network** menu and enter the full `http://` address if your browser switches to HTTPS. Guest networks may block connections between devices.
- **Typing goes to the wrong window.** Focus the text field or app you want to control on your PC, then type from your phone. Elevated apps and Windows security prompts can block remote input.
- **You want to disconnect paired phones.** Choose **Reset pairing** in the Windows app to revoke existing sessions and generate a new code. Closing the app stops its local server.

## Project layout

| Path | Purpose |
| --- | --- |
| `main_windows.go` | Windows launcher, network selector, and QR display |
| `server.go` | Local HTTP server, pairing, and WebSocket controls |
| `input_windows.go` | Windows input events and streaming service launch |
| `network_windows.go` | Windows IPv4 adapter fallback |
| `web/` | Phone interface, streaming catalog, font, and icons |
| `winres.json`, `app.ico`, and `rsrc_windows_*.syso` | Windows manifest, icon, and executable metadata |
| `vendor/` | Bundled Go dependencies |
| `tools/` | Optional icon generators |

## Optional asset regeneration

Run these commands from the folder containing `go.mod`. The browser icon script requires Python and CairoSVG. The Windows icon script requires Python and Pillow. Go downloads the pinned Windows resource tool on its first run.

```powershell
python .\tools\generate_web_icons.py
python .\tools\generate_windows_icon.py
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres.json --arch amd64,arm64
```

## Verification and notices

Both Windows builds were compiled from the source ZIP with Go 1.25.10. The x64 build was run in Wine, and pairing and phone controls were tested with Android and iPhone browser profiles. Third-party license notices are in `Third-party-notices.txt` and `vendor/`.
