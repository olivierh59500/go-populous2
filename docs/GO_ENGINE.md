# Independent Go engine

The current playable application is implemented in Go but still depends on
`populous.ii` at runtime. The intended architecture is a standalone Go game,
using the original graphics, samples, music sequences and campaign data only.
`go-populous` provides the reference organization: a data loader, explicit
simulation state, ordinary Go rules, Ebitengine drawing and Go audio replay.

## Current dependency

`Bundle.Executable` holds the parsed HUNK executable. `LoadFS` reads its resource
and sprite descriptors, gameplay constants, animation definitions and sound
sequences. `NativeRuntimeHost` then relocates its segments and exposes CODE/BSS
through shared memory aliases. Menus, rendering, input, audio and simulation
controllers retain original offsets and register contexts.

Production does not use the local 68000 analysis interpreter. Nevertheless,
requiring CODE bytes and their instruction/address layout is an executable
dependency, even when the operation itself is written in Go. Selecting the
older `cmd/populous2-legacy` command does not remove this dependency: its bundle
and several World rules also read the executable.

## Replacement boundaries

| Area | Required independent implementation |
|---|---|
| Asset loading | Fixed Go file catalog and decoders for packed graphics, sprite differences, landscape and campaign files; explicit image/font/sample/music metadata |
| Game state | Go terrain, follower, town, deity, projectile and environmental state; no relocated memory ownership or self-modifying CODE aliases |
| Simulation | Named movement, founding, growth, combat, AI and power operations; explicit deterministic order and timing |
| Presentation | Go menus/widgets, picking and compositor using decoded art; callbacks identify actions rather than executable procedure addresses |
| Audio | Go sample mixer and music sequencer using extracted audio data, without executable driver or CIA/register-controller dependency |
| Campaign | Go world selection, progression, score, experience, awards and ending state |
| Persistence/network | Explicit Go save and multiplayer state; original GAM compatibility remains a separate codec |
| Reference validation | Original executable, HUNK addresses and CPU comparisons restricted to optional analysis tools and reference tests |

Original graphics, fonts, palettes, sprite metadata and music sequences that
happen to be stored inside the original executable can be extracted locally
as ordinary typed asset data. Executable instructions, procedure dispatch
addresses and emulated memory images must not become runtime data under
another filename. Game rules belong in maintainable Go definitions and code.

## Completion checks

The independent application must build and run after `populous.ii` and all
HUNK/CODE dumps have been removed from its prepared runtime assets. Its
production dependency graph must exclude executable parsing, relocation,
register-machine callbacks and address-based instruction dispatch. It must
retain all original powers, heroes, campaign, menus, audio, save/load and
multiplayer behavior through ordinary Go subsystems.

The existing translation and finite reference corpora remain useful for
comparing results during migration. They must not be mistaken for completion
of the independent engine. Each replacement is validated before the default
launcher switches to the new engine.

The first boundary now has a Go resource-file catalog. `LoadResourceSetFS`
lists and unpacks the 26 external files without opening `populous.ii`.
`cmd/assetcheck` inventory and `-decode` use this path. Its separate `-images`
reference export still needs executable-based sprite metadata.

Tests open a data filesystem that rejects executable reads and verify the four
landscapes, tile banks, sprite differences and 1,000 campaign records. This is a
preparatory change; the complete game still needs the remaining replacements
above. The next playable slice is an independent menu and early conquest,
including terrain sculpting, founding and growth, followers, mana and AI.

## Independent application in progress

`cmd/populous2-go` is a separate migration target. Its production dependencies
are `internal/app`, `internal/engine`, `internal/visualassets` and
`internal/music`; they do not import the original-executable readers or the
register-based translation. It loads a directory of PNG atlases, semantic
animation/music metadata, signed PCM and decoded campaign/landscape data.

Prepare the private assets once with the import tools, then run the new target:

```sh
go run ./cmd/export-visual-assets -output assets/generated
go run ./cmd/export-audio-assets -output assets/generated/audio
go run ./cmd/populous2-go -data assets/generated
```

For a self-contained desktop or mobile build, export the same assets into the
dedicated runtime package before compiling:

```sh
go run ./cmd/export-visual-assets -output assets/runtime/data
go run ./cmd/export-audio-assets -output assets/runtime/data/audio
go build -o bin/populous2-go ./cmd/populous2-go
```

The built game embeds only the exported art, music and campaign files. It can
run outside the repository. A clean source checkout contains a placeholder;
original assets in this build directory are excluded locally and rejected by
the repository's asset-protection hook.

The independent slice currently has Go menus/profile/world selection,
isometric rendering and picking, the 1,000 campaign records, four landscapes,
propagated terrain changes, settlement founding/economy, ordinary followers,
opposing land AI, tactical modes and Go music/sample replay. Terrain-generation
digests and sample-replay comparisons establish specific migrated behavior.
Further follower decisions, effects, rendering layers and campaign presentation
still require migration and fidelity checks.

All 29 power commands are admitted by the independent dispatcher, including
six hero conversions. This describes their migrated command paths, not full
game completion: interactions, simulation and presentation remain subject to
the concrete gaps in [GO_ENGINE_AUDIT.md](GO_ENGINE_AUDIT.md).

The application now has deity profiles and passwords, campaign scores and
progression, original ending playback, options, a detached map editor, semantic
JSON saves and asynchronous two-player TCP sessions. Original GAM
interoperability is being added as a separate file codec. The default
`cmd/populous2` still uses the reference translation until the independent
application passes all completion checks.

Generated artwork and audio remain local and excluded from Git. Import tools
may inspect the original executable to recover its actual art/music data; the
independent game neither opens that file nor receives a substitute instruction
or memory image.

## Subsequent simulation slices

The follower layer now uses continuous fixed-point positions, explicit motion
legs, ordered search rings, occupancy pressure and reciprocal combat state.
Tests compare terrain digests and selected numeric movement/damage results;
whole-game follower parity is still being expanded.

Nature and fire controllers are being migrated into named Go effect states.
All families reserve from one 250-slot effect budget, while scenery has its
separate 200-slot budget. Their presence in source does not make a power
available: admission stays disabled until the world interactions, victim states,
rendering and full casting path have been bound and checked.

| Family | Current independent boundary | Remaining integration |
|---|---|---|
| Nature | Sampled forest/restoration/swamp placement and timed fungus generations; original central fungus maps compared | Hero immunity, complete scenario options, border semantics and presentation/admission |
| Fire | Fixed-point columns, rain, volcano and lava controllers with typed habitat callbacks; original column traces compared | Shared terrain damage, burning victims, heroes, linked environmental interactions and presentation/admission |
| Followers | Typed movement, ordered decisions, towns, pressure and reciprocal combat | Remaining special followers, complete contact/hero/hazard compositions and fidelity coverage |

The early independent menu intentionally exposes the migrated conquest path;
missing editor, results, network or other modes are not simulated by acknowledging
unimplemented actions.

## Independent controls

| Action | Input |
|---|---|
| Sculpt land / release a town group | Left / right click |
| Choose a power | Tab, then category and power |
| Settlement / rally / join / fight | 1 / 2 / 3 / 4 |
| Direction for directional powers | Q / E |
| Activate the lightning marker | Enter |
| Move the viewport | Arrows or click the overview |
| Deity profile / world / multiplayer | Main menu |
| Options / detached map editor | O / P |
| Load / save the independent session | F9 / F10 |
| Help / return to menu | H / Escape |

Desktop two-player sessions can also be started with `-listen address:port`
and `-connect address:port`. A waiting or interrupted connection keeps the
window responsive and pauses simulation. Cosmetic audio/framebuffer state is
separate from the synchronized world.
