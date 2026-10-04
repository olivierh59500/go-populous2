# Go Populous II

An ongoing Go/Ebitengine recreation of **Populous II: Trials of the Olympian
Gods**, using its original Amiga graphics, campaign and audio resources.

The playable version includes an isometric world, followers, settlements,
power selection, computer opposition, a demonstration mode and Go saves.
Native translations now replace several parts of the supplied Populous I
foundation: terrain generation, random streams, starting populations, town
work, mana costs, ordinary fractional walking, hero creation and persistent
ground effects.

**The complete original feature set is still being converted.** Several
powers, hero movement, native combat, campaign progression and
multiplayer remain incomplete. [The feature inventory](docs/FEATURES.md)
distinguishes native translations from provisional behavior.

![Go Populous II](screenshots/game.png)

![Original deity portrait and profile editor](screenshots/deity.png)

![Native fire columns climbing and burning the landscape](screenshots/fire-columns.png)

![Native Whirlpool terrain and Basalt propagation](screenshots/water-effects.png)

![Three native whirlwinds in the diagnostic presentation](screenshots/whirlwinds.png)

![Original follower animation banks with native fractional walking](screenshots/ordinary-motion.png)

## Run

Go 1.25 or newer and the normal Ebitengine platform prerequisites are required.
All runtime resources are embedded; local reference disks are not needed to
build or run the game.

```sh
go run ./cmd/populous2
go run ./cmd/populous2 -play
go run ./cmd/populous2 -custom
go run ./cmd/populous2 -demo -world 0
go run ./cmd/populous2 -play -code DOEGAC
go run ./cmd/populous2 -deity
go run ./cmd/populous2 -fire-columns
go run ./cmd/populous2 -whirlwinds
go run ./cmd/populous2 -fungus
go run ./cmd/populous2 -custom -rules
go build -o bin/populous2 ./cmd/populous2
```

The default window is 960 × 720, with a 640 × 480 logical display and doubled
Amiga artwork. Input updates at 60 Hz; the simulation uses the nominal PAL
VBlank cadence of 50 updates per second. `-simulation-rate` selects a diagnostic
rate. Original CPU-bound throughput and special idle pacing remain comparison
targets; the viewport-size value eight is not a simulation-rate setting.

`-custom` currently exposes all 29 Amiga powers for testing. It does not yet
apply the original conquest-based custom-game unlocking policy.

`-fire-columns`, `-whirlwinds` and `-fungus` place effects near the camera for
inspecting original composite art and controllers. The whirlwind and fungus
presentations also disable follower attrition and fatal water as
diagnostic overrides.
For a bounded application-buffer capture:

```sh
go run ./cmd/populous2 -whirlwinds -frames 180 -capture-update 12 -screenshot /tmp/populous2-whirlwinds.png
```

The screenshot path must not exist. Native pickup/release, town collapse and
child whirlpools are still being converted; the current whirlwind does not
substitute generic area damage for those interactions. Campaign movement and
opponent pacing remain incomplete and can produce unusually early results.

## Controls

| Action | Input |
|---|---|
| Raise/lower terrain | Left/right click |
| Release a group from a dwelling | Right click on the dwelling |
| Move the camera | WASD, arrows, or the world map |
| Select a power | Element, power, then target |
| Paint/remove roads | Hold left/right mouse button and move |
| Extend city walls | Place each cell beside an existing wall |
| Effect direction | Q / E |
| Papal magnet / find the leader | M / C |
| Settle, gather, fight, follow | 1 / 2 / 3 / 4 |
| Pause / help | Space / H |
| Mute audio | N |
| Save / load | F5 / F9 |
| Menu / fullscreen | Escape / F |
| Continue after a result | Enter |
| Create/edit the deity from the menu | G or the deity button |
| View/edit scenario rules | O (editable separately for each side in custom games) |

Prices shown are the actual mana balance costs, including the original
per-element experience reductions. The native score and sample bank play
through the Go audio reader. Hardware timing and all sound-event bindings
still need verification.

Saves use `go-populous2.sav`; `POPULOUS2_SAVE_PATH` selects another path.
Version 13 preserves the mixed actor graph and movement-pressure bytes, native
Basalt/Whirlpool controllers and ordinary walkers' fractional positions, animation clocks
and timers, along with follower hazard states and sound events, fungus bounds
and pending references, both scenario option words, native effects and death
animations, the deity profile, scenery, the 32-bit random state, follower
references, town work, infection and persistent ground effects.

Earlier Go saves remain readable. Versions 1–2 receive the corrected
Helen/tsunami ID mapping; version 12 also corrects earlier basalt/whirlpool IDs
30/31. Generic whirlwinds migrate into native effect records. Earlier fungus
damage marks migrate to native seeds and collecting controllers. Original
Amiga GAM saves are not yet supported.

Earlier generic whirlpools migrate to native controller records. Earlier
basalt marks retain their existing terrain and become persistent native-family
tiles. Old saves did not record mixed actor order/pressure; their graph is
initialized from the retained actor pools during migration.

The deity screen uses the original three-part face artwork, eight variants per
part, five starting bolts and one experience unit per allocated bolt. Click the
profile code to enter an original sixteen-letter password. The separate name
field is not included in that code. Campaign scoring and experience awards
still require their full native statistic integration.

## Native data and verification

The decoder is currently specific to the supplied French executable revision.
`POPULOUS2_DATA_DIR` can select another extracted installation containing the
same executable and resource catalog. Unsupported layouts fail during loading.

```sh
go run ./cmd/assetcheck
go run ./cmd/assetcheck -images /tmp/populous2-images
go test ./...
go vet ./...
go test -race ./internal/amiga ./internal/populous2 ./internal/legacy
go run ./cmd/simcheck -world 0 -ticks 4800
```

`simcheck` and the data/simulation tests work without a display. Terrain tests
compare all 4,225 heights against eight executions of the original 68000
routine. Nine ground-effect references compare complete tile maps and final
random states. Other checks cover original resource integrity, graphics,
wide follower IDs, mana, hero attributes, save continuation, audio and slopes.
Ordinary follower tests compare 72 native movement traces and retain the
original per-owner animation banks, including high-speed facing quirks.
Target selection, waiting, swimming, hero movement and combat still use
inherited adapters. These checks establish the tested routines; they do not establish complete
original-game parity.

[Native rules](docs/NATIVE_RULES.md), [conversion notes](docs/PORTAGE.md),
[validation](docs/VALIDATION.md) and [provenance](docs/PROVENANCE.md) document
the implementation and its remaining limits.

## Layout

| Directory | Purpose |
|---|---|
| `internal/amiga` | Strict ADF/OFS/FFS and Hunk readers |
| `internal/populous2` | Native resource, campaign, rule and sound translations |
| `internal/legacy` | Adapted foundation from the supplied Populous I recreation |
| `internal/fixedstep` | Simulation scheduler |
| `internal/game` | Ebitengine interface, rendering and audio integration |
| `assets/amiga` | Original runtime resources and executable tables |
| `cmd` | Game, asset inspection and simulation commands |
| `tools` | Optional bounded disassembly helper |
| `docs` | Feature inventory, native offsets and validation evidence |

The reused Go code retains GPL-3.0 licensing. Original game resources have
separate provenance and rights; see [PROVENANCE.md](docs/PROVENANCE.md).
