# Independent engine implementation audit

Initial review date: 6 October 2026. The reviewed runtime entry point is
`cmd/populous2-go`; the regular `cmd/populous2` launcher still uses the earlier
translation. This document distinguishes working Go subsystems, available
player actions, and original-game features that still need integration. The
detailed review below records the earlier migration snapshot; the current
status section supersedes items that have since been completed.

## Current integration status

Subsequent work on 6 October added all 29 admitted power commands and their
presentation, typed mixed-actor membership, the original world-script executor
and neutral inventions, construction/visibility/hazard scenario handling,
campaign results/profile progression and the full animated ending.

The independent menu now includes options, a detached map editor, custom game
configuration, save/load actions and TCP multiplayer setup. The options/editor
tests verify atomic Apply/Cancel and reject modification during a network
session. Independent saves preserve validated continuation state; the original
GAM codec is being extended with file-only artwork-token compatibility.

Town art now includes neighboring structures and dynamic population/faction
flags. The complete ending has compact indexed frames and an exact 150-PAL
framebuffer comparison. Neutral sprites and the source startup scenery sampler
are imported or implemented through named asset/game fields. Animation cues are
connected through a transition gate, and music and sound effects can be muted
separately.

The remaining checks are more specific than the original broad list:
whole-game follower/contact and AI fidelity, complete original GAM
continuation/export coverage, and final visual, audio, interaction and mobile
validation of the assembled application. World-code entry, opponent setup
details and real, separately paced power-help previews are connected. The
default launcher switch and release packaging still require the assembled
independent target to pass those checks. A catalogue flag or isolated test is
not a substitute for those final integration checks.

### Assembled-engine verification

The standalone build embeds the exported asset package and has been run from
outside the repository without any original executable or disk-image path.
Mouse controls now expose powers, tactical modes, help, options and the save
browser. Save replacement requires confirmation; invalid or oversized files
cannot replace the live world. Original GAM import is connected through a
separate file codec, and an original campaign save has been loaded, advanced,
exported and reloaded through the independent interface.

All 29 powers pass a lifecycle test covering more than 400 simulation passes
and three saved continuation checkpoints. Mixed actor membership is updated
at actual creation, movement and removal points. Retained deaths clean up
leaders, claims and statistics once while keeping their terminal artwork.
These checks establish valid continuation, not complete original-game parity.

A bounded comparison of world 12, landscape 0, seed `0x15b0`, now matches 160
simulation passes for all 4,225 terrain corners, the 400 follower slots'
identity/population/state/position/town stage, both mana balances and RNG.
It found and corrected passive mana, contact waiting/completion and the search
eligibility of cultivated land. This compares one physics pass at a time;
it does not prove every input/render interaction, campaign world or effect
combination. Original GAM continuation has separate active-state tests.

## Architecture

The independent runtime loads PNG atlases, named animation compositions,
palette/font metadata, signed PCM, musical events, landscape files and the
1,000-world campaign. Its project dependencies are `internal/app`,
`internal/engine`, `internal/visualassets` and `internal/music`. The dependency
test in `cmd/populous2-go/dependencies_test.go` rejects the executable-based
translation, HUNK loader and earlier game packages.

The two export commands may inspect a private original executable during
asset preparation. Their output contains artwork and audio data, not a renamed
instruction stream or relocated memory image. Generated original art/audio
remains excluded from Git.

Replacing the default launcher is a separate completion step. The reference
commands should remain explicitly identified as reference applications after
the independent version becomes the default.

## Available power families

At the initial review, 26 of the 29 catalogue entries were enabled. All 29
commands are now admitted. An enabled flag
means an action is admitted by the Go dispatcher; it does not establish full
parity for every interaction with every other power.

| Family | Enabled actions | Remaining disabled actions |
|---|---|---|
| People | Raise/lower, Papal magnet, Perseus, Plague, Armageddon | None |
| Plants | Trees, Flowers, Swamp, Fungus, Adonis | None |
| Earth | Road, Wall, Earthquake, Batholith, Heracles | None |
| Air | Lightning, Whirlwind, Odysseus | Storm, Wind |
| Fire | Fire Column, Fire Rain, Achilles | Volcano |
| Water | Basalt, Whirlpool, Baptism, Helen, Tsunami | None |

Storm, Wind and Volcano already have portions of their controllers in source.
Their admission, mixed-pool scheduling and artwork are now connected. Completion
requires their actual casting paths, shared-pool order, victims, ground changes,
audio cues and visible artwork to work together. Wind movement still needs the
ordered mixed-actor traversal that can visit a moved follower later in the
same pass. Tests for isolated effects cannot replace this interaction proof.

## Simulation and terrain

The world uses named Go terrain, follower, deity, town, hero and effect values.
Ordinary followers have fixed-point positions, ordered searches, typed contact
and combat state. Six hero kinds, immunity helpers, reciprocal claims and
specific contact behaviors exist. Settlement support has 19 stages and a
source-number comparison corpus. The shared effect budget is 250; scenery
and walls each have 200 slots. The simulation processes the pools in a defined
order rather than granting each power a separate unlimited allocation budget.

Remaining simulation work includes:

- Complete mixed-actor cell membership for every creator, mover and cleanup,
  including walls, scenery, magnets, ground-linked effects and followers.
  Preserve insertion/traversal order where it affects repeated movement,
  lightning blocking, contact and neutral interactions.
- Initial scenery generation and all neutral/environmental actors. At review
  time, the independent world creates the terrain and two factions, but does
  not consume the original 60-byte world script to establish ambient effects.
- Campaign scenario execution and timed disasters. `WorldParameters` is
  decoded but no independent world-script executor currently consumes it.
- Full terrain-hazard lifecycle composition. Fatal-water mode, ordinary water
  deaths and fatal-ground cases need their complete named transitions and
  artwork, rather than generic attrition or immediate removal.
- Reconcile permanent basalt surfaces with new corner geometry. Terrain edits
  must preserve basalt while updating its surface-shape artwork and must clear
  transient overlays consistently for every natural and player operation.

Deliberately bounded behavior replaces the original's corrupt out-of-map
memory accesses and instruction-byte aliases. Such cases should be documented
as compatibility boundaries, not reproduced by adding CPU memory to the Go
world. Ordinary valid-map behavior still needs numeric and visible comparison.

## Scenario options

All ten scenario options are decoded to named booleans. Shallow-swamp handling
is bound to the follower path. New construction-admission and overview
visibility helpers were being integrated during this review; the remaining
end-to-end requirements are:

| Option | Required behavior |
|---|---|
| Build anywhere / sea-level building | Bind the named admission helper and verify original vertex-height and proximity rules |
| Forbid enemy terrain | Reject the original restricted terrain-edit targets |
| Forbid raise / forbid lower | Apply each prohibition before mutation or mana charge |
| Fatal water | Original fatal water entry, immunity and retained death |
| Hide enemy on map | Bind the visibility helper to the local-side overview |
| Disable emigration | Verify the manual/right-button sprog restriction without incorrectly disabling unrelated natural births |
| Hide disasters on map | Filter overview disaster markers according to the local-side rule |

An encoded option alone is not an implemented scenario. The admission tests
must assert unchanged mana and world state on rejection, and side-specific
options must be exercised with differing local and opponent values.

## Campaign, profile and results

The engine decodes all 1,000 worlds and their passwords. Profile face selection,
experience allocation, profile passwords, score arithmetic and score-based
world progression have independent Go implementations and numeric tests.
The application transfers the selected profile's experience into a new game.

Completion still requires:

- Bind every score statistic to actual events. `Metric` and `LeaderLosses`
  currently exist as fields without complete event updates. Population/mana
  peaks, battle wins and weighted power use have explicit recorders.
- Original opponent briefing, power-help previews, award presentation and all
  result metrics. The current brief/result screens are a smaller Go interface.
- Animated campaign ending and text playback. The asset exporter currently
  exports only the first ending bitmap; completing world 999 returns to the
  menu with a message instead of playing the original ending sequence.
- Preserve profile and campaign position through app-level persistence,
  including continued experience allocation after a victory.

## Menus and controls

The independent application currently has five screens: main menu, profile,
conquest briefing, play and campaign result. The power chooser, terrain picking,
camera movement, tactical mode keys and directed-power rotation are working
Go input paths.

The complete application still needs the original menu capabilities:

- Custom/practice and map-editor modes, their terrain and faction tools, and
  navigation back to play.
- Options, pause, sound/music controls and side-specific scenario editing.
- In-game save/load browser, overwrite/error states and current profile/world
  restoration.
- World-code entry, opponent details and interactive, correctly paced power
  help rather than world selection solely through arrow keys.
- Multiplayer setup and error/recovery UI.

The current power menu uses text buttons over the original background. A full
Go UI can keep maintainable widgets while retaining the original capabilities;
it must not acknowledge a button that has no underlying implementation.

## Rendering

Terrain tiles, followers, hero walking art, trees/boulders, many effect and
victim states, roads, walls, overview and palettes use imported artwork. The
image loader has standalone synthetic tests and optional full private-art
round-trip comparisons.

Important remaining presentation checks are:

- Composite towns beyond their center image: adjacent town pieces, stage
  composition, population-dependent banner height and owner/tick variants.
  The current `town/<side>/<stage>` export uses initial population zero.
- Draw the rally magnet/leader indicators and original HUD quantities with
  usable clickable controls, rather than relying on keyboard shortcuts and
  an otherwise static background.
- Bind plague overlays, baptism conversion, every retained water/fatal death
  and remaining effect state to its named artwork. Review their precedence
  against contact, combat and hero rendering.
- Preserve correct depth order and viewport clipping through high terrain,
  large effects, neighboring town pieces and the overview border.
- Respect the local-side scenario visibility rules in the overview.

The independent engine must also remain smooth at the calibrated simulation
rate. The display/input cadence is 50 PAL updates per second; simulation work
has its own cadence. Layer count or software compositing must not alter the
world speed or audio timeline.

## Audio

The Go sequencer loads semantic score events, envelopes and PCM and has replay
comparison tests. Background music is connected to the Ebitengine audio
player. The current application triggers cue 78 when starting a game and after
a successful click.

Artwork-frame `SoundCue` metadata now feeds a gate that admits a cue once per
simulation pass. Repeated display frames do not restart it. The soundtrack
and foreground effects have separate mute controls. Complete audible-event
coverage and shared-channel priority still need assembled-game comparison;
PCM decoder parity alone does not establish every gameplay trigger.

## Persistence, multiplayer and Android

`internal/engine/save.go` has a versioned semantic snapshot, validates detached
candidates and includes private motion, RNG and effect reservation state.
An app-level session wraps it with profile, camera, local camp, campaign
position, selected power, direction, custom-game mode and pause state. The
save browser lists JSON/GAM files, confirms replacement, and validates detached
candidates before replacing the current game.

Original GAM interchange uses the separate `internal/gamcodec` file boundary.
It translates original user-save fields to named state and preserves reserved
file fields outside the world. New files can be created from Go state. Tests
cover original byte round trips, metadata edits, ordinary play, AI, combat,
retained cleanup, transport and lightning continuation. Pending AI commands
outside the original saved span and unsupported file phases are rejected.

`internal/network` now synchronizes two semantic Go worlds with validated
initial snapshots and ordered lockstep input rounds. Its TCP tests cover
framing, mismatched rules/digests and disconnected rounds before mutation.
The application exposes host/join setup and an asynchronous controller, with
local-side controls/results and visible disconnect errors. Displayed worlds
remain detached from in-flight TCP work. A failed round pauses the session;
automatic reconnection/resynchronization is not implemented. Saves and live
rule editing require leaving the two-player session.

No Android launcher or build target for this independent runtime was present
at review. After the desktop engine is complete, its lifecycle/input/audio
adapter and asset installation should be connected to the Android guide and
verified with a release build; an existing earlier APK does not demonstrate
that the replacement runtime is mobile-ready.

## Completion evidence

The existing tests establish individual boundaries: terrain generation,
movement/contact/combat, town support, profile/passwords, campaign arithmetic,
sample replay and several original numeric effect traces. Optional private
tests compare original art and additional source traces without distributing
the original assets.

Complete migration evidence must additionally include an assets-only clean
build/run, all 29 usable powers, all scenario behaviors, representative
cross-family effect combinations, a full campaign result/progression/ending
path, app save/load and multiplayer continuation. A final desktop visual/audio
comparison should use representative early and later worlds. The independent
launcher can then replace the default while the reference applications remain
separately named tools.
