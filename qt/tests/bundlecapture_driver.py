#!/usr/bin/env python3
"""Capture each viewport in its own real-CardStore/X11 process."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument("--runner", required=True)
parser.add_argument("--imports", type=Path, required=True)
parser.add_argument("--cache", type=Path, required=True)
parser.add_argument("--display", required=True)
parser.add_argument("--output", type=Path, required=True)
args = parser.parse_args()
args.output.mkdir(parents=True, exist_ok=True)
env = dict(os.environ, DISPLAY=args.display, QT_QPA_PLATFORM="xcb",
           QT_QUICK_CONTROLS_STYLE="org.kde.desktop", QT_QUICK_BACKEND="software",
           QT_QUICK_CONTROLS_MATERIAL_THEME="Dark", LB_BUNDLE_CACHE=str(args.cache.resolve()),
           LB_BUNDLE_PROOF=str(args.output.resolve()))


def snapshot(path):
    with sqlite3.connect(Path(path).as_uri() + "?mode=ro", uri=True) as database:
        cards = database.execute("SELECT payload FROM cards ORDER BY position").fetchall()
        outbox = database.execute("SELECT payload FROM outbox ORDER BY seq").fetchall()
    return {"cards": len(cards), "outbox": len(outbox),
            "cards_sha256": hashlib.sha256(json.dumps(cards).encode()).hexdigest(),
            "outbox_sha256": hashlib.sha256(json.dumps(outbox).encode()).hexdigest()}


receipts = []
cancel_receipts = []
source_sha = hashlib.sha256(args.cache.read_bytes()).hexdigest()
statuses = []
with (args.output / "run.log").open("w") as log:
    for width in (520, 598, 1440):
        process = subprocess.Popen([args.runner, "-input", str(Path(__file__).with_name("bundlecapture.qml")),
                                    "-import", str(args.imports)],
                                   env=dict(env, LB_BUNDLE_WIDTH=str(width)), stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT, text=True, bufsize=1)
        database_path = None
        before_cancel = None
        try:
            for line in process.stdout:
                print(line, end="", flush=True)
                log.write(line)
                log.flush()
                if "BUNDLE_CACHE " in line:
                    database_path = json.loads(line.split("BUNDLE_CACHE ", 1)[1])["path"]
                if "BUNDLE_CANCELLED " in line:
                    after_cancel = snapshot(database_path)
                    if before_cancel != after_cancel:
                        raise RuntimeError("Cancel changed the durable cards or outbox")
                    cancel_receipts.append(dict(width=width, before=before_cancel, after=after_cancel,
                                                unchanged=True, database_path=database_path))
                if "BUNDLE_INPUT " not in line:
                    continue
                request = json.loads(line.split("BUNDLE_INPUT ", 1)[1])
                action = request["action"]
                if action == "archive-click":
                    before_cancel = snapshot(database_path)
                search = ["xdotool", "search", "--onlyvisible", "--pid", str(process.pid),
                          "--name", "^Bundle proof " + str(width) + "$"]
                window = subprocess.check_output(search, env=env, text=True).splitlines()[-1]
                focus = ["xdotool", "windowraise", window, "windowfocus", "--sync", window]
                subprocess.run(focus, env=env, check=True)
                move = ["xdotool", "mousemove", "--sync", str(request["x"]), str(request["y"])]
                if action in ("Return", "space"):
                    subprocess.run(move, env=env, check=True)
                    command = ["xdotool", "key", action]
                else:
                    command = move + ["click", "1"]
                result = subprocess.run(command, env=env, check=True)
                receipts.append(dict(request, width=width, pid=process.pid, window=window,
                                     focus_command=focus, command=command, exit_status=result.returncode))
            statuses.append(dict(width=width, exit_status=process.wait()))
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait()

source_unchanged = hashlib.sha256(args.cache.read_bytes()).hexdigest() == source_sha
(args.output / "input-receipts.json").write_text(json.dumps(receipts, indent=2) + "\n")
(args.output / "cancel-receipts.json").write_text(json.dumps({"viewports": cancel_receipts,
    "source_cache_sha256": source_sha, "source_cache_unchanged": source_unchanged,
    "runner_statuses": statuses}, indent=2) + "\n")
print("RUNNER_EXITS", json.dumps(statuses))
if not source_unchanged or len(cancel_receipts) != 3:
    raise SystemExit("Incomplete Cancel proof or modified frozen source")
raise SystemExit(max(status["exit_status"] for status in statuses))
