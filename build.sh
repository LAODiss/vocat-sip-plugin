#!/bin/bash
# Build script for vocat-sip-plugin

set -e

PLUGIN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$PLUGIN_DIR/backend"
FRONTEND_DIR="$PLUGIN_DIR/frontend"
ASSETS_DIR="$PLUGIN_DIR/assets"

echo "Building VoCat SIP Plugin..."

# Build backend
echo "Building backend..."
cd "$BACKEND_DIR"
go mod tidy
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o vocat-sip-backend .
# Cross-compile for other architectures
for arch in arm64 386 armv7; do
  echo "Building for linux/$arch..."
  GOOS=linux GOARCH=$arch go build -trimpath -ldflags="-s -w" -o vocat-sip-backend-$arch .
done

# Build frontend
echo "Building frontend..."
cd "$FRONTEND_DIR"
npm ci
npm run build

# Create plugin package
echo "Creating plugin package..."
cd "$PLUGIN_DIR"

# Clean and create dist
rm -rf dist
mkdir -p dist/vocat-sip

# Copy manifest
cp vocat-plugin.json dist/vocat-sip/

# Copy backend binaries
mkdir -p dist/vocat-sip/backend
cp "$BACKEND_DIR"/vocat-sip-backend* dist/vocat-sip/backend/
# Rename for manifest compatibility
cp "$BACKEND_DIR"/vocat-sip-backend dist/vocat-sip/backend/vocat-sip-backend

# Copy frontend assets
cp -r "$ASSETS_DIR"/* dist/vocat-sip/

# Create zip
cd dist
zip -r vocat-sip-plugin.zip vocat-sip/

echo ""
echo "Build complete!"
echo "Plugin package: $PLUGIN_DIR/dist/vocat-sip-plugin.zip"
echo ""
echo "To install in VoCat:"
echo "1. Enable developer mode: vocat develop enable"
echo "2. Go to VoCat Web UI -> Extensions -> Upload Plugin"
echo "3. Select the generated zip file"