#!/usr/bin/env python3
"""Reuse tst_bundleexpand with the installed desktop style and real X11 input."""
import argparse
import json
import os
from pathlib import Path
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
receipts = []
with (args.output / "run.log").open("w") as log:
    process = subprocess.Popen([args.runner, "-input", str(Path(__file__).with_name("bundlecapture.qml")),
                                "-import", str(args.imports)], env=env, stdout=subprocess.PIPE,
                               stderr=subprocess.STDOUT, text=True, bufsize=1)
    width = None
    for line in process.stdout:
        print(line, end="", flush=True)
        log.write(line)
        log.flush()
        if "BUNDLE_CONTRACT " in line:
            width = json.loads(line.split("BUNDLE_CONTRACT ", 1)[1])["width"]
        if "BUNDLE_INPUT " not in line:
            continue
        request = json.loads(line.split("BUNDLE_INPUT ", 1)[1])
        action = request["action"]
        window = subprocess.check_output(["xdotool", "search", "--onlyvisible", "--name", "^Bundle proof " + str(width) + "$"], env=env, text=True).splitlines()[-1]
        subprocess.run(["xdotool", "windowraise", window, "windowfocus", "--sync", window], env=env, check=True)
        command = ["xdotool", "mousemove", "--sync", str(request["x"]), str(request["y"])]
        if action in ("Return", "space"):
            subprocess.run(command, env=env, check=True)
            command = ["xdotool", "key", action]
        else:
            command += ["click", "1"]
        result = subprocess.run(command, env=env, check=True)
        receipts.append(dict(request, command=command, exit_status=result.returncode))
    status = process.wait()
(args.output / "input-receipts.json").write_text(json.dumps(receipts, indent=2) + "\n")
print("RUNNER_EXIT", status)
raise SystemExit(status)
