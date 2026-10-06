# Original-reference implementation status

The supplied Amiga executable has 29 active power slots. The number 26 refers
to the external resource catalog, not the number of spells. The Go program is
playable by default through the independent Go engine described in
[GO_ENGINE.md](GO_ENGINE.md). This document records the earlier translation,
retained as `cmd/populous2-native` for comparison. Source-controller and
whole-main comparisons establish the documented scenarios; a selectable
power alone is not used as evidence of original behavior.

## Engine and presentation

| Area | Current implementation | Fidelity/coverage boundaries |
|---|---|---|
| Resources | Original compression, masks, sprite differences, relocated descriptors, actual allocations/encoded filesystem and retained error retries; native startup/load/reload composition | Broader cross-effect/resource interaction and final end-to-end validation |
| Native rendering | Complete normal/alternate rendering, HUD, selected panel, hardware cursor, editor/debug text and retained protection waits; active-table CPU/DMA comparisons; shared physical owners; native desktop integration target runs original menu and terrain | Native host-input and paired save/load checks pass; further multi-window sessions can extend coverage |
| Campaign | 1,000 worlds, passwords, actual raw campaign loads/chooser/help/opponent, initial-menu→conquest/custom startup composition, world creators/templates and scripted events | Long conquest pacing and broader scenario/opponent validation |
| Terrain | Native four-hill generation and shared tree/boulder actor pool; complete native reference comparisons | Later environmental simulation and full linked actor occupancy |
| Terrain editing | Native height permissions, per-side prohibitions/enemy-land protection, propagated debit and picking | Remaining effect/editor-mode interactions |
| Followers | 399 usable records; all 35 active native state families in the register-bearing World dispatcher; 452 native follower passes and 168 composed main-physics/session comparisons, including filled pools | All native families are bound; configured corpora and stock/campaign main frames have bounded evidence |
| Towns | 19 stages, native mixed support/cache, 49-cell compositor and native founding/economy/emigration in World; rare births allocate original neutral actors | Native AI and neutral/environment controllers compared; additional interaction scenarios can extend coverage |
| Hero creation | Complete native leader conversion, marker relocation, town farm cleanup, attributes and retained fields; native routing/combat, Adonis splitting, Helen captivity and verified water/swamp immunities | Broader environment and transport combinations outside the native comparison corpora |
| Audio | 31 samples, 133 patterns, native four-channel device/CIA replay, raster DMA transport and shared CODE access; native desktop sound/queue/pause/resume integration | Original beam/CIA phase and analog/PWM fidelity, complete animation-triggered cues and interactive timing comparison |
| Saving | Raw GAM DOS save/load/list/overwrite with actual filesystem counts, partial operations and source pointer ownership; idle cache refresh without World regeneration; legacy typed APIs remain | Actual browser save/load and paired GAM continuation verified; additional save/revision scenarios can extend coverage |
| Interface | Original raw initial menu/chooser/in-game/options/serial/profile/panel/input/editor/protection and deity/name/password controllers; complete physical file browser; native desktop menu/terrain/protection/deity/file captures | Broader interactive save/network/terrain transition validation |
| Multiplayer | Original raw command/serial protocol and retained handshake/resume; paired physical world constructors and commands match; configured asynchronous TCP/serial menu ports in native desktop | Actual TCP and source failure/retry/save paths verified; physical serial timing and further two-window sessions remain host/coverage boundaries |
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
| Batholith | 15 | 48 | Native DF68 sampled raise/boulder allocation and actual D81E terrain effects; creator and shared consequence controllers compared independently and in configured simulation |
| Heracles | 16 | 60 | Native creation, wrapping population doubling, capped speed bonus and shared combat/routing; controlled original combat and town-destruction compositions compared |
| Lightning | 18 | 28 / 30 / 32 | Native marker/activation/dismissal, bolts, gradual victims, beams and town reform; complete FX/scorch/unlink and siege/hero cleanup controllers have original references |
| Whirlwind | 19 | 22 | Complete native raw phases/motion, linked pickup, town farm collapse, transport/landing/release and water-child creation; mutable register/source/owner-word aliases retained; World full-memory and saved continuation comparisons pass |
| Storm | 20 | 64 | Native raw cloud creation/admission, thunder/cooldown, exact terrain scorch and linked victim scan; World/runtime/save comparisons pass |
| Odysseus | 21 | 66 | Native creation, doubled/capped speed, original art and shared routing/combat; controlled original combat compositions compared |
| Hurricane wind | 22 | 76 | Native four-direction parcel scans and mixed-actor fractional pushing, boundary cleanup, overlay clearing and raw pointer guard; complete World comparisons and saved continuation pass |
| Fire column | 24 | 6 | Complete raw native phases/routing/movement, exact new-cell scorch and linked damage, founder/hero/leader/town branches, retained mortality and scripted owner-word creation; full World memory/RNG and save comparisons pass |
| Fire rain | 25 | 38 | Native unlinked meteor delay, decoded falling art, height-aware impact and exact victim/terrain callbacks; World/save continuation pass |
| Volcano | 26 | 62 | Native crater growth/terrain preservation, raw fire-column eruption, lava/Basalt creation and contact states; World full-eruption comparisons pass |
| Achilles | 27 | 68 | Native creation, original art, routing/combat and victim/death controllers; controlled original combat compositions compared |
| Whirlpool | 31 | 24 | Native water admission, animation/motion, real coast lowering, lifetime and ordered pool execution; full controller and shared water/prepass consequences compared |
| Basalt | 30 | 74 | Native linked propagation, cardinal directions, persistent terrain prefix, original shapes and actual child/unlink operations; full controller, lava/Basalt and terrain barriers compared |
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

The retained `cmd/populous2-native` reference command runs
the assembled register-bearing session,
physical resource host, original screen/input and four-channel audio. Bounded
desktop captures verify its initial menu, terrain and protection requester.
The raw result, award and ending controllers and animation decoder are
verified against the original CPU and bound in the native desktop. Complete
stock/campaign frames and recorded interactive paths have source comparisons.
Additional sessions can extend coverage within the stated machine boundaries.
The inherited diagnostic engine remains available as `cmd/populous2-legacy`.

The [original instruction manual](https://ts.popre.net/Archive/Downloads/Docs/populous2.pdf)
provides the player-facing behavior; the supplied executable determines the
Amiga-specific slot and routine mappings.
