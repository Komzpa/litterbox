#!/usr/bin/env python3
"""Render the actual adaptive icon resources as circle and squircle previews."""
from pathlib import Path
import re
import xml.etree.ElementTree as ET
from PIL import Image, ImageDraw

res = Path(__file__).resolve().parents[1] / "android" / "res"
ns = "{http://schemas.android.com/apk/res/android}"
color = ET.parse(res / "values/launcher_colors.xml").getroot().find("color").text
vector = ET.parse(res / "drawable/launcher_foreground.xml").getroot()
size = 432
image = Image.new("RGBA", (size, size), color)
for path in vector.findall("path"):
    rgba = path.get(ns + "strokeColor").lstrip("#")
    alpha, rgb = int(rgba[:2], 16), rgba[2:]
    coords = [float(n) * 4 for n in re.findall(r"[\d.]+", path.get(ns + "pathData"))]
    width = float(path.get(ns + "strokeWidth")) * 4
    layer = Image.new("RGBA", image.size)
    draw = ImageDraw.Draw(layer)
    draw.line(coords, fill=tuple(bytes.fromhex(rgb)) + (alpha,), width=round(width))
    for x, y in (coords[:2], coords[2:]):
        draw.ellipse((x-width/2, y-width/2, x+width/2, y+width/2), fill=tuple(bytes.fromhex(rgb)) + (alpha,))
    image = Image.alpha_composite(image, layer)
# Android's visible 72dp viewport is the central 288 pixels of the 108dp layers.
image = image.crop((72, 72, 360, 360))
out = Path(__file__).resolve().parents[2] / "receipts" / "adaptive-icon-preview"
out.mkdir(parents=True, exist_ok=True)
for shape, exponent in (("circle", 2), ("squircle", 4)):
    mask = Image.new("L", image.size)
    pixels = mask.load()
    for y in range(image.height):
        for x in range(image.width):
            dx = abs((x + .5) / 144 - 1)
            dy = abs((y + .5) / 144 - 1)
            pixels[x, y] = 255 if dx**exponent + dy**exponent <= 1 else 0
    preview = image.copy()
    preview.putalpha(mask)
    preview.save(out / (shape + ".png"))
    print(out / (shape + ".png"))
