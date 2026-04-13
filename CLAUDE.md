# Project

Go/Bubbletea TUI tool that builds and installs a Magisk GPU driver module for Pimax Portal Retro (SD865, Android 10). Drivers sourced from Retroid Pocket Mini firmware.

## Commands

- `go build -o build/pimaxportal ./cmd/pimaxportal/` — build
- `go test ./...` — run tests
- `GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/pimaxportal.exe ./cmd/pimaxportal/` — cross-compile for Windows

## Structure

- `cmd/pimaxportal/` — TUI app (main, app model)
- `cmd/pimaxportal/commands/` — pipeline: download, 7z extract, lpunpack, ext4 extract, package, install
- `cmd/pimaxportal/commands/adb/` — ADB wrapper
- `cmd/pimaxportal/ui/` — Bubbletea components (styles, logo, device info, progress)
- `modules/gpu-drivers/` — Magisk module template (metadata only, no binaries)
- `embed.go` — embeds module template via `//go:embed all:modules/gpu-drivers`
- `assets/` — DEPENDENCIES.md, demo GIF

## External Dependencies

- `adb` — required at runtime
- `7z` — required at runtime for ext4 extraction
- `bodgit/sevenzip` — Go library for .7z archive extraction

## Conventions

- No Claude co-author or mention in commits
- `docs/` is untracked and not in .gitignore — never stage it
- `.gitignore` covers *.so, *.img, *.zip, *.7z, *.apk, *.jar, build/, originals/, .DS_Store
- License: GPL v3

## Gotchas

- `diskFree()` is platform-split: `diskfree_unix.go` / `diskfree_windows.go`
- LP metadata offset is 12288 (geometry at 4096 + two 4KB geometry blocks), not 8192
- `go-diskfs` ext4 reader fails on Android vendor images (checksum type 0) — we shell out to `7z` instead
- `//go:embed all:modules/gpu-drivers` needs the `all:` prefix for nested META-INF dirs
- Pimax stock launcher has a boot race — `service.sh` auto-restarts it 15s after boot
