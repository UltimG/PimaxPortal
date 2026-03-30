# EDL Recovery Guide: Pimax Portal

This guide covers how to recover a bricked Pimax Portal (Retro/Lite) by flashing stock firmware through Qualcomm's **Emergency Download (EDL)** mode using the open-source **qdl** tool.

**When you need this:** Your device is stuck in a bootloop, won't reach Android, or is otherwise unresponsive. Common causes include bad overclock settings, broken Magisk modules, or corrupted system partitions.

---

## Overview

EDL (Emergency Download mode) is a low-level recovery mode built into all Qualcomm SoCs. It operates below the bootloader — even if your bootloader, recovery, and system partitions are completely destroyed, EDL can still reflash everything from scratch.

The process:
1. Enter EDL mode on the device
2. Use `qdl` on your computer to upload a programmer and flash firmware
3. Device boots to stock firmware

---

## Prerequisites

### Stock Firmware

Obtain the stock Pimax Portal firmware image. The file you need is:

```
Portal-Lite_kona-1.1.6-20230922-14-user
```

This is a flat build containing the `kona/` subdirectory with all partition images, rawprogram XMLs, and patch XMLs. We do not distribute this firmware — source it from Pimax support channels or community resources.

Once obtained, extract it. You should have a `kona/` directory containing files like:

```
kona/
├── prog_firehose_ddr.elf       (Firehose programmer — uploaded to device)
├── rawprogram_unsparse0.xml    (partition map — references split super images)
├── rawprogram[1-5].xml         (partition maps for other physical partitions)
├── patch[0-5].xml              (post-flash partition table patches)
├── super_[1-5].img             (system/vendor/product in sparse chunks)
├── boot.img, recovery.img      (kernel, recovery)
├── userdata_[1-14].img         (user data partition)
└── ... (abl.elf, tz.mbn, xbl.elf, modem, etc.)
```

### qdl (Qualcomm Download Tool)

`qdl` is an open-source tool that implements the Sahara and Firehose protocols for communicating with Qualcomm devices in EDL mode. It runs natively on macOS and Linux.

**Source:** https://github.com/linux-msm/qdl

---

## Building qdl from Source

### macOS

```bash
# Install dependencies
brew install libusb libxml2 pkg-config

# Clone and build
git clone https://github.com/linux-msm/qdl.git
cd qdl

# libxml2 is keg-only on macOS — pkg-config needs the path
export PKG_CONFIG_PATH="/opt/homebrew/opt/libxml2/lib/pkgconfig:$PKG_CONFIG_PATH"
make -j$(sysctl -n hw.ncpu)
```

> **Intel Mac note:** The Homebrew prefix is `/usr/local` instead of `/opt/homebrew`. Adjust the `PKG_CONFIG_PATH` accordingly:
> ```bash
> export PKG_CONFIG_PATH="/usr/local/opt/libxml2/lib/pkgconfig:$PKG_CONFIG_PATH"
> ```

### Linux (Debian/Ubuntu)

```bash
sudo apt install build-essential libxml2-dev libusb-1.0-0-dev pkg-config
git clone https://github.com/linux-msm/qdl.git
cd qdl && make -j$(nproc)
```

### Linux (Arch)

```bash
sudo pacman -S base-devel libxml2 libusb pkgconf
git clone https://github.com/linux-msm/qdl.git
cd qdl && make -j$(nproc)
```

### Verify the Build

```bash
./qdl --version
./qdl --help
```

You should see usage information with options like `--storage`, `--debug`, etc.

Optionally install system-wide:

```bash
sudo make install    # Installs to /usr/local/bin/
```

---

## Step 1: Enter EDL Mode

With the device powered off and unplugged:

1. **Hold the EDL button combination** for the Pimax Portal (consult your device documentation — typically involves holding specific volume key(s) while connecting USB)
2. **Connect the USB cable** to your computer while holding the buttons
3. **Verify the device is detected:**

**macOS/Linux:**
```bash
# Check for Qualcomm EDL device (VID:PID 05c6:9008)
# macOS
system_profiler SPUSBDataType 2>/dev/null | grep -A2 "Qualcomm"

# Linux
lsusb | grep 05c6:9008
```

**Windows (MSYS2):**
Check Device Manager for "Qualcomm HS-USB QDLoader 9008" under Ports (COM & LPT).

> **Tip:** Use a direct USB port on your computer, not a hub or dock. EDL's Sahara protocol is timing-sensitive and USB hubs can cause handshake failures.

---

## Step 2: Flash Stock Firmware

Navigate to the firmware directory and run `qdl`:

```bash
cd /path/to/Portal-Lite_kona-1.1.6-20230922-14-user/kona
```

```bash
sudo qdl --storage ufs \
  prog_firehose_ddr.elf \
  rawprogram_unsparse0.xml \
  rawprogram1.xml rawprogram2.xml rawprogram3.xml \
  rawprogram4.xml rawprogram5.xml \
  patch0.xml patch1.xml patch2.xml \
  patch3.xml patch4.xml patch5.xml
```

> **Note:** `sudo` is required on macOS/Linux for raw USB device access.

> **Important:** Use `rawprogram_unsparse0.xml` (not `rawprogram0.xml`). The stock firmware ships with the super partition split into sparse chunks (`super_1.img` through `super_5.img`). The `rawprogram_unsparse0.xml` references these split files correctly, while `rawprogram0.xml` expects a single `super.img` that doesn't exist in the distribution.

### What happens during flashing

1. **Sahara handshake** — `qdl` uploads `prog_firehose_ddr.elf` (the Firehose programmer) to the device's RAM
2. **Firehose protocol** — the programmer takes over and receives partition images from `qdl`, writing them directly to UFS storage
3. **Patching** — GPT partition tables are patched with correct LBA offsets

Successful output looks like:

```
waiting for programmer...
flashed "super" successfully
flashed "super" successfully at 41774kB/s
flashed "super" successfully at 43569kB/s
flashed "super" successfully at 41259kB/s
flashed "super" successfully
flashed "recovery_a" successfully at 51200kB/s
flashed "vbmeta_system_a" successfully
flashed "metadata" successfully
...
flashed "boot_a" successfully at 32768kB/s
...
flashed "spunvm" successfully at 348kB/s
flashed "logfs" successfully
flashed "PrimaryGPT" successfully
flashed "BackupGPT" successfully
52 patches applied
partition 1 is now bootable
```

The flash takes a few minutes depending on USB speed.

---

## Step 3: Boot the Device

After `qdl` completes:

1. Disconnect the USB cable
2. Power on the device normally
3. First boot after a full flash may take longer than usual (1-3 minutes)
4. The device should boot to stock Pimax launcher

---

## Troubleshooting

### "Waiting for EDL device" hangs

`qdl` doesn't see a QDLoader 9008 device.

- Verify the device is in EDL mode (screen should be completely black, not showing a bootloader or logo)
- Try a different USB cable or port — use USB 2.0 if possible
- On macOS, check that no other process has claimed the USB device
- On Linux, ensure you have permissions: `sudo` or add a udev rule:
  ```bash
  # /etc/udev/rules.d/99-qdl.rules
  SUBSYSTEM=="usb", ATTR{idVendor}=="05c6", ATTR{idProduct}=="9008", MODE="0666"
  ```
  Then: `sudo udevadm control --reload-rules`

### "Unable to read packet header. Only read 0 bytes" (Sahara failure)

The device is detected but the Sahara handshake fails.

- **Retry** — unplug the device, re-enter EDL mode, and try again. This often works on the second attempt as the USB device re-enumerates to a stable state.
- **USB hub/dock** — connect directly to the computer
- **VM passthrough** — if running in a VM (Parallels, VMware, VirtualBox), USB passthrough adds latency that can break the timing-sensitive Sahara protocol. Use `qdl` natively on your host OS instead.

### "unable to open super.img...failing"

You used `rawprogram0.xml` instead of `rawprogram_unsparse0.xml`. The firmware ships with split sparse images (`super_1.img` through `super_5.img`), and `rawprogram_unsparse0.xml` references these correctly.

### "The system cannot find the file specified" (QFIL)

If using QFIL instead of `qdl`:
- Copy firmware to a local path (e.g., `C:\firmware\kona\`). QFIL's FHLoader cannot reliably access files through network/UNC paths (`\\Mac\Home\...`, mapped drives).
- Ensure QFIL version is from 2020+ — older versions (pre-2020) don't support SM8250/Kona targets properly.

### Device boots to fastboot instead of EDL

Fastboot is not EDL. If you see a fastboot screen or `fastboot devices` detects the device, it means the bootloader is functional and you may not need EDL at all. Try:

```bash
# From fastboot, you can flash individual partitions:
fastboot flash boot boot.img
fastboot flash recovery recovery.img
fastboot reboot
```

If you specifically need EDL, power off and use the EDL button combination rather than fastboot's `oem edl` command.

### Flash completes but device still bootloops

- Ensure you flashed **all** rawprogram and patch XMLs, not just a subset
- Try adding `--finalize-provisioning` flag to configure UFS provisioning:
  ```bash
  sudo qdl --storage ufs --finalize-provisioning prog_firehose_ddr.elf ...
  ```
  (Only needed if UFS logical units were corrupted)

---

## Why qdl Instead of QFIL?

| | qdl | QFIL |
|---|---|---|
| **Platform** | macOS, Linux | Windows only |
| **License** | BSD 3-Clause (open source) | Proprietary (Qualcomm) |
| **VM required on Mac** | No — runs natively | Yes — needs Parallels/VMware |
| **USB reliability** | Direct access, no VM layer | VM USB passthrough can drop |
| **Version issues** | Single codebase, always current | Old versions lack SoC support |
| **Size** | ~700 KB binary | ~50 MB installer |

For the Pimax Portal specifically, `qdl` running natively on macOS or Linux avoids the most common flashing failure (Sahara timeouts through VM USB passthrough).

---

## Background: How EDL Flashing Works

For those interested in what's happening under the hood:

### The Two Protocols

**Sahara** is the initial protocol. When a Qualcomm SoC enters EDL mode, its Primary Boot Loader (PBL) — burned into ROM — starts a Sahara server over USB. It requests a programmer binary (in our case `prog_firehose_ddr.elf`) which gets loaded into the device's RAM and executed. Sahara is simple: the device asks for a file by ID, the host sends it, done.

**Firehose** is the flashing protocol. Once the programmer is running, it speaks Firehose — an XML-based protocol where the host sends commands ("program this image to this sector on this LUN") and the programmer writes data to the device's storage. The rawprogram XMLs define what goes where; the patch XMLs apply fixups to partition tables after flashing.

### The Firmware Layout

The Pimax Portal uses UFS storage with multiple physical partitions (LUNs). The rawprogram XMLs are split by physical partition number:

| XML | Physical Partition | Contents |
|-----|-------------------|----------|
| `rawprogram_unsparse0.xml` | LUN 0 | Super (system/vendor/product), recovery, vbmeta, metadata, userdata |
| `rawprogram1.xml` | LUN 1 | XBL (bootloader first stage) |
| `rawprogram2.xml` | LUN 2 | XBL backup |
| `rawprogram3.xml` | LUN 3 | DDR training data |
| `rawprogram4.xml` | LUN 4 | Boot, modem, TZ, HYP, ABL, DSP, devcfg, etc. |
| `rawprogram5.xml` | LUN 5 | SPUNVM, logfs |

### Sparse Images

Large partition images (super, userdata) are stored in Android sparse format and split into numbered chunks. Sparse images only contain the non-empty blocks, significantly reducing the firmware distribution size. The Firehose programmer handles decompressing sparse chunks back to raw data before writing to storage.

### A/B Slot Scheme

The Pimax Portal uses Android's A/B partition scheme for safe OTA updates. Every bootable partition exists in two copies — slot A and slot B. In the stock firmware image, only slot A is populated; slot B partitions are empty.

During normal OTA updates:
1. Device boots from active slot (A)
2. Update is written to inactive slot (B)
3. Boot flag switches atomically to B
4. If B fails to boot, device automatically falls back to A

This means a full EDL flash only needs to restore slot A — the device will boot from it and slot B remains empty until the next OTA.

### Firehose Programmer Variants

The firmware includes two programmer ELFs:

| File | Size | Purpose |
|------|------|---------|
| `prog_firehose_ddr.elf` | 684 KB | Full programmer with DDR memory initialization. Faster flashing. **Use this one.** |
| `prog_firehose_lite.elf` | 683 KB | Minimal programmer without DDR init. Fallback if DDR variant fails. |

### Firmware Variants (erase_persist vs ignore_persist)

The firmware ships with three rawprogram0 variants in subdirectories:

| Directory | Persist partition | When to use |
|-----------|-------------------|-------------|
| `kona/` (root) | Preserved | **Default — use this for recovery.** Keeps device calibration and config. |
| `ignore_persist/` | Preserved | Same as root — persist partition is not touched. |
| `erase_persist/` | Wiped and reflashed | Full factory reset. Erases device-specific persistent data. Only use if you suspect persist corruption. |

The persist partition (~32 MB) stores device-specific calibration data, SELinux contexts, and framework properties. In most recovery scenarios you want to preserve it.

### Provision Files (Do Not Use)

The firmware contains `provision_default.xml` and `provision_ufs31.xml`. These are **factory-only** UFS provisioning configs that define the physical LUN geometry. **Never flash these** — they are a one-time operation and using the wrong config can permanently misconfigure UFS storage.

### Selective Wipe Files

The `wipe_rawprogram_PHY*.xml` files target individual UFS LUNs:

| File | Target | Effect |
|------|--------|--------|
| `wipe_rawprogram_PHY0.xml` | LUN 0 | Wipes super, recovery, userdata, metadata |
| `wipe_rawprogram_PHY1.xml` | LUN 1 | Wipes boot LUN A (XBL) |
| `wipe_rawprogram_PHY2.xml` | LUN 2 | Wipes boot LUN B (XBL backup) |
| `wipe_rawprogram_PHY4.xml` | LUN 4 | Wipes all system firmware (boot, modem, TZ, etc.) |
| `wipe_rawprogram_PHY5.xml` | LUN 5 | Wipes modem/RF config partitions |

These are destructive — they overwrite partitions with zeros. Only useful if you need to securely erase a specific LUN before reflashing.

---

## License

GPL v3
