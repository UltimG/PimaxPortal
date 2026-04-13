#!/system/bin/sh

SKIPMOUNT=false
PROPFILE=false
POSTFSDATA=false
LATESTARTSERVICE=true

ui_print "- Pimax Portal GPU Driver Upgrade"
ui_print "- Source: Retroid Pocket Mini (Android 10, SD865)"
ui_print "- OpenGL: Qualcomm V@0764.0 (E031.45.03.01)"
ui_print "- Replaces: V@0655.0 (E031.40.09.00)"
ui_print ""

# --- Vulkan driver selection ---

USE_TURNIP=0

# Non-interactive override: if flag file exists, auto-select Turnip.
# Create with: adb shell touch /sdcard/pimax_turnip
if [ -f /sdcard/pimax_turnip ]; then
  USE_TURNIP=1
  ui_print "- Found /sdcard/pimax_turnip — auto-selecting Mesa Turnip"
else
  # Interactive volume key selection
  ui_print "- Select Vulkan driver:"
  ui_print "    Vol UP   = Qualcomm V@0764 (tested, default)"
  ui_print "    Vol DOWN = Mesa Turnip 25.3.4 (open source, Vulkan 1.3)"
  ui_print ""
  ui_print "- Waiting 10 seconds... (default: Qualcomm)"

  # Read volume key press via getevent with timeout
  VOLKEY=""
  for i in 1 2 3 4 5 6 7 8 9 10; do
    timeout 1 /system/bin/getevent -lqc 1 > /tmp/vk_event 2>&1
    if grep -q 'KEY_VOLUMEDOWN *DOWN' /tmp/vk_event 2>/dev/null; then
      VOLKEY="down"
      break
    elif grep -q 'KEY_VOLUMEUP *DOWN' /tmp/vk_event 2>/dev/null; then
      VOLKEY="up"
      break
    fi
  done
  rm -f /tmp/vk_event

  if [ "$VOLKEY" = "down" ]; then
    USE_TURNIP=1
  fi
fi

if [ "$USE_TURNIP" = "1" ]; then
  ui_print "- Vulkan: Mesa Turnip 25.3.4 (Vulkan 1.3, open source)"
  # Replace Qualcomm Vulkan with Turnip (64-bit only)
  cp -f "$MODPATH/turnip/vulkan.kona.so" "$MODPATH/system/vendor/lib64/hw/vulkan.kona.so"
  # Remove Qualcomm Vulkan debug layer (incompatible with Turnip)
  rm -f "$MODPATH/system/vendor/lib64/libVkLayer_q3dtools.so"
else
  ui_print "- Vulkan: Qualcomm V@0764 (default)"
fi

# Clean up bundled Turnip files (not needed after install)
rm -rf "$MODPATH/turnip"

# --- Permissions ---

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
