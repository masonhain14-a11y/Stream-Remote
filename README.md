# Stream Remote

Stream Remote is a portable Windows app that turns a phone browser into a
streaming launcher, trackpad, keyboard, and media remote. The Windows app hosts
the phone interface on your local network. See `Start-here.txt` for pairing and
usage instructions.

## Windows compatibility

| System type in Settings → System → About | Build |
| --- | --- |
| Windows 10 or 11, x64-based processor (Intel or AMD) | x64 |
| Windows 10 or 11, ARM-based processor | ARM64 |

The release targets 64-bit Windows 10 and Windows 11. Verification covered an
x64 runtime smoke test in Wine, Android and iPhone browser profiles, and
compilation for both architectures.

## Build on Windows

Install [Go](https://go.dev/dl/) 1.25 or later for your PC's architecture.
Go 1.25.10 was used to verify this archive. Extract the entire ZIP and open
PowerShell in the folder containing `go.mod`. Run this command to build
for the architecture of your Go installation.

```powershell
$env:CGO_ENABLED = '0'
go build -mod=vendor -trimpath '-ldflags=-s -w -H=windowsgui' -o 'Stream Remote.exe' .
```

Open the generated `Stream Remote.exe`. The phone interface, icons, and font
are embedded in the executable. The release build uses the bundled dependencies
in `vendor/` and the Windows resources already present in the folder.

To cross-compile for a different Windows architecture, set
`$env:GOOS = 'windows'` and either
`$env:GOARCH = 'amd64'` for x64 or `$env:GOARCH = 'arm64'` for ARM64 before
running the same build command.

## Source guide

- `main_windows.go` creates the Windows window and displays the pairing code
  and QR link.
- `server.go` serves the phone interface and handles pairing and commands.
- `input_windows.go` sends input to Windows and launches streaming services.
- `network_windows.go` provides the Windows IPv4 adapter fallback.
- `web/` contains the phone interface, streaming catalog, icons, and font.
- `winres.json` and `app.ico` define the Windows manifest, version metadata,
  and app icon. The included `rsrc_windows_*.syso` files link those resources
  into the x64 and ARM64 builds.
- `tools/` contains the optional icon generation scripts.
- `Third-party-notices.txt` and `vendor/` contain dependency license notices.

## Optional asset regeneration

Run these commands from the folder containing `go.mod`. The browser icon
script requires Python and CairoSVG. The Windows icon script requires Python
and Pillow. The resource-generation command fetches its pinned Go tool on the
first run.

```powershell
python .\tools\generate_web_icons.py
python .\tools\generate_windows_icon.py
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres.json --arch amd64,arm64
```
