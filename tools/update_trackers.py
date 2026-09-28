#!/usr/bin/env python3
"""Regenerate the embedded MailTrackerBlocker tracker snapshot."""
from __future__ import annotations

import json
import re
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
COMMIT = "7463682a39af055a488539a3d0aa7898437b33f5"
BASE = f"https://raw.githubusercontent.com/apparition47/MailTrackerBlocker/{COMMIT}"
SOURCE = "Source/MTBBlockedMessage.m"
OUTPUT = ROOT / "server/internal/mailhtml/trackers/data/trackers.json"


def fetch(path: str) -> str:
    with urllib.request.urlopen(f"{BASE}/{path}", timeout=30) as response:
        return response.read().decode("utf-8")


def objc_string(match: re.Match[str]) -> str:
    return json.loads('"' + match.group(1) + '"')


def parse_patterns(source: str) -> dict[str, list[str]]:
    source = source[source.index("+ (NSDictionary*)getTrackerDict {"):]
    source = re.sub(r'"(?:\\.|[^"\\])*"|//[^\n]*',
                    lambda m: m.group(0) if m.group(0).startswith('"') else "",
                    source)
    result: dict[str, list[str]] = {}
    entry = re.compile(r'@"((?:\\.|[^"\\])*)"\s*:\s*@\[')
    pos = 0
    while match := entry.search(source, pos):
        name = objc_string(match)
        cursor = match.end()
        depth = 1
        in_string = escaped = False
        while cursor < len(source) and depth:
            char = source[cursor]
            if in_string:
                if escaped:
                    escaped = False
                elif char == "\\":
                    escaped = True
                elif char == '"':
                    in_string = False
            elif char == '"':
                in_string = True
            elif char == "[":
                depth += 1
            elif char == "]":
                depth -= 1
            cursor += 1
        if depth:
            raise ValueError(f"unterminated tracker array for {name}")
        values = [objc_string(value) for value in re.finditer(r'@"((?:\\.|[^"\\])*)"', source[match.end():cursor - 1])]
        if not values:
            raise ValueError(f"empty tracker rules for {name}")
        result[name] = values
        pos = cursor
    if len(result) < 100:
        raise ValueError(f"unexpectedly few vendors parsed: {len(result)}")
    return result


def main() -> None:
    data = {
        "source": "https://github.com/apparition47/MailTrackerBlocker",
        "commit": COMMIT,
        "license": "BSD-3-Clause",
        "vendors": parse_patterns(fetch(SOURCE)),
    }
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
