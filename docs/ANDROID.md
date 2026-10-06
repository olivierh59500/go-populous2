# Populous II Go on Android

The Android application uses the same independent Go engine as the desktop game.
The landscape interface adds touch controls and a larger playfield. The original
campaign, terrain rules, powers and PAL timing remain shared with the desktop
version. Android's Java activity only hosts Ebitengine and Bluetooth services.

## Build and launch

Prepare the locally generated resources described in the main README before
building. Android embeds `assets/runtime/data/`; no original game executable is
included or executed. Original and generated resources remain excluded from Git.

```sh
./scripts/build-android.sh
./scripts/run-android.sh
```

The first command builds without using a device. The second builds, installs and
launches on exactly one authorised USB device. To reuse an unchanged Go AAR after
editing only the Android shell, add `--skip-bind`.

The generated APK is
`android/app/build/outputs/apk/debug/app-debug.apk`. It is a development build
signed with Android's debug key, not a release signed for public distribution.

The pinned toolchain is Ebitengine/ebitenmobile 2.9.11, Java 17, Gradle 8.11.1,
Android Gradle Plugin 8.10.1, SDK/build tools 36 and NDK 28.2.13676358. The package
is `com.olivierh.populous2`, separate from Populous Go. This target supports
`arm64-v8a`, Android 6.0/API 23 and later. The build checks its manifest, signature
and 16 KiB native-library alignment.

`ANDROID_HOME` and `JAVA_HOME` override SDK and JDK discovery. Go build and Gradle
caches default to `.local/android/`, which is excluded locally. Existing cache
locations can be selected with `GOCACHE` and `GRADLE_USER_HOME`.

## Bluetooth multiplayer

Bluetooth Classic carries the same deterministic two-player protocol as TCP.
Host and join requests are handled by Android outside the game update loop.
The host becomes discoverable temporarily; the joining player selects a paired
or nearby device. Permission is requested only when the corresponding operation
needs it. Devices without a Bluetooth adapter can still play locally.

The RFCOMM service has its own Populous II UUID, so it cannot accidentally join
a Populous Go session. A private authenticated loopback connection joins the
Android byte stream to the Go network controller. Connection failures and
cancellation return to the interface instead of blocking rendering.

Two Android devices are required to verify a complete Bluetooth match. A single
USB phone can verify permissions, host setup, discovery, cancellation, touch
input and lifecycle behavior, but cannot demonstrate a real two-phone match.

## Touch interface

The playfield fills the landscape screen above the bottom toolbar. Dragging
with two fingers pans the view; a single tap applies the selected tool. **Raise** and **Lower** share
the original terrain permissions, including the visible follower/town rule.
**Flag** places the rally point, **Look** inspects a group, **Powers** opens the
six elements and their available powers, and **People** selects settle, rally,
join or fight. **Map** shows the world overview and **Menu** opens session
actions.

Directional powers display **Dir -** and **Dir +** controls. Lightning provides
separate **Fire** and **Cancel** buttons after placing its marker. **Group**
shows the selected follower or town's original artwork, population and weapons.
The overview accepts a tap to move the camera to another region of the world.

Terrain and actor artwork is cached between the original 12.5 Hz main frames;
the HUD and input continue at 50 Hz. Moving the camera or publishing a network
world refreshes the scene without resetting the camera. Construction requests
carry a checked mask of actually visible ground parcels, so a wider screen does
not grant permission to sculpt hidden or off-screen land.

Touch menus use the same game actions as the desktop interface. Conquest offers
world navigation and world-code entry, opponent details and original animated
spell previews. Deity editing includes experience allocation, face selection,
name entry and the original password codec. Rules stay read-only during a
campaign; custom options and audio are edited in a draft until **Apply**.
The save browser pages through files and confirms replacements explicitly.

The on-screen ASCII keyboard supplies names, passwords, world codes, special
codes, filenames, TCP addresses and editor numbers without requiring a physical
keyboard. **Original details** shows the original requester artwork where
additional information or a portrait is useful. Editor tools and event fields
also use touch controls, while painting targets the original preview map.

## Lifecycle

The activity keeps the display awake while the game is visible, supports both
landscape rotations and hides system bars after installing the Ebitengine view.
Audio starts on the Go game's first update, after Android has supplied a context.
Pause suspends Ebitengine and cancels unfinished touch gestures. Permission and
pairing dialogs preserve the pending Bluetooth request; final activity teardown
closes it. Saves use Android's private files directory.

## Validation

The debug build was installed on a Pixel 10a running Android 17/API 37. Device
checks covered the wide landscape scene, six power categories, conquest flow,
Bluetooth host permissions and waiting state, cancellation, private JSON saving,
return from the launcher, and keep-screen-on behavior. Source-only tests cover
all 29 power choices, lightning activation/dismissal, direction changes, editor
painting, view navigation, gesture cancellation and synchronized camera updates.

Two authenticated simulated transport peers exchange the complete host snapshot
and deterministic input rounds; disconnect stops mutation. This validates the
Go framing and session path, not a radio match between two physical phones.

Scene caching removes repeated terrain painting between main cycles. The local
540 × 240 CPU benchmark measured about 0.64 ms for a full scene and 0.007 ms for
a cached frame, with no allocations in the cache path. These are Mac M4 Max
measurements, not Pixel frame-time measurements. Android's ordinary ViewRoot
frame statistics do not measure Ebitengine's SurfaceView game frames reliably.
