#!/usr/bin/env bash
# Package dist/Scrubadubber.app into dist/scrubadubber.dmg with a drag-to-
# Applications layout. Run build-app.sh first.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/dist/Scrubadubber.app"
DMG="$ROOT/dist/scrubadubber.dmg"
STAGE="$ROOT/dist/dmg-stage"

[ -d "$APP" ] || { echo "missing $APP - run build-app.sh first" >&2; exit 1; }

rm -rf "$STAGE" "$DMG"
mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"

hdiutil create -volname "Scrubadubber" -srcfolder "$STAGE" -ov -format UDZO "$DMG"
rm -rf "$STAGE"

echo "built $DMG"
