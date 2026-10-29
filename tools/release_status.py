#!/usr/bin/env python3
"""Check that the served Android release is the one this source tree built.

Exits 0 when the working tree's Android versionCode, the served manifest
version, the served APK digest and receipts/published-releases.json agree and no
file under qt/ changed after the published source commit.  Exits 1 otherwise,
printing one line per disagreement.  See AGENTS.md.

Read-only: never restarts the server, never touches /etc/litterbox.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
CMAKE = ROOT / "qt/CMakeLists.txt"
LEDGER = ROOT / "receipts/published-releases.json"
DB_ENV = Path("/etc/litterbox/db.env")


def tree_version() -> tuple[int, str]:
    text = CMAKE.read_text(encoding="utf-8")
    code = re.search(r"QT_ANDROID_VERSION_CODE\s+(\d+)", text)
    name = re.search(r'QT_ANDROID_VERSION_NAME\s+"([^"]+)"', text)
    if not code:
        raise SystemExit("qt/CMakeLists.txt: no QT_ANDROID_VERSION_CODE")
    return int(code.group(1)), name.group(1) if name else ""


def served_config() -> dict[str, str]:
    """Read the live server configuration without requiring root."""
    try:
        text = DB_ENV.read_text(encoding="utf-8")
    except (PermissionError, FileNotFoundError):
        out = subprocess.run(
            ["sudo", "-n", "cat", str(DB_ENV)],
            capture_output=True, text=True, check=False,
        )
        if out.returncode != 0:
            return {}
        text = out.stdout
    values: dict[str, str] = {}
    for line in text.splitlines():
        key, sep, value = line.partition("=")
        if sep and key.startswith("LITTERBOX_ANDROID_"):
            values[key] = value.strip().strip("'\"")
    return values


def sha256(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def git(*args: str) -> str:
    out = subprocess.run(
        ["git", "-C", str(ROOT), *args],
        capture_output=True, text=True, check=False,
    )
    return out.stdout.strip() if out.returncode == 0 else ""


def load_ledger() -> list[dict[str, Any]]:
    if not LEDGER.exists():
        return []
    data = json.loads(LEDGER.read_text(encoding="utf-8"))
    releases = data.get("releases") if isinstance(data, dict) else data
    return [entry for entry in releases or [] if entry.get("versionCode")]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true",
                       help="fail on any disagreement (default: report only)")
    parser.add_argument("--json", action="store_true", help="machine output")
    args = parser.parse_args()

    problems: list[str] = []
    notes: list[str] = []

    tree_code, tree_name = tree_version()
    served = served_config()
    served_version = served.get("LITTERBOX_ANDROID_VERSION", "")
    served_apk = served.get("LITTERBOX_ANDROID_APK_PATH", "")
    releases = load_ledger()

    if not served_version:
        problems.append("served version unknown: /etc/litterbox/db.env unreadable")
    elif int(served_version) != tree_code:
        problems.append(
            f"tree versionCode {tree_code} != served {served_version}: "
            "the built release is not the served one (publish or roll back)")

    if served_apk:
        if not Path(served_apk).exists():
            problems.append(f"served APK missing: {served_apk}")
        else:
            served_sha = sha256(served_apk)
            match = [r for r in releases if int(r["versionCode"]) == int(served_version or 0)]
            if not match:
                problems.append(
                    f"no ledger entry for served versionCode {served_version}")
            elif match[-1].get("sha256") != served_sha:
                problems.append(
                    f"served APK sha256 {served_sha} != ledger {match[-1].get('sha256')}")
            else:
                notes.append(f"served APK {served_version} sha256 matches ledger")

    latest = max(releases, key=lambda r: int(r["versionCode"])) if releases else None
    if latest and latest.get("source"):
        changed = git("log", "--oneline", f"{latest['source']}..HEAD", "--", "qt/")
        if changed:
            count = len(changed.splitlines())
            problems.append(
                f"{count} qt/ commit(s) after published source {latest['source'][:12]}: "
                "unreleased mobile change")
            notes.extend(changed.splitlines()[:5])
    if latest and int(latest["versionCode"]) > tree_code:
        problems.append(
            f"ledger versionCode {latest['versionCode']} > tree {tree_code}")

    if args.json:
        print(json.dumps({
            "tree": {"versionCode": tree_code, "versionName": tree_name},
            "served": {"version": served_version, "apk": served_apk},
            "ledger": latest,
            "problems": problems,
            "notes": notes,
        }, indent=2))
    else:
        for note in notes:
            print(f"ok   {note}")
        for problem in problems:
            print(f"FAIL {problem}")
        if not problems:
            print(f"release status: published {tree_code} ({tree_name}) is served")
    return 1 if (problems and args.check) else 0


if __name__ == "__main__":
    sys.exit(main())
