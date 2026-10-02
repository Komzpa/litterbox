#!/usr/bin/env bash
set -euo pipefail

QT_ANDROID_ROOT=${QT_ANDROID_ROOT:-"$HOME/Qt/6.7.3/android_arm64_v8a"}
QT_HOST_ROOT=${QT_HOST_ROOT:-"$HOME/6.7.3/gcc_64"}
ANDROID_SDK_SOURCE=${ANDROID_SDK_SOURCE:-/usr/lib/android-sdk}
ANDROID_NDK_ROOT=${ANDROID_NDK_ROOT:-"$ANDROID_SDK_SOURCE/ndk/29.0.14206865"}
ANDROID_SDK_ROOT=${ANDROID_SDK_ROOT:-"$HOME/.local/share/litterbox/android-sdk-qt6"}
KIRIGAMI_ROOT=${KIRIGAMI_ROOT:-"$HOME/Qt/kirigami-android"}
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
import glob
import re
import subprocess
import sys

build_dir, apk = sys.argv[1:]
records = []
for path in glob.glob(f"{build_dir}/.qt/qml_imports/*_conf.cmake"):
    if path.endswith("/tst_api_conf.cmake"):
        continue  # Test-only imports are not packaged into the application APK.
    text = open(path, encoding="utf-8").read()
    records.extend((path, fields) for fields in re.findall(r'qml_import_scanner_import_\d+ "([^"]+)"', text))
if not records:
    raise SystemExit(f"QML import check failed: no *_conf.cmake import records under {build_dir}/.qt/qml_imports")

allowed_unresolved = {"QML", "QtQuick.Controls.Windows", "QtQuick.Controls.macOS", "QtQuick.Controls.iOS", "org.kde.breeze"}
apk_paths = subprocess.check_output(["unzip", "-Z1", apk], text=True).splitlines()
errors = []
for path, record in records:
    get = lambda key: re.search(rf"(?:^|;){key};([^;]*);", record)
    value = lambda key: (match.group(1) if (match := get(key)) else "")
    if value("TYPE") != "module":
        continue
    name = value("NAME") or "<unnamed>"
    plugin = value("PLUGIN")
    if not value("PATH") or not plugin:
        if name not in allowed_unresolved:
            errors.append(f"{name}: module record has no PATH or PLUGIN ({path})")
        continue
    plugin_stem = plugin.removeprefix("qml_")
    expected = re.compile(rf"^lib/[^/]+/libqml_.*{re.escape(plugin_stem)}.*\.so$")
    if not any(expected.match(member) for member in apk_paths):
        errors.append(f"{name}: PLUGIN {plugin} has no matching lib/<abi>/libqml_*{plugin_stem}*.so in {apk}")
if errors:
    print("QML import check failed:", file=sys.stderr)
    print("\n".join(f"  {error}" for error in errors), file=sys.stderr)
    raise SystemExit(1)
print(f"QML import check passed: {len(records)} import records checked in {apk}")
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
    -DANDROID_PLATFORM="android-$ANDROID_API"
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
