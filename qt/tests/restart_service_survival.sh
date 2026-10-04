#!/usr/bin/env bash
# Regression for desktop launches managed as systemd services with
# KillMode=control-group: the upgraded process must leave the old cgroup.
# Usage: DISPLAY=:99 tests/restart_service_survival.sh <litterbox-qt> <workdir>
set -euo pipefail
BINARY=${1:?usage: restart_service_survival.sh <binary> <workdir>}
WORK=${2:?usage: restart_service_survival.sh <binary> <workdir>}
: "${DISPLAY:?run under an owned Xvfb display}"
if pgrep -a litterbox-qt; then
    echo 'Refusing to run while another Litterbox process exists' >&2
    exit 1
fi

UNIT="lb-selfupdate-parent-$$.service"
PREFIX="$WORK/prefix"
PROFILE="$WORK/profile"
mkdir -p "$PREFIX/bin" "$PREFIX/share/applications" "$PROFILE"
cp "$BINARY" "$PREFIX/bin/litterbox-qt"
printf '[Desktop Entry]\nName=Litterbox\n' > "$PREFIX/share/applications/litterbox-qt.desktop"
printf '{"server_url":"http://127.0.0.1:1","token":"restart-test"}\n' > "$PROFILE/profile.json"
export HOME="$WORK/home" XDG_CONFIG_HOME="$WORK/home/.config" XDG_DATA_HOME="$WORK/home/.local/share" XDG_CACHE_HOME="$WORK/home/.cache"
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_CACHE_HOME"
cleanup() {
    systemctl --user stop "$UNIT" >/dev/null 2>&1 || true
    if [[ -n "${NEW_PID:-}" ]]; then kill "$NEW_PID" 2>/dev/null || true; fi
    rm -rf "$WORK"
}
trap cleanup EXIT

systemd-run --user --unit="$UNIT" --no-block --property=KillMode=control-group \
    --setenv="DISPLAY=$DISPLAY" --setenv=QT_QPA_PLATFORM=xcb \
    --setenv="QT_QUICK_CONTROLS_STYLE=org.kde.desktop" --setenv=LIBGL_ALWAYS_SOFTWARE=1 \
    --setenv="QTWEBENGINE_CHROMIUM_FLAGS=--disable-gpu" --setenv="HOME=$HOME" \
    --setenv="XDG_CONFIG_HOME=$XDG_CONFIG_HOME" --setenv="XDG_DATA_HOME=$XDG_DATA_HOME" \
    --setenv="XDG_CACHE_HOME=$XDG_CACHE_HOME" -- \
    "$PREFIX/bin/litterbox-qt" --test-profile "$PROFILE"
OLD_PID=''
for _ in $(seq 1 100); do
    OLD_PID=$(systemctl --user show "$UNIT" -p MainPID --value 2>/dev/null || true)
    [[ "$OLD_PID" =~ ^[1-9][0-9]*$ ]] && break
    sleep 0.1
done
[[ "$OLD_PID" =~ ^[1-9][0-9]*$ ]] || { echo 'service did not start' >&2; exit 1; }
sleep 5
[[ "$(tr '\0' '\n' < "/proc/$OLD_PID/environ" | grep -c '^LB_RESTART_GENERATION=')" == 0 ]]
BEFORE=$(stat -Lc %i "/proc/$OLD_PID/exe")
cp "$PREFIX/bin/litterbox-qt" "$PREFIX/bin/litterbox-qt.dpkg-new"
mv -f "$PREFIX/bin/litterbox-qt.dpkg-new" "$PREFIX/bin/litterbox-qt"
AFTER=$(stat -c %i "$PREFIX/bin/litterbox-qt")
NEW_PID=''
for _ in $(seq 1 30); do
    for pid in $(pgrep -x litterbox-qt || true); do
        [[ "$pid" != "$OLD_PID" ]] && NEW_PID=$pid
    done
    [[ -n "$NEW_PID" ]] && break
    sleep 1
done
[[ -n "$NEW_PID" ]] || { echo "replacement died with parent service; old inode=$BEFORE new inode=$AFTER" >&2; exit 1; }
[[ "$(tr '\0' '\n' < "/proc/$NEW_PID/environ" | grep '^LB_RESTART_GENERATION=')" == LB_RESTART_GENERATION=1 ]]
[[ "$(stat -Lc %i "/proc/$NEW_PID/exe")" == "$AFTER" ]]
for _ in $(seq 1 30); do kill -0 "$NEW_PID"; sleep 1; done
echo "PASS: service pid $OLD_PID replaced by scope pid $NEW_PID, generation=1, inode=$AFTER, survived 30s (old inode=$BEFORE)"
chmod +x /home/kom/litterbox-wt/selfupdate/qt/tests/restart_service_survival.sh
