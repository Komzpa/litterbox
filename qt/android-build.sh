#!/usr/bin/env bash
set -euo pipefail

QT_ANDROID_ROOT=${QT_ANDROID_ROOT:-"$HOME/Qt/6.7.3/android_arm64_v8a"}
QT_HOST_ROOT=${QT_HOST_ROOT:-"$HOME/6.7.3/gcc_64"}
ANDROID_SDK_SOURCE=${ANDROID_SDK_SOURCE:-/usr/lib/android-sdk}
ANDROID_NDK_ROOT=${ANDROID_NDK_ROOT:-"$ANDROID_SDK_SOURCE/ndk/29.0.14206865"}
ANDROID_SDK_ROOT=${ANDROID_SDK_ROOT:-"$HOME/.local/share/litterbox/android-sdk-qt6"}
KIRIGAMI_ROOT=${KIRIGAMI_ROOT:-"$HOME/Qt/kirigami-android"}
ANDROID_OPENSSL_ROOT=${ANDROID_OPENSSL_ROOT:-}
ANDROID_API=${ANDROID_API:-34}
ANDROID_BUILD_TOOLS=${ANDROID_BUILD_TOOLS:-35.0.1}
APP_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
BUILD_DIR=${BUILD_DIR:-"$APP_ROOT/build/android-arm64"}
KEYSTORE=${KEYSTORE:-"$HOME/.local/share/litterbox/android-debug.keystore"}
DEVICE=${ANDROID_SERIAL:-R5CX80MZ1SN}
ADB=${ADB:-adb}
check_qml_apk_imports() {
    local build_dir=$1 apk=$2
    python3 - "$build_dir" "$apk" <<'PY'
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET
import zipfile

build_dir, apk = map(Path, sys.argv[1:])
build_dir = build_dir.resolve()
target = "litterbox-qt"

def fail(message):
    raise SystemExit(f"QML import check failed: {message}")

def run(*args):
    return subprocess.check_output(args, text=True)

# Qt 6.7's configure scanner unconditionally adds TARGET SOURCE_DIR as
# -rootPath (Qt6QmlMacros.cmake). That recursively includes tests, desktop
# branches and foreign build trees. Re-scan the *same target's* generated
# resource manifests instead; do not filter failed records by module name.
rsp = build_dir / f".qt/qml_imports/{target}_conf.rsp"
if not rsp.is_file():
    fail(f"missing production scanner response file: {rsp}")
args = rsp.read_text().splitlines()
if "-qrcFiles" not in args:
    fail("production target has no resource manifest inputs")
qrcs = [Path(p) for p in args[args.index("-qrcFiles") + 1:]]
imports = [args[i + 1] for i, arg in enumerate(args) if arg == "-importPath"]
cache = (build_dir / "CMakeCache.txt").read_text()
host = re.search(r"^QT_HOST_PATH:[^=]*=(.+)$", cache, re.M)
if not host:
    fail("QT_HOST_PATH missing from Android build cache")
scanner = Path(host[1]) / "libexec/qmlimportscanner"

# Use CMake's production link command, not source-tree guesses or test
# targets. Fail closed for build generators without this evidence.
link_file = build_dir / f"CMakeFiles/{target}.dir/link.txt"
if not link_file.is_file():
    fail(f"missing production link evidence: {link_file} (use Unix Makefiles)")
link = shlex.split(link_file.read_text())
library = build_dir / link[link.index("-o") + 1]
objects = [build_dir / word for word in link if word.endswith(".o")]
tool_dir = Path(link[0]).parent
nm = tool_dir / "llvm-nm"
readelf = tool_dir / "llvm-readelf"

def build_id(path):
    match = re.search(r"Build ID: (\w+)", run(str(readelf), "-n", str(path)))
    if not match:
        fail(f"no ELF build ID: {path}")
    return match[1]

with zipfile.ZipFile(apk) as archive, tempfile.TemporaryDirectory(prefix="lb-qml-apk-") as temp:
    temp = Path(temp)
    members = archive.namelist()
    app_members = [p for p in members if p.startswith("lib/") and Path(p).name == library.name]
    if len(app_members) != 1:
        fail(f"expected one packaged {library.name}, found {app_members}")
    packaged = temp / library.name
    packaged.write_bytes(archive.read(app_members[0]))
    if build_id(packaged) != build_id(library):
        fail("APK and production link output have different ELF build IDs")
    symbols = run(str(nm), "--demangle", "--defined-only", str(library))

    resources = {}
    for qrc in qrcs:
        # A manifest alone is not a provider: its compiled resource must be
        # part of this target and retained in the packaged ELF's build.
        generated = qrc.with_name("qrc_" + qrc.stem + ".cpp")
        resource_objects = [obj for obj in objects if obj.name == generated.name + ".o"]
        if not resource_objects or f"qInitResources_{qrc.stem.replace('-', '_')}(" not in symbols:
            fail(f"resource manifest is not linked into production: {qrc}")
        for group in ET.parse(qrc).getroot().findall("qresource"):
            for entry in group.findall("file"):
                source = Path(entry.text)
                if not source.is_absolute():
                    source = qrc.parent / source
                alias = group.get("prefix", "").rstrip("/") + "/" + entry.get("alias", source.name)
                resources[alias] = source

    # Recover only registrations compiled into production objects. Require
    # the initializer to survive linking, a call from another linked object,
    # and the URI/type strings in the provider object. Never inspect stripped
    # APK symbols, nor registrations in desktop/test source files.
    providers = set()
    object_symbols = {obj: run(str(nm), "--demangle", str(obj)) for obj in objects}
    for obj in objects:
        dependency = Path(str(obj) + ".d")
        if not dependency.is_file():
            continue
        dependency_text = dependency.read_text().replace("\\\n", " ")
        sources = re.findall(r"(?:^|\s)(/[^\s]+\.cpp)(?=\s|$)", dependency_text)
        for source in sources:
            text = Path(source).read_text()
            for match in re.finditer(r'void\s+(\w+::\w+)\s*\(\s*\)\s*\{', text):
                initializer = match[1]
                depth, end = 1, match.end()
                while depth and end < len(text):
                    depth += (text[end] == "{") - (text[end] == "}")
                    end += 1
                body = text[match.end():end - 1]
                for registration in re.finditer(r'qmlRegisterType<\s*(\w+)\s*>\(\s*"([^"]+)"\s*,\s*\d+\s*,\s*\d+\s*,\s*"([^"]+)"', body):
                    cpp_type, uri, qml_type = registration.groups()
                    defined = re.search(rf"\bT {re.escape(initializer)}\(\)", symbols)
                    called = any(re.search(rf"\bU {re.escape(initializer)}\(\)", syms) for other, syms in object_symbols.items() if other != obj)
                    data = obj.read_bytes()
                    if defined and called and uri.encode() + b"\0" in data and qml_type.encode() + b"\0" in data and cpp_type in object_symbols[obj]:
                        providers.add(uri)

    output = temp / "production.cmake"
    command = [str(scanner), "-rootPath", str(temp), "-cmake-output", "-output-file", str(output)]
    for path in imports:
        command += ["-importPath", path]
    command += ["-qrcFiles", *map(str, qrcs)]
    subprocess.run(command, check=True)
    records = re.findall(r'qml_import_scanner_import_\d+ "([^"]+)"', output.read_text())
    if not records:
        fail("no production resource import records")

    # Validate the executable's pluginless local QML module even if no QML
    # file imports its URI (the entry point may load a resource URL directly).
    local_modules = set()
    for alias, source in resources.items():
        if source.name != "qmldir":
            continue
        text = source.read_text()
        module = re.search(r"^module (\S+)$", text, re.M)
        if not module or re.search(r"^(?:optional )?plugin ", text, re.M):
            continue
        entries = re.findall(r"^(?:\w+\s+)?(?:\d+\.\d+\s+)?(\S+\.(?:qml|js))$", text, re.M)
        prefix = alias.rsplit("/", 1)[0]
        if not entries or any(prefix + "/" + entry not in resources for entry in entries):
            fail(f"pluginless module lacks linked QML/script resources: {module[1]}")
        local_modules.add(module[1])

    allowed_unresolved = {"QML", "QtQuick.Controls.Windows", "QtQuick.Controls.macOS", "QtQuick.Controls.iOS", "org.kde.breeze"}
    # Android's ABI suffix is part of the deployed plugin filename.
    abi = app_members[0].split("/")[1]
    needed_members = set(app_members)
    errors = []
    for record in records:
        def value(key):
            match = re.search(rf"(?:^|;){key};([^;]*);", record)
            return match[1] if match else ""
        if value("TYPE") != "module":
            continue
        name, plugin = value("NAME"), value("PLUGIN")
        if not plugin:
            if name not in providers | local_modules | allowed_unresolved:
                errors.append(f"{name}: no linked runtime provider or resource-backed QML module")
            continue
        if not value("PATH"):
            errors.append(f"{name}: plugin has no resolved import PATH")
            continue
        # Match the exact plugin in the application's ABI, not a substring
        # of another plugin's filename.
        stem = plugin if plugin.startswith("qml_") else "qml_" + value("RELATIVEPATH").replace("/", "_") + "_" + plugin
        expected = f"lib/{abi}/lib{stem}_{abi}.so"
        if expected not in members:
            errors.append(f"{name}: required plugin {expected} missing from APK")
        else:
            needed_members.add(expected)
    # A plugin without its DT_NEEDED provider is not a packaged provider.
    # NDK API stubs, not an ad-hoc soname allowlist, identify system libs.
    sysroot_arg = next((arg.split("=", 1)[1] for arg in link if arg.startswith("--sysroot=")), "")
    android_api = re.search(r"--target=\S*android(\d+)", " ".join(link))
    if not sysroot_arg or not android_api:
        fail("missing Android sysroot/API link evidence")
    checked = set()
    while needed_members - checked:
        member = sorted(needed_members - checked)[0]
        checked.add(member)
        extracted = temp / Path(member).name
        extracted.write_bytes(archive.read(member))
        for soname in re.findall(r"\(NEEDED\).*?\[([^]]+)\]", run(str(readelf), "-d", str(extracted))):
            packaged_dependency = f"lib/{abi}/{soname}"
            if packaged_dependency in members:
                needed_members.add(packaged_dependency)
            elif not list(Path(sysroot_arg).glob(f"usr/lib/*/{android_api[1]}/{soname}")):
                errors.append(f"{member}: required ELF provider {soname} missing from APK")
    if errors:
        fail("\n  " + "\n  ".join(errors))
    print(f"QML production import check passed: {len(records)} records; linked C++ providers={sorted(providers)}; resource modules={sorted(local_modules)}; APK build ID={build_id(packaged)}. Runtime paint not checked.")
PY
}

if [[ ${1:-} == --check-qml-apk ]]; then
    [[ $# == 3 ]] || { echo "Usage: $0 --check-qml-apk BUILD_DIR APK" >&2; exit 2; }
    check_qml_apk_imports "$2" "$3"
    exit
fi

GRADLE_OPTS="${GRADLE_OPTS:-} -Dorg.gradle.daemon=false -Dorg.gradle.workers.max=2 -XX:ActiveProcessorCount=2"
export GRADLE_OPTS
BUILD_JOBS=${BUILD_JOBS:-4}

for tool in cmake "$ADB" python3; do
    command -v "$tool" >/dev/null || { echo "Missing required tool: $tool" >&2; exit 1; }
done
for path in "$QT_ANDROID_ROOT/bin/qt-cmake" "$QT_HOST_ROOT/bin/androiddeployqt" "$ANDROID_NDK_ROOT/build/cmake/android.toolchain.cmake" "$ANDROID_SDK_SOURCE/platforms/android-$ANDROID_API/android.jar" "$KIRIGAMI_ROOT/lib/cmake/KF6Kirigami/KF6KirigamiConfig.cmake"; do
    test -e "$path" || { echo "Required Android toolchain file not found: $path" >&2; exit 1; }
done
for library in libcrypto_3.so libssl_3.so; do
    test -n "$ANDROID_OPENSSL_ROOT" && test -f "$ANDROID_OPENSSL_ROOT/$library" || {
        echo "Set ANDROID_OPENSSL_ROOT to the ABI-matched OpenSSL 3 runtime directory (missing $library)" >&2
        exit 1
    }
done

mkdir -p "$ANDROID_SDK_ROOT"
for component in cmdline-tools licenses ndk platform-tools; do
    if [[ ! -e "$ANDROID_SDK_ROOT/$component" ]]; then
        ln -s "$ANDROID_SDK_SOURCE/$component" "$ANDROID_SDK_ROOT/$component"
    fi
done
for component in platforms build-tools; do
    mkdir -p "$ANDROID_SDK_ROOT/$component"
done
ln -sfn "$ANDROID_SDK_SOURCE/platforms/android-$ANDROID_API" "$ANDROID_SDK_ROOT/platforms/android-$ANDROID_API"
ln -sfn "$ANDROID_SDK_SOURCE/build-tools/$ANDROID_BUILD_TOOLS" "$ANDROID_SDK_ROOT/build-tools/$ANDROID_BUILD_TOOLS"

export JAVA_HOME=${JAVA_HOME:-/usr/lib/jvm/java-21-openjdk-amd64}
"$QT_ANDROID_ROOT/bin/qt-cmake" -S "$APP_ROOT" -B "$BUILD_DIR" \
    -DCMAKE_BUILD_TYPE=Release \
    -DQT_HOST_PATH="$QT_HOST_ROOT" \
    -DCMAKE_PREFIX_PATH="$KIRIGAMI_ROOT" \
    -DKF6Kirigami_DIR="$KIRIGAMI_ROOT/lib/cmake/KF6Kirigami" \
    -DKF6KirigamiPlatform_DIR="$KIRIGAMI_ROOT/lib/cmake/KF6KirigamiPlatform" \
    -DANDROID_SDK_ROOT="$ANDROID_SDK_ROOT" \
    -DANDROID_NDK_ROOT="$ANDROID_NDK_ROOT" \
    -DANDROID_PLATFORM="android-$ANDROID_API" \
    "-DQT_ANDROID_EXTRA_LIBS=$ANDROID_OPENSSL_ROOT/libcrypto_3.so;$ANDROID_OPENSSL_ROOT/libssl_3.so"
cmake --build "$BUILD_DIR" --target litterbox-qt_make_apk --parallel "$BUILD_JOBS"

APK=${APK:-"$BUILD_DIR/android-build/build/outputs/apk/release/android-build-release-unsigned.apk"}
test -f "$APK" || { echo "Expected APK was not produced: $APK" >&2; exit 1; }
APKSIGNER="$ANDROID_SDK_SOURCE/build-tools/$ANDROID_BUILD_TOOLS/apksigner"
AAPT="$ANDROID_SDK_SOURCE/build-tools/$ANDROID_BUILD_TOOLS/aapt"
test -x "$APKSIGNER" && test -x "$AAPT" || { echo "Android signing/build tools missing under $ANDROID_SDK_SOURCE/build-tools/$ANDROID_BUILD_TOOLS" >&2; exit 1; }
if [[ ! -f "$KEYSTORE" ]]; then
    mkdir -p "$(dirname "$KEYSTORE")"
    "$JAVA_HOME/bin/keytool" -genkeypair -keystore "$KEYSTORE" -storepass android -keypass android \
        -alias androiddebugkey -dname 'CN=Android Debug,O=Android,C=US' -keyalg RSA -keysize 2048 -validity 10000 -noprompt
fi
SIGNED_APK=${SIGNED_APK:-"${APK%.apk}-signed.apk"}
cp "$APK" "$SIGNED_APK"
"$APKSIGNER" sign --ks "$KEYSTORE" --ks-pass pass:android --key-pass pass:android "$SIGNED_APK"
BADGING=$("$AAPT" dump badging "$SIGNED_APK")
PACKAGE=$(printf '%s\n' "$BADGING" | python3 -c 'import re,sys; s=sys.stdin.read(); m=re.search(r"^package: name=\x27([^\x27]+)\x27", s, re.M); print(m.group(1) if m else "")')
ACTIVITY=$(printf '%s\n' "$BADGING" | python3 -c 'import re,sys; s=sys.stdin.read(); m=re.search(r"^launchable-activity: name=\x27([^\x27]+)\x27", s, re.M); print(m.group(1) if m else "")')
test -n "$PACKAGE" && test -n "$ACTIVITY" || { echo "Could not read package/activity from $SIGNED_APK" >&2; exit 1; }
"$ADB" -s "$DEVICE" install -r "$SIGNED_APK"
"$ADB" -s "$DEVICE" shell am force-stop "$PACKAGE"
"$ADB" -s "$DEVICE" shell am start -W -n "$PACKAGE/$ACTIVITY"
sleep "${SCREENSHOT_DELAY:-2}"
if ! "$ADB" -s "$DEVICE" shell dumpsys activity activities | python3 -c 'import re,sys; s=sys.stdin.read(); pkg,activity=sys.argv[1:]; component=re.escape(pkg+"/"+activity); sys.exit(0 if re.search(r"(?:mResumedActivity|topResumedActivity).*"+component, s) else 1)' "$PACKAGE" "$ACTIVITY"; then
    echo "Refusing screenshot: $PACKAGE/$ACTIVITY is not the resumed Android activity" >&2
    exit 1
fi
SCREENSHOT=${SCREENSHOT:-"$BUILD_DIR/android-app.png"}
mkdir -p "$(dirname "$SCREENSHOT")"
"$ADB" -s "$DEVICE" exec-out screencap -p > "$SCREENSHOT"
printf 'APK: %s\nPackage: %s\nInstalled on: %s\nScreenshot: %s\n' "$SIGNED_APK" "$PACKAGE" "$DEVICE" "$SCREENSHOT"
