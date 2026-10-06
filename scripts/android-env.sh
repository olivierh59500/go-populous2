#!/bin/sh
# Shared Android build settings. Source this file from a script in scripts/.

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
android_sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}
java_home_path=${JAVA_HOME:-}
ebiten_version=v2.9.11
android_api=23
compile_sdk=36
build_tools_version=36.0.0
ndk_version=28.2.13676358
application_id=com.olivierh.populous2

if [ -z "$android_sdk" ] && [ -d /opt/homebrew/share/android-commandlinetools ]; then
    android_sdk=/opt/homebrew/share/android-commandlinetools
fi
if [ -z "$android_sdk" ] && [ -d "$HOME/Library/Android/sdk" ]; then
    android_sdk=$HOME/Library/Android/sdk
fi
if [ -z "$java_home_path" ] && [ -d /opt/homebrew/opt/openjdk@17 ]; then
    java_home_path=/opt/homebrew/opt/openjdk@17
fi
if [ -z "$java_home_path" ] && [ -d /Applications/Android\ Studio.app/Contents/jbr/Contents/Home ]; then
    java_home_path=/Applications/Android\ Studio.app/Contents/jbr/Contents/Home
fi

if [ -z "$android_sdk" ] || [ ! -x "$android_sdk/platform-tools/adb" ]; then
    echo "Android SDK not found; set ANDROID_HOME." >&2
    exit 1
fi
if [ ! -f "$android_sdk/platforms/android-$compile_sdk/android.jar" ]; then
    echo "Android platform $compile_sdk is missing from $android_sdk." >&2
    exit 1
fi
for android_tool in aapt apksigner zipalign; do
    if [ ! -x "$android_sdk/build-tools/$build_tools_version/$android_tool" ]; then
        echo "Android Build Tools $build_tools_version are incomplete ($android_tool)." >&2
        exit 1
    fi
done
if [ ! -d "$android_sdk/ndk/$ndk_version" ]; then
    echo "Android NDK $ndk_version is missing from $android_sdk." >&2
    exit 1
fi
if [ -z "$java_home_path" ] || [ ! -x "$java_home_path/bin/java" ]; then
    echo "Java 17 not found; set JAVA_HOME." >&2
    exit 1
fi
if ! "$java_home_path/bin/java" -version 2>&1 | grep -q 'version "17\.'; then
    echo "Java 17 is required (JAVA_HOME=$java_home_path)." >&2
    exit 1
fi
if ! command -v go >/dev/null 2>&1; then
    echo "Go is not available in PATH." >&2
    exit 1
fi
if [ ! -x "$project_root/android/gradlew" ]; then
    echo "The Android Gradle wrapper is missing." >&2
    exit 1
fi

export ANDROID_HOME="$android_sdk"
export ANDROID_SDK_ROOT="$android_sdk"
export JAVA_HOME="$java_home_path"
export PATH="$java_home_path/bin:$android_sdk/platform-tools:$PATH"
export GOWORK=off
export GOCACHE=${GOCACHE:-"$project_root/.local/android/go-build"}
export GRADLE_USER_HOME=${GRADLE_USER_HOME:-"$project_root/.local/android/gradle"}

adb_path="$android_sdk/platform-tools/adb"
apk_path="$project_root/android/app/build/outputs/apk/debug/app-debug.apk"
aapt_path="$android_sdk/build-tools/$build_tools_version/aapt"
apksigner_path="$android_sdk/build-tools/$build_tools_version/apksigner"
zipalign_path="$android_sdk/build-tools/$build_tools_version/zipalign"
