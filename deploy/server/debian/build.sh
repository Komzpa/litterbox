#!/bin/sh
# Build the litterbox-server .deb from a pinned release binary.
# Usage: deploy/server/debian/build.sh [<binary> [<version> [<outdir>]]]
# Defaults: the pinned frozen-candidate binary, version from debian/control,
# output next to this script's parent (deploy/server/).
set -eu
HERE=$(dirname "$0")
ROOT=$(cd "$HERE/../../.." && pwd)
BIN="${1:-/tmp/lb-release-eNjx0y/litterbox}"
OUTDIR="${3:-$ROOT/deploy/server}"
VER="${2:-$(sed -n 's/^Version: //p' "$HERE/control")}"
ARCH=$(dpkg --print-architecture)
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT
mkdir -p "$STAGE/DEBIAN" "$STAGE/opt/litterbox/server" \
  "$STAGE/lib/systemd/system" "$STAGE/usr/share/litterbox-server/db"
find "$STAGE" -type d -exec chmod 0755 {} +
cp "$BIN" "$STAGE/opt/litterbox/server/litterbox"
chmod 0755 "$STAGE/opt/litterbox/server/litterbox"
cp "$ROOT/deploy/systemd/litterbox-server.service" \
  "$STAGE/lib/systemd/system/litterbox-server.service"
chmod 0644 "$STAGE/lib/systemd/system/litterbox-server.service"
cp "$ROOT/server/db/"*.sql "$STAGE/usr/share/litterbox-server/db/"
chmod 0644 "$STAGE"/usr/share/litterbox-server/db/*.sql
sed "s/^Version: .*/Version: $VER/" "$HERE/control" > "$STAGE/DEBIAN/control"
cp "$HERE/postinst" "$STAGE/DEBIAN/postinst"
chmod 0755 "$STAGE/DEBIAN/postinst"
dpkg-deb --root-owner-group --build "$STAGE" "$OUTDIR/litterbox-server_${VER}_${ARCH}.deb"
echo "built: $OUTDIR/litterbox-server_${VER}_${ARCH}.deb"
sha256sum "$OUTDIR/litterbox-server_${VER}_${ARCH}.deb"
