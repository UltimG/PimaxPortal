# GPU Overclocking Guide (KonaBess)

This guide walks you through manually overclocking the Adreno 650 GPU on the **Pimax Portal Retro** using KonaBess. The Portal runs a Snapdragon XR2 (SM8250) with the GPU clocked at **855 MHz** stock — already higher than a standard SD865 (587 MHz). We can push it further.

**Tested frequencies on the Pimax Portal:**

| Frequency | Voltage | Stability | Performance Gain |
|-----------|---------|-----------|-----------------|
| 855 MHz | NOM_L1 (320) | Stock | Baseline |
| 905 MHz | NOM_L1 (320) | Stable | ~1% |
| 940 MHz | TURBO_L1 (416) | Stable | ~1.8% |
| 985 MHz | TURBO_L1 (416) | Stable | ~2.3% |
| 1000 MHz | TURBO_L1 (416) | Thermal throttle | Worse than 985 |

**Wild Life benchmark results:**

| Clock | Score | Avg FPS |
|-------|-------|---------|
| 855 MHz | 5564 | 33.32 |
| 940 MHz | 5666 | 33.93 |
| 985 MHz | 5693 | 34.09 |
| 1000 MHz | 5689 | 34.07 |

985 MHz at TURBO_L1 is the safe sweet spot — highest sustained frequency without thermal throttling or stability risk.

---

## Prerequisites

- **Rooted** Pimax Portal Retro (Magisk)
- **KonaBess** APK installed on the device
  - Download from [GitHub](https://github.com/libxzr/KonaBess) (works on Android 9+)

## Important: Backup First

Before making any changes, back up your boot partition. Connect via ADB and run:

```bash
adb shell su -c "dd if=/dev/block/by-name/boot_a of=/sdcard/boot_backup.img bs=4096"
adb pull /sdcard/boot_backup.img
```

Keep this file safe. If anything goes wrong, you can restore it:

```bash
adb push boot_backup.img /sdcard/boot_backup.img
adb shell su -c "dd if=/sdcard/boot_backup.img of=/dev/block/by-name/boot_a bs=4096"
adb reboot
```

---

## How GPU Overclocking Works

GPU frequencies are defined in the **Device Tree Binary (DTB)** embedded in the boot partition. The DTB contains an OPP (Operating Performance Points) table that maps each frequency to a voltage level. The kernel reads this table at boot and configures the GPU accordingly.

KonaBess modifies this table by:

1. Extracting the boot image
2. Decompiling the DTB
3. Editing the GPU frequency/voltage table
4. Repacking and flashing the modified boot image

**You cannot add frequencies above the stock maximum via sysfs at runtime** — only KonaBess (or similar DTB editors) can do this.

---

## Step 1: Open KonaBess

Launch KonaBess on the device. It will ask for root permission — grant it.

Select **Snapdragon 865 / 870 / 888** (kona platform) when prompted.

## Step 2: Read Current Boot Image

KonaBess will read the boot image from the active partition. This takes a few seconds. Once loaded, you'll see the current GPU frequency table:

```
855 MHz  — NOM_L1
720 MHz  — NOM_L1
670 MHz  — NOM_L1
587 MHz  — NOM
525 MHz  — SVS_L2
490 MHz  — SVS_L1
442 MHz  — SVS_L0
400 MHz  — SVS
305 MHz  — LOW_SVS
```

## Step 3: Change the Top Frequency

You cannot add new entries in KonaBess — only modify existing ones.

Tap the **855 MHz** entry and change the frequency to your target:

| Target | Change frequency to | Change voltage to |
|--------|-------------------|-------------------|
| 905 MHz | 905 | Keep NOM_L1 |
| 940 MHz | 940 | TURBO_L1 |
| 985 MHz | 985 | TURBO_L1 |

**Do not change the voltage for 905 MHz** — the XR2 silicon handles it at stock voltage. For 940+ MHz, bump to TURBO_L1.

## Step 4: Flash

Tap the flash/save button. KonaBess will repack the boot image with the modified DTB and flash it to the boot partition.

**Reboot** the device for changes to take effect.

## Step 5: Verify

After reboot, verify the new frequency via ADB:

```bash
adb shell su -c "cat /sys/class/kgsl/kgsl-3d0/gpu_available_frequencies"
```

The first number should be your new frequency in Hz (e.g., `985000000`).

---

## Monitoring GPU Temperature

Check if the GPU is throttling during load:

```bash
adb shell "su -c 'while true; do echo GPU: $(cat /sys/class/kgsl/kgsl-3d0/gpuclk)Hz thermal:$(cat /sys/class/thermal/thermal_zone7/temp)mC throttle:$(cat /sys/class/kgsl/kgsl-3d0/thermal_pwrlevel); sleep 2; done'"
```

| Value | Meaning |
|-------|---------|
| `thermal_pwrlevel = 0` | Not throttling — GPU can reach max frequency |
| `thermal_pwrlevel > 0` | Throttling — GPU is capped below max |

The Pimax Portal has active cooling (fan). GPU temperatures of 60–75°C under load are normal. Throttling typically kicks in around 85–95°C.

---

## Recovery

### Black Screen After Overclock

If the device boots to a black screen (GPU can't sustain the frequency):

1. **ADB may still work** — try:
   ```bash
   adb push boot_backup.img /sdcard/boot_backup.img
   adb shell su -c "dd if=/sdcard/boot_backup.img of=/dev/block/by-name/boot_a bs=4096"
   adb reboot
   ```

2. **Fastboot** — if ADB doesn't respond, hold **Power + Volume Down** to enter bootloader:
   ```bash
   fastboot flash boot_a boot_backup.img
   fastboot reboot
   ```

3. **EDL** — last resort if fastboot doesn't work. See [EDL Recovery Guide](edl-recovery-guide.md).

### Magisk Safe Mode

Holding Volume Down during boot enters Magisk safe mode. This disables Magisk modules but **does not help with DTB changes** — the overclock is in the boot image itself, not a module.

---

## Technical Details

### What's in the DTB

The Pimax Portal boot image contains **7 concatenated DTBs**:

| DTBs | Platform | Top Frequency | Notes |
|------|----------|---------------|-------|
| #1, #2, #3, #5, #6 | Kona v2 (XR2) | 855 MHz | Active — these get modified |
| #4, #7 | Kona v1 (base SD865) | 480 MHz | Inactive — leave alone |

Each kona v2 DTB has 4 speed-bin tables. Only speed-bin 0 (with 855 MHz top) is active on the Portal.

### Two Tables Modified

KonaBess modifies two structures:

1. **`qcom,gpu-pwrlevels`** — the frequency table the GPU driver reads
   - Property: `qcom,gpu-freq` (uint32, Hz)

2. **`gpu-opp-table_v2`** — the frequency-to-voltage mapping
   - Property: `opp-hz` (uint64, Hz) — frequency
   - Property: `opp-microvolt` (uint32) — RPMH voltage level

### RPMH Voltage Levels

| Value | Name | Typical Use |
|-------|------|-------------|
| 64 | LOW_SVS | 305 MHz |
| 128 | SVS | 400 MHz |
| 144 | SVS_L0 | 442 MHz |
| 192 | SVS_L1 | 490 MHz |
| 224 | SVS_L2 | 525 MHz |
| 256 | NOM | 587 MHz |
| 320 | NOM_L1 | 670–855 MHz (stock max) |
| 384 | TURBO | ~905 MHz |
| 416 | TURBO_L1 | 940–985 MHz |
| 448 | TURBO_HIGH | 1 GHz+ (silicon lottery) |

### Why the XR2 Can Clock Higher

The Snapdragon XR2 is a specially binned SM8250 for XR applications. Standard SD865 tops out at 587 MHz (speed-bin 0) or 670 MHz (speed-bin 1). The XR2's DTB defines 855 MHz as the top frequency at only NOM_L1 voltage — the same voltage standard devices use for 670 MHz. This means the Portal's silicon has significant overclocking headroom compared to typical SD865 devices.

### Automated Alternative

The TUI tool includes a built-in GPU Overclock screen that automates the entire process — backup, DTB patching, and flashing — without needing KonaBess. See [TUI Tool (Automated)](automated-install.md).
