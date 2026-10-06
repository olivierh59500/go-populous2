# Populous II conversion status

The supplied Amiga executable has 29 active power slots. The number 26 refers
to the external resource catalog, not the number of spells. The Go program is
playable, but its original-game feature set is not yet complete. A selectable
power is not sufficient evidence that its original behavior has been ported.

## Engine and presentation

| Area | Current implementation | Remaining work |
|---|---|---|
| Resources | Original compression, masks, sprite differences, relocated descriptors, actual allocations/encoded filesystem and retained error retries; native startup/load/reload composition | Broader cross-effect/resource interaction and final end-to-end validation |
| Native rendering | Complete normal/alternate rendering, HUD, selected panel, hardware cursor, editor/debug text and retained protection waits; active-table CPU/DMA comparisons; shared physical owners; native desktop integration target runs original menu and terrain | Final interactive host-input and paired save/load validation |
| Campaign | 1,000 worlds, passwords, actual raw campaign loads/chooser/help/opponent, initial-menu→conquest/custom startup composition, world creators/templates and scripted events | Long conquest pacing and broader scenario/opponent validation |
| Terrain | Native four-hill generation and shared tree/boulder actor pool; complete native reference comparisons | Later environmental simulation and full linked actor occupancy |
| Terrain editing | Native height permissions, per-side prohibitions/enemy-land protection, propagated debit and picking | Remaining effect/editor-mode interactions |
| Followers | 399 usable records; all 35 active native state families in the register-bearing World dispatcher; 452 native follower passes and 168 composed main-physics/session comparisons, including filled pools | Final completion audit of native dispatch and composed gameplay evidence |
| Towns | 19 stages, native mixed support/cache, 49-cell compositor and native founding/economy/emigration in World; rare births allocate original neutral actors | Complete land AI, all neutral/environmental interactions and end-to-end campaign parity |
| Hero creation | Complete native leader conversion, marker relocation, town farm cleanup, attributes and retained fields; native routing/combat, Adonis splitting, Helen captivity and verified water/swamp immunities | Broader environment and transport combinations outside the native comparison corpora |
| Audio | 31 samples, 133 patterns, native four-channel device/CIA replay, raster DMA transport and shared CODE access; native desktop sound/queue/pause/resume integration | Original beam/CIA phase and analog/PWM fidelity, complete animation-triggered cues and interactive timing comparison |
| Saving | Raw GAM DOS save/load/list/overwrite with actual filesystem counts, partial operations and source pointer ownership; idle cache refresh without World regeneration; legacy typed APIs remain | Interactive save/load and two-player/cross-effect interoperability |
| Interface | Original raw initial menu/chooser/in-game/options/serial/profile/panel/input/editor/protection and deity/name/password controllers; complete physical file browser; native desktop menu/terrain/protection/deity/file captures | Broader interactive save/network/terrain transition validation |
| Multiplayer | Original raw command/serial protocol and retained handshake/resume; paired physical world constructors and commands match; configured asynchronous TCP/serial menu ports in native desktop | Desktop interactive network inputs, connection-failure recovery and paired save interoperability |
| Campaign progression | Native identity-based elimination, score/overflow, bolt awards, loss/win world steps, original result text/layout/wait, saved one-time application and actual raw381E result controller and animation decoder/final Zeus ending | Visual transition validation and full conquest pacing/script/AI parity |

## Power inventory

Mana lookup and elemental cost reductions are translated for every entry below.
Command numbers refer to the executable's original dispatcher. Coverage
notes distinguish translated bodies from bounded composed comparison cases.

| Power | Slot | Native command | Current behavior and remaining differences |
|---|---:|---:|---|
| Raise/lower land | 0 | 2 / 4 | Native height admission, prohibitions, propagated enemy-land protection/costs, raw sculpture and editor/battle-mode gates; actual mouse input and neighbor/pixel comparisons pass on all four landscapes |
| Papal magnet | 1 | 8 | Native marker/leader initialization, routing, waiting, mode changes, merging and double attrition; actual startup/17500/full-main/TCP compositions compared |
| Perseus | 2 | 36 | Native creation/art, hero decisions, fractional movement and combat; controlled original hero/contact/aftermath compositions compared |
| Plague | 3 | 78 | Complete native signed-kind/owner-byte cast, per-record vulture phase and original zero-damage prepass, actual merge/birth inheritance and retained death; World raw cast comparisons pass |
| Armageddon | 4 | 72 | Complete native eligible-state scan: plague cleanup and random conversion into the first four heroes; enables direct hero terrain raising, with admitted no-op recasts; World full-memory/RNG and saved continuation comparisons pass |
| Forest | 6 | 46 | Complete native sampled raw allocation, signed tile/owner aliases, created-tree deity metric, aging/burial and mixed fire/town interactions; full World memory/RNG comparisons pass; native age-based emergence/clipping is rendered |
| Renew land | 7 | 80 | Complete native raster-shape placement of tile 245, including occupied/hero cells and retained negative-owner DIVU aliases; full World memory/RNG comparisons pass; no separate popularity write exists in this body |
| Swamp | 8 | 54 | Complete native sampled placement and retained death, Adonis immunity, victim-side shallow restoration, leader release and terminal dispatch; full World memory/RNG comparisons pass |
| Fungus | 9 | 26 | Complete raw native collection/generation and shared-pool reuse, signed period/cadence, actual adjacent-BSS edge accesses and mature mortality through native follower dispatch; full World memory/RNG and old-save continuation comparisons pass |
| Adonis | 10 | 58 | Native creation, routing/combat and post-victory splitting/full-pool behavior; individual and controlled composed references compared |
| Roads | 12 | 42 | Native painting/removal, connected/slope art, fractional road-speed bonus/saturation and mask-based Fungus exclusion; controlled four-landscape comparisons pass, with broader environment combinations separate |
| City walls | 13 | 34 | Native linked placement, connected art/gates, sculpt protection, signed breaking and unsigned passage thresholds, fractional movement and terminal break art; live neighbor pointers retain source stale entries. Controlled cases cover blocked/admitted/breaking crossings; broader environment/gate combinations remain |
| Earthquake | 14 | 40 | Native directed creation, fissure branching, original terrain reconstruction and fade; raw slot/full-pool aliases retained and World comparisons pass |
| Batholith | 15 | 48 | Native sampled raising/boulder allocation with oracle references and held-button use; remaining destruction interactions pending |
| Heracles | 16 | 60 | Native creation, wrapping population doubling, capped speed bonus and shared combat/routing; controlled original combat and town-destruction compositions compared |
| Lightning | 18 | 28 / 30 / 32 | Native marker/activation/bolts, gradual victims, procedural beams and living-town native farm reform; remaining terrain/hero cleanup branches pending |
| Whirlwind | 19 | 22 | Complete native raw phases/motion, linked pickup, town farm collapse, transport/landing/release and water-child creation; mutable register/source/owner-word aliases retained; World full-memory and saved continuation comparisons pass |
| Storm | 20 | 64 | Native raw cloud creation/admission, thunder/cooldown, exact terrain scorch and linked victim scan; World/runtime/save comparisons pass |
| Odysseus | 21 | 66 | Native creation, doubled/capped speed, original art and shared routing/combat; controlled original combat compositions compared |
| Hurricane wind | 22 | 76 | Native four-direction parcel scans and mixed-actor fractional pushing, boundary cleanup, overlay clearing and raw pointer guard; complete World comparisons and saved continuation pass |
| Fire column | 24 | 6 | Complete raw native phases/routing/movement, exact new-cell scorch and linked damage, founder/hero/leader/town branches, retained mortality and scripted owner-word creation; full World memory/RNG and save comparisons pass |
| Fire rain | 25 | 38 | Native unlinked meteor delay, decoded falling art, height-aware impact and exact victim/terrain callbacks; World/save continuation pass |
| Volcano | 26 | 62 | Native crater growth/terrain preservation, raw fire-column eruption, lava/Basalt creation and contact states; World full-eruption comparisons pass |
| Achilles | 27 | 68 | Native creation, original art, routing/combat and victim/death controllers; controlled original combat compositions compared |
| Whirlpool | 31 | 24 | Native four-water admission, terrain animation, motion, direct coast lowering, lifetime and ordered shared-pool execution; broader terrain/actor interactions pending |
| Basalt | 30 | 74 | Native linked propagation actors, four cardinal directions, persistent terrain prefix and original sculpture shapes; remaining environmental interactions pending |
| Baptismal fonts | 32 | 52 | Complete native sampled placement, original delayed conversion and hero tables, leader release and fractional movement; full World memory/RNG comparisons pass |
| Helen | 33 | 70 | Native targeting/capture without owner conversion, fractional captive routing/link repair, cleanup/release and water immunity; complete controlled capture→removal comparisons. Broader collateral/environment combinations remain |
| Tidal wave | 34 | 56 | Native four adjacent-water fronts, fixed fractional speed, lateral cloning/newborn cadence, shore lowering and height/Basalt barriers; World pool/map/height and saved continuation comparisons pass; drowning uses the shared native water prepass |

The independent Challenge executable and its scenario extension are separate
from the main two-disk game and have not been integrated.

## Evidence

[NATIVE_RULES.md](NATIVE_RULES.md) records translated routines and their offsets.
Tests compare native terrain/ground-effect references, validate original assets,
and exercise saves, group limits, mana, geometry and audio determinism. These
checks establish the documented slices of behavior; they do not establish
complete Amiga gameplay parity.

## Live runtime integration

The default `cmd/populous2` command and its `cmd/populous2-native` alias run
the assembled register-bearing session,
physical resource host, original screen/input and four-channel audio. Bounded
desktop captures verify its initial menu, terrain and protection requester.
The raw result, award and ending controllers and animation decoder are
verified against the original CPU and bound in the native desktop. Complete
stock/campaign pacing and broader interactive
validation remain part of the final fidelity audit. The inherited diagnostic
engine remains available as `cmd/populous2-legacy`.

The [original instruction manual](https://ts.popre.net/Archive/Downloads/Docs/populous2.pdf)
provides the player-facing behavior; the supplied executable determines the
Amiga-specific slot and routine mappings.
