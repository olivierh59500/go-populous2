# Populous II conversion status

The supplied Amiga executable has 29 active power slots. The number 26 refers
to the external resource catalog, not the number of spells. The Go program is
playable, but its original-game feature set is not yet complete. A selectable
power is not sufficient evidence that its original behavior has been ported.

## Engine and presentation

| Area | Current implementation | Remaining work |
|---|---|---|
| Resources | Original compression, tile fragments, masks, sprite differences and relocated descriptors | Original menu composition and font use |
| Campaign | 1,000 worlds, passwords, template mana/attrition, per-side rules/requester and opponent experience | Scripted events and opponent personalities |
| Terrain | Native four-hill generation and shared tree/boulder actor pool; complete native reference comparisons | Later environmental simulation and full linked actor occupancy |
| Terrain editing | Native height permissions, per-side prohibitions/enemy-land protection, propagated debit and picking | Remaining effect/editor-mode interactions |
| Followers | 399 usable records, native ordinary search/pressure/road decisions and 8.8 motion; World entry, delayed contact, combat, rewards and retained aftermath; raw actor/deity/marker bytes | Complete ordinary prepass, remaining hazard/captive handlers, native magnet decisions, hero creation/motion boundaries and inventions |
| Towns | 19 stages, native mixed support/cache, 49-cell compositor and founding/contact; separately verified native economy | Full native economy/birth dispatcher, land AI and end-to-end town parity |
| Hero creation | Correct attributes/art, Adonis splitting, Helen captivity and verified water/swamp immunities | Native subpixel routing, combat and remaining state transitions |
| Audio | 31 samples, 133 patterns, native note/envelope data and cast cue bindings | Hardware timing/mixing comparison and all animation-triggered cues |
| Saving | Version 17 retains actor/deity/marker bytes, managed contacts/combat outcomes, original control latches, town overlays and per-side rules | Original Amiga GAM interoperability |
| Interface | Playable controls, native deity faces/experience allocation/profile codes, demo and saving | Original menu composition, statistics and end sequence |
| Multiplayer | Deterministic state hashes retained in the engine | Two-player transport and shared command scheduling |
| Campaign progression | Current prototype advances one world after a victory | Native score, awards, world skipping and final Zeus outcome |

## Power inventory

Mana lookup and elemental cost reductions are translated for every entry below.
Command numbers refer to the executable's original dispatcher, and help locate
the remaining simulation routines.

| Power | Slot | Native command | Current behavior and remaining differences |
|---|---:|---:|---|
| Raise/lower land | 0 | 2 / 4 | Native height admission, prohibitions, propagated enemy-land protection and costs; inherited propagation; special editor/battle modes pending |
| Papal magnet | 1 | 8 | Leader destination and native sound cues; inherited follower routing |
| Perseus | 2 | 36 | Native conversion/art; inherited knight movement and combat |
| Plague | 3 | 78 | Actor infection, contact, suppressed town mana, Armageddon removal and native vulture art/caw; detailed disease states pending |
| Armageddon | 4 | 72 | Global battle and plague removal; inherited combat/central gathering |
| Forest | 6 | 46 | Native sampled allocation, original variants, signed aging/burial counters and rendering; popularity and remaining actor interactions pending |
| Renew land | 7 | 80 | Native sampled placement and tile 245; separate from the fungus automaton; popularity interactions pending |
| Swamp | 8 | 54 | Native placement/tiles, victim-side shallow rule and Heracles immunity; native death animation pending |
| Fungus | 9 | 26 | Native seed/collection, staged B3/S23 automaton, cadence, mature-tile mortality/Adonis immunity and retained death art; full linked death state and adjacent-BSS edge behavior pending |
| Adonis | 10 | 58 | Native conversion/art and post-victory hero splitting; native movement/combat and pool-full edge state pending |
| Roads | 12 | 42 | Continuous painting/removal and native connected/slope tile art; walking-speed and fungus-barrier interactions pending |
| City walls | 13 | 34 | Native placement/art/gates, saves, sculpt protection, crossing thresholds and terminal break art; fractional climb/hero attack states pending |
| Earthquake | 14 | 40 | Inherited earthquake with native cost/sound; directed native fault pending |
| Batholith | 15 | 48 | Native sampled raising/boulder allocation with oracle references and held-button use; remaining destruction interactions pending |
| Heracles | 16 | 60 | Native double population and speed bonus; native combat/routing pending |
| Lightning | 18 | 28 / 30 / 32 | Native marker/activation/bolts, gradual victims, procedural beams and living-town native farm reform; remaining terrain/hero cleanup branches pending |
| Whirlwind | 19 | 22 | Native fixed-point trajectory, phases, experience lifetime, composite art and frame cues; pickup/release, town collapse and child whirlpools pending |
| Storm | 20 | 64 | Provisional timed area damage; native rain/lightning/terrain simulation pending |
| Odysseus | 21 | 66 | Native doubled speed attribute/art; runtime movement still inherited |
| Hurricane wind | 22 | 76 | Provisional directed movement/damage; native pushing and terrain interactions pending |
| Fire column | 24 | 6 | Native fixed-point pool, phases, uphill routing, experience lifetime and current-cell burns; full mixed actor-list parity/timing pending |
| Fire rain | 25 | 38 | Provisional timed damage; native drops and burn propagation pending |
| Volcano | 26 | 62 | Inherited volcano with native cost/sound; native lava, basalt and damage recovery pending |
| Achilles | 27 | 68 | Native conversion/art; provisional burning; native movement/combat pending |
| Whirlpool | 31 | 24 | Native four-water admission, terrain animation, motion, direct coast lowering, lifetime and ordered shared-pool execution; broader terrain/actor interactions pending |
| Basalt | 30 | 74 | Native linked propagation actors, four cardinal directions, persistent terrain prefix and original sculpture shapes; remaining environmental interactions pending |
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
