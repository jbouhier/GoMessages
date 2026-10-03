#!/bin/sh
# Bundle GoMessages as a macOS app with a real Dock icon.
# Usage: scripts/bundle.sh [target/debug/gomessages|target/release/gomessages]
set -eu
BIN="${1:-target/debug/gomessages}"
APP="dist/GoMessages.app"
VERSION="$(sed -n 's/^version = "\(.*\)"/\1/p' Cargo.toml | head -n 1)"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp "$BIN" "$APP/Contents/MacOS/GoMessages"
cp "$(dirname "$0")/../assets/GoMessages.icns" "$APP/Contents/Resources/GoMessages.icns"

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>GoMessages</string>
  <key>CFBundleDisplayName</key><string>GoMessages</string>
  <key>CFBundleIdentifier</key><string>dev.gomessages.app</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleExecutable</key><string>GoMessages</string>
  <key>CFBundleIconFile</key><string>GoMessages</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>LSMinimumSystemVersion</key><string>12.0</string>
  <key>NSHighResolutionCapable</key><string>True</string>
  <key>NSBluetoothAlwaysUsageDescription</key><string>GoMessages checks Bluetooth proximity when pairing with your phone.</string>
</dict>
</plist>
PLIST
echo "bundle: $APP (copy to /Applications, then run the .app, not the binary)"
