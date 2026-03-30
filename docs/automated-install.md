# Automated Tool

The `pimaxportal` TUI tool automates both **rooting** and **GPU driver upgrades** for the Pimax Portal Retro.

![pimaxportal TUI demo](img/portal.gif)

## Prerequisites

- [ADB and 7z installed](dependencies.md)
- `fastboot` in PATH (ships with android-platform-tools, required for rooting)
- USB debugging enabled on the device
- Pimax Portal connected via USB

## Usage

Download the latest release for your platform from [GitHub Releases](https://github.com/UltimG/PimaxPortal/releases), then run:

```bash
./pimaxportal
```

The tool opens with a sidebar menu. Use arrow keys or mouse to select a screen, Tab to switch between sidebar and content.

---

## Rooting

Select **Rooting** from the sidebar and press Enter (or click the button).

The tool will:

1. **Download** the latest Magisk APK from GitHub
2. **Install** Magisk on the device via ADB
3. **Download** the stock boot.img and push it to the device (internal storage + SD card)
4. **Show instructions** to patch boot.img in the Magisk app
5. **Auto-detect** the patched image when Magisk finishes
6. **Reboot** to FastbootD and flash the patched boot image
7. **Reboot** to Android with root active

The only manual step is patching boot.img inside the Magisk app (steps shown on screen). Everything else is automated.

**Important:** The Pimax Portal uses **FastbootD** (userspace fastboot) for flashing, not the UEFI bootloader. The tool handles this automatically — `fastboot flash` through the regular bootloader does not work on this device.

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

### Root (Magisk)

Open Magisk app on the device, tap **Uninstall** > **Complete Uninstall**. Or flash the stock boot.img via FastbootD:

```bash
adb reboot fastboot
fastboot flash boot boot.img
fastboot reboot
```

### GPU Drivers

Open Magisk app, go to **Modules**, tap the trash icon next to "Pimax Portal GPU Driver Upgrade". Reboot.

Or via ADB:

```bash
adb shell su -c 'rm -rf /data/adb/modules/pimax-portal-gpu-drivers'
adb reboot
```
