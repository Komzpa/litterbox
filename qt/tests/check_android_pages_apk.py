#!/usr/bin/env python3
"""Reject build-host page URLs and verify raw Qt page resources in an APK."""
import argparse
import ctypes
import ctypes.util
from pathlib import Path
import re
import sys
import zipfile
import zlib


def resource_payloads(data):
    # Qt rcc stores raw, zlib or zstd-compressed payloads in the native library.
    yield data
    for match in re.finditer(b"\x78[\x01\x5e\x9c\xda]", data):
        try:
            decoder = zlib.decompressobj()
            payload = decoder.decompress(data[match.start():], 2 * 1024 * 1024)
            if decoder.eof:
                yield payload
        except zlib.error:
            pass
    positions = list(re.finditer(b"\x28\xb5\x2f\xfd", data))
    if not positions:
        return
    library = ctypes.util.find_library("zstd")
    if not library:
        raise RuntimeError("libzstd is required to inspect zstd Qt resources")
    zstd = ctypes.CDLL(library)
    zstd.ZSTD_getFrameContentSize.argtypes = [ctypes.c_void_p, ctypes.c_size_t]
    zstd.ZSTD_getFrameContentSize.restype = ctypes.c_ulonglong
    zstd.ZSTD_decompress.argtypes = [ctypes.c_void_p, ctypes.c_size_t, ctypes.c_void_p, ctypes.c_size_t]
    zstd.ZSTD_decompress.restype = ctypes.c_size_t
    zstd.ZSTD_isError.argtypes = [ctypes.c_size_t]
    zstd.ZSTD_isError.restype = ctypes.c_uint
    for match in positions:
        source = data[match.start():]
        size = zstd.ZSTD_getFrameContentSize(source, len(source))
        if not 0 < size <= 2 * 1024 * 1024:
            continue
        destination = ctypes.create_string_buffer(size)
        actual = zstd.ZSTD_decompress(destination, size, source, len(source))
        if not zstd.ZSTD_isError(actual):
            yield destination.raw[:actual]


def check(apk, pages):
    with zipfile.ZipFile(apk) as archive:
        members = [name for name in archive.namelist()
                   if re.fullmatch(r"lib/[^/]+/liblitterbox-qt[^/]*\.so", name)]
        if not members:
            raise RuntimeError("APK has no Litterbox application library")
        expected = sorted(page for page in pages.glob("*.qml")
                          if page.name != "MailBodyDesktop.qml")
        if not expected:
            raise RuntimeError("No source pages found")
        for member in members:
            data = archive.read(member)
            for encoding in ("utf-8", "utf-16-le", "utf-16-be"):
                text = data.decode(encoding, errors="ignore")
                if re.search(r"/home/[^\x00\s\"']*/(?:qt/)?pages(?:/|\x00)", text):
                    raise RuntimeError(f"{member}: absolute /home build-host page path remains")
            prefix = "qrc:/qt/qml/litterbox/qml/pages"
            if not any(prefix.encode(encoding) in data
                       for encoding in ("utf-8", "utf-16-le", "utf-16-be")):
                raise RuntimeError(f"{member}: canonical qrc page URL is missing")
            payloads = list(resource_payloads(data))
            for page in expected:
                if page.name.encode("utf-16-be") not in data:
                    raise RuntimeError(f"{member}: resource name missing: {page.name}")
                if not any(page.read_bytes() in payload for payload in payloads):
                    raise RuntimeError(f"{member}: exact resource payload missing: {page.name}")
    print(f"PASS: {len(expected)} Android page resources, qrc URL, no absolute home page path: {apk}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("apk", type=Path)
    parser.add_argument("pages", type=Path)
    args = parser.parse_args()
    try:
        check(args.apk, args.pages)
    except (RuntimeError, OSError, zipfile.BadZipFile) as error:
        print(f"FAIL: {error}", file=sys.stderr)
        sys.exit(1)
