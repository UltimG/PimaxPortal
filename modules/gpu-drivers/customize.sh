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
