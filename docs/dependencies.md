# Dependencies

This tool requires **ADB** and **7z** to be installed and available in your PATH.

---

## ADB (Android Debug Bridge)

### macOS

```bash
brew install android-platform-tools
```

### Linux (Ubuntu / Debian)

```bash
sudo apt install adb
```

### Linux (Arch)

```bash
sudo pacman -S android-tools
```

### Windows

1. Download the [SDK Platform-Tools](https://developer.android.com/studio/releases/platform-tools) ZIP
2. Extract to a permanent location (e.g. `C:\platform-tools`)
3. Add to PATH:
   - Open **Settings > System > About > Advanced system settings**
   - Click **Environment Variables**
   - Under **System variables**, select **Path** and click **Edit**
   - Click **New** and add `C:\platform-tools`
   - Click **OK** to save

---

## 7z (7-Zip)

### macOS

```bash
brew install p7zip
```

### Linux (Ubuntu / Debian)

```bash
sudo apt install p7zip-full
```

### Linux (Arch)

```bash
sudo pacman -S p7zip
```

### Windows

1. Download and install [7-Zip](https://7-zip.org)
2. Add to PATH:
   - Default install location is `C:\Program Files\7-Zip`
   - Open **Settings > System > About > Advanced system settings**
   - Click **Environment Variables**
   - Under **System variables**, select **Path** and click **Edit**
   - Click **New** and add `C:\Program Files\7-Zip`
   - Click **OK** to save
3. Open a **new** terminal and verify: `7z` should print version info

---

## Device Setup

### Enable USB Debugging

1. On your Pimax Portal Retro, go to **Settings > About tablet**
2. Tap **Build number** seven times to enable Developer Options
3. Go back to **Settings > System > Developer options**
4. Enable **USB debugging**
5. Connect the device to your computer via USB
6. When prompted on the device screen, tap **Allow** to authorize the computer

### Grant Shell Root Access in Magisk

The tool needs root (su) access via ADB to install Magisk modules. On first use, Magisk will show a prompt on the device screen asking to grant access to the shell.

If no prompt appears:
1. Open **Magisk** app on the device
2. Go to **Settings**
3. Make sure **Superuser** is enabled
4. Go to the **Superuser** tab and check that **Shell** has been granted access

## Verify

```bash
adb devices
```

You should see:

```
List of devices attached
XXXXXXXX    device
```

If the device shows as `unauthorized`, check the device screen for the authorization prompt and tap **Allow**.
