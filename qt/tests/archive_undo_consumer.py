#!/usr/bin/env python3
"""Real Qt Archive -> server ACK -> Undo journey for one disposable mail card.

Run only through TestArchiveUndoNativeConsumer (pg_virtualenv, litterbox_test_
database, registered loopback fake Gmail). Private Xvfb/HOME/XDG; no display,
phone or production state is touched. Exits non-zero unless the actual Qt
cache restored the card in order and the neighbour never moved.
"""
import argparse, hashlib, json, os, pathlib, sqlite3, subprocess, sys

p = argparse.ArgumentParser()
p.add_argument("--binary", required=True)
p.add_argument("--output", required=True)
a = p.parse_args()
out = pathlib.Path(a.output).resolve()
url, token, card, neighbor = (os.environ[k] for k in ("LB_UNDO_URL", "LB_UNDO_TOKEN", "LB_UNDO_CARD", "LB_UNDO_NEIGHBOR"))
env = {k: v for k, v in os.environ.items() if k in ("PATH", "USER", "LOGNAME", "LANG", "LC_ALL")}
for name, var in (("home", "HOME"), ("runtime", "XDG_RUNTIME_DIR"), ("config", "XDG_CONFIG_HOME"), ("data", "XDG_DATA_HOME"), ("cache", "XDG_CACHE_HOME")):
    d = out / name
    d.mkdir(mode=0o700, exist_ok=True)
    env[var] = str(d)
env.update(DBUS_SYSTEM_BUS_ADDRESS="unix:path=/nonexistent", QT_QPA_PLATFORM="xcb", QT_QUICK_BACKEND="software", QT_QUICK_CONTROLS_STYLE="org.kde.desktop")
profile = out / "profile"
profile.mkdir(mode=0o700, exist_ok=True)
(profile / "profile.json").write_text(json.dumps({"server_url": url, "token": token}))
(profile / "profile.json").chmod(0o600)
state = "JSON.stringify({at:Date.now(),online:store.online,ids:store.cardIds(),pending:store.pendingOps,undoAvailable:archiveUndoAvailable,token:bundleArchiveToken,message:bundleArchiveStatus})"
guard = "(function(){const ids=store.cardIds();if(!store.online||ids.indexOf('%s')<0||ids.indexOf('%s')<0)throw new Error('PRECONDITION');return JSON.stringify({online:store.online,ids:ids});})()" % (card, neighbor)
script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [412, 915]}, {"wait": 1500}, {"eval": state}, {"eval": guard}, {"shot": "01-before-archive"},
          {"click": "doneButton-" + card}, {"eval": state}, {"shot": "02-after-archive"}, {"wait": 1500}, {"eval": state},
          {"click": "bundleArchiveUndoButton"}, {"eval": state}, {"shot": "03-immediate-undo"}, {"wait": 1800}, {"eval": state},
          {"eval": "store.refresh()"}, {"wait": 1500}, {"eval": state}, {"shot": "04-settled-after-snapshot"}]
import urllib.request, urllib.error
def get(path, bearer=True):
    req = urllib.request.Request(url + path, headers={"Authorization": "Bearer " + token} if bearer else {})
    try:
        with urllib.request.urlopen(req, timeout=5) as r:
            return {"status": r.status, "body": json.load(r)}
    except urllib.error.HTTPError as e:
        return {"status": e.code, "body": e.read().decode()}
(out / "authenticated-before.json").write_text(json.dumps(get("/v1/cards"), indent=1))
(out / "unauthenticated-control.json").write_text(json.dumps(get("/v1/cards", False), indent=1))
(out / "script.json").write_text(json.dumps(script, indent=1))
x = subprocess.Popen(["Xvfb", "-displayfd", "1", "-screen", "0", "800x1100x24", "-nolisten", "tcp"], env=env, stdout=subprocess.PIPE, stderr=(out / "xvfb.log").open("w"), text=True)
try:
    env["DISPLAY"] = ":" + x.stdout.readline().strip()
    cmd = ["dbus-run-session", "--", a.binary, "--capture-script", str(profile), str(out / "script.json"), str(out / "captures")]
    (out / "captures").mkdir(exist_ok=True)
    with (out / "native.log").open("w") as log:
        rc = subprocess.run(cmd, env=env, stdout=log, stderr=subprocess.STDOUT, timeout=120).returncode
    states = [json.loads(l.split(": ", 1)[1]) for l in (out / "native.log").read_text().splitlines() if l.startswith("Capture script eval step") and ': {"at"' in l]
    cache = sqlite3.connect(profile / "cards.sqlite")
    cached = [r[0] for r in cache.execute("SELECT id FROM cards ORDER BY position")]
    outbox = list(cache.execute("SELECT op_id,payload FROM outbox"))
    receipt = {"exit": rc, "binary_sha256": hashlib.sha256(pathlib.Path(a.binary).read_bytes()).hexdigest(), "states": states, "cache_ids": cached, "outbox": outbox,
               "screenshots": {q.name: hashlib.sha256(q.read_bytes()).hexdigest() for q in sorted((out / "captures").glob("*.png"))}}
    (out / "driver-receipt.json").write_text(json.dumps(receipt, indent=1))
    # states: 0 loaded, 1 archived, 2 acknowledged, 3 after Undo click, 4 settled, 5 after snapshot
    ok = rc == 0 and len(states) == 6 and neighbor in states[0]["ids"] and card in states[0]["ids"]
    ok = ok and card not in states[1]["ids"] and states[2]["pending"] == 0 and card in states[3]["ids"] and card in states[5]["ids"]
    ok = ok and states[5]["ids"] == states[0]["ids"] and cached == states[0]["ids"]
    sys.exit(0 if ok else 1)
finally:
    x.terminate()
    x.wait(timeout=10)
