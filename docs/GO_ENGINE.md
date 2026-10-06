# Independent Go engine

The default `cmd/populous2` application uses ordinary Go state and rules,
Ebitengine rendering and a Go PCM sequencer. It does not load `populous.ii`,
HUNK segments, instruction dumps, relocated memory or CPU register contexts.
The old translation remains available as `cmd/populous2-native` for comparison.
`cmd/populous2-go` is an alias of the independent desktop launcher.

## Architecture

| Package | Responsibility |
|---|---|
| `internal/engine` | Terrain, followers, towns, six heroes, all 29 powers, AI, scenarios, campaign rules and semantic snapshots |
| `internal/app` | Menus, input, isometric compositor, profile/campaign presentation, save browser and asynchronous network UI |
| `internal/visualassets` | PNG atlases, palettes, fonts, named animation compositions and original ending playback data |
| `internal/music` | Semantic musical events, signed PCM and Go replay/mixing |
| `internal/gamcodec` | Original user-save file conversion at a strict boundary |
| `internal/network` | Validated snapshots, ordered player commands and TCP lockstep |
| `assets/runtime` | Locally embedded portable asset package |
| `internal/desktop` | Shared desktop command configuration and Ebitengine startup |

The engine owns checked actor IDs and geometric cells. Mixed cell membership
preserves actual insertion, movement and removal order. Effects share a bounded
250-slot pool; followers have 399 usable slots, scenery and walls 200 each.
Game behavior is expressed with named fields, deterministic phase order and
ordinary Go operations. File-format token conversion remains inside GAM
interoperability and never selects runtime instruction bodies.

## Prepare assets and run

The import tools may inspect the original disks/executable locally to recover
actual art, fonts, animation compositions, samples, music and campaign data.
Their portable output contains no instruction stream or CPU memory image.
Original resources and generated assets stay excluded from Git.

```sh
sh tools/exclude-local-assets.sh
go run ./cmd/import-assets -adf "/path/to/disk A.adf" -adf "/path/to/disk B.adf"
go run ./cmd/export-visual-assets -output assets/runtime/data
go run ./cmd/export-audio-assets -output assets/runtime/data/audio
go build -o bin/populous2 ./cmd/populous2
```

This build can run outside the repository. A clean checkout contains an
embeddable placeholder so source tools and the game compile before import.
For an external asset installation, export to another directory and launch
`go run ./cmd/populous2 -data /path/to/portable-assets`.
See [ASSET_SETUP.md](ASSET_SETUP.md) for supported disk revisions.

## Controls

| Action | Input |
|---|---|
| Sculpt land / release a town group | Left / right click |
| Choose a power | Tab, then category and power |
| Settlement / rally / join / fight | 1 / 2 / 3 / 4 |
| Direction for directional powers | Q / E |
| Activate the lightning marker | Enter |
| Move the viewport | Arrows or click the overview |
| Deity profile / world / multiplayer | Main menu |
| Options / detached map editor | O / P; EDIT in options |
| Browse/load / browse/save | F9 / F10; Load Game in the main menu |
| Pause / help / return | Space / H / Escape |

The gameplay panel also offers mouse controls. The options/editor operate on
validated detached drafts; Cancel preserves the live game. Power help creates
an isolated, paced demonstration using the real effect controllers.

## Persistence and multiplayer

The save browser lists JSON/GAM files and requires confirmation before replacing
an existing destination. Invalid files cannot replace the current world. JSON
preserves the full semantic Go session, profile, campaign position, local camp,
camera, selected power, direction, custom setup and pause state.

GAM uses the original 56,690-byte file layout. Supported mappings include custom
player settings, profile/camera, AI choices/cooldowns/requests, statistics, mixed
actor links, effects, appearance variants, hero claims, contact waits and retained
deaths. Reserved user-save fields remain outside the simulation. Original GAM
cannot represent pending Go input orders or an active deferred settlement deadline
as separate fields; export rejects these states and suggests lossless JSON.
Unknown original records and missing artwork mappings are also explicit errors.

Two-player sessions use `-listen address:port` and `-connect address:port`, or the
menu. The background network worker owns a detached world and only publishes a
completed round. A failed or mismatched connection pauses simulation and shows
an error; automatic reconnection/resynchronization is not implemented. Leave
the session before saving/loading or editing live rules.

## Verification boundaries

The default and alias launchers have dependency tests rejecting the original
translation and executable loaders. The standalone build is checked outside the
repository. Source comparisons cover terrain, town support/economy, movement,
contacts, heroes, effects, campaign arithmetic, art/anchors/depth and audio.
All 29 effects are exercised through complete lifetimes and saved continuation.

The optional integrated reference comparison uses world 12, landscape 0,
seed `0x15b0`, with 160 complete no-input main frames, including rendering,
clock, simulation and deferred commands. It compares all terrain corners,
follower identity/population/state/fractional position/town stage, mana,
foundation metrics and RNG. A separate mixed lava case covers follower, tree,
wall and magnet. These are reproducible bounded checks, not exhaustive proof
of every combination in all 1,000 worlds.

```sh
POPULOUS2_REFERENCE_COMPARE=1 go test ./internal/populous2 -run '^TestIndependentEngineCampaignReferenceOptional$' -count=1 -v
```

The display/input cadence is 50 PAL updates per second; simulation advances at
12.5 passes per second. Audio and ending presentation retain separate clocks.
The rendering tests compare original masked pixels, fractional surfaces,
340 traversal cases, high-object overlap and 162 terrain-edge backdrop cases.
Original assets used by these optional checks are not distributed.
