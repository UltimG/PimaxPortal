# Rooting Guide: Pimax Portal

This guide covers how to root the Pimax Portal (Retro/Lite) using Magisk.

**What rooting enables:** Root access is required to install Magisk modules (like the GPU driver upgrade), modify system settings, use root-only apps, and access the full Android shell.

> **Prefer automation?** The [pimaxportal TUI tool](automated-install.md) automates this entire process. Select "Rooting" from the sidebar and follow the on-screen instructions. The manual steps below are for users who prefer to do it by hand.

---

## Prerequisites

| Requirement | Details |
|-------------|---------|
| ADB | See [Dependencies](dependencies.md) for install instructions |
| USB debugging | Enabled on the device (Settings > System > Developer options) |
| Stock boot.img | Provided in this repository ([download](https://github.com/UltimG/PimaxPortal/raw/main/assets/firmware/boot.img)) |

The stock boot.img matches firmware version `Portal-Lite_kona-1.1.6-20230922-14-user`.

---

## Step 1: Install Magisk on the Device

Download the latest Magisk APK from the [official releases](https://github.com/topjohnwu/Magisk/releases) page.

Push and install via ADB:

```bash
adb install Magisk-*.apk
```

Or transfer the APK to the device and install it manually through a file manager.

---

## Step 2: Push boot.img to the Device

```bash
adb push boot.img /sdcard/
```

If the device has an SD card and internal storage is low:

```bash
# Check SD card mount point
adb shell ls /storage/

# Push to SD card
adb push boot.img /storage/<SD_CARD_ID>/
```

---

## Step 3: Patch boot.img with Magisk

1. Open the **Magisk** app on the device
2. Tap **Install** next to "Magisk"
3. Select **"Select and Patch a File"**
4. Navigate to `boot.img` (internal storage or SD card) and select it
5. Tap **"Let's Go"**
6. Wait for patching to complete — Magisk will show "All done!"

The patched image is saved to `/sdcard/Download/magisk_patched-XXXXX_XXXXX.img`.

---

## Step 4: Pull the Patched Image

```bash
adb pull /sdcard/Download/magisk_patched-*.img magisk_patched.img
```

---

## Step 5: Flash via FastbootD

**Important:** The Pimax Portal uses a UEFI bootloader that does **not** support flash commands in normal fastboot mode. You must use **FastbootD** (userspace fastboot) instead.

```bash
# Reboot into FastbootD (NOT "adb reboot bootloader")
adb reboot fastboot
```

Wait for the device to enter FastbootD mode, then flash:

```bash
fastboot flash boot magisk_patched.img
```

You should see:

```
Sending 'boot_a' (98304 KB)                        OKAY [  2.343s]
Writing 'boot_a'                                   OKAY [  0.794s]
Finished. Total time: 3.157s
```

Reboot:

```bash
fastboot reboot
```

---

## Step 6: Verify Root

After the device boots:

1. Open the **Magisk** app — it should show Magisk version and "Installed" status
2. Verify via ADB:

```bash
adb shell su -c id
# Expected: uid=0(root) gid=0(root)
```

---

## Common Mistakes

### "Writing 'boot_a' FAILED (remote: 'unknown command')"

You rebooted into the UEFI bootloader (`adb reboot bootloader`) instead of FastbootD (`adb reboot fastboot`). These are two different modes:

| Mode | Command | Flash support |
|------|---------|---------------|
| UEFI fastboot | `adb reboot bootloader` | **No** — limited to getvar and reboot only |
| FastbootD | `adb reboot fastboot` | **Yes** — full read/write/flash support |

Reboot the device back to Android, then use `adb reboot fastboot`.

### Magisk app shows "No root"

The patched boot.img may not have been flashed to the active slot. Check which slot is active:

```bash
fastboot getvar current-slot
# Expected: a
```

If slot B is active, flash to it explicitly:

```bash
fastboot flash boot_b magisk_patched.img
```

### Device bootloops after flashing

The patched boot.img doesn't match the installed firmware version. Make sure you patched the boot.img that matches your current firmware — not one from a different version or device.

To recover:
- Enter FastbootD (`adb reboot fastboot` from recovery, or hold volume keys during boot)
- Flash the original stock boot.img:

```bash
fastboot flash boot boot.img
fastboot reboot
```

If you can't reach FastbootD, follow the [EDL Recovery Guide](edl-recovery-guide.md) for a full reflash.

---

## Keeping Root After OTA Updates

If Pimax pushes an OTA update, it will overwrite the patched boot image and remove root. To preserve root:

1. **Before updating:** Open Magisk > Install > "Install to Inactive Slot (After OTA)"
2. **After updating:** The inactive slot gets the OTA; Magisk automatically patches it before the slot switch

If you lose root after an update, repeat this guide with the new firmware's boot.img.

---

## Boot Image Details

For reference, the stock boot.img contains:

| Component | Details |
|-----------|---------|
| Kernel | Linux 4.19.81-perf (SMP PREEMPT) |
| Build date | Sep 22, 2023 |
| Compiler | clang 8.0.12 (Android NDK) |
| Header version | 2 |
| OS version | Android 10.0.0 (2020-08) |
| Page size | 4096 |
| Kernel size | 38.3 MB |
| Ramdisk size | 0.8 MB |

---

## License

GPL v3
