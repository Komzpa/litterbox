#!/bin/env bash
# Restart-on-upgrade acceptance harness (run on an owned Xvfb display).
#
# Usage: tests/restart_on_upgrade.sh <litterbox-qt-binary> <workdir>
#
# Scenario 1 (installed-style): the app runs from <workdir>/prefix/bin/litterbox-qt
# whose prefix carries share/applications/litterbox-qt.desktop — the layout the
# package installs. The binary is then replaced atomically the way dpkg does
# (write a fresh file, rename it over the path). The running app must restart
# into the new binary within 10 s, exit cleanly itself, keep its outbox and
# card rows and its window geometry, and must not restart again afterwards.
#
# Scenario 2 (build directory): a copy outside a bin/ + share/applications
# layout must do nothing when replaced — the app keeps running unchanged.
#
# This harness fails when run against a binary without the UpdateWatcher
# (e.g. the 0.1-8 build): scenario 1 times out waiting for the restart.
set -euo pipefail

BINARY=${1:?usage: restart_on_upgrade.sh <binary> <workdir>}
WORK=${2:?usage: restart_on_upgrade.sh <binary> <workdir>}
: "${DISPLAY:?DISPLAY must be set; run on an owned Xvfb display}"

RESTART_THRESHOLD_SECONDS=10
CARD_ID="aaaa1111-1111-4111-8111-111111111111"
OUTBOX_OP_ID="22222222-2222-4222-8222-222222222222"

rm -rf "$WORK"
mkdir -p "$WORK"

# --- disposable stub server: serves one card, holds the event stream open and
# refuses operations so the seeded outbox entry stays pending across the
# restart. Port is ephemeral; nothing here touches the production server.
cat > "$WORK/stub_server.py" <<'PY'
import http.server, json, socketserver, sys, time

CARD_ID = sys.argv[2]

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_GET(self):
        if self.path == "/v1/cards":
            body = json.dumps({"now": [{"id": CARD_ID, "title": "Upgrade survives",
                                       "source": "manual"}],
                               "later": [], "missed": []}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        elif self.path == "/v1/cards/events":
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            while True:  # hold the stream open like the real endpoint
                time.sleep(60)
        else:
            self.send_response(404)
            self.end_headers()

    def do_POST(self):
        self.send_response(503)  # keep operations pending in the outbox
        self.end_headers()

class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True

with Server(("127.0.0.1", 0), Handler) as server:
    with open(sys.argv[1], "w") as port_file:
        port_file.write(str(server.server_address[1]))
    server.serve_forever()
PY

python3 "$WORK/stub_server.py" "$WORK/port" "$CARD_ID" &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null || true; kill "${OLD_PID:-}" "${NEW_PID:-}" "${PLAIN_PID:-}" 2>/dev/null || true' EXIT
for _ in $(seq 50); do [ -s "$WORK/port" ] && break; sleep 0.1; done
PORT=$(cat "$WORK/port")
echo "stub server on 127.0.0.1:$PORT (pid $SERVER_PID)"

now_ms() { date +%s%3N; }

wait_for() { # wait_for <timeout-seconds> <command...>
    local deadline=$(( $(now_ms) + $1 * 1000 ))
    while [ "$(now_ms)" -lt "$deadline" ]; do
        if "${@:2}" >/dev/null 2>&1; then return 0; fi
        sleep 0.2
    done
    return 1
}

# Only main application processes matter: match the binary as argv[0] (the
# path alone would also substring-match the cp/mv that stage the dpkg-new
# replacement), and skip Chromium/WebEngine helpers, which run the same
# binary with --type= flags.
app_pids() {
    local pid
    for pid in $(pgrep -f "^$1"'( |$)' || true); do
        if tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null | grep -q -- '--type='; then continue; fi
        echo "$pid"
    done
}

new_pid_other_than() { # new_pid_other_than <binary-path> <old-pid>
    app_pids "$1" | grep -vx "$2" | head -1 || true
}

# Predicate for wait_for: true once a second process runs from the path.
replacement_appeared() { [ -n "$(new_pid_other_than "$1" "$2")" ]; }

setup_profile() { # setup_profile <profile-dir>
    mkdir -p "$1"
    cat > "$1/profile.json" <<EOF
{"server_url": "http://127.0.0.1:$PORT", "token": "e2e-test-token"}
EOF
    # Seed one durable outbox entry before the app starts: it must survive the
    # upgrade restart (the server refuses operations, so it stays pending).
    sqlite3 "$1/cards.sqlite" <<EOF
CREATE TABLE IF NOT EXISTS outbox (seq INTEGER PRIMARY KEY AUTOINCREMENT, op_id TEXT UNIQUE NOT NULL, payload TEXT NOT NULL);
INSERT INTO outbox(op_id, payload) VALUES('$OUTBOX_OP_ID',
  '{"op_id":"$OUTBOX_OP_ID","card_id":"$CARD_ID","type":"note","args":{"note":"survives restart"}}');
EOF
}

# Environment for the app under test, exported once so the launch command is
# the binary itself: a backgrounded wrapper function would make $! a subshell
# pid instead of the application pid.
export QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop \
    LIBGL_ALWAYS_SOFTWARE=1 QTWEBENGINE_CHROMIUM_FLAGS="--disable-gpu" \
    HOME="$WORK/home" XDG_CONFIG_HOME="$WORK/home/.config" \
    XDG_DATA_HOME="$WORK/home/.local/share" XDG_CACHE_HOME="$WORK/home/.cache"

screenshot() { # screenshot <name>; every xwd call carries the display explicitly
    xwd -display "$DISPLAY" -root -out "$WORK/$1.xwd"
    convert "$WORK/$1.xwd" "$WORK/$1.png"
    rm -f "$WORK/$1.xwd"
}

window_geometry() { # window_geometry <wm-class>; prints "WxH+X+Y"
    local window
    window=$(xdotool search --class "$1" 2>/dev/null | head -1) || return 1
    [ -n "$window" ] || return 1
    xdotool getwindowgeometry --shell "$window" | awk -F= '
        /^WIDTH=/ { w = $2 } /^HEIGHT=/ { h = $2 }
        /^X=/ { x = $2 } /^Y=/ { y = $2 }
        END { printf "%dx%d%+d%+d", w, h, x, y }'
}

fail() {
    echo "FAIL: $1"
    screenshot failed || true
    exit 1
}

# ---------------------------------------------------------------------------
echo "== scenario 1: installed-style prefix restarts into the replaced binary =="
PREFIX="$WORK/prefix"
mkdir -p "$PREFIX/bin" "$PREFIX/share/applications"
cp "$BINARY" "$PREFIX/bin/litterbox-qt"
cp "$BINARY" "$WORK/replacement"
printf '\n# LB-RESTART-MARKER\n' >> "$WORK/replacement"
chmod +x "$WORK/replacement"
# The package installs this desktop file; its presence marks an installed
# prefix to the UpdateWatcher.
printf '[Desktop Entry]\nName=Litterbox\n' > "$PREFIX/share/applications/litterbox-qt.desktop"

PROFILE1="$WORK/profile1"
setup_profile "$PROFILE1"
"$PREFIX/bin/litterbox-qt" --test-profile "$PROFILE1" &
OLD_PID=$!
echo "old app pid $OLD_PID"

wait_for 15 sqlite3 "$PROFILE1/cards.sqlite" "SELECT COUNT(*) FROM cards WHERE id='$CARD_ID'" \
    || fail "app did not fetch the stub card within 15 s"
wait_for 5 sh -c "sqlite3 '$PROFILE1/cards.sqlite' 'SELECT COUNT(*) FROM outbox' | grep -qx 1" \
    || fail "seeded outbox entry disappeared before the upgrade"
sleep 1

# Ask the running window for a specific size so geometry persistence is
# observable; the size is what the replacement instance must come back with.
if geometry_before=$(window_geometry litterbox-qt); then
    echo "geometry before: $geometry_before"
fi
GEOMETRY_WINDOW=$(xdotool search --class litterbox-qt 2>/dev/null | head -1 || true)
if [ -n "$GEOMETRY_WINDOW" ]; then
    xdotool windowsize "$GEOMETRY_WINDOW" 777 666
    sleep 1
fi
screenshot before-upgrade

REPLACED_AT=$(now_ms)
# dpkg-style install: stage a fresh file and rename it over the path.
cp "$WORK/replacement" "$PREFIX/bin/litterbox-qt.dpkg-new"
mv -f "$PREFIX/bin/litterbox-qt.dpkg-new" "$PREFIX/bin/litterbox-qt"
echo "binary replaced atomically at $(date +%T)"

wait_for "$RESTART_THRESHOLD_SECONDS" replacement_appeared "$PREFIX/bin/litterbox-qt" "$OLD_PID" \
    || fail "no replacement process within ${RESTART_THRESHOLD_SECONDS}s (watcher did not restart)"
NEW_PID=$(new_pid_other_than "$PREFIX/bin/litterbox-qt" "$OLD_PID")
RESTART_MS=$(( $(now_ms) - REPLACED_AT ))
echo "new app pid $NEW_PID after ${RESTART_MS} ms"

wait_for 10 sh -c "! kill -0 $OLD_PID 2>/dev/null" || fail "old process $OLD_PID still running"
set +e
wait "$OLD_PID"
OLD_STATUS=$?
set -e
[ "$OLD_STATUS" -eq 0 ] || fail "old process exited with $OLD_STATUS, not a clean quit"
echo "old app exited cleanly (status $OLD_STATUS)"

# The new process must be running the replacement bytes, not the old file.
grep -aq "LB-RESTART-MARKER" "/proc/$NEW_PID/exe" \
    || fail "new process $NEW_PID is not running the replaced binary"

# No data loss: the durable outbox entry and the fetched card survive.
OUTBOX_COUNT=$(sqlite3 "$PROFILE1/cards.sqlite" "SELECT COUNT(*) FROM outbox")
CARD_COUNT=$(sqlite3 "$PROFILE1/cards.sqlite" "SELECT COUNT(*) FROM cards WHERE id='$CARD_ID'")
INTEGRITY=$(sqlite3 "$PROFILE1/cards.sqlite" "PRAGMA integrity_check")
[ "$OUTBOX_COUNT" = "1" ] || fail "outbox lost entries: $OUTBOX_COUNT"
[ "$CARD_COUNT" = "1" ] || fail "card row lost: $CARD_COUNT"
[ "$INTEGRITY" = "ok" ] || fail "database integrity: $INTEGRITY"
grep -q "windowGeometry" "$PROFILE1/settings.ini" \
    || fail "window geometry was not persisted on quit"
echo "data intact: outbox=$OUTBOX_COUNT card=$CARD_COUNT integrity=$INTEGRITY"

if [ -n "${GEOMETRY_WINDOW:-}" ]; then
    sleep 2
    geometry_after=$(window_geometry litterbox-qt) || fail "replacement window not found"
    echo "geometry after: $geometry_after"
    case "$geometry_after" in
        777x666*) echo "window geometry restored (777x666)" ;;
        *) fail "window geometry not restored: $geometry_after" ;;
    esac
fi

# Loop guard: an idle watcher must not restart again.
sleep 6
STILL=$(app_pids "$PREFIX/bin/litterbox-qt")
[ "$STILL" = "$NEW_PID" ] || fail "restart loop: pids now [$STILL], expected only $NEW_PID"
echo "no restart loop: still only pid $NEW_PID"
screenshot after-upgrade

kill "$NEW_PID" 2>/dev/null || true
wait_for 5 sh -c "! kill -0 $NEW_PID 2>/dev/null" || true

# ---------------------------------------------------------------------------
echo "== scenario 2: build-directory copy does nothing when replaced =="
mkdir -p "$WORK/build"
cp "$BINARY" "$WORK/build/litterbox-qt"
chmod +x "$WORK/build/litterbox-qt"
PROFILE2="$WORK/profile2"
setup_profile "$PROFILE2"
"$WORK/build/litterbox-qt" --test-profile "$PROFILE2" &
PLAIN_PID=$!
wait_for 15 sqlite3 "$PROFILE2/cards.sqlite" "SELECT COUNT(*) FROM cards WHERE id='$CARD_ID'" \
    || fail "build-directory app did not fetch the stub card"

cp "$WORK/replacement" "$WORK/build/litterbox-qt.dpkg-new"
mv -f "$WORK/build/litterbox-qt.dpkg-new" "$WORK/build/litterbox-qt"
echo "build-directory binary replaced at $(date +%T)"

sleep 6
kill -0 "$PLAIN_PID" 2>/dev/null || fail "build-directory app exited on replacement"
REPLACED_OTHER=$(new_pid_other_than "$WORK/build/litterbox-qt" "$PLAIN_PID")
[ -z "$REPLACED_OTHER" ] || fail "build-directory app spawned a replacement: $REPLACED_OTHER"
echo "build-directory app kept running (pid $PLAIN_PID), no restart"
kill "$PLAIN_PID" 2>/dev/null || true

echo "PASS: installed-style upgrade restarted within ${RESTART_MS} ms (< ${RESTART_THRESHOLD_SECONDS} s),"
echo "      old app quit cleanly with data intact, no restart loop; build-directory run untouched."
