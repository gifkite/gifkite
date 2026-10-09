#!/usr/bin/env bash
# Gifkite macOS Fast Installer
# Installs Gifkite to /Applications and configures Gatekeeper permissions
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/gifkite/gifkite/main/install.sh | bash

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}==>${NC} Installing ${GREEN}Gifkite${NC} for macOS..."

TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo -e "${BLUE}==>${NC} Fetching latest release info from GitHub..."
RELEASE_JSON=$(curl -sSL https://api.github.com/repos/gifkite/gifkite/releases/latest || true)
TAG_NAME=$(echo "$RELEASE_JSON" | grep '"tag_name":' | head -n1 | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$TAG_NAME" ]; then
    TAG_NAME="v0.1.0"
fi

ZIP_URL="https://github.com/gifkite/gifkite/releases/download/${TAG_NAME}/Gifkite-macOS.zip"

echo -e "${BLUE}==>${NC} Downloading Gifkite (${TAG_NAME})..."
curl -fL --progress-bar "$ZIP_URL" -o "$TMP_DIR/Gifkite-macOS.zip"

echo -e "${BLUE}==>${NC} Extracting application..."
unzip -q -o "$TMP_DIR/Gifkite-macOS.zip" -d "$TMP_DIR"

if [ ! -d "$TMP_DIR/Gifkite.app" ]; then
    echo -e "${YELLOW}Warning:${NC} Could not find Gifkite.app in archive."
    exit 1
fi

echo -e "${BLUE}==>${NC} Moving to /Applications/Gifkite.app..."
# If already running, politely prompt or close
pkill -f "Gifkite.app/Contents/MacOS/gifkite" 2>/dev/null || true
rm -rf /Applications/Gifkite.app
cp -R "$TMP_DIR/Gifkite.app" /Applications/

# Clear macOS quarantine attribute to prevent Gatekeeper untrusted developer block
echo -e "${BLUE}==>${NC} Authorizing application (clearing Gatekeeper quarantine attribute)..."
xattr -cr /Applications/Gifkite.app 2>/dev/null || true

echo -e "${GREEN}==> Gifkite successfully installed to /Applications!${NC}"
echo -e "${BLUE}==>${NC} Launching Gifkite..."
open /Applications/Gifkite.app
