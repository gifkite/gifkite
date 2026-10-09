#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

VERSION="${1:-${VERSION:-0.2.0}}"
BUNDLE_VERSION="${VERSION#v}"

# Kill any currently running Gifkite process before building
pkill -f "Gifkite.app/Contents/MacOS/gifkite" || true

echo "Building gifkite binary (version: $VERSION)..."
go build -tags private_mac_apis -ldflags="-X main.Version=$VERSION -s -w" -o gifkite .

APP="Gifkite.app"
echo "Packaging $APP bundle..."
mkdir -p "$APP/Contents/MacOS"
mkdir -p "$APP/Contents/Resources"

cp gifkite "$APP/Contents/MacOS/gifkite"
if [ -f "assets/AppIcon.icns" ]; then
    cp assets/AppIcon.icns "$APP/Contents/Resources/AppIcon.icns"
fi

cat << EOF > "$APP/Contents/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>gifkite</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>com.gifkite.app</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>Gifkite</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>$BUNDLE_VERSION</string>
    <key>CFBundleVersion</key>
    <string>$BUNDLE_VERSION</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSSupportsAutomaticGraphicsSwitching</key>
    <true/>
    <key>NSScreenCaptureUsageDescription</key>
    <string>Gifkite requires screen recording permission to capture and record GIFs of your screen.</string>
</dict>
</plist>
EOF

if command -v codesign >/dev/null 2>&1; then
    SIGN_IDENTITY="${DEVELOPER_ID_APPLICATION:-${APPLE_SIGNING_IDENTITY:--}}"
    ENTITLEMENTS_FLAG=""
    if [ -f "entitlements.plist" ]; then
        ENTITLEMENTS_FLAG="--entitlements entitlements.plist"
    fi
    echo "Signing $APP with identity '$SIGN_IDENTITY' and Hardened Runtime..."
    codesign --force --deep --options runtime $ENTITLEMENTS_FLAG --sign "$SIGN_IDENTITY" "$APP"
fi

echo "Done! You can run: open Gifkite.app"
