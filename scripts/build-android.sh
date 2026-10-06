#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$script_dir/android-env.sh"

case "${1:-}" in
    "") skip_bind=false ;;
    --skip-bind) skip_bind=true ;;
    *) echo "Usage: $0 [--skip-bind]" >&2; exit 1 ;;
esac
if [ "$#" -gt 1 ]; then
    echo "Usage: $0 [--skip-bind]" >&2
    exit 1
fi

cd "$project_root"
module_ebiten_version=$(go list -m -f '{{.Version}}' github.com/hajimehoshi/ebiten/v2)
if [ "$module_ebiten_version" != "$ebiten_version" ]; then
    echo "Ebitengine mismatch: go.mod=$module_ebiten_version, ebitenmobile=$ebiten_version." >&2
    exit 1
fi
# A clean checkout contains only a placeholder. These resources are generated
# locally from the user's original game disks and must never enter Git.
for required_asset in visuals.json campaign.dat audio/score.json hud.json conquest.json spell-help.json; do
    if [ ! -f "$project_root/assets/runtime/data/$required_asset" ]; then
        echo "Missing embedded resource: $required_asset. Export and prepare runtime assets first." >&2
        exit 1
    fi
done

mkdir -p "$project_root/android/app/libs" "$GOCACHE" "$GRADLE_USER_HOME"
if [ "$skip_bind" = false ]; then
    echo "Building the Go/Ebitengine library for arm64-v8a."
    go run "github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@$ebiten_version" \
        bind \
        -target android/arm64 \
        -androidapi "$android_api" \
        -javapkg "$application_id" \
        -o android/app/libs/populous2.aar \
        ./mobile
elif [ ! -f android/app/libs/populous2.aar ]; then
    echo "--skip-bind requires a previously generated populous2.aar." >&2
    exit 1
fi

echo "Building the debug APK."
"$project_root/android/gradlew" -p "$project_root/android" --console=plain assembleDebug

echo "Checking package, API levels, signature and 16 KiB alignment."
if [ ! -f "$apk_path" ]; then
    echo "APK missing after build: $apk_path" >&2
    exit 1
fi
apk_badging=$("$aapt_path" dump badging "$apk_path")
printf '%s\n' "$apk_badging" | grep -q "package: name='$application_id'"
printf '%s\n' "$apk_badging" | grep -q "sdkVersion:'$android_api'"
printf '%s\n' "$apk_badging" | grep -q "targetSdkVersion:'$compile_sdk'"
printf '%s\n' "$apk_badging" | grep -q "launchable-activity: name='$application_id.MainActivity'"
printf '%s\n' "$apk_badging" | grep -q "native-code: 'arm64-v8a'"
"$apksigner_path" verify "$apk_path"
"$zipalign_path" -c -P 16 4 "$apk_path"
printf 'APK ready: %s\n' "$apk_path"
