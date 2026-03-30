# Pimax Portal Modding

Tools and guides for the **Pimax Portal Retro** (SD865, Android 10).

## What's This?

The Pimax Portal Retro ships with outdated Adreno 650 GPU drivers (V@0655.0, June 2022) that cause Vulkan rendering crashes and graphical glitches. This project provides:

- **An automated TUI tool** that downloads, extracts, and installs updated GPU drivers from the Retroid Pocket Mini firmware as a Magisk module
- **Manual guides** for users who prefer to do it themselves
- **EDL recovery instructions** for unbricking your device

The replacement drivers (V@0764.0, January 2024) are a drop-in upgrade — both devices run Snapdragon 865 + Android 10.

## Guides

| Guide | Description |
|-------|-------------|
| [Automated Install](automated-install.md) | Use the TUI tool to build and install the GPU driver module |
| [Manual Module Guide](manual-module-guide.md) | Build the Magisk module by hand, step by step |
| [EDL Recovery](edl-recovery-guide.md) | Recover a bricked device using open-source `qdl` |
| [Dependencies](dependencies.md) | Install ADB, 7z, and set up USB debugging |

## Quick Start

```bash
# macOS
brew install android-platform-tools p7zip

# Download the latest release
# https://github.com/UltimG/PimaxPortal/releases

# Run it
./pimaxportal
```

Connect your Pimax Portal via USB with USB debugging enabled. The tool handles everything else.

## Requirements

- **Pimax Portal Retro** with Magisk installed
- **ADB** and **7z** on your computer (see [Dependencies](dependencies.md))
- USB debugging enabled on the device
- ~25 GB free disk space (for firmware download and extraction)

## License

GPL v3
