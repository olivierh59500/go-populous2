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
| Followers | 399 usable records, native ordinary search/pressure/road decisions and 8.8 motion; full terrain prepass, water/conversion/burning, magnet/captive routes, exact crossing admission and retained contact/combat outcomes in World | Full linked effects, remaining animation boundaries and inventions |
| Towns | 19 stages, native mixed support/cache, 49-cell compositor and native founding/economy/emigration in World; rare births allocate original neutral actors | Complete land AI, all neutral/environmental interactions and end-to-end campaign parity |
| Hero creation | Complete native leader conversion, marker relocation, town farm cleanup, attributes and retained fields; native routing/combat, Adonis splitting, Helen captivity and verified water/swamp immunities | Remaining combined environmental, wall-climb and animation interactions |
| Audio | 31 samples, 133 patterns, native note/envelope data and cast cue bindings | Hardware timing/mixing comparison and all animation-triggered cues |
| Saving | Version 23 retains native frame counter, game/profile identity, command weights and latched/applied campaign results alongside retained actors and controllers | Original Amiga GAM interoperability |
| Interface | Playable controls, native deity faces/experience allocation/profile codes, demo and saving | Original menu composition, statistics and end sequence |
| Multiplayer | Deterministic state hashes retained in the engine | Two-player transport and shared command scheduling |
| Campaign progression | Native identity-based elimination, score/overflow, bolt awards, loss/win world steps, final-world reset and saved one-time application | Original result-screen composition, final Zeus presentation and full conquest pacing/script/AI parity |

## Power inventory

Mana lookup and elemental cost reductions are translated for every entry below.
Command numbers refer to the executable's original dispatcher, and help locate
the remaining simulation routines.

| Power | Slot | Native command | Current behavior and remaining differences |
|---|---:|---:|---|
| Raise/lower land | 0 | 2 / 4 | Native height admission, prohibitions, propagated enemy-land protection and costs; inherited propagation; special editor/battle modes pending |
| Papal magnet | 1 | 8 | Native marker/leader routing, destination waiting, mode changes, merging and original double attrition; remaining command/native-init fidelity |
| Perseus | 2 | 36 | Complete native creation, original art, shared native hero decisions and combat; combined interaction coverage pending |
| Plague | 3 | 78 | Complete native signed-kind/owner-byte cast, per-record vulture phase and original zero-damage prepass, actual merge/birth inheritance and retained death; World raw cast comparisons pass |
| Armageddon | 4 | 72 | Complete native eligible-state scan: plague cleanup and random conversion into the first four heroes; enables direct hero terrain raising, with admitted no-op recasts; World full-memory/RNG and saved continuation comparisons pass |
| Forest | 6 | 46 | Complete native sampled raw allocation, signed tile/owner aliases, created-tree deity metric, aging/burial and mixed fire/town interactions; full World memory/RNG comparisons pass; native age-based emergence/clipping is rendered |
| Renew land | 7 | 80 | Complete native raster-shape placement of tile 245, including occupied/hero cells and retained negative-owner DIVU aliases; full World memory/RNG comparisons pass; no separate popularity write exists in this body |
| Swamp | 8 | 54 | Native placement/tiles, shallow restoration, hero immunity and retained death/terminal dispatch |
| Fungus | 9 | 26 | Native seed/collection, staged B3/S23 automaton, cadence, mature-tile mortality/Adonis immunity and retained death art; full linked death state and adjacent-BSS edge behavior pending |
| Adonis | 10 | 58 | Complete native creation, shared routing/combat and verified native post-victory split/full-pool behavior; combined interactions pending |
| Roads | 12 | 42 | Continuous painting/removal and native connected/slope tile art; walking-speed and fungus-barrier interactions pending |
| City walls | 13 | 34 | Native placement/art/gates, saves, sculpt protection, crossing thresholds and terminal break art; fractional climb/hero attack states pending |
| Earthquake | 14 | 40 | Native directed creation, fissure branching, original terrain reconstruction and fade; raw slot/full-pool aliases retained and World comparisons pass |
| Batholith | 15 | 48 | Native sampled raising/boulder allocation with oracle references and held-button use; remaining destruction interactions pending |
| Heracles | 16 | 60 | Complete native creation, wrapping population doubling, capped speed bonus and shared native combat/routing; combined interactions pending |
| Lightning | 18 | 28 / 30 / 32 | Native marker/activation/bolts, gradual victims, procedural beams and living-town native farm reform; remaining terrain/hero cleanup branches pending |
| Whirlwind | 19 | 22 | Complete native raw phases/motion, linked pickup, town farm collapse, transport/landing/release and water-child creation; mutable register/source/owner-word aliases retained; World full-memory and saved continuation comparisons pass |
| Storm | 20 | 64 | Native raw cloud creation/admission, thunder/cooldown, exact terrain scorch and linked victim scan; World/runtime/save comparisons pass |
| Odysseus | 21 | 66 | Complete native creation, doubled/capped speed, original art and shared native routing/combat; combined interactions pending |
| Hurricane wind | 22 | 76 | Native four-direction parcel scans and mixed-actor fractional pushing, boundary cleanup, overlay clearing and raw pointer guard; complete World comparisons and saved continuation pass |
| Fire column | 24 | 6 | Complete raw native phases/routing/movement, exact new-cell scorch and linked damage, founder/hero/leader/town branches, retained mortality and scripted owner-word creation; full World memory/RNG and save comparisons pass |
| Fire rain | 25 | 38 | Native unlinked meteor delay, decoded falling art, height-aware impact and exact victim/terrain callbacks; World/save continuation pass |
| Volcano | 26 | 62 | Native crater growth/terrain preservation, raw fire-column eruption, lava/Basalt creation and contact states; World full-eruption comparisons pass |
| Achilles | 27 | 68 | Complete native creation, original art and shared native routing/combat; provisional incidental burning and combined interactions pending |
| Whirlpool | 31 | 24 | Native four-water admission, terrain animation, motion, direct coast lowering, lifetime and ordered shared-pool execution; broader terrain/actor interactions pending |
| Basalt | 30 | 74 | Native linked propagation actors, four cardinal directions, persistent terrain prefix and original sculpture shapes; remaining environmental interactions pending |
| Baptismal fonts | 32 | 52 | Native sampled placement and tiles 143–144; faith reversal on entry; original entry animation/immunities pending |
| Helen | 33 | 70 | Capture without faith conversion, follower chain, release on death and water immunity; exact subpixel routing/collateral death animation pending |
| Tidal wave | 34 | 56 | Native four adjacent-water fronts, fixed fractional speed, lateral cloning/newborn cadence, shore lowering and height/Basalt barriers; World pool/map/height and saved continuation comparisons pass; drowning uses the shared native water prepass |

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
