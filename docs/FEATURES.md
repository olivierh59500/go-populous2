# Populous II conversion status

The supplied Amiga executable has 29 active power slots. The number 26 refers
to the external resource catalog, not the number of spells. The Go program is
playable, but its original-game feature set is not yet complete. A selectable
power is not sufficient evidence that its original behavior has been ported.

## Engine and presentation

| Area | Current implementation | Remaining work |
|---|---|---|
| Resources | Original compression, tile fragments, masks, sprite differences and relocated descriptors | Original menu composition and font use |
| Campaign | 1,000 worlds, passwords, full random seed, starting templates and opponent experience | All scenario options, scripted events and opponent personalities |
| Terrain | Native four-hill generation; eight complete vertex-grid comparisons against the 68000 routine | Native trees, boulders and later environmental simulation |
| Terrain editing | Propagation, per-change mana debit, original graphics-bank selection and slope-aware picking | Every original construction restriction and effect interaction |
| Followers | 399 usable records, 16-bit occupancy references and persistent state | Native linked tile occupancy, subpixel movement, combat and invention rules |
| Towns | 19 stages, original work/growth/mana/capacity/emigration tables | Exact farm repainting, city composition and native land AI |
| Hero creation | Correct attributes/art, Adonis splitting, Helen captivity and verified water/swamp immunities | Native subpixel routing, combat and remaining state transitions |
| Audio | 31 samples, 133 patterns, native note/envelope data and cast cue bindings | Hardware timing/mixing comparison and all animation-triggered cues |
| Saving | Validated version 4 Go saves, wide RNG state and earlier water-ID migration | Original Amiga GAM interoperability and deity profiles |
| Interface | Playable menu, camera, minimap, power selection, pause, demo and saving | Original menus, god creation, experience allocation, statistics and end sequence |
| Multiplayer | Deterministic state hashes retained in the engine | Two-player transport and shared command scheduling |
| Campaign progression | Current prototype advances one world after a victory | Native score, awards, world skipping and final Zeus outcome |

## Power inventory

Mana lookup and elemental cost reductions are translated for every entry below.
Command numbers refer to the executable's original dispatcher, and help locate
the remaining simulation routines.

| Power | Slot | Native command | Current behavior and remaining differences |
|---|---:|---:|---|
| Raise/lower land | 0 | 2 / 4 | Native graphics and cost accounting; inherited height propagation; construction options incomplete |
| Papal magnet | 1 | 8 | Leader destination and native sound cues; inherited follower routing |
| Perseus | 2 | 36 | Native conversion/art; inherited knight movement and combat |
| Plague | 3 | 78 | Actor infection, contact, suppressed town mana, Armageddon removal and native vulture art/caw; detailed disease states pending |
| Armageddon | 4 | 72 | Global battle and plague removal; inherited combat/central gathering |
| Forest | 6 | 46 | Provisional tree markers; native tree actor allocation/animation and popularity pending |
| Renew land | 7 | 80 | Native sampled placement and tile 245; later greenery spread and popularity pending |
| Swamp | 8 | 54 | Native placement/tiles and either-side entry deaths; Heracles immunity; native death animation/options pending |
| Fungus | 9 | 26 | Provisional propagation and damage; native controller/automaton pending |
| Adonis | 10 | 58 | Native conversion/art and post-victory hero splitting; native movement/combat and pool-full edge state pending |
| Roads | 12 | 42 | Continuous painting/removal and native connected/slope tile art; walking-speed and fungus-barrier interactions pending |
| City walls | 13 | 34 | Provisional line obstacles; original connected-wall rules and breaking/climbing pending |
| Earthquake | 14 | 40 | Inherited earthquake with native cost/sound; directed native fault pending |
| Batholith | 15 | 48 | Provisional raising/boulders; original held-button expansion pending |
| Heracles | 16 | 60 | Native double population and speed bonus; native combat/routing pending |
| Lightning | 18 | 28 / 30 | Provisional area damage; native target/effect animation pending |
| Whirlwind | 19 | 22 | Provisional moving damage and native paired sounds; original trajectory/state machine pending |
| Storm | 20 | 64 | Provisional timed area damage; native rain/lightning/terrain simulation pending |
| Odysseus | 21 | 66 | Native doubled speed attribute/art; runtime movement still inherited |
| Hurricane wind | 22 | 76 | Provisional directed movement/damage; native pushing and terrain interactions pending |
| Fire column | 24 | 6 | Provisional wandering damage; native uphill routing, burning and lifetime pending |
| Fire rain | 25 | 38 | Provisional timed damage; native drops and burn propagation pending |
| Volcano | 26 | 62 | Inherited volcano with native cost/sound; native lava, basalt and damage recovery pending |
| Achilles | 27 | 68 | Native conversion/art; provisional burning; native movement/combat pending |
| Whirlpool | 30 | 74 | Provisional stationary water effect; native coastline erosion and multiplication pending |
| Basalt | 31 | 24 | Provisional straight raised path; native bridge construction/animation pending |
| Baptismal fonts | 32 | 52 | Native sampled placement and tiles 143–144; faith reversal on entry; original entry animation/immunities pending |
| Helen | 33 | 70 | Capture without faith conversion, follower chain, release on death and water immunity; exact subpixel routing/collateral death animation pending |
| Tidal wave | 34 | 56 | Provisional directed wave; native world-wide water simulation pending |

The independent Challenge executable and its scenario extension are separate
from the main two-disk game and have not been integrated.

## Evidence

[NATIVE_RULES.md](NATIVE_RULES.md) records translated routines and their offsets.
Tests compare native terrain/ground-effect references, validate original assets,
and exercise saves, group limits, mana, geometry and audio determinism. These
checks establish the documented slices of behavior; they do not establish
complete Amiga gameplay parity.

The [original instruction manual](https://ts.popre.net/Archive/Downloads/Docs/populous2.pdf)
provides the player-facing behavior; the supplied executable determines the
Amiga-specific slot and routine mappings.
