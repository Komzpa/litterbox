#!/usr/bin/env python3
"""Publish one prebuilt, accepted Litterbox release cohort. No builds or migrations.

    sudo python3 deploy/release.py plan cohort.json
    sudo python3 deploy/release.py apply cohort.json --receipt /var/backups/litterbox/release-<id>
    sudo python3 deploy/release.py rollback /var/backups/litterbox/release-<id>

Manifest: source_sha (full Git SHA); artifacts.server/desktop contain path,
sha256, package, version, architecture, binary_sha256; artifacts.android contains
path, sha256, package, version_code, version_name, signer_sha256. proofs is a list
of {path, sha256, artifacts: [names], accepted: true, bindings: {dotted_key: value}}.
Each artifact needs a receipt binding its hash and source; accepted receipts must
have status PASS/ACCEPTED/accepted. target contains host, service, environment,
server_binary, desktop_binary, unit_path, origins, credential_loader, aapt,
apksigner, publisher_inputs/before_receipt ({path,sha256}), rollback_debs
(server/desktop {path,sha256}), and drop_ins (exact list of {path,sha256}, [] for
none). All paths are absolute. Receipts contain no tokens.

--install-mobile DEVICE_KEY opts into only this app's package upgrade. mobile
binds helper {path,sha256}, registry, lock_path, adb, endpoint, serial, model and
old_apk (the same metadata fields as artifacts.android). The existing PhoneAdb
public identity methods and canonical phone lock are reused; no GUI, permission,
pairing, debug setup, ADB server restart, uninstall or other package is allowed.

The rollback packet is root-private, includes the exact old packages, executable,
all environment sources, unit and declared drop-in bytes/ownership/modes, and
survives partial failure. Rollback is explicit:
Android downgrade/data restoration cannot safely be automatic. Never uninstall.
"""
import argparse
import fcntl
import importlib
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import socket
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile


class ReleaseError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise ReleaseError(message)


def digest(path):
    with open(path, "rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def load(path):
    with open(path) as stream:
        return json.load(stream)


def checked(ref):
    path = Path(ref["path"])
    require(path.is_absolute() and path.is_file(), "missing absolute artifact/receipt path")
    require(digest(path) == ref["sha256"], "artifact/receipt SHA-256 mismatch: " + str(path))
    return path


def run(*args, binary=False):
    result = subprocess.run([str(a) for a in args], capture_output=True, timeout=180)
    # Child output can contain environment or credentials. Never echo it on failure.
    require(result.returncode == 0, "command failed (exit %s): %s" %
            (result.returncode, shlex.join([str(a) for a in args])))
    return result.stdout if binary else result.stdout.decode().strip()


def package(ref, binary_path):
    path = checked(ref)
    fields = run("dpkg-deb", "-f", path, "Package", "Version", "Architecture")
    metadata = dict(line.split(": ", 1) for line in fields.splitlines())
    members = {}
    with tarfile.open(fileobj=io.BytesIO(run("dpkg-deb", "--fsys-tarfile", path, binary=True))) as archive:
        for item in archive:
            name = "/" + item.name.removeprefix("./").lstrip("/")
            require(".." not in Path(name).parts and not name.startswith("/etc/"),
                    "package contains unsafe path or configuration")
            if item.isfile():
                with archive.extractfile(item) as stream:
                    members[name] = hashlib.file_digest(stream, "sha256").hexdigest()
    require(binary_path in members, "package executable missing")
    if "package" in ref:
        for key in ("package", "version", "architecture"):
            require(metadata[key.title()] == ref[key], "Debian " + key + " mismatch")
        require(members[binary_path] == ref["binary_sha256"], "packaged executable mismatch")
    return metadata, members


def apk(ref, target):
    path = checked(ref)
    badging = run(target["aapt"], "dump", "badging", path)
    match = re.search(r"^package: name='([^']+)' versionCode='([0-9]+)' versionName='([^']+)'", badging, re.M)
    require(match is not None, "APK metadata missing")
    package_name, version, name = match.groups()
    signing = run(target["apksigner"], "verify", "--print-certs", path)
    signers = re.findall(r"^Signer #\d+ certificate SHA-256 digest: ([0-9a-f]{64})$", signing, re.M)
    require(signers == [ref["signer_sha256"]], "APK signer mismatch")
    require((package_name, int(version), name) ==
            (ref["package"], int(ref["version_code"]), ref["version_name"]), "APK identity mismatch")
    with zipfile.ZipFile(path) as archive:
        for name in ("libcrypto_3.so", "libssl_3.so", "libplugins_tls_qopensslbackend_arm64-v8a.so"):
            require("lib/arm64-v8a/" + name in archive.namelist(), "APK TLS runtime missing")


def proof_gates(manifest):
    bound, accepted = set(), set()
    for ref in manifest["proofs"]:
        receipt = load(checked(ref))
        for key, expected in ref["bindings"].items():
            value = receipt
            for part in key.split("."):
                value = value[int(part)] if isinstance(value, list) else value[part]
            require(value == expected, "proof binding mismatch: " + key)
        values = list(ref["bindings"].values())
        require(manifest["source_sha"] in values, "proof does not bind frozen source")
        for name in ref["artifacts"]:
            require(manifest["artifacts"][name]["sha256"] in values, "proof does not bind artifact")
            bound.add(name)
            if ref.get("accepted"):
                require(receipt.get("accepted") is not False and
                        (receipt.get("status") in ("PASS", "ACCEPTED", "accepted") or receipt.get("accepted") is True),
                        "proof not accepted")
                accepted.add(name)
    require(bound == accepted == {"server", "desktop", "android"}, "all three accepted identity proofs required")


def service(target):
    fields = run("systemctl", "show", target["service"], "--property=ActiveState,MainPID,ExecStart,EnvironmentFiles,FragmentPath,DropInPaths")
    result = {"EnvironmentFiles": []}
    for line in fields.splitlines():
        key, value = line.split("=", 1)
        if key == "EnvironmentFiles":
            # systemctl show emits one path (ignore_errors=yes/no) per source.
            match = re.fullmatch(r"(.*) \(ignore_errors=(yes|no)\)", value)
            require(match is not None, "invalid systemd EnvironmentFiles entry")
            result[key].append({"path": match[1], "ignore_errors": match[2]})
        else:
            result[key] = value
    require(result["ActiveState"] == "active", "server service is not active")
    require("path=" + target["server_binary"] + " ;" in result["ExecStart"], "service executable mismatch")
    require(any(entry["path"] == target["environment"] for entry in result["EnvironmentFiles"]),
            "service environment mismatch")
    require(result["FragmentPath"] == target["unit_path"], "service unit path mismatch")
    declared = target["drop_ins"]
    require(isinstance(declared, list), "drop_ins must be an explicit list")
    paths = [ref["path"] for ref in declared]
    require(len(paths) == len(set(paths)), "duplicate declared drop-in")
    require(set(shlex.split(result["DropInPaths"])) == set(paths), "service drop-in set mismatch")
    for ref in declared:
        checked(ref)
    return result


def composition(target, state):
    sources = state["EnvironmentFiles"]
    paths = dict.fromkeys([entry["path"] for entry in sources] + [ref["path"] for ref in target["drop_ins"]])
    files = []
    for path in paths:
        source = Path(path)
        require(source.is_absolute(), "service composition path must be absolute")
        if not source.exists():
            require(all(entry["ignore_errors"] == "yes" for entry in sources if entry["path"] == path),
                    "service environment source missing")
            files.append({"path": path, "absent": True})
            continue
        require(source.is_file(), "service composition source must be a file")
        stat = source.stat()
        files.append({"path": path, "sha256": digest(source), "mode": stat.st_mode & 0o7777,
                      "uid": stat.st_uid, "gid": stat.st_gid})
    return {"environment_files": sources, "files": files}


def desktop_processes(target):
    found = []
    for entry in Path("/proc").iterdir():
        if not entry.name.isdecimal():
            continue
        try:
            exe = entry / "exe"
            if os.readlink(exe).removesuffix(" (deleted)") == target["desktop_binary"]:
                # Do not read command lines or environment (may contain credentials).
                found.append({"pid": int(entry.name), "uid": entry.stat().st_uid, "sha256": digest(exe)})
        except (FileNotFoundError, ProcessLookupError, PermissionError):
            continue
    return found


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(origin, path, token=None):
    headers = {"Authorization": "Bearer " + token} if token else {}
    req = urllib.request.Request(origin + path, headers=headers)
    try:
        with urllib.request.build_opener(NoRedirect).open(req, timeout=20) as response:
            return response.status, response.read(256 * 1024 * 1024 + 1)
    except urllib.error.HTTPError as error:
        return error.code, b""
    except (OSError, urllib.error.URLError):
        raise ReleaseError("release endpoint unreachable") from None


def live(target, source, android, token):
    for origin in target["origins"]:
        parsed = urllib.parse.urlsplit(origin)
        require(parsed.scheme in ("http", "https") and parsed.netloc and not parsed.username
                and not parsed.password and not parsed.query and not parsed.fragment and not parsed.path,
                "origin must be a credential-free origin")
        status, body = request(origin, "/v1/version")
        require(status == 200 and json.loads(body)["server_build"] == source, "live server stamp mismatch")
        for path in ("/v1/android/update", "/v1/android/update.apk"):
            require(request(origin, path)[0] == 401, "anonymous release access is not protected")
        status, body = request(origin, "/v1/android/update", token)
        require(status == 200, "authenticated manifest unavailable")
        require(json.loads(body) == {"version": str(android["version_code"]), "package": android["package"],
                                    "sha256": android["sha256"], "download_path": "/v1/android/update.apk"},
                "served APK manifest mismatch")
        status, body = request(origin, "/v1/android/update.apk", token)
        require(status == 200 and hashlib.sha256(body).hexdigest() == android["sha256"], "served APK bytes mismatch")


def installed(package_name):
    text = run("dpkg-query", "-W", "-f=${Status}\n${Version}\n${Architecture}", package_name).splitlines()
    require(len(text) == 3 and text[0] == "install ok installed", "package is not fully installed")
    return text[1:]


def mobile(manifest, device_key, install_path=None):
    spec, candidate = manifest["mobile"], manifest["artifacts"]["android"]
    require(candidate["package"] == "org.qtproject.example.litterbox_qt", "mobile package is not Litterbox")
    helper = checked(spec["helper"])
    require(helper.name == "phone_adb.py", "canonical phone helper required")
    sys.path.insert(0, str(helper.parent))
    module = importlib.import_module("phone_adb")
    PhoneAdb = module.PhoneAdb
    require(Path(spec["registry"]).resolve() == module.DEFAULT_REGISTRY_PATH.resolve(), "noncanonical mobile registry")
    require(Path(spec["lock_path"]).resolve() == module.DEFAULT_LOCK_PATH.resolve(), "noncanonical mobile lock")
    phone = PhoneAdb(registry_path=Path(spec["registry"]), lock_path=Path(spec["lock_path"]),
                     adb_binary=spec["adb"], device_key=device_key)
    registered = phone.registry["devices"][device_key]
    require((registered["expected_serial"], registered["expected_model"]) == (spec["serial"], spec["model"]),
            "mobile registry identity mismatch")
    require(phone.serial == spec["endpoint"], "mobile endpoint differs from verified registry endpoint")
    # The already-running ADB server is a prerequisite, not something to start.
    require(not any(os.environ.get(k) for k in ("ADB_SERVER_SOCKET", "ANDROID_ADB_SERVER_PORT", "ANDROID_SERIAL")),
            "ambient ADB routing is not permitted")
    with socket.create_connection(("127.0.0.1", 5037), timeout=3):
        pass
    normalize = lambda text: re.sub(r"[^a-z0-9]", "", text.lower())
    rows = [row.split() for row in phone.devices().stdout.splitlines() if row.startswith(spec["endpoint"] + " ")
            or row.startswith(spec["endpoint"] + "\t")]
    require(len(rows) == 1 and rows[0][1] == "device" and
            any(part.startswith("model:") and normalize(part[6:]) == normalize(spec["model"]) for part in rows[0]),
            "mobile transport identity mismatch")
    require(phone.getprop("ro.serialno").stdout.strip() == spec["serial"] and
            normalize(phone.getprop("ro.product.model").stdout.strip()) == normalize(spec["model"]),
            "mobile live identity mismatch")
    paths = phone.package_path(candidate["package"]).stdout.splitlines()
    require(len(paths) == 1 and re.fullmatch(r"package:/[A-Za-z0-9_./=+~-]+\.apk", paths[0]), "mobile package path ambiguous")
    apk(spec["old_apk"], manifest["target"])
    require(spec["old_apk"]["package"] == candidate["package"] and
            spec["old_apk"]["signer_sha256"] == candidate["signer_sha256"] and
            int(spec["old_apk"]["version_code"]) < int(candidate["version_code"]), "mobile upgrade lineage mismatch")
    adb = [spec["adb"], "-s", spec["endpoint"]]
    # Lock the same inode as PhoneAdb, without calling its private wrappers or
    # broadening its capture-only API. Recheck identity while holding the lock.
    with phone.lock_path.open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        require(run(*adb, "shell", "getprop", "ro.serialno") == spec["serial"] and
                normalize(run(*adb, "shell", "getprop", "ro.product.model")) == normalize(spec["model"]), "mobile identity changed")
        require(run(*adb, "shell", "sha256sum", paths[0][8:]).split()[0] == spec["old_apk"]["sha256"],
                "mobile installed APK differs from rollback artifact")
        expected = spec["old_apk"]
        if install_path:
            require(digest(install_path) == candidate["sha256"], "mobile staged APK changed")
            run(*adb, "install", "-r", install_path)
            expected = candidate
        installed_path = run(*adb, "shell", "pm", "path", candidate["package"])
        require(re.fullmatch(r"package:/[A-Za-z0-9_./=+~-]+\.apk", installed_path), "installed APK path ambiguous")
        require(run(*adb, "shell", "sha256sum", installed_path[8:]).split()[0] == expected["sha256"], "installed mobile hash mismatch")
        require(run(*adb, "shell", "getprop", "ro.serialno") == spec["serial"] and
                normalize(run(*adb, "shell", "getprop", "ro.product.model")) == normalize(spec["model"]),
                "mobile identity changed during readback")
        metadata = run(*adb, "shell", "dumpsys", "package", candidate["package"])
        require(re.search(r"\bversionCode=" + str(expected["version_code"]) + r"\b", metadata), "installed mobile version mismatch")
    return {"status": "installed_verified" if install_path else "validated", "serial": spec["serial"],
            "model": spec["model"], "sha256": expected["sha256"], "version_code": expected["version_code"]}


def validate(manifest, args):
    require(re.fullmatch(r"[0-9a-f]{40}", manifest["source_sha"]), "full frozen source SHA required")
    target, artifacts = manifest["target"], manifest["artifacts"]
    require(set(artifacts) == {"server", "desktop", "android"}, "exactly three release artifacts required")
    for ref in artifacts.values():
        require(ref["source_sha"] == manifest["source_sha"], "artifact source mismatch")
    require(socket.gethostname() == target["host"], "wrong release host")
    require(target["service"] == "litterbox-server.service", "wrong service")
    for key, expected in (("server_binary", "/opt/litterbox/server/litterbox"),
                          ("desktop_binary", "/usr/bin/litterbox-qt"), ("environment", "/etc/litterbox/db.env")):
        require(target[key] == expected, "noncanonical " + key)
    inputs = load(checked(target["publisher_inputs"]))
    before = load(checked(target["before_receipt"]))
    for key, input_key in (("host", "host"), ("service", "service"), ("environment", "environment_path"),
                           ("server_binary", "server_binary"), ("credential_loader", "credential_loader"), ("origins", "origins")):
        require(target[key] == inputs[input_key], "publisher target mismatch: " + key)
    require(manifest["source_sha"] == inputs["candidate_source"], "publisher source mismatch")
    candidate = artifacts["android"]
    for key, input_key in (("path", "candidate_apk"), ("sha256", "candidate_sha256"),
                           ("package", "candidate_package"), ("signer_sha256", "candidate_signer")):
        require(candidate[key] == inputs[input_key], "publisher APK mismatch: " + key)
    require(str(candidate["version_code"]) == inputs["candidate_version"], "publisher APK version mismatch")
    proof_gates(manifest)
    apk(candidate, target)
    require(candidate["signer_sha256"] == before["signer_sha256"], "signing lineage changed")
    require(int(candidate["version_code"]) > int(before["manifest"]["version"]), "APK must be strictly newer")
    require(candidate["package"] == before["manifest"]["package"], "Android package changed")
    require(digest(target["environment"]) == inputs["environment_sha256"], "environment changed since receipt")
    require(digest(target["server_binary"]) == inputs["server_binary_sha256"], "server changed since receipt")
    old_apk = inputs["rollback_apk_preserve"]
    require(digest(old_apk) == before["apk_sha256"], "old APK unavailable for rollback")
    require(inputs["environment_release_values"]["LITTERBOX_ANDROID_APK_PATH"] == old_apk, "rollback APK path mismatch")
    env = Path(target["environment"]).read_text()
    for key, value in inputs["environment_release_values"].items():
        lines = re.findall(r"^" + key + r"=(.*)$", env, re.M)
        require(len(lines) == 1 and shlex.split(lines[0]) == [value], "environment release binding mismatch")
    for name, binary in (("server", target["server_binary"]), ("desktop", target["desktop_binary"])):
        ref = artifacts[name]
        require(ref["package"] == "litterbox-" + ("server" if name == "server" else "qt"), "wrong package name")
        _, members = package(ref, binary)
        old_meta, old_members = package(target["rollback_debs"][name], binary)
        require(old_meta["Package"] == ref["package"], "wrong rollback package")
        require(installed(ref["package"]) == [old_meta["Version"], old_meta["Architecture"]], "rollback package version mismatch")
        require(digest(binary) == old_members[binary], "rollback package executable mismatch")
        if name == "server":
            require("/lib/systemd/system/litterbox-server.service" in members, "server unit absent from package")
            # Read Go's existing stamp; desktop provenance is bound by build receipt,
            # never by injecting a new custom ELF note or stamping the current HEAD.
            with tempfile.TemporaryDirectory(prefix="litterbox-stamp-") as temporary:
                run("dpkg-deb", "-x", ref["path"], temporary)
                stamp = run("go", "version", "-m", str(Path(temporary) / binary.lstrip("/")))
                require("main.buildSHA=" + manifest["source_sha"] in stamp, "server build stamp mismatch")
                require((Path(temporary) / "lib/systemd/system/litterbox-server.service").read_bytes() ==
                        Path(target["unit_path"]).read_bytes(), "server unit change requires separate review")
            with tempfile.TemporaryDirectory(prefix="litterbox-control-") as temporary:
                run("dpkg-deb", "-e", ref["path"], temporary)
                canonical = Path(__file__).parent / "server/debian/postinst"
                require((Path(temporary) / "postinst").read_bytes() == canonical.read_bytes(), "noncanonical server postinst")
                require(not any((Path(temporary) / p).exists() for p in ("preinst", "prerm", "postrm")), "unexpected server maintainer script")
    state = service(target)
    require(digest("/proc/" + state["MainPID"] + "/exe") == inputs["server_binary_sha256"], "live server executable mismatch")
    current_composition = composition(target, state)
    staged = Path(inputs["proposed_immutable_apk_path"])
    require(staged.parent == Path(target["server_binary"]).parent and
            staged.name == "litterbox-qt-v%s-%s.apk" % (candidate["version_code"], candidate["sha256"][:8]), "APK immutable path mismatch")
    require(not staged.exists() or digest(staged) == candidate["sha256"], "immutable APK path collision")
    token = load(target["credential_loader"])["token"]
    require(isinstance(token, str) and token, "missing enrolled device token")
    old_android = dict(version_code=before["manifest"]["version"], package=before["manifest"]["package"], sha256=before["apk_sha256"])
    live(target, inputs["server_source"], old_android, token)
    run("dpkg", "--no-act", "--install", artifacts["server"]["path"], artifacts["desktop"]["path"])
    if args.install_mobile:
        mobile(manifest, args.install_mobile)
    return inputs, before, token, desktop_processes(target), current_composition


def atomic_bytes(path, data, mode=None):
    path = Path(path)
    previous = path.stat() if path.exists() else None
    fd, temporary = tempfile.mkstemp(prefix="." + path.name + ".", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            if previous:
                os.fchown(stream.fileno(), previous.st_uid, previous.st_gid)
            os.fchmod(stream.fileno(), mode if mode is not None else previous.st_mode & 0o7777)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def save(path, value):
    atomic_bytes(path, (json.dumps(value, indent=2) + "\n").encode(), 0o600)


def verify_installed(manifest, token, prior, timeout, expected_composition):
    target, artifacts = manifest["target"], manifest["artifacts"]
    for name, binary in (("server", target["server_binary"]), ("desktop", target["desktop_binary"])):
        ref = artifacts[name]
        require(installed(ref["package"]) == [ref["version"], ref["architecture"]], "installed package identity mismatch")
        require(digest(binary) == ref["binary_sha256"], "installed executable mismatch")
    state = service(target)
    require(composition(target, state) == expected_composition, "service composition changed")
    require(digest("/proc/" + state["MainPID"] + "/exe") == artifacts["server"]["binary_sha256"], "server did not adopt release")
    live(target, manifest["source_sha"], artifacts["android"], token)
    if not prior:
        return {"status": "not_running_before_release", "processes": []}
    deadline = time.monotonic() + timeout
    while True:
        current = desktop_processes(target)
        # All previously running owners must be represented; old mappings must go.
        if {p["uid"] for p in prior} <= {p["uid"] for p in current} and all(
                p["sha256"] == artifacts["desktop"]["binary_sha256"] for p in current):
            return {"status": "adopted", "processes": current}
        require(time.monotonic() < deadline, "desktop automatic adoption not observed; no GUI restart attempted")
        time.sleep(1)


def apply(manifest, args, validated):
    inputs, before, token, prior, current_composition = validated
    require(os.geteuid() == 0, "apply requires root")
    root = Path(args.receipt)
    require(root.is_absolute() and not root.exists() and root.parent.is_dir(), "receipt must be a new absolute directory with existing parent")
    target, artifacts = manifest["target"], manifest["artifacts"]
    root.mkdir(mode=0o700)
    receipt = {"status": "preparing", "source_sha": manifest["source_sha"], "artifacts": {name: "validated" for name in artifacts},
               "mobile": "not_requested", "rollback_command": shlex.join([sys.executable, str(Path(__file__).resolve()), "rollback", str(root)])}
    backup = {"host": target["host"], "target": target, "files": [], "packages": [], "before": before,
              "server_source": inputs["server_source"], "desktop_processes": prior, "adoption_timeout": args.adoption_timeout,
              "composition": current_composition}
    try:
        # Durable exact backups and copied artifacts precede all installation.
        paths = dict.fromkeys([target["environment"], target["unit_path"], target["server_binary"], target["desktop_binary"],
                              inputs["rollback_apk_preserve"]] +
                             [ref["path"] for ref in current_composition["files"] if not ref.get("absent")])
        for index, path in enumerate(paths):
            source = Path(path)
            dest = root / ("old-file-%d" % index)
            shutil.copy2(source, dest)
            stat = source.stat()
            backup["files"].append({"path": path, "backup": str(dest), "sha256": digest(dest), "mode": stat.st_mode & 0o7777,
                                    "uid": stat.st_uid, "gid": stat.st_gid})
        for name in ("server", "desktop"):
            dest = root / ("old-" + name + ".deb")
            shutil.copy2(checked(target["rollback_debs"][name]), dest)
            backup["packages"].append({"path": str(dest), "sha256": digest(dest)})
        if args.install_mobile:
            dest = root / "old-mobile.apk"
            shutil.copy2(checked(manifest["mobile"]["old_apk"]), dest)
            backup["mobile"] = {"device": args.install_mobile, "old_apk": {"path": str(dest), "sha256": digest(dest)},
                                "restoration": "No automatic Android downgrade or uninstall; preserve app data and review separately."}
        save(root / "rollback.json", backup)
        save(root / "cohort.json", manifest)
        save(root / "receipt.json", receipt)
        require(composition(target, service(target)) == current_composition, "service composition changed before installation")
        for name, ref in artifacts.items():
            dest = root / ("new-" + name + (".apk" if name == "android" else ".deb"))
            shutil.copy2(checked(ref), dest)
            require(digest(dest) == ref["sha256"], "staged artifact changed")
        apk_path = Path(inputs["proposed_immutable_apk_path"])
        if not apk_path.exists():
            # Exclusive creation: an existing immutable release is never overwritten.
            with apk_path.open("xb") as stream:
                stream.write((root / "new-android.apk").read_bytes())
                os.fchmod(stream.fileno(), 0o644)
                stream.flush()
                os.fsync(stream.fileno())
        require(digest(apk_path) == artifacts["android"]["sha256"], "staged APK mismatch")
        receipt["artifacts"]["android"] = "staged"
        receipt["artifacts"]["server"] = receipt["artifacts"]["desktop"] = "installation_attempted"
        save(root / "receipt.json", receipt)
        run("dpkg", "--install", root / "new-server.deb", root / "new-desktop.deb")
        receipt["artifacts"]["server"] = receipt["artifacts"]["desktop"] = "installed_pending_readback"
        values = {"LITTERBOX_ANDROID_APK_PATH": str(apk_path), "LITTERBOX_ANDROID_VERSION": str(artifacts["android"]["version_code"]),
                  "LITTERBOX_ANDROID_PACKAGE": artifacts["android"]["package"]}
        original = Path(target["environment"]).read_bytes()
        require(hashlib.sha256(original).hexdigest() == inputs["environment_sha256"], "environment changed during installation")
        updated = original
        for key, value in values.items():
            require(re.fullmatch(r"[A-Za-z0-9_./-]+", value), "unsafe release environment value")
            updated, count = re.subn(rb"(?m)^" + key.encode() + rb"=[^\r\n]*", key.encode() + b"=" + value.encode(), updated)
            require(count == 1, "release key missing or duplicated")
        atomic_bytes(target["environment"], updated)
        run("systemctl", "restart", target["service"])
        expected_composition = dict(current_composition, files=[dict(ref) for ref in current_composition["files"]])
        for ref in expected_composition["files"]:
            if ref["path"] == target["environment"]:
                ref["sha256"] = hashlib.sha256(updated).hexdigest()
        receipt["desktop_adoption"] = verify_installed(manifest, token, prior, args.adoption_timeout, expected_composition)
        receipt["artifacts"] = {"server": "installed_verified", "desktop": "installed_verified", "android": "published_verified"}
        if args.install_mobile:
            receipt["mobile"] = "installation_attempted"
            save(root / "receipt.json", receipt)
            receipt["mobile"] = mobile(manifest, args.install_mobile, root / "new-android.apk")
        receipt["status"] = "success"
    except (Exception, KeyboardInterrupt) as error:
        receipt["status"] = "failed"
        receipt["error"] = str(error) if isinstance(error, ReleaseError) else type(error).__name__
        receipt["restoration"] = "Explicit rollback required; never downgrade/uninstall Android automatically. Exact packet: " + str(root / "rollback.json")
        raise
    finally:
        save(root / "receipt.json", receipt)
        print(json.dumps(receipt, indent=2))


def rollback(root):
    require(os.geteuid() == 0, "rollback requires root")
    root = Path(root)
    packet = load(root / "rollback.json")
    require(socket.gethostname() == packet["host"], "wrong rollback host")
    for ref in packet["packages"]:
        checked(ref)
    for ref in packet["files"]:
        checked({"path": ref["backup"], "sha256": ref["sha256"]})
    output = root / ("rollback-%d.json" % time.time_ns())
    restored = {"source_sha": packet["server_source"], "target": packet["target"], "artifacts": {}}
    for name, ref in zip(("server", "desktop"), packet["packages"]):
        binary = packet["target"][name + "_binary"]
        metadata, members = package(ref, binary)
        restored["artifacts"][name] = {"package": metadata["Package"], "version": metadata["Version"],
                                       "architecture": metadata["Architecture"], "binary_sha256": members[binary]}
    result = {"status": "failed", "mobile": "not_modified; any installed upgrade needs separately reviewed data-safe restoration"}
    try:
        run("dpkg", "--install", *(ref["path"] for ref in packet["packages"]))
        for ref in packet["files"]:
            atomic_bytes(ref["path"], Path(ref["backup"]).read_bytes(), ref["mode"])
            os.chown(ref["path"], ref["uid"], ref["gid"])
            os.chmod(ref["path"], ref["mode"])
        run("systemctl", "daemon-reload")
        run("systemctl", "restart", packet["target"]["service"])
        for ref in packet["files"]:
            require(digest(ref["path"]) == ref["sha256"], "restored file mismatch")
            stat = Path(ref["path"]).stat()
            require((stat.st_mode & 0o7777, stat.st_uid, stat.st_gid) == (ref["mode"], ref["uid"], ref["gid"]),
                    "restored file metadata mismatch")
        before = packet["before"]
        android = {"version_code": before["manifest"]["version"], "package": before["manifest"]["package"], "sha256": before["apk_sha256"]}
        restored["artifacts"]["android"] = android
        result["desktop_adoption"] = verify_installed(restored, load(packet["target"]["credential_loader"])["token"],
                                                     packet["desktop_processes"], packet["adoption_timeout"], packet["composition"])
        result["status"] = "server_desktop_publication_restored"
    finally:
        save(output, result)
        print(json.dumps(result))


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("action", choices=("plan", "apply", "rollback"))
    parser.add_argument("manifest", help="cohort JSON, or rollback receipt directory")
    parser.add_argument("--receipt", help="new private backup/receipt directory (apply only)")
    parser.add_argument("--install-mobile", metavar="DEVICE_KEY", help="explicit registry device; install only identity-verified Litterbox")
    parser.add_argument("--adoption-timeout", type=int, default=90)
    args = parser.parse_args()
    try:
        if args.action == "rollback":
            rollback(args.manifest)
            return 0
        require(args.action != "apply" or args.receipt, "apply requires --receipt")
        manifest = load(args.manifest)
        validated = validate(manifest, args)
        if args.action == "plan":
            print(json.dumps({"status": "validated_dry_run", "source_sha": manifest["source_sha"],
                              "steps": ["save old debs, binaries, all environment sources, unit, declared drop-ins and APK to private rollback packet",
                                        "stage all three hash-checked artifacts; install both debs with dpkg",
                                        "atomically replace only three Android environment keys; restart server once",
                                        "read all origin manifests/downloads, executable hashes and desktop adoption",
                                        "install and read back explicit verified mobile" if args.install_mobile else "phone untouched; no installed-phone claim"],
                              "artifacts": {name: {k: v for k, v in ref.items() if k in ("sha256", "version", "version_code", "package")}
                                            for name, ref in manifest["artifacts"].items()}}, indent=2))
        else:
            apply(manifest, args, validated)
        return 0
    except (Exception, KeyboardInterrupt) as error:
        print("release failed: " + (str(error) if isinstance(error, ReleaseError) else type(error).__name__), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
