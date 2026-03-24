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
