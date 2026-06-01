#!/usr/bin/env bash
# Build Scrubadubber.app as a universal (amd64 + arm64) menubar agent and
# ad-hoc code-sign it. Usage: packaging/macos/build-app.sh [version]
set -euo pipefail

VERSION="${1:-dev}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/dist/Scrubadubber.app"
MACOS="$APP/Contents/MacOS"
RES="$APP/Contents/Resources"
PKG="github.com/salehkreiner/scrubadubber/internal/version.Version"

rm -rf "$APP"
mkdir -p "$MACOS" "$RES"

# systray requires cgo (Cocoa); build each arch then lipo into a fat binary.
for ARCH in amd64 arm64; do
  echo "building darwin/${ARCH}..."
  CGO_ENABLED=1 GOOS=darwin GOARCH="${ARCH}" \
    go build -trimpath -ldflags "-X $PKG=$VERSION" \
    -o "$MACOS/scrubadubber-${ARCH}" ./cmd/scrubadubber
done
lipo -create -output "$MACOS/scrubadubber" "$MACOS/scrubadubber-amd64" "$MACOS/scrubadubber-arm64"
rm -f "$MACOS/scrubadubber-amd64" "$MACOS/scrubadubber-arm64"

sed "s/__VERSION__/$VERSION/g" "$ROOT/packaging/macos/Info.plist" > "$APP/Contents/Info.plist"

# Ad-hoc signature (full Developer ID notarization is a later upgrade).
codesign --force --deep --sign - "$APP"

echo "built $APP"
