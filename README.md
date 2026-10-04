# Go Populous II

An ongoing Go/Ebitengine recreation of **Populous II: Trials of the Olympian
Gods**, using its original Amiga graphics, campaign and audio resources.

The playable version includes an isometric world, followers, settlements,
power selection, computer opposition, a demonstration mode and Go saves.
Native translations now replace several parts of the supplied Populous I
foundation: terrain generation, random streams, starting populations, town
work, mana costs, hero creation and persistent ground effects.

**The complete original feature set is still being converted.** Several
powers, native movement/combat, god creation, campaign progression and
multiplayer remain incomplete. [The feature inventory](docs/FEATURES.md)
distinguishes native translations from provisional behavior.

![Go Populous II](screenshots/game.png)

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
go build -o bin/populous2 ./cmd/populous2
```

The default window is 960 × 720, with a 640 × 480 logical display and doubled
Amiga artwork. Input updates at 60 Hz; the current simulation advances eight
times per second. Original-machine timing remains a comparison target.

`-custom` currently exposes all 29 Amiga powers for testing. It does not yet
apply the original conquest-based custom-game unlocking policy.

## Controls

| Action | Input |
|---|---|
| Raise/lower terrain | Left/right click |
| Release a group from a dwelling | Right click on the dwelling |
| Move the camera | WASD, arrows, or the world map |
| Select a power | Element, power, then target |
| Paint/remove roads | Hold left/right mouse button and move |
| Provisional wall line | Two endpoint clicks |
| Effect direction | Q / E |
| Papal magnet / find the leader | M / C |
| Settle, gather, fight, follow | 1 / 2 / 3 / 4 |
| Pause / help | Space / H |
| Mute audio | N |
| Save / load | F5 / F9 |
| Menu / fullscreen | Escape / F |
| Continue after a result | Enter |

Prices shown are the actual mana balance costs, including the original
per-element experience reductions. The native score and sample bank play
through the Go audio reader. Hardware timing and all sound-event bindings
still need verification.

Saves use `go-populous2.sav`; `POPULOUS2_SAVE_PATH` selects another path.
Version 5 preserves scenery, the 32-bit random state, follower references, town work,
infection and persistent ground effects. Earlier Go saves remain readable;
versions 1–2 also receive the corrected Helen/tsunami ID mapping. Original
Amiga GAM saves are not yet supported.

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
These checks establish the tested routines; they do not establish complete
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
