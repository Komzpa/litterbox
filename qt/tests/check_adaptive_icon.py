#!/usr/bin/env python3
"""Check the Qt Android launcher's adaptive resources."""
from pathlib import Path
import xml.etree.ElementTree as ET

android = Path(__file__).resolve().parents[1] / "android"
res = android / "res"
ns = "{http://schemas.android.com/apk/res/android}"
app = ET.parse(android / "AndroidManifest.xml").getroot().find("application")
assert app is not None
for attribute in ("icon", "roundIcon"):
    reference = app.get(ns + attribute, "")
    assert reference.startswith("@mipmap/"), (attribute, reference)
    name = reference.removeprefix("@mipmap/")
    icon = ET.parse(res / "mipmap-anydpi-v26" / (name + ".xml")).getroot()
    assert icon.tag == "adaptive-icon", name
    for layer in ("foreground", "background", "monochrome"):
        element = icon.find(layer)
        assert element is not None, (name, layer)
        drawable = element.get(ns + "drawable", "")
        assert drawable.startswith("@"), (layer, drawable)
        kind, resource = drawable[1:].split("/", 1)
        if kind == "color":
            assert any(e.get("name") == resource for f in (res / "values").glob("*.xml") for e in ET.parse(f).getroot().findall("color")), drawable
        else:
            assert kind == "drawable" and (res / kind / (resource + ".xml")).is_file(), drawable
print("Qt manifest adaptive icon and all three layers resolve")
