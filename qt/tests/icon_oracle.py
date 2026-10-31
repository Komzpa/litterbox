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
    icon must be added to the map before it can ship;
(e) every named or conditional QML icon declaration matches its callsite entry,
    including ordered branches, and every mapped callsite still exists.
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


# Lex strings and comments before braces, so QML/JavaScript expressions cannot
# change the enclosing-object stack by containing a brace in a string.
QML_TOKEN = re.compile(
    r'(?P<comment>//[^\n]*|/\*[\s\S]*?\*/)|'
    r'(?P<string>"(?:\\.|[^"\\\n])*"|\'(?:\\.|[^\'\\\n])*\'|`(?:\\.|[^`\\])*`)|'
    r'(?P<identity>\b(?:id\s*:\s*[A-Za-z_]\w*|objectName\s*:\s*"(?:\\.|[^"\\])*"))|'
    r'(?P<assignment>(?<![\w.])(?:icon\.name|source)\s*:\s*[^;\n}]+)|'
    r'(?P<open>\{)|(?P<close>\})'
)


def declarations(path, qt):
    """Return named icon choices with their enclosing QML identity chain."""
    text = path.read_text()
    stack = []
    found = []
    for match in QML_TOKEN.finditer(text):
        kind = match.lastgroup
        token = match.group()
        if kind == "open":
            before = text[max(0, match.start() - 256):match.start()]
            opener = re.search(r'([\w.]+)\s*$', before)
            stack.append({"kind": opener.group(1) if opener else "", "id": None, "object": None})
        elif kind == "close":
            if stack:
                stack.pop()
        elif kind == "identity" and stack:
            name, value = token.split(":", 1)
            stack[-1]["object" if name.strip() == "objectName" else "id"] = value.strip().strip('"')
        elif kind == "assignment":
            property_name, rhs = token.split(":", 1)
            property_name = property_name.strip()
            is_icon_source = property_name == "source" and stack and stack[-1]["kind"] in ("Icon", "Kirigami.Icon")
            if property_name != "icon.name" and not is_icon_source:
                continue
            rhs = COMPARISON.sub("", rhs)
            values = [s for s in re.findall(r'"([^"]*)"', rhs) if not s or ICON_NAME.fullmatch(s)]
            # Renderer bindings forward a caller's choice, rather than declaring one.
            if not values and not re.search(r'["\']', rhs):
                continue
            found.append((list(stack), values, text.count("\n", 0, match.start()) + 1))

    bases = []
    for nodes, values, line in found:
        chain = "/".join(node["id"] or node["object"] for node in nodes if node["id"] or node["object"])
        bases.append(chain)
    result = []
    for ordinal, ((nodes, values, line), chain) in enumerate(zip(found, bases), 1):
        # Unnamed elements and duplicate identity chains need a deterministic suffix.
        if not nodes or not (nodes[-1]["id"] or nodes[-1]["object"]) or bases.count(chain) > 1:
            chain += "#%d" % ordinal
        key = "%s::%s" % (path.relative_to(qt), chain)
        result.append((key, values, line))
    return result


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
    mapping = json.loads(semantic_map.read_text())
    known = set(mapping["known_icons"])
    for name in sorted(set(used) - known):
        failures.append("(d) icon '%s' used at %s is not in %s known_icons" %
                        (name, used[name][0], semantic_map.relative_to(qt) if semantic_map.is_relative_to(qt) else semantic_map))

    # (e) Match the reviewed choices in both directions, including branch order.
    expected = mapping["callsites"]
    callsites_seen = set()
    for path in sources:
        for key, values, number in declarations(path, qt):
            for name in values:
                if name and name not in bundled:
                    failures.append("(a) %s:%d icon '%s' has no bundled SVG" % (path.relative_to(qt), number, name))
                if name and name not in known:
                    failures.append("(d) %s:%d icon '%s' is not in known_icons" % (path.relative_to(qt), number, name))
            if key in callsites_seen:
                failures.append("(e) %s:%d duplicate icon callsite %s" % (path.relative_to(qt), number, key))
            callsites_seen.add(key)
            if key not in expected:
                failures.append("(e) %s:%d unmapped icon callsite %s: %s" % (path.relative_to(qt), number, key, values))
            elif values != expected[key]:
                failures.append("(e) %s:%d icon mismatch at %s: got %s, expected %s" % (path.relative_to(qt), number, key, values, expected[key]))
    for key in sorted(set(expected) - callsites_seen):
        failures.append("(e) mapped icon callsite no longer exists: %s" % key)

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
    entries = mapping["entries"]
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
    print("icon oracle: %d failure(s), %d bundled icons, %d semantic entries, %d callsites" % (len(failures), len(bundled), len(entries), len(callsites_seen)))
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
