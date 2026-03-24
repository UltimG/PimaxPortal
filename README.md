# Pimax Portal Retro GPU Driver Fix

A TUI tool that fixes broken GPU drivers on the **Pimax Portal Retro** handheld.

![Demo](assets/portal.gif)

## Why

The Pimax Portal Retro ships with outdated Adreno 650 drivers (V@0655.0, June 2022) that cause Vulkan rendering crashes, graphical glitches, and poor performance in many games.

## The Fix

This tool replaces the stock drivers with newer Adreno 650 drivers (V@0764.0, January 2024) extracted from the **Retroid Pocket Mini** firmware. Both devices share the same Snapdragon 865 SoC and run Android 10, making the drivers fully compatible.

## Quick Start

1. Download the latest binary for your platform from [Releases](https://github.com/UltimG/PimaxPortal/releases)
2. Install ADB and 7z on your computer ([setup guide](assets/DEPENDENCIES.md))
3. Connect your Pimax Portal Retro via USB
4. Run the tool from the directory where you downloaded it and press **Enter**

```bash
# macOS/Linux — make it executable first
chmod +x pimaxportal
./pimaxportal

# Or install to PATH so you can run it from anywhere
sudo mv pimaxportal /usr/local/bin/
pimaxportal
```

```powershell
# Windows — run from the download folder
.\pimaxportal.exe
```

The tool will walk you through the rest.

## What the Tool Does

When you press Enter, the tool automatically:

1. Downloads the Retroid Pocket Mini firmware
2. Extracts the GPU driver files from the firmware image
3. Packages them into a Magisk module
4. Pushes and installs the module on your device via ADB

All steps run in sequence with progress displayed in the terminal. If no device is connected, the tool builds the module and saves it locally.

## Known Quirks

- The stock launcher may restart once after the module is installed. This is expected and handled automatically by the module.

## Install via Homebrew

```
brew tap UltimG/pimaxportal && brew install pimaxportal
```

## Building from Source

```
go build -o pimaxportal ./cmd/pimaxportal/
```

Requires Go 1.26 or later.

## Driver Provenance

The GPU drivers are extracted from publicly available Retroid Pocket Mini firmware. The Retroid Pocket Mini uses the same Qualcomm Snapdragon 865 SoC and Android 10 OS as the Pimax Portal Retro, so the Adreno 650 drivers are binary-compatible.

## Requirements

- **Pimax Portal Retro** with Magisk root
- **ADB** and **7z** installed on your computer ([setup guide](assets/DEPENDENCIES.md))
- USB connection to the device

## License

[GPL-3.0](LICENSE)
