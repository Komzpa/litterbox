#!/usr/bin/env python3
"""Real Qt Archive -> server ACK -> Undo journey for one disposable mail card.

Run only through TestArchiveUndoNativeConsumer (pg_virtualenv, litterbox_test_
database, registered loopback fake Gmail). Private Xvfb/HOME/XDG; no display,
phone or production state is touched. Exits non-zero unless the actual Qt
cache restored the card in order and the neighbour never moved.
"""
import argparse, hashlib, json, os, pathlib, sqlite3, subprocess, sys, threading, time, urllib.error, urllib.request

p = argparse.ArgumentParser()
p.add_argument("--binary", required=True)
p.add_argument("--output", required=True)
p.add_argument("--entry", choices=("toolbar", "row", "bundle", "bundle_pinned", "end", "external", "spam", "done", "snooze"), default="toolbar")
a = p.parse_args()
out = pathlib.Path(a.output).resolve()
body = "<p>B</p>"
url, token, card, neighbor = (os.environ[k] for k in ("LB_UNDO_URL", "LB_UNDO_TOKEN", "LB_UNDO_CARD", "LB_UNDO_NEIGHBOR"))
extra = os.environ.get("LB_UNDO_EXTRA", "")
pinned = os.environ.get("LB_UNDO_PINNED", "")
entry = a.entry
initial_ids = [pinned, card, extra, neighbor] if entry == "bundle_pinned" else [card] + ([extra] if extra else []) + [neighbor]
env = {k: v for k, v in os.environ.items() if k in ("PATH", "USER", "LOGNAME", "LANG", "LC_ALL")}
for name, var in (("home", "HOME"), ("runtime", "XDG_RUNTIME_DIR"), ("config", "XDG_CONFIG_HOME"), ("data", "XDG_DATA_HOME"), ("cache", "XDG_CACHE_HOME")):
    d = out / (out.name + "-" + name)
    d.mkdir(mode=0o700, exist_ok=True)
    env[var] = str(d)
env.update(DBUS_SYSTEM_BUS_ADDRESS="unix:path=/nonexistent", QT_QPA_PLATFORM="xcb", QT_QUICK_BACKEND="software", QT_QUICK_CONTROLS_STYLE="org.kde.desktop")
profile = out.parent / (out.name + "-profile")
profile.mkdir(mode=0o700, exist_ok=True)
(profile / "profile.json").write_text(json.dumps({"server_url": url, "token": token}))
(profile / "profile.json").chmod(0o600)
state = "JSON.stringify({at:Date.now(),online:store.online,ids:store.cardIds(),pending:store.pendingOps,undoAvailable:archiveUndoAvailable,token:bundleArchiveToken,message:bundleArchiveStatus,readerOpen:readerOpen,selected:selectedCardId,depth:stack.depth,body:((stack.currentItem && stack.currentItem.html) ? stack.currentItem.html : ((typeof mailDetailPage !== 'undefined' && mailDetailPage && mailDetailPage.html) ? mailDetailPage.html : ''))})"
guard = "(function(){const ids=store.cardIds();const expected=%s;if(!store.online||JSON.stringify(ids)!==JSON.stringify(expected))throw new Error('PRECONDITION');return JSON.stringify({online:store.online,ids:ids});})()" % json.dumps(initial_ids)
leg = "JSON.stringify({leg:\'%s\',ids:store.cardIds(),pending:store.pendingOps,undoAvailable:archiveUndoAvailable,readerOpen:readerOpen,selected:selectedCardId})"
reject = os.environ.get("LB_UNDO_REJECT") == "1"
external_result = {}
external_worker = None
if reject:
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [412, 915]}, {"wait": 1500}, {"eval": state}, {"eval": guard},
              {"openCard": card}, {"wait": 1500}, {"eval": state},
              {"click": "archiveMail"}, {"wait": 4000}, {"eval": state},
              {"wait": 2000}, {"eval": state}, {"shot": "reject-after-archive"}]
elif entry == "toolbar":
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [412, 915]}, {"wait": 1500}, {"eval": state}, {"eval": guard}, {"shot": "01-before-archive"},
              {"openCard": card}, {"wait": 1500}, {"eval": state},
              {"click": "archiveMail"}, {"wait": 4000}, {"eval": state}, {"shot": "02-reader-successor-B"},
              {"eval": "window.undoBundleArchive()"}, {"eval": state}, {"shot": "03-immediate-undo"}, {"wait": 1800}, {"eval": state},
              {"eval": "store.refresh()"}, {"wait": 1500}, {"eval": state}, {"shot": "04-settled-after-snapshot"},
              {"eval": "if(stack.depth>1)stack.pop()"}, {"wait": 1000},
              {"eval": "store.online=false"}, {"wait": 1500}, {"eval": leg % "offline-start"},
              {"shot": "05-offline-reader"},
              {"click": "doneButton-" + neighbor}, {"wait": 1500}, {"eval": leg % "offline-archived"},
              {"shot": "06-offline-archived"},
              {"click": "bundleArchiveUndoButton"}, {"wait": 500}, {"eval": leg % "offline-undone"},
              {"eval": "store.online=true; store.refresh()"}, {"wait": 1500}, {"eval": leg % "offline-settled"}]
elif entry == "row":
    action = "doneButton-" + card
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [1440, 1000]}, {"wait": 1500}, {"eval": state}, {"eval": guard}, {"shot": entry + "-before"},
              {"openCard": card}, {"wait": 1500}, {"eval": state},
              {"click": action}, {"wait": 4000}, {"eval": state}, {"shot": entry + "-successor-B"},
              {"eval": "window.undoBundleArchive()"}, {"wait": 2000}, {"eval": state}, {"shot": entry + "-undo-keeps-B"},
              {"eval": "store.refresh()"}, {"wait": 1500}, {"eval": state}]
elif entry == "bundle":
    action = "archiveBundle-" + card
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [1440, 1000]}, {"wait": 1500}, {"eval": state}, {"eval": guard}, {"shot": entry + "-before"},
              {"click": "bundleToggle-" + card}, {"wait": 800}, {"eval": state},
              {"openCard": card}, {"wait": 1500}, {"eval": state},
              {"click": action}, {"wait": 4000}, {"eval": state}, {"shot": entry + "-successor-B"},
              {"eval": "window.undoBundleArchive()"}, {"wait": 2000}, {"eval": state}, {"shot": entry + "-undo-keeps-B"},
              {"eval": "store.refresh()"}, {"wait": 1500}, {"eval": state}]
elif entry == "bundle_pinned":
    action = "archiveBundle-" + card
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [1440, 1000]}, {"wait": 1500}, {"eval": state}, {"eval": guard}, {"shot": entry + "-before"},
              {"click": "bundleToggle-" + card}, {"wait": 800}, {"eval": state},
              {"openCard": pinned}, {"wait": 1500}, {"eval": state},
              {"click": action}, {"wait": 4000}, {"eval": state}, {"shot": entry + "-pinned-keeps-P"},
              {"eval": "window.undoBundleArchive()"}, {"wait": 2000}, {"eval": state}, {"shot": entry + "-undo-restores"},
              {"openCard": card}, {"wait": 1500}, {"eval": state},
              {"click": action}, {"wait": 4000}, {"eval": state}, {"shot": entry + "-successor-B"},
              {"eval": "window.undoBundleArchive()"}, {"wait": 2000}, {"eval": state}, {"shot": entry + "-undo-keeps-B"},
              {"eval": "store.refresh()"}, {"wait": 1500}, {"eval": state}]
elif entry == "end":
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [1440, 1000]}, {"wait": 1500}, {"eval": state}, {"eval": guard},
              {"openCard": neighbor}, {"wait": 1500}, {"eval": state}, {"click": "archiveMail"}, {"wait": 4000}, {"eval": state}, {"shot": "end-closes-without-wrap"},
              {"eval": "window.undoBundleArchive()"}, {"wait": 2000}, {"eval": state}, {"shot": "end-undo-stays-in-list"}]
elif entry == "external":
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [1440, 1000]}, {"wait": 1500}, {"eval": state}, {"eval": guard},
              {"openCard": card}, {"wait": 1500}, {"eval": state}, {"eval": "JSON.stringify({externalReady:readerOpen,selected:selectedCardId})"},
              {"shot": "external-before-snapshot"}, {"wait": 1000}, {"eval": "store.refresh()"}, {"wait": 3000}, {"eval": state}, {"shot": "external-no-advance"}]
elif entry in ("spam", "done", "snooze"):
    action = {"spam": "actionRow-spam-", "snooze": "actionRow-snooze-"}.get(entry)
    script = [{"eval": "store.online=true; store.refresh()"}, {"resize": [1440, 1000]}, {"wait": 1500}, {"eval": state}, {"eval": guard},
              {"openCard": card}, {"wait": 1500}, {"eval": state}]
    if entry == "done":
        script.append({"click": "doneButton-" + card})
    else:
        script.extend([{"click": "inboxActions", "index": 0}, {"wait": 300}, {"click": action + card}])
        if entry == "snooze":
            script.extend([{"wait": 300}, {"click": "snoozePreset-hour"}])
    script.extend([{"wait": 4000}, {"eval": state}, {"shot": entry + "-no-advance"}])

def remove_externally_when_reader_is_open(log_path):
    marker = '"externalReady":true,"selected":"' + card + '"'
    deadline = time.monotonic() + 45
    while time.monotonic() < deadline:
        try:
            if marker in log_path.read_text(errors="replace"):
                req = urllib.request.Request(url + "/v1/__test/external-archive/" + card, data=b"", method="POST",
                                            headers={"Authorization": "Bearer " + token})
                with urllib.request.urlopen(req, timeout=5) as response:
                    external_result["status"] = response.status
                return
        except FileNotFoundError:
            pass
        except Exception as exc:
            external_result["error"] = str(exc)
            return
        time.sleep(0.05)
    external_result["error"] = "reader-open marker not observed"

if entry == "external":
    external_worker = threading.Thread(target=remove_externally_when_reader_is_open,
                                       args=(out / "native.log",), daemon=True)
    external_worker.start()
def get(path, bearer=True):
    req = urllib.request.Request(url + path, headers={"Authorization": "Bearer " + token} if bearer else {})
    try:
        with urllib.request.urlopen(req, timeout=5) as r:
            return {"status": r.status, "body": json.load(r)}
    except urllib.error.HTTPError as e:
        return {"status": e.code, "body": e.read().decode()}
(out / "authenticated-before.json").write_text(json.dumps(get("/v1/cards"), indent=1))
(out / "unauthenticated-control.json").write_text(json.dumps(get("/v1/cards", False), indent=1))
(out.parent / (out.name + "-script.json")).write_text(json.dumps(script, indent=1))
x = subprocess.Popen(["Xvfb", "-displayfd", "1", "-screen", "0", "800x1100x24", "-nolisten", "tcp"], env=env, stdout=subprocess.PIPE, stderr=(out / "xvfb.log").open("w"), text=True)
try:
    env["DISPLAY"] = ":" + x.stdout.readline().strip()
    cmd = ["dbus-run-session", "--", a.binary, "--capture-script", str(profile), str(out.parent / (out.name + "-script.json")), str(out.parent / (out.name + "-captures"))]
    (out.parent / (out.name + "-captures")).mkdir(exist_ok=True)
    with (out / "native.log").open("w") as log:
        rc = subprocess.run(cmd, env=env, stdout=log, stderr=subprocess.STDOUT, timeout=120).returncode
    if external_worker:
        external_worker.join(timeout=5)
        if external_worker.is_alive():
            external_result["error"] = "external snapshot trigger did not finish"
    native_log = (out / "native.log").read_text(errors="replace")
    states = [json.loads(l.split(": ", 1)[1]) for l in native_log.splitlines() if l.startswith("Capture script eval step") and ': {"at"' in l]
    cache = sqlite3.connect(profile / "cards.sqlite")
    cached = [r[0] for r in cache.execute("SELECT id FROM cards ORDER BY position")]
    outbox = list(cache.execute("SELECT op_id,payload FROM outbox"))
    receipt = {"entry": entry, "exit": rc, "binary_sha256": hashlib.sha256(pathlib.Path(a.binary).read_bytes()).hexdigest(),
               "states": states, "cache_ids": cached, "outbox": outbox, "external": external_result,
               "screenshots": {q.name: hashlib.sha256(q.read_bytes()).hexdigest() for q in sorted((out.parent / (out.name + "-captures")).glob("*.png"))}}
    legs = [json.loads(l.split(": ", 1)[1]) for l in native_log.splitlines() if l.startswith("Capture script eval step") and '"leg"' in l]
    legmap = {l["leg"]: l for l in legs}
    if reject:
        ok = rc == 0 and len(states) == 4
        ok = ok and states[0]["ids"] == initial_ids and not states[0]["readerOpen"]
        ok = ok and states[1]["readerOpen"] and states[1]["selected"] == card
        ok = ok and any("Action failed" in s["message"] for s in states[2:])
        for s in states[2:]:
            ok = ok and "Archived" not in s["message"] and card in s["ids"]
        ok = ok and not outbox
    elif entry == "toolbar":
        ok = rc == 0 and len(states) == 6 and states[0]["ids"] == initial_ids
        ok = ok and states[1]["readerOpen"] and states[1]["selected"] == card
        ok = ok and states[2]["ids"] == initial_ids[1:] and states[2]["readerOpen"] and states[2]["selected"] == neighbor
        ok = ok and states[3]["readerOpen"] and states[3]["selected"] == neighbor and states[3]["ids"] == initial_ids
        ok = ok and states[4]["pending"] == 0 and states[5]["ids"] == initial_ids and cached == initial_ids
        ok = ok and not outbox and states[5]["body"] != "" and "B" in states[5]["body"]
        ok = ok and len(legs) == 4 and legmap["offline-start"]["ids"] == initial_ids
        ok = ok and legmap["offline-archived"]["pending"] == 1 and neighbor not in legmap["offline-archived"]["ids"]
        ok = ok and legmap["offline-undone"]["pending"] == 0 and legmap["offline-undone"]["ids"] == initial_ids
        ok = ok and legmap["offline-settled"]["ids"] == initial_ids
    elif entry == "row":
        ok = rc == 0 and len(states) == 5 and states[0]["ids"] == initial_ids
        ok = ok and states[1]["readerOpen"] and states[1]["selected"] == card
        ok = ok and states[2]["ids"] == [neighbor] and states[2]["readerOpen"] and states[2]["selected"] == neighbor
        ok = ok and states[2]["body"] != "" and "B" in states[2]["body"]
        ok = ok and states[3]["ids"] == initial_ids and states[3]["readerOpen"] and states[3]["selected"] == neighbor
        ok = ok and states[4]["ids"] == initial_ids and states[4]["readerOpen"] and states[4]["selected"] == neighbor
        ok = ok and states[4]["body"] != "" and "B" in states[4]["body"] and cached == initial_ids and not outbox
    elif entry == "bundle":
        ok = rc == 0 and len(states) == 6 and states[0]["ids"] == initial_ids
        ok = ok and not states[1]["readerOpen"] and states[1]["ids"] == initial_ids
        ok = ok and states[2]["readerOpen"] and states[2]["selected"] == card
        ok = ok and states[3]["ids"] == [neighbor] and states[3]["readerOpen"] and states[3]["selected"] == neighbor
        ok = ok and states[3]["body"] != "" and "B" in states[3]["body"]
        ok = ok and states[4]["ids"] == initial_ids and states[4]["readerOpen"] and states[4]["selected"] == neighbor
        ok = ok and states[5]["ids"] == initial_ids and states[5]["readerOpen"] and states[5]["selected"] == neighbor
        ok = ok and states[5]["body"] != "" and "B" in states[5]["body"] and cached == initial_ids and not outbox
    elif entry == "bundle_pinned":
        ok = rc == 0 and len(states) == 9 and states[0]["ids"] == initial_ids
        ok = ok and not states[1]["readerOpen"] and states[1]["ids"] == initial_ids
        ok = ok and states[2]["readerOpen"] and states[2]["selected"] == pinned
        ok = ok and states[2]["body"] != "" and "P" in states[2]["body"]
        ok = ok and states[3]["ids"] == [pinned, neighbor] and states[3]["readerOpen"] and states[3]["selected"] == pinned
        ok = ok and states[3]["body"] != "" and "P" in states[3]["body"]
        ok = ok and states[4]["ids"] == initial_ids and states[4]["readerOpen"] and states[4]["selected"] == pinned
        ok = ok and states[4]["body"] != "" and "P" in states[4]["body"]
        ok = ok and states[5]["readerOpen"] and states[5]["selected"] == card and states[5]["ids"] == initial_ids
        ok = ok and states[6]["ids"] == [pinned, neighbor] and states[6]["readerOpen"] and states[6]["selected"] == neighbor
        ok = ok and states[6]["body"] != "" and "B" in states[6]["body"]
        ok = ok and states[7]["ids"] == initial_ids and states[7]["readerOpen"] and states[7]["selected"] == neighbor
        ok = ok and states[7]["body"] != "" and "B" in states[7]["body"]
        ok = ok and states[8]["ids"] == initial_ids and states[8]["readerOpen"] and states[8]["selected"] == neighbor
        ok = ok and states[8]["body"] != "" and "B" in states[8]["body"] and cached == initial_ids and not outbox
    elif entry == "end":
        ok = rc == 0 and len(states) == 4 and states[0]["ids"] == initial_ids
        ok = ok and states[1]["readerOpen"] and states[1]["selected"] == neighbor
        ok = ok and neighbor not in states[2]["ids"] and not states[2]["readerOpen"] and states[2]["depth"] == 1
        ok = ok and states[2]["selected"] != card
        ok = ok and states[3]["ids"] == initial_ids and not states[3]["readerOpen"] and states[3]["depth"] == 1
        ok = ok and cached == initial_ids and not outbox
    elif entry == "external":
        ok = rc == 0 and len(states) == 3 and states[0]["ids"] == initial_ids
        ok = ok and states[1]["readerOpen"] and states[1]["selected"] == card
        ok = ok and external_result.get("status") == 204 and not external_result.get("error")
        ok = ok and card not in states[2]["ids"] and not states[2]["readerOpen"] and states[2]["depth"] == 1
        ok = ok and states[2]["selected"] != neighbor and not outbox
    else:
        ok = rc == 0 and len(states) == 3 and states[0]["ids"] == initial_ids
        ok = ok and states[1]["readerOpen"] and states[1]["selected"] == card
        ok = ok and card not in states[2]["ids"] and neighbor in states[2]["ids"]
        ok = ok and not states[2]["readerOpen"] and states[2]["depth"] == 1 and states[2]["selected"] != neighbor
        ok = ok and not outbox
    receipt["reject"] = reject
    receipt["legs"] = legs
    (out / "driver-receipt.json").write_text(json.dumps(receipt, indent=1))
    sys.exit(0 if ok else 1)
finally:
    x.terminate()
    x.wait(timeout=10)
