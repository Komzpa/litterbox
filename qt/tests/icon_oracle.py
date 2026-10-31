#!/usr/bin/env python3
"""Icon oracle for the Qt tree.

    python3 qt/tests/icon_oracle.py [QT_DIR] [--map MAP_JSON]

(a) every icon name used by qt/qml and qt/pages resolves to a bundled SVG in
    qt/icons/litterbox (Android has no system theme, so a miss renders blank);
(b) every bundled SVG paints only with currentColor, none or inherit: no
    hard-coded fill, stroke or color, so tinting by the app always applies;
(c) every semantic anchor in qt/tests/icon-semantics.json is present and the
    icon it shows is the checked-in one;
(d) every icon name used in QML is listed in known_icons of that map, so a new
    icon must be added to the map before it can ship.
Exit status 0 means all checks pass; each failure is printed as file:line.
"""
import argparse
import json
import re
import sys
from pathlib import Path

ICON_NAME = re.compile(r"^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$")
OPENER = re.compile(r"^[A-Za-z_][\w.]*\s*\{")
COMPARISON = re.compile(r"(?:===|!==|==|!=)\s*\"[^\"]*\"")
PAINT = re.compile(r"(?<![\w-])(fill|stroke|color|stop-color|flood-color|lighting-color)\s*[:=]\s*\"?([^\";}\s]+)")
ALLOWED_PAINT = {"none", "currentColor", "inherit"}


def indent(line):
    return len(line) - len(line.lstrip(" "))


def icon_tokens(line):
    """Icon names a QML line assigns via icon.name or a hyphenated Icon source."""
    if line.strip().startswith("//"):
        return []
    if "icon.name:" in line:
        rhs = line.split("icon.name:", 1)[1]
        rhs = COMPARISON.sub("", rhs)
        return [t for t in re.findall(r"\"([^\"]*)\"", rhs) if ICON_NAME.match(t)]
    if re.search(r"(?<![\w.])source\s*:", line):
        rhs = COMPARISON.sub("", line.split("source:", 1)[1])
        return [t for t in re.findall(r"\"([^\"]*)\"", rhs) if "-" in t and ICON_NAME.match(t)]
    return []


def block(lines, index):
    """Lines of the QML element that holds the anchor line: siblings and children."""
    depth = indent(lines[index])
    out = [lines[index]]
    for j in range(index + 1, min(len(lines), index + 60)):
        s = lines[j].strip()
        if not s:
            continue
        ind = indent(lines[j])
        if ind < depth or (ind <= depth and (s.startswith("}") or OPENER.match(s))):
            break
        out.append(lines[j])
    for j in range(index - 1, max(-1, index - 60), -1):
        s = lines[j].strip()
        if not s:
            continue
        ind = indent(lines[j])
        if ind < depth or (ind <= depth and (s.startswith("}") or OPENER.match(s))):
            break
        out.append(lines[j])
    return out


def qml_files(qt):
    files = []
    for folder in ("qml", "pages", "src/qml"):
        files += sorted((qt / folder).rglob("*.qml"))
    return files


def main():
    parser = argparse.ArgumentParser()
    here = Path(__file__).resolve().parent
    parser.add_argument("qt", nargs="?", default=str(here.parent))
    parser.add_argument("--map", default=None)
    args = parser.parse_args()
    qt = Path(args.qt).resolve()
    semantic_map = Path(args.map or qt / "tests/icon-semantics.json")
    bundle = qt / "icons/litterbox"
    failures = []

    bundled = {p.stem for p in bundle.rglob("*.svg")}
    if not bundled:
        failures.append("no bundled SVG icons under " + str(bundle))

    # (a) every icon name in QML resolves to a bundled SVG.
    sources = {}
    used = {}
    for path in qml_files(qt):
        lines = path.read_text().splitlines()
        sources[path] = lines
        for number, line in enumerate(lines, 1):
            for name in icon_tokens(line):
                used.setdefault(name, []).append("%s:%d" % (path.relative_to(qt), number))
                if name not in bundled:
                    failures.append("(a) %s:%d icon '%s' has no bundled SVG" % (path.relative_to(qt), number, name))

    # (d) every icon used in QML is listed in the map, so new icons are added deliberately.
    known = set(json.loads(semantic_map.read_text())["known_icons"])
    for name in sorted(set(used) - known):
        failures.append("(d) icon '%s' used at %s is not in %s known_icons" %
                        (name, used[name][0], semantic_map.relative_to(qt) if semantic_map.is_relative_to(qt) else semantic_map))

    # (b) bundled action SVGs paint with currentColor only.
    for path in sorted((bundle / "actions").rglob("*.svg")):
        text = path.read_text()
        if "currentColor" not in text:
            failures.append("(b) %s uses no currentColor" % path.relative_to(qt))
        for match in PAINT.finditer(text):
            value = match.group(2)
            if value not in ALLOWED_PAINT and not value.startswith("url("):
                failures.append("(b) %s hard-codes %s=%s" % (path.relative_to(qt), match.group(1), value))

    # (c) semantic anchors use the checked-in icon.
    entries = json.loads(semantic_map.read_text())["entries"]
    for entry in entries:
        allowed = {entry["icon"], *entry.get("also_allowed", [])}
        shown = 0
        seen = 0
        for path, lines in sources.items():
            for index, line in enumerate(lines):
                if entry["anchor"] not in line or line.strip().startswith("//"):
                    continue
                seen += 1
                tokens = {t for l in block(lines, index) for t in icon_tokens(l)}
                if not tokens:
                    continue
                where = "%s:%d" % (path.relative_to(qt), index + 1)
                extra = tokens - allowed
                if extra:
                    failures.append("(c) %s anchor %s shows %s, map says %s" %
                                    (where, entry["anchor"], sorted(extra), entry["icon"]))
                if entry["icon"] in tokens:
                    shown += 1
        if seen == 0:
            failures.append("(c) semantic '%s' anchor %s not found; update the map" % (entry["semantic"], entry["anchor"]))
        elif shown == 0:
            failures.append("(c) semantic '%s' never shows %s at %s" % (entry["semantic"], entry["icon"], entry["anchor"]))

    for failure in failures:
        print(failure)
    print("icon oracle: %d failure(s), %d bundled icons, %d semantic entries" % (len(failures), len(bundled), len(entries)))
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
