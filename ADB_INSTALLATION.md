# ADB Installation Guide

ADB (Android Debug Bridge) is the only external dependency required by this tool. Follow the instructions for your platform below.

## macOS

```bash
brew install android-platform-tools
```

## Linux (Ubuntu / Debian)

```bash
sudo apt install adb
```

## Linux (Arch)

```bash
sudo pacman -S android-tools
```

## Windows

1. Download the [SDK Platform-Tools](https://developer.android.com/studio/releases/platform-tools) ZIP from developer.android.com
2. Extract the ZIP to a permanent location (e.g. `C:\platform-tools`)
3. Add the folder to your system PATH:
   - Open **Settings > System > About > Advanced system settings**
   - Click **Environment Variables**
   - Under **System variables**, select **Path** and click **Edit**
   - Click **New** and add the path to the extracted folder (e.g. `C:\platform-tools`)
   - Click **OK** to save

## Enable USB Debugging on Your Device

These steps apply to all platforms:

1. On your Pimax Portal Retro, go to **Settings > About tablet**
2. Tap **Build number** seven times to enable Developer Options
3. Go back to **Settings > System > Developer options**
4. Enable **USB debugging**
5. Connect the device to your computer via USB
6. When prompted on the device screen, tap **Allow** to authorize the computer

## Verify

Run the following command to confirm ADB can see your device:

```bash
adb devices
```

You should see output like:

```
List of devices attached
XXXXXXXX    device
```

If the device shows as `unauthorized`, check the device screen for the authorization prompt and tap **Allow**.
