#!/usr/bin/env bash
# Regression for Plasma desktop services managed with KillMode=control-group.
# Usage: DISPLAY=:99 restart_service_survival.sh <litterbox-qt> <workdir>
set -euo pipefail
BINARY=${1:?usage: restart_service_survival.sh <binary> <workdir>}
WORK=${2:?usage: restart_service_survival.sh <binary> <workdir>}
: "${DISPLAY:?run under an owned Xvfb display}"
if pgrep -a litterbox-qt; then
    echo 'Refusing to run while another Litterbox process exists' >&2
    exit 1
fi

UNIT="app-lb-selfupdate-$$.service"
RESTART_UNIT=''
PREFIX="$WORK/prefix"
PROFILE="$WORK/profile"
mkdir -p "$PREFIX/bin" "$PREFIX/share/applications" "$PROFILE"
cp "$BINARY" "$PREFIX/bin/litterbox-qt"
printf '[Desktop Entry]\nName=Litterbox\n' > "$PREFIX/share/applications/litterbox-qt.desktop"

SERVER_URL=''
TOKEN=''
while IFS= read -r line; do
    case "$line" in
        server_url=*) SERVER_URL=${line#server_url=} ;;
        token=*) TOKEN=${line#token=} ;;
    esac
done < /home/kom/.config/Litterbox/Litterbox.conf
[[ -n "$SERVER_URL" && -n "$TOKEN" ]] || { echo 'Litterbox test profile credentials are unavailable' >&2; exit 1; }
printf '{"server_url":"%s","token":"%s"}\n' "$SERVER_URL" "$TOKEN" > "$PROFILE/profile.json"
export HOME="$WORK/home" XDG_CONFIG_HOME="$WORK/home/.config" XDG_DATA_HOME="$WORK/home/.local/share" XDG_CACHE_HOME="$WORK/home/.cache"
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_CACHE_HOME"
cleanup() {
    systemctl --user stop "$UNIT" >/dev/null 2>&1 || true
    if [[ -n "$RESTART_UNIT" ]]; then systemctl --user stop "$RESTART_UNIT" >/dev/null 2>&1 || true; fi
    rm -rf "$WORK"
}
trap cleanup EXIT

ENV_ARGS=(--setenv="DISPLAY=$DISPLAY" --setenv=QT_QPA_PLATFORM=xcb
    --setenv=QT_QUICK_CONTROLS_STYLE=org.kde.desktop --setenv=LIBGL_ALWAYS_SOFTWARE=1
    --setenv=QTWEBENGINE_CHROMIUM_FLAGS=--disable-gpu --setenv="HOME=$HOME"
    --setenv="PATH=$PATH" --setenv="XDG_CONFIG_HOME=$XDG_CONFIG_HOME"
    --setenv="XDG_DATA_HOME=$XDG_DATA_HOME" --setenv="XDG_CACHE_HOME=$XDG_CACHE_HOME"
    --setenv=LB_RESTART_GENERATION=0)
for name in WAYLAND_DISPLAY XAUTHORITY DBUS_SESSION_BUS_ADDRESS XDG_RUNTIME_DIR; do
    if [[ -n "${!name:-}" ]]; then ENV_ARGS+=("--setenv=$name=${!name}"); fi
done
systemd-run --user --unit="$UNIT" --no-block --property=KillMode=control-group \
    "${ENV_ARGS[@]}" -- "$PREFIX/bin/litterbox-qt" --test-profile "$PROFILE"

OLD_PID=''
for _ in $(seq 1 100); do
    OLD_PID=$(systemctl --user show "$UNIT" -p MainPID --value 2>/dev/null || true)
    [[ "$OLD_PID" =~ ^[1-9][0-9]*$ ]] && break
    sleep 0.1
done
[[ "$OLD_PID" =~ ^[1-9][0-9]*$ ]] || { echo 'service did not start' >&2; exit 1; }
sleep 5
[[ -e "/proc/$OLD_PID/exe" ]] || { echo 'parent service exited before replacement' >&2; exit 1; }
OLD_GENERATION=$(tr '\0' '\n' < "/proc/$OLD_PID/environ" | grep '^LB_RESTART_GENERATION=' | cut -d= -f2 || echo 0)
OLD_GENERATION=${OLD_GENERATION:-0}
EXPECTED_GENERATION=$((OLD_GENERATION + 1))
BEFORE=$(stat -Lc %i "/proc/$OLD_PID/exe")
cp "$PREFIX/bin/litterbox-qt" "$PREFIX/bin/litterbox-qt.dpkg-new"
mv -f "$PREFIX/bin/litterbox-qt.dpkg-new" "$PREFIX/bin/litterbox-qt"
AFTER=$(stat -c %i "$PREFIX/bin/litterbox-qt")
[[ "$BEFORE" != "$AFTER" ]] || { echo 'atomic replacement did not change the binary inode' >&2; exit 1; }
RESTART_UNIT="litterbox-qt-restart-${OLD_PID}-${EXPECTED_GENERATION}.service"

NEW_PID=''
for _ in $(seq 1 30); do
    for pid in $(pgrep -x litterbox-qt || true); do
        [[ "$pid" != "$OLD_PID" ]] && NEW_PID=$pid
    done
    if [[ -n "$NEW_PID" && ! -e "/proc/$OLD_PID/exe" ]]; then break; fi
    sleep 1
done
[[ ! -e "/proc/$OLD_PID/exe" ]] || { echo "old service pid $OLD_PID did not exit" >&2; exit 1; }
[[ -n "$NEW_PID" && -e "/proc/$NEW_PID/exe" ]] || { echo "replacement died with parent service; old pid=$OLD_PID old inode=$BEFORE new inode=$AFTER" >&2; exit 1; }
NEW_GENERATION=$(tr '\0' '\n' < "/proc/$NEW_PID/environ" | grep '^LB_RESTART_GENERATION=' | cut -d= -f2 || true)
[[ "$NEW_GENERATION" == "$EXPECTED_GENERATION" ]] || { echo "expected generation $EXPECTED_GENERATION, got ${NEW_GENERATION:-unset}" >&2; exit 1; }
[[ "$(stat -Lc %i "/proc/$NEW_PID/exe")" == "$AFTER" ]] || { echo 'replacement is not running the installed inode' >&2; exit 1; }
for _ in $(seq 1 10); do kill -0 "$NEW_PID" || { echo "replacement pid $NEW_PID exited before 10 seconds" >&2; exit 1; }; sleep 1; done
printf 'PASS: restart_service_survival old_pid=%s new_pid=%s generation=%s inode=%s survived=10s time=%s\n' \
    "$OLD_PID" "$NEW_PID" "$NEW_GENERATION" "$AFTER" "$(TZ=Asia/Tbilisi date '+%Y-%m-%dT%H:%M:%S%z')"
