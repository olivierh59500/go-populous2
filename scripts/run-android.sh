#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$script_dir/android-env.sh"

"$script_dir/build-android.sh" "$@"
device_count=$("$adb_path" devices | awk 'NR > 1 && $2 == "device" { count++ } END { print count + 0 }')
if [ "$device_count" -ne 1 ]; then
    echo "Exactly one authorised Android device is required (found $device_count)." >&2
    "$adb_path" devices -l >&2
    exit 1
fi

echo "Installing Populous II Go."
"$adb_path" install -r "$apk_path"
"$adb_path" shell am force-stop "$application_id"
"$adb_path" shell am start -n "$application_id/.MainActivity"
