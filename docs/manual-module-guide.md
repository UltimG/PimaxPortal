# Manual Magisk Module Guide: Pimax Portal GPU Driver Upgrade

This guide walks you through manually building and installing a Magisk module that replaces the stock Adreno 650 GPU drivers on the **Pimax Portal Retro** with newer drivers from the **Retroid Pocket Mini** firmware.

**What this fixes:** The stock Pimax Portal ships with outdated Adreno 650 drivers (V@0655.0, June 2022) that cause Vulkan rendering crashes and graphical glitches. The RP Mini drivers (V@0764.0, January 2024) are a drop-in replacement — both devices run Snapdragon 865 + Android 10.

## Prerequisites

| Tool | Purpose | Install |
|------|---------|---------|
| `adb` | Push files to device, run shell commands | `brew install android-platform-tools` (macOS) / `apt install adb` (Linux) |
| `7z` (7-Zip) | Extract .7z archives and files from ext4 images | `brew install p7zip` (macOS) / `apt install p7zip-full` (Linux) |
| `zip` | Create the Magisk module zip | Pre-installed on most systems |
| Magisk | Root + module installer on the device | Already installed on the Pimax Portal |

**Disk space:** ~25 GB free (firmware download is ~13 GB, super partition is ~4.7 GB, vendor image is ~1.2 GB).

---

## Step 1: Download the Retroid Pocket Mini Firmware

The firmware is hosted as a split 7z archive (4 parts) on GitHub:

```bash
mkdir -p firmware && cd firmware

# Download all 4 parts
BASE="https://github.com/TheGammaSqueeze/Retroid_Pocket_Stock_Firmware/releases/download"
TAG="Android10_RPMini_V1.0.0.310_20240926_181623_user"

for i in 001 002 003 004; do
  curl -L -O "${BASE}/${TAG}/${TAG}.7z.${i}"
done
```

You should have 4 files:
```
Android10_RPMini_V1.0.0.310_20240926_181623_user.7z.001
Android10_RPMini_V1.0.0.310_20240926_181623_user.7z.002
Android10_RPMini_V1.0.0.310_20240926_181623_user.7z.003
Android10_RPMini_V1.0.0.310_20240926_181623_user.7z.004
```

> **Tip:** If a download gets interrupted, `curl -L -C - -O <url>` will resume it.

---

## Step 2: Extract the Super Partition from the Firmware

The firmware archive contains a full flash image. We only need the super partition (`lun0_super.bin`), which holds all logical partitions including vendor.

```bash
cd ..
mkdir -p extracted

# Extract only lun0_super.bin from the split 7z archive
7z e firmware/Android10_RPMini_V1.0.0.310_20240926_181623_user.7z.001 \
  flash/lun0_super.bin -oextracted -y
```

This produces `extracted/lun0_super.bin` (~4.7 GB).

---

## Step 3: Extract the Vendor Partition from the Super Image

The super image uses Android's **Logical Partition (LP)** format, which packs multiple partitions (system, vendor, product, etc.) into a single image. We need to extract `vendor_a`.

### Option A: Use lpunpack (recommended)

If you have `lpunpack` from the Android build tools or [simg2img/lpunpack](https://github.com/nicholasgasior/golds) toolchain:

```bash
lpunpack extracted/lun0_super.bin extracted/
# This produces: extracted/vendor_a.img
```

### Option B: Use simg2img tools

Some Android tool packages provide `lpunpack`:

```bash
# On Arch Linux
sudo pacman -S android-tools
lpunpack extracted/lun0_super.bin extracted/

# On Ubuntu/Debian — may need to build from source
# See: https://android.googlesource.com/platform/system/extras/+/refs/heads/master/partition_tools/
```

### Option C: Manual extraction with Python

If you don't have `lpunpack`, you can use the Python `simg2img` / `lpunpack` tools:

```bash
pip install simg2img
# Or use: https://github.com/nicholasgasior/golds
```

After extraction you should have `vendor_a.img` (~1.2 GB). You can delete `lun0_super.bin` to reclaim space:

```bash
rm extracted/lun0_super.bin
```

---

## Step 4: Extract GPU Driver Files from vendor_a.img

The vendor image is an ext4 filesystem. We use `7z` to extract individual files from it.

Create the output directory structure and extract all 34 driver files:

```bash
mkdir -p drivers

# --- 64-bit Vulkan HAL ---
7z e extracted/vendor_a.img lib64/hw/vulkan.kona.so -odrivers/lib64/hw -y

# --- 64-bit EGL/GLES libraries ---
for lib in eglSubDriverAndroid libEGL_adreno libGLESv1_CM_adreno \
           libGLESv2_adreno libq3dtools_adreno libq3dtools_esx; do
  7z e extracted/vendor_a.img "lib64/egl/${lib}.so" -odrivers/lib64/egl -y
done

# --- 64-bit support libraries ---
for lib in libadreno_app_profiles libadreno_utils libgsl \
           libllvm-glnext libllvm-qcom libllvm-qgl; do
  7z e extracted/vendor_a.img "lib64/${lib}.so" -odrivers/lib64 -y
done

# --- 64-bit only: Vulkan validation layer ---
7z e extracted/vendor_a.img lib64/libVkLayer_q3dtools.so -odrivers/lib64 -y

# --- 32-bit Vulkan HAL ---
7z e extracted/vendor_a.img lib/hw/vulkan.kona.so -odrivers/lib/hw -y

# --- 32-bit EGL/GLES libraries ---
for lib in eglSubDriverAndroid libEGL_adreno libGLESv1_CM_adreno \
           libGLESv2_adreno libq3dtools_adreno libq3dtools_esx; do
  7z e extracted/vendor_a.img "lib/egl/${lib}.so" -odrivers/lib/egl -y
done

# --- 32-bit support libraries ---
for lib in libadreno_app_profiles libadreno_utils libgsl \
           libllvm-glnext libllvm-qcom libllvm-qgl; do
  7z e extracted/vendor_a.img "lib/${lib}.so" -odrivers/lib -y
done

# --- Adreno 650 firmware blobs ---
for fw in a650_gmu.bin a650_sqe.fw a650_zap.b00 a650_zap.b01 \
          a650_zap.b02 a650_zap.elf a650_zap.mdt; do
  7z e extracted/vendor_a.img "firmware/${fw}" -odrivers/firmware -y
done
```

**Verify you got all 34 files:**
```bash
find drivers -type f | wc -l
# Should output: 34
```

You can now delete the vendor image:
```bash
rm extracted/vendor_a.img
```

---

## Step 5: Create the Magisk Module Structure

A Magisk module is a zip file with a specific directory layout. Magisk overlays files under `system/` onto the real `/system` partition at boot using a bind-mount mechanism — no actual system partition modification occurs.

```bash
mkdir -p module
```

### 5a. Module Metadata — `module.prop`

```bash
cat > module/module.prop << 'EOF'
id=pimax-portal-gpu-drivers
name=Pimax Portal GPU Driver Upgrade (RP Mini V@0764)
version=v1.0
versionCode=1
author=UltimG
description=Adreno 650 GPU drivers from Retroid Pocket Mini (V@0764.0, Jan 2024). Replaces stock V@0655.0 (Jun 2022). Fixes Vulkan crashes and improves performance.
EOF
```

**Fields explained:**
| Field | Purpose |
|-------|---------|
| `id` | Unique module identifier (no spaces, used as directory name under `/data/adb/modules/`) |
| `name` | Human-readable name shown in Magisk Manager |
| `version` | Display version string |
| `versionCode` | Integer version for update detection (higher = newer) |
| `author` | Your name |
| `description` | Shown in Magisk Manager module list |

### 5b. Installer Stub — `META-INF/com/google/android/update-binary`

This is the entry point Magisk calls when installing the module. It loads Magisk's helper functions and delegates to them:

```bash
mkdir -p module/META-INF/com/google/android

cat > module/META-INF/com/google/android/update-binary << 'BINEOF'
#!/sbin/sh

#################
# Initialization
#################

umask 022

# echo before loading util_functions
ui_print() { echo "$1"; }

require_new_magisk() {
  ui_print "*******************************"
  ui_print " Please install Magisk v20.4+! "
  ui_print "*******************************"
  exit 1
}

#########################
# Load util_functions.sh
#########################

OUTFD=$2
ZIPFILE=$3

mount /data 2>/dev/null

[ -f /data/adb/magisk/util_functions.sh ] || require_new_magisk
. /data/adb/magisk/util_functions.sh
[ $MAGISK_VER_CODE -lt 20400 ] && require_new_magisk

install_module
exit 0
BINEOF

chmod +x module/META-INF/com/google/android/update-binary
```

**How this works:**
1. Sets up a minimal `ui_print` function for output before Magisk's helpers are loaded
2. Mounts `/data` to access Magisk's installation directory
3. Sources `/data/adb/magisk/util_functions.sh` — this gives access to `set_perm`, `set_perm_recursive`, `ui_print` (real version), and `install_module`
4. Requires Magisk v20.4+ (version code 20400)
5. Calls `install_module` which handles copying module files to `/data/adb/modules/<id>/`

### 5c. Updater Script Marker — `META-INF/com/google/android/updater-script`

Magisk uses this file to identify the zip as a Magisk module (not a regular recovery zip):

```bash
echo '#MAGISK' > module/META-INF/com/google/android/updater-script
```

### 5d. Installation Script — `customize.sh`

This script runs during module installation (after files are copied). It sets permissions and cleans shader caches:

```bash
cat > module/customize.sh << 'EOF'
#!/system/bin/sh

ui_print "- Pimax Portal GPU Driver Upgrade"
ui_print "- Source: Retroid Pocket Mini (Android 10, SD865)"
ui_print "- Driver: Adreno 650 V@0764.0 (E031.45.03.01)"
ui_print "- Replaces: V@0655.0 (E031.40.09.00)"
ui_print ""

# Set correct permissions for vendor libs
set_perm_recursive $MODPATH/system/vendor/lib 0 0 0755 0644
set_perm_recursive $MODPATH/system/vendor/lib64 0 0 0755 0644
set_perm_recursive $MODPATH/system/vendor/firmware 0 0 0755 0644

# Clean GPU shader caches to avoid stale compiled shaders
ui_print "- Cleaning shader caches..."
find /data -type d -name "gpu_cache" -exec rm -rf {} + 2>/dev/null
find /data -type d -name "GPUCache" -exec rm -rf {} + 2>/dev/null
find /data -type f -name "*.qpg" -delete 2>/dev/null
find /data -type f -name "*.bqp" -delete 2>/dev/null

ui_print "- Done. Reboot to apply."
EOF
```

**Why shader cache cleaning matters:** The GPU compiles shaders at runtime and caches them on disk. Old cached shaders compiled by the V@0655 driver are incompatible with the V@0764 driver and can cause crashes or rendering glitches. The `*.qpg` and `*.bqp` files are Qualcomm's compiled shader cache formats.

**Permission format:** `set_perm_recursive <path> <owner> <group> <dirmode> <filemode>`
- `0 0` = root:root ownership
- `0755` = directories are rwxr-xr-x
- `0644` = files are rw-r--r--

### 5e. Post-Boot Service — `service.sh`

This script runs every boot (in Magisk's `late_start` service mode). It fixes a Pimax-specific launcher crash:

```bash
cat > module/service.sh << 'EOF'
#!/system/bin/sh

# Runs after boot completes (late_start service mode)
# Fix: stock Pimax launcher crashes on boot because GameDisplayService
# isn't ready when the launcher's MainViewModel tries to call
# GameDisplay.getDisplayPanel(). Restarting the launcher after a delay
# allows the service to initialize first.

(
  sleep 15
  # Check if launcher has crashed (look for crash count)
  if dumpsys activity processes 2>/dev/null | grep -q "com.android.launcher3.*crash"; then
    am force-stop com.android.launcher3
    sleep 2
    am start -n com.android.launcher3/.Launcher
  fi
) &
EOF
```

**Why this is needed:** The Pimax Portal has a boot race condition. The stock launcher (`com.android.launcher3`) tries to call `GameDisplay.getDisplayPanel()` via `GameDisplayService` before that service is ready, causing a crash. This script waits 15 seconds after boot, checks if the launcher crashed, and restarts it if so. The subshell `( ... ) &` runs in the background so it doesn't block the boot sequence.

> **Note:** If you don't experience launcher crashes on boot, you can omit this file.

---

## Step 6: Add Driver Files to the Module

Magisk's `system/vendor/` directory maps to the device's `/vendor/` partition. Copy the extracted drivers into this structure:

```bash
mkdir -p module/system/vendor

cp -r drivers/lib     module/system/vendor/
cp -r drivers/lib64   module/system/vendor/
cp -r drivers/firmware module/system/vendor/
```

Your final module structure should look like:

```
module/
├── META-INF/
│   └── com/google/android/
│       ├── update-binary          (Magisk installer entry point)
│       └── updater-script         (#MAGISK marker)
├── module.prop                    (Module metadata)
├── customize.sh                   (Install-time script)
├── service.sh                     (Post-boot launcher fix)
└── system/
    └── vendor/
        ├── lib/                   (32-bit libraries — 13 files)
        │   ├── hw/
        │   │   └── vulkan.kona.so
        │   └── egl/
        │       ├── eglSubDriverAndroid.so
        │       ├── libEGL_adreno.so
        │       ├── libGLESv1_CM_adreno.so
        │       ├── libGLESv2_adreno.so
        │       ├── libq3dtools_adreno.so
        │       └── libq3dtools_esx.so
        │   ├── libadreno_app_profiles.so
        │   ├── libadreno_utils.so
        │   ├── libgsl.so
        │   ├── libllvm-glnext.so
        │   ├── libllvm-qcom.so
        │   └── libllvm-qgl.so
        ├── lib64/                 (64-bit libraries — 14 files)
        │   ├── hw/
        │   │   └── vulkan.kona.so
        │   ├── egl/
        │   │   ├── eglSubDriverAndroid.so
        │   │   ├── libEGL_adreno.so
        │   │   ├── libGLESv1_CM_adreno.so
        │   │   ├── libGLESv2_adreno.so
        │   │   ├── libq3dtools_adreno.so
        │   │   └── libq3dtools_esx.so
        │   ├── libadreno_app_profiles.so
        │   ├── libadreno_utils.so
        │   ├── libgsl.so
        │   ├── libllvm-glnext.so
        │   ├── libllvm-qcom.so
        │   ├── libllvm-qgl.so
        │   └── libVkLayer_q3dtools.so  (64-bit only)
        └── firmware/              (GPU firmware blobs — 7 files)
            ├── a650_gmu.bin
            ├── a650_sqe.fw
            ├── a650_zap.b00
            ├── a650_zap.b01
            ├── a650_zap.b02
            ├── a650_zap.elf
            └── a650_zap.mdt
```

**Total: 34 driver/firmware files + 5 module files = 39 files**

---

## Step 7: Package the Module as a Zip

```bash
cd module
zip -r ../pimax-gpu-drivers.zip . \
  -x '*.DS_Store' -x '__MACOSX/*'
cd ..
```

> **Important:** The zip must be created from inside the `module/` directory so paths are relative (e.g., `module.prop`, not `module/module.prop`).

**Verify the zip structure:**
```bash
unzip -l pimax-gpu-drivers.zip | head -20
```

You should see paths like `module.prop`, `META-INF/com/google/android/update-binary`, `system/vendor/lib64/hw/vulkan.kona.so`, etc. — no leading `module/` prefix.

---

## Step 8: Install on the Pimax Portal

### 8a. Connect via ADB

```bash
adb devices
# Should show your Pimax Portal (fujilite)
```

### 8b. Push the Module

```bash
adb push pimax-gpu-drivers.zip /sdcard/pimax-gpu-drivers.zip
```

### 8c. Install via Magisk

```bash
adb shell su -c 'magisk --install-module /sdcard/pimax-gpu-drivers.zip'
```

> **Note:** The device will show a Magisk superuser prompt. You must tap **Allow** on the device screen to grant root access. If you don't see it, check Magisk Manager's superuser settings.

You should see output like:
```
- Pimax Portal GPU Driver Upgrade
- Source: Retroid Pocket Mini (Android 10, SD865)
- Driver: Adreno 650 V@0764.0 (E031.45.03.01)
- Replaces: V@0655.0 (E031.40.09.00)

- Cleaning shader caches...
- Done. Reboot to apply.
```

### 8d. Cleanup and Reboot

```bash
adb shell rm /sdcard/pimax-gpu-drivers.zip
adb reboot
```

---

## Verifying the Installation

After reboot, verify the new drivers are active:

```bash
# Check GPU driver version via SurfaceFlinger
adb shell dumpsys SurfaceFlinger | grep -i "driver version"
# Expected: something containing V@0764 or E031.45.03.01

# Check Vulkan driver
adb shell dumpsys SurfaceFlinger | grep -i vulkan

# Verify module is active in Magisk
adb shell su -c 'ls /data/adb/modules/pimax-portal-gpu-drivers/'
```

---

## Uninstalling

### Via Magisk Manager
Open Magisk Manager on the device, go to Modules, and tap the trash icon next to "Pimax Portal GPU Driver Upgrade". Reboot.

### Via ADB
```bash
adb shell su -c 'rm -rf /data/adb/modules/pimax-portal-gpu-drivers'
adb reboot
```

---

## How Magisk Modules Work (Background)

For those curious about the underlying mechanism:

1. **No system modification:** Magisk uses a "systemless" approach. The real `/vendor` partition is never modified. Instead, Magisk bind-mounts modified files on top of the originals at boot time.

2. **Module directory:** After installation, module files live at `/data/adb/modules/pimax-portal-gpu-drivers/`. The `system/vendor/` subtree mirrors the real vendor partition.

3. **Boot sequence:**
   - Magisk patches the boot image to run early in the init process
   - It reads each module's `system/` tree and bind-mounts those files over the real partition
   - After mount, it runs each module's `service.sh` in `late_start` service context

4. **Safe to revert:** Since the real system/vendor partitions are untouched, removing the module and rebooting restores the original drivers instantly.

5. **`customize.sh` vs `service.sh`:**
   - `customize.sh` runs **once** at install time (sets permissions, cleans caches)
   - `service.sh` runs **every boot** (handles the launcher race condition fix)

---

## Complete File List Reference

<details>
<summary>All 34 driver files extracted from vendor_a.img</summary>

**64-bit (lib64/) — 14 files:**
| File | Purpose |
|------|---------|
| `lib64/hw/vulkan.kona.so` | Vulkan HAL implementation for Snapdragon 865 (Kona) |
| `lib64/egl/eglSubDriverAndroid.so` | EGL sub-driver for Android |
| `lib64/egl/libEGL_adreno.so` | Adreno EGL implementation |
| `lib64/egl/libGLESv1_CM_adreno.so` | OpenGL ES 1.x (Common Profile) |
| `lib64/egl/libGLESv2_adreno.so` | OpenGL ES 2.0/3.x |
| `lib64/egl/libq3dtools_adreno.so` | Qualcomm 3D debug/profiling tools |
| `lib64/egl/libq3dtools_esx.so` | Qualcomm 3D tools (ESX variant) |
| `lib64/libadreno_app_profiles.so` | Per-app GPU performance profiles |
| `lib64/libadreno_utils.so` | Adreno utility functions |
| `lib64/libgsl.so` | Graphics Support Library (GPU command submission) |
| `lib64/libllvm-glnext.so` | LLVM backend for shader compilation |
| `lib64/libllvm-qcom.so` | Qualcomm LLVM extensions |
| `lib64/libllvm-qgl.so` | LLVM Qualcomm GL pipeline |
| `lib64/libVkLayer_q3dtools.so` | Vulkan validation/debug layer (64-bit only) |

**32-bit (lib/) — 13 files:**
Same as 64-bit except no `libVkLayer_q3dtools.so` (Vulkan layers are 64-bit only).

**Firmware (firmware/) — 7 files:**
| File | Purpose |
|------|---------|
| `firmware/a650_gmu.bin` | Graphics Management Unit microcode |
| `firmware/a650_sqe.fw` | Shader Queue Engine firmware |
| `firmware/a650_zap.b00` | Secure boot loader segment 0 |
| `firmware/a650_zap.b01` | Secure boot loader segment 1 |
| `firmware/a650_zap.b02` | Secure boot loader segment 2 |
| `firmware/a650_zap.elf` | ZAP shader processor ELF binary |
| `firmware/a650_zap.mdt` | ZAP metadata (segment table) |

</details>

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `7z` can't extract from vendor_a.img | Make sure you have `p7zip-full` (not just `p7zip`). The `7z` command must support ext4 filesystem extraction. |
| Magisk says "version too old" | Update Magisk to v20.4 or newer |
| Root access denied during install | Open Magisk Manager on the device, go to Superuser settings, and make sure shell/ADB access is set to Allow |
| Launcher keeps crashing after reboot | The `service.sh` fix should handle this. If not, manually run: `adb shell am force-stop com.android.launcher3 && adb shell am start -n com.android.launcher3/.Launcher` |
| Graphics glitches after install | Clear shader caches manually: `adb shell su -c 'find /data -type d -name "gpu_cache" -exec rm -rf {} +'` then reboot |
| Want to go back to stock drivers | Uninstall the module (see Uninstalling section) and reboot |

---

## License

GPL v3
