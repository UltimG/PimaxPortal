# Automated Tool

The `pimaxportal` TUI tool automates **GPU driver upgrades** and **GPU overclocking** for the Pimax Portal Retro. The device must already be rooted — see the [Manual Rooting Guide](rooting-guide.md).

![pimaxportal TUI demo](img/portal.gif)

## Prerequisites

- [ADB and 7z installed](dependencies.md)
- Pimax Portal Retro **already rooted with Magisk** (see [Rooting Guide](rooting-guide.md))
- USB debugging enabled on the device
- Pimax Portal connected via USB

## Usage

Download the latest release for your platform from [GitHub Releases](https://github.com/UltimG/PimaxPortal/releases), then run:

```bash
./pimaxportal
```

The tool opens with a sidebar menu. Use arrow keys or mouse to select a screen, Tab to switch between sidebar and content.

---

## GPU Driver Fix

Select **GPU Drivers** from the sidebar and press Enter.

The tool will:

1. **Download** the Retroid Pocket Mini firmware (~13 GB, split 7z archive)
2. **Extract** the super partition from the firmware
3. **Extract** the vendor partition from the super image
4. **Extract** 34 GPU driver files from the vendor image
5. **Package** everything into a Magisk module zip
6. **Install** the module via ADB + Magisk

The tool caches intermediate results — if interrupted, it resumes from where it left off.

### What Gets Installed

The Magisk module replaces GPU drivers on the `/vendor` partition using Magisk's systemless overlay — no actual system partition modification occurs.

| Component | Stock | Upgraded |
|-----------|-------|----------|
| Driver version | V@0655.0 (E031.40.09.00) | V@0764.0 (E031.45.03.01) |
| Driver date | June 2022 | January 2024 |
| Source device | Pimax Portal | Retroid Pocket Mini |

Both devices share the same Snapdragon 865 SoC and Android 10, making the drivers fully compatible.

---

## Build from Source

```bash
# Clone
git clone https://github.com/UltimG/PimaxPortal.git
cd PimaxPortal

# Build
go build -o build/pimaxportal ./cmd/pimaxportal/

# Run
./build/pimaxportal
```

## Uninstalling

### GPU Drivers

Open Magisk app, go to **Modules**, tap the trash icon next to "Pimax Portal GPU Driver Upgrade". Reboot.

Or via ADB:

```bash
adb shell su -c 'rm -rf /data/adb/modules/pimax-portal-gpu-drivers'
adb reboot
```
