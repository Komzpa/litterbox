#!/usr/bin/env python3
"""Drive the real X11 window while Qt's 1 ms GUI heartbeat measures it.

Start an owned Xvfb with systemd-run --user before calling this script.
"""
import argparse
import os
from pathlib import Path
import subprocess
import threading
import time

parser = argparse.ArgumentParser()
parser.add_argument("--runner", required=True)
parser.add_argument("--display", required=True)
parser.add_argument("--output", type=Path, required=True)
parser.add_argument("--imports", type=Path, help="Use 5fadb81's qml-imports for the negative control")
parser.add_argument("--test", action="append", default=[], help="Run only a named Qt Quick Test function")
args = parser.parse_args()
args.output.mkdir(parents=True, exist_ok=True)
env = dict(os.environ, DISPLAY=args.display, QT_QPA_PLATFORM="xcb",
           QT_QUICK_CONTROLS_STYLE="org.kde.desktop")
here = Path(__file__).resolve().parent
threads = []
thread_errors = []

def xdo(*command):
    return subprocess.check_output(["env", "DISPLAY=" + args.display, "xdotool", *map(str, command)], text=True)

def resize(fixture):
    try:
        wid = xdo("search", "--onlyvisible", "--name", "^Mail resize " + fixture + "$").strip().splitlines()[0]
        start = time.monotonic()
        count = 0
        while time.monotonic() - start < 10:
            # Sweep narrow/wide, rather than just asking twice for the same geometry.
            phase = count % 80
            width = 598 + int((phase if phase < 40 else 80 - phase) * 842 / 40)
            height = 760 + (count % 20) * 6
            xdo("windowsize", wid, width, height)
            count += 1
            time.sleep(0.012)
        if count < 100:
            raise RuntimeError("Fewer than 100 delivered resize requests")
        line = f"RESIZE_REQUESTS {fixture} {count}\n"
        print(line, end="", flush=True)
        log.write(line)
        log.flush()
    except Exception as error:
        thread_errors.append(str(error))

def screenshot(fixture):
    wid = xdo("search", "--onlyvisible", "--name", "^Mail resize " + fixture + "$").strip().splitlines()[0]
    raw = args.output / (fixture + "-after.xwd")
    png = args.output / (fixture + "-after.png")
    subprocess.run(["env", "DISPLAY=" + args.display, "xwd", "-silent", "-id", wid, "-out", str(raw)], check=True)
    subprocess.run(["convert", str(raw), str(png)], check=True)
    raw.unlink()

def record_host_state(phase):
    memory = {key: int(value.split()[0]) for key, value in
              (line.split(":", 1) for line in Path("/proc/meminfo").read_text().splitlines())}
    line = (f"HOST_STATE {phase} loadAverage={os.getloadavg()} "
            f"swapUsedMiB={(memory['SwapTotal'] - memory['SwapFree']) / 1024:.1f} "
            f"memAvailableMiB={memory['MemAvailable'] / 1024:.1f}\n")
    print(line, end="", flush=True)
    log.write(line)
    log.flush()

with (args.output / "run.log").open("w") as log:
    record_host_state("start")
    process = subprocess.Popen([args.runner, "-input", str(here / "mailresize.qml"),
                                "-import", str(args.imports or here / "qml-imports"), *args.test], env=env,
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1)
    for line in process.stdout:
        print(line, end="", flush=True)
        log.write(line)
        log.flush()
        if "RESIZE_READY " in line:
            fixture = line.split("RESIZE_READY ", 1)[1].strip()
            thread = threading.Thread(target=resize, args=(fixture,))
            thread.start()
            threads.append(thread)
        elif "RESIZE_SCREENSHOT " in line:
            for thread in threads:
                thread.join()
            screenshot(line.split("RESIZE_SCREENSHOT ", 1)[1].strip())
    status = process.wait()
    for thread in threads:
        thread.join()
    record_host_state("end")
print("RUNNER_EXIT", status)
if thread_errors:
    raise RuntimeError("; ".join(thread_errors))
raise SystemExit(status)
