# Validation

Checks run on macOS ARM64 with Go 1.27.1 and Ebitengine 2.9.11.

| Check | Result |
|---|---|
| `go test ./...` | Passed |
| `go vet ./...` | Passed |
| Race tests for Amiga data, simulation and adapted engine | Passed |
| Original resource catalog | 26 resources decoded, compressed XOR checks verified |
| Landscape graphics | Four variants, 255 tiles and 830 sprite descriptors each |
| Campaign data | 1,000 worlds with native codes and starting templates |
| Terrain oracle | Eight native executions, 4,225 matching heights per seed |
| Ground-effect oracle | Nine flat cast maps/RNG; 5,430 font/swamp cases and 28,062 full-memory/RNG frames also replayed through World callbacks |
| Fungus oracle | Six original controller traces, complete tile maps, packed record fields, collection and generation timing |
| Fungus mortality oracle | Seventeen native common-prepass cases: fresh/mature tiles, all heroes, both sides, sound arguments and retained occupancy |
| Scenery oracle | Original object pools/RNG, including register-continuing seed 777 |
| Batholith oracle | Four height/object/RNG references covering raising and boulders |
| Wall oracle | Nine actor/head states, gates, joins and construction ticks |
| Deity oracle | 1,666 native encodes/decodes; allocation, face-cycle and scalar threshold references |
| Actor projection oracle | 576 original slope/fraction positions |
| Fire column oracle | Twelve full traces: actor, RNG and every terrain tile per update |
| Whirlwind oracle | Twelve parent traces, 3,550 matching updates and 713 child requests; actor, RNG and terrain compared |
| Followers | 399 usable records; references above 255 survive save/load |
| Ordinary target selection | 51 original selector cases; World replays 49 nondelegated cases with native target, leg, road-speed restore and RNG |
| Starting/emigrant attributes | Ten original allocation cases and all nineteen emigration stages distinguish search byte from weapon strength |
| Follower motion oracle | 72 traces / 528 updates, direction-image banks, cell heads and two full-dispatch/prepass references |
| Follower motion integration | Fixed-leg Core replays, precise crossing contacts, same-slot generations and save 12 continuation |
| Mana | Native divisor thresholds, quarter-mana units and propagated sculpt debits |
| Heroes | Six conversion types and eight-direction composite artwork; 1,104 full original creation cases match standalone and World callbacks, with native farm/marker writes, overflow and sound boundaries; all six heroes pass 180-update saved continuation |
| Ground rules | Persistent fonts/swamps/greenery, two-way faith reversal and plague contact |
| Audio | 31 samples, 133 patterns, signed PCM and read-size-independent playback |
| Save continuation | Mixed native flame/whirlwind records, town work, experience, random state and earlier controller/ID migrations |
| Fungus continuation | Version 10 preserves pending controllers, working bounds and tile stages; prototype marks migrate to seeds |
| Scenery continuation | Original variants, pool recycling, burial counters and save/load |
| Flame deaths | Retained death frames/slot reservations, current-cell damage and four-neighbor tree spread |
| Scenario runtime | Independent side rules, height admission, atomic prohibited edits, water, sprog, map visibility and save bindings |
| Water controllers | 166 Whirlpool cases/3,599 ticks and 358 Basalt cases; world replays 40 ordered Basalt traces/7,228 passes |
| Whirlwind water children | Eight combined world replays, 5,090 full pool passes, 644 native checkpoints and 74 births; complete pool/grid/RNG agree |
| Lightning | Five lifecycle traces/825 updates, 205 native victim cases, 16 beam segment cases and 12 endpoint cases; World replays all lifecycle traces and 66 ordinary-victim states |
| Native town compositor | 530 complete native map/overlay/record comparisons; World replays 523 noncompeting cases and tests mixed tree/boulder support and 49-cell cleanup |
| Native town center artwork | 233 original renderer comparisons covering all 19 stages, both owners, tick phases, population height and signed-word/division overflow; desktop castle capture inspected |
| Native actor byte image | Eleven original instruction aliases, 1,050 physical slots and retained-byte codec checks; version 15 continuation, final effect writes, reused followers and struck/demoted towns |
| Retained runtime memory | Actor/global seam, bounded aliases, 984 retained marker/deity bytes and replay of 14 original mixed graph operations |
| Native marker rules | 15 complete original image/grid comparisons for creation and signed-byte relocation, plus mixed-chain and version 16 save checks |
| Follower cleanup | 49 full native BSS comparisons, ordered writes and register outputs; raw/typed hydration preserves retained bytes |
| Follower combat | 79 original states 14/16 comparisons, reciprocal contacts, RNG and DIVU overflow |
| Combat winner | 87 complete native routine comparisons with real town/cleanup/graph callbacks and exact Adonis clone failure behavior |
| Town combat | 70 native destruction/reform cases, complete maps/overlays/records/deity data and original A0 alias behavior |
| Combat aftermath | 90 native cases and 1,214 updates across retained deaths, winner recovery, town destruction/ruins and linked cardinal-neighbor effects |
| World contact/combat integration | Native crossing prepares reciprocal battle without early damage; friendly homing merges only at completion; passive defender consumes no RNG; normal World loop, same-update waiting search and retained death/save continuation checked |
| Native prepass | 210 complete original BSS/source/sound comparisons with real cleanup, leader and farm operations; integrated into managed dispatch |
| Native attrition | 180 original stored-result, signed-overflow and retained-death comparisons |
| Hero decisions | 1,571 complete native BSS cases across all six hero types, target ties, wall thresholds, raw aliases and homing branches |
| Native direct raising | 666 cases/948 operations compare all 4,225 heights, Basalt preservation and direct operation admission |
| Raw follower contact | 292 original CPU cases covering all hero types, 19 town stages, self/overlapping references, Helen captive chains and exact sound/write boundaries |
| Terrain follower handlers | 554 original complete water/conversion/burning cases; World tests verify immediate water dispatch, population counted once, deity reference and fatal-water terminal/save reservation |
| Native magnet routing | 663 original route/wait/arrival cases including initial double attrition; World tests cover real marker arrival, leader claim, wait cadence and save continuation |
| Aftermath aliases | 84 original cases/496 updates across dispatch aliases and hero recovery images, without normalizing stored states |
| Neutral creation | 105 complete image/grid comparisons, first-free owner-byte scan, full pool and two-record creation |
| Neutral runtime | 252 original movement/effect-boundary cases compare complete image/grid/RNG and primitive arguments; external effect bodies and nonempty victim neighborhoods remain separate requirements |
| Captive routes | 446 complete native cases including retained original A1 after backlink repair, signed owner tests, raw `$35c` aliases and zero-timer motion |
| Native retained victim | State `$46`: 101 original cases/540 updates, complete countdown/removal, static art and leader relocation |
| Crossing admission | 2,528 original cases cover all raw tiles/heights and rock/wall thresholds, exact hint writes and signed stage indexing; World wall-break case verifies no position jump |
| Native town economy | 253 complete routines across four LAND tables and two full-pass birth fixtures; World tests cover mana/growth, same-pass emigration, rare slot-250 neutral creation and saved continuation |
| Native primitive creators | 383 original raw whirlwind/fire-column/single-tree cases retain owner-word aliases, recycled bytes, complete graph/RNG state and XP bypass |
| Economy simulation regression | 600+30 updates/save per original landscape; headless worlds 0/100 reach 12,000 updates and world 40 reaches the current prototype victory path; no original campaign pacing claim |
| Native Earthquake | 34 original scenarios/3,067 updates; World replays 31 scenarios/3,061 updates with complete raw pool, map, height and RNG comparisons |
| Native Volcano/Lava | 30 crater/eruption sequences and 356 Lava cases; World replays 24 complete unoccupied-pool eruptions, including raw owner-word aliases, real child creators and terrain reconstruction |
| Environmental saves | Version 19 preserves controller tags independently of kind; tests cover full-pool debit, water rejection, recycled Basalt/Fungus kinds, directed quake and eruption continuation |
| Native Storm | 710 original cases/1,705 samples, including actual local/script command-context aliases; World replays 546 runtime cases/1,541 updates against complete raw memory/RNG |
| Native FireRain | 809 original cases with 1,450 runtime updates; real shared scorch/damage callbacks and original fall-layer offsets; World delay/link/impact/save tests pass |
| Weather integration | Native cast admission/debit versus partial-pool creation, command alias preservation/bounds, normal-loop terminal release, delayed meteor visibility and old-save migration |
| Native Hurricane | 47 original cases/586 updates match complete BSS, map, overlays and mixed actor pools; World repeats all cases with real move/cleanup callbacks and verifies saved continuation |
| Native Tsunami | 914 original cases/1,328 updates cover adjacent-water starts, cloning, newborn skip, barriers, direct lowering and seven hero/Helen flood cases; World replays 40 ordered pool/shore cases over 800 updates, matching complete pool/map and 4,225 heights |
| Wind/wave saves | Version 21 retains unlinked wind and linked wave controllers/direction words; full-pool debit, recycled art suppression, native wave layers, saved continuation and no-debit/RNG old-save migration checked |
| Native Plague | 1,360 complete CPU cases cover signed-kind casting, per-record zero-damage prepass and real merge/birth inheritance boundaries; World repeats all 386 raw-memory casts and verifies actual merge/overlay/mortality composition |
| Native Armageddon | 720 complete native state-filter/conversion/cleanup/command-boundary cases match World BSS/RNG; admitted recasts, debit and 300-update saved continuation pass |
| Waiting continuation | 32 additional original state10/animation0 references cover ordinary image-loop continuation and signed timer expiry after hero search found no target |
| Disease/global saves | Version 22 preserves plague phases and native terrain permission; old disease/war state migrates without a replayed cast or mutated snapshot |
| Forest/Renew/scenery | 4,522 complete original memory/RNG cases cover all terrain codes, signed owner/XP and DIVU aliases, sampled creation, metric/debit, every signed age and mixed fire/town chains; World repeats all cases including 4,020 raw aging/fire updates, pool reuse and saved continuation |
| Native campaign result | 1,096 original score/award/progression cases, including divide-zero and quotient-overflow, are replayed through retained World statistics; tests cover weighted command exceptions, frame/low-word clock, exact zero/identity order, editor suppression and pending/applied result save continuation |
| Complete Whirlwind composition | 789 original CPU cases/19,296 frames match full World BSS/map/actors/command/RNG through real graph/farm/cleanup callbacks; tests cover lift/transport/release/landing, saved continuation, recycled creation words and actual routed owner-word water aliases |
| Complete Fire Column composition | 1,169 original CPU cases/12,094 frames match full World BSS/map/actors/command/RNG with actual town/scorch/damage helpers; mixed retained mortality, saved continuation, creation jitter/RNG and zero-speed fault prefixes pass |
| Scenery age renderer | 5,340 original CPU cases across twelve animation frames, 2,512 blits; native signed-age anchor/visible-height/source-plane clipping plans drive Ebitengine source subrectangles |
| Scenario script World adapter | All 272 original cases/278 updates match full memory/RNG, including 36 actual neutral/editor command compositions; all twelve commands present in the 1,000-world script catalog have native World bodies, ordered one-event dispatch, Storm caller/table alias and saved continuation |
| Native script/control saves | Version 24 retains BSS DC4..F44, raw cursor/table/scratch; partial RNG byte/word writes reach the generator, and older saves load the previously unrecorded table/cursor |
| Native actor graph | Mixed-pool signed references, full coordinates, linked heads and pressure; version 13 continuation and membership validation |
| Native direct terrain | 66 cases/77 edits compare all 4,225 heights, map corners, Armageddon bypass and persistent Basalt shapes |
| Scenario oracle | 648 native decision/init/attrition cases, including 400 mixed observer/victim-side checks |
| Scenario native oracle | 248 cases: 112 height admissions, eight initializations and 128 per-owner attrition updates |
| Deity interface | Native face parts, name, experience, password import and version 6 profile saves |
| Desktop application | Bounded launch, native score playback and application-buffer PNG capture; three whirlwind composites and ordinary native walking art, shutdown after 180 updates |

The native oracle executes the supplied executable's relocated routines inside
an isolated memory image. Its harness and raw results remain local; production
tests store derived numeric state traces and reference hashes, without original
executable bytes or audiovisual assets. The game executes Go code.

Whirlwind integration tests check native cast debits, shared-pool failure,
first-free reuse, correct animation banks across all landscapes, phase-aware
save validation and deterministic mixed-effect continuation. Pickup, release,
town collapse and allocated child whirlpools are outside the parent-controller
fixtures; no generic area damage is substituted for those pending interactions.

Scenario fixtures compare height admission, zero-extended initial balances,
attrition survivors/death decisions, observer visibility and victim-side swamp
removal. Death traces stop before native animation helpers. Ordinary walking
now uses the native motion dispatch; other follower state handlers remain adapted. These fixtures do not establish full native
follower timing, death transitions or propagated-edit rollback parity.

Fungus fixtures include inland oscillation, stable growth, row-edge aliasing,
collecting/recast/successive-controller phases, high experience and the first
bottom-edge generation. Later access beyond the map remains bounded rather
than emulating adjacent original BSS. Tests check complete map states and
controller bytes at recorded native boundaries. The world adapter has separate
seed/debit/pool-failure and save-continuation regressions; native follower
mortality uses the native mature/fresh distinction, Adonis immunity, decoded
death artwork and cue. Save tests preserve frame completion/slot release and
reject forged lifetimes or duplicate death reservations. Native full linked
death records and redispatch timing remain outside those adapter checks.
The world comparison covers the sixteen two-side mortality fixtures; the
native neutral-owner bypass is established by the isolated rules fixture but
is not yet representable in the inherited follower pool. A bounded desktop
capture also verifies ordinary/hero death artwork and the surviving Adonis.

Scenario attrition references compare population arithmetic and native death
decisions, rather than complete original death animation or fractional movement.
The traced earlier world-zero prototype ended at update 48 because its generic
Lightning area damage removed the remaining enemy groups. Native marker/bolt
integration now leaves both camps alive at update 600 in the same bounded
custom demonstration. This verifies removal of that specific discrepancy;
it does not establish native campaign pacing or opponent strategy.

Graphics-bank tests cover the shore/raised-land distinction and every water
animation phase. Repeated raises/lowers verify shared-corner continuity and
slope representability. Picking follows the actual four-corner surface.

These tests validate the documented portions of the conversion. They do not
establish complete original-game parity; [FEATURES.md](FEATURES.md) lists the
remaining work. Previous prototype simulation hashes are superseded by the
native economy, random generator and terrain changes.

Ordinary walking uses the native controller. Its world adapter replays the
fixed-leg portion of all 72 references, stopping before the untranslated target
decision boundary. The standalone dispatcher references verify same-update
fallthrough separately. Waiting, water and hero motion are not covered by these
ordinary walking traces. Version 12 also migrates earlier swapped basalt and
whirlpool IDs, and rejects invalid motion coordinates, velocities, phases,
image banks and pool-relative references.

Ordinary decisions now consume the mixed native map directly: preferred
search, linked boulder exclusion, road priority and pressure tie-breaking.
They do not create a settlement by returning a zero inherited tile delta.
Native settlement/contact begins at the separate entry handler. World tests
also preserve accelerated/saturated road legs through saves; magnet and hero
decision routines remain outside this ordinary selector's replay scope.

Version 13 world tests compare the complete mixed graph/pressure through native
Basalt propagation and combined Whirlwind/Whirlpool lifetimes. Later-slot child
controllers run during their birth pass. Mixed-actor saves preserve links and
accumulated pressure; malformed graph/actor membership is rejected. Whirlpool
audio depends on the current view but does not change the saved state.
The controlled combined-water references contain no followers or scenery and
therefore do not prove Whirlwind pickup/release or all environment interactions.

Lightning world tests cover free marker placement, unconditional activation
debits/partial volleys, gradual signed damage, managed population exclusion,
ownership-based reservations, stale effect references, first water entry and
saved continuation. Town reform uses the translated native evaluator and
49-cell compositor. The retained 210-case identity data documents its work,
tick and cleanup boundaries. The full founding/contact dispatcher and native
opponent/remaining town-work states are separate requirements; these map
comparisons do not prove them.
