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
| Fungus oracle | 1,465 original CPU cases/24,748 full BSS+RNG frames match World callbacks, including raw pool/edge aliases and retained follower consumers |
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
| Fungus continuation | Version 26 stores native pending references; version 25/prototype migration preserves mana/RNG and 180-frame continuation |
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

Fungus checks cover full retained memory through `$eb90`, rather than only the
4,096 terrain cells. The raw World controller matches collection/recycling,
aging, synchronous generation, signed period branches and edge accesses into
clock/overlay/view/record data. Consumer comparisons use real native farm,
leader, cleanup and graph callbacks, including delayed terminal dispatch and
hero immunity. Smaller World regressions cover debit, full-pool planting,
recasts, saved continuation and old collecting-reference migration. These prove
this controller composition; complete campaign and cross-power pacing remain
separate requirements.

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

## Final campaign presentation

The final-world Game branch now plays the supplied END.PAK animation and
original CODE text before restarting world zero. NativeEndingPlayback adds the
original one initial VBlank and four waits per loop, independently of gameplay
speed. Both retained scroll phases reproduce all 1,220 existing original-CPU
frame references when driven from the 60-update/50-PAL scheduler. Input is
cleared after the initial wait and exits after the next text/delta/swap boundary.
Result cues use original descriptors $168/$500 once per result; the ending adds
no separate cue and closes audio on exit. Compile checks cover the Game binding;
interactive desktop/GPU appearance and the remaining result/menu composition
are still required validation.

## Native scene initialization

NewWorld now composes the original control suffix and 58-byte player templates,
including mode 14, command ownership/transport, viewport 8, session seed/RNG,
compiled policy lists and the eight-byte opponent profile copy. Constructor
checks use all 1,000 independently executed campaign-template CPU references;
the helper-level corpus also covers control/profile modes and deferred records.
Template loading retains the actual XP and bolt writes before later Go bridges.
This establishes scene metadata; the policy dispatcher still requires its
complete frame register continuation and normal-player command stage before
replacing the inherited live strategy loop.

## Saved-game application paths

The menu/F9 requester and `-load-game` share native `.GAM` loading; F5 opens
the native save requester, using the same atomic export API. Explicit file APIs
retain Go JSON serialization by destination extension.
The default is `go-populous2.GAM`. File regressions resume 25 updates plus 80 after
reload in both formats and verify exact native transfer length, saved camera
origin and failure preservation of an existing file. Camera origin uses the
same retained view words as native aliases. GUI mana, power/terrain commands,
magnet modes and visibility use the selected profile side, including owner 2.
The original file requester is now bound to Game; interactive desktop checks
and source-exact keyboard/editor timing remain open.

## Startup and result application binding

Game now displays the independently verified original startup image, maps its
five native marker actions, and composes the original result requester over the
retained game background. Overlay checks preserve untouched pixels and match
all 124 CPU-verified result compositions, including opaque index-zero glyph cells.
The result uses a separate 50-PAL/60-update scheduler for its 101-blank delay;
mouse continuation uses the native requester action and keyboard Enter remains
a convenience. Startup tests cover all five visible rows and both hidden rows.
The current in-game backdrop/control drawing remains the conversion baseline;
full native in-game menus and interactive desktop validation are still open.
Out-of-map camera aliases are retained in view memory and the Go draw bridge
bounds its terrain-array accesses; original raw out-of-world framebuffer output
has not been reproduced by that guard.

## Native deity and options screens

The full deity screen now uses CODE760e requester fields, native bolt symbols,
its own CODE33844 palette, twelve XP strips, face backing and all three masked
parts at the original coordinates/order. Raw FACES.PAK interleaves five
sixteen-bit words with a transparent-mask bit. The actual resource loader
executes $1069c, complementing the mask and rearranging contiguous planes before
$f3a0 draws them. The CPU harness now executes that preparation before its
register-driven block blits and verifies all 512 face
combinations: prepared text, display password and complete RGBA framebuffer agree,
including bolt balances beyond the password import range. Hardware logic follows
[AmigaOS's documented minterm truth table](https://wiki.amigaos.net/wiki/Graphics_Minterms);
production remains the Go image decoder/compositor.

Deity mouse actions use the original b882 table; name/code entry, XP allocation,
face cycling and continuation retain the native geometry. The original Options
requester replaces the modern checkbox list, including both sides, ten flags,
reaction-speed arrows, conquest admission and the MUSI music toggle. Its prepared
text/frame source remains covered by the 2,048 native option references. Game
binding compiles; interactive GPU/presentation validation remains open.

## File requester host binding and birth-block backing

Startup Load, F5/F9 and the in-game save button now open the original compiled
file requester. Actual host callbacks enumerate without reordering, resolve
case-insensitive names, retain file identity on overwrite, load native GAM data
and atomically replace complete saves. Transfer tests cover row selection,
confirmed/declined overwrite, persisted camera, missing-file error dialogs and
missing-parent failures. Existing 171 original-CPU controller references and
3,000 marker probes remain the requester geometry/action evidence. Gameplay
updates stop while the requester is open. Current Game text entry accepts ASCII
with a 38-byte capacity; the original keyboard/VBlank editor timing remains a
separate integration step. GPU interaction is not yet verified.

BSS $dc2 now retains the full pool-birth block word, including partial-byte writes
and accesses crossing into $dc4 control data. The legacy public bool remains a
compatible view. Follower-pass clearing and allocation failure write the original
zero/one values. Save version 29 retains raw words; older bool-only snapshots
migrate to zero/one. Seam, snapshot, reset and GAM-extent regressions pass; this
state lies outside the original GAM transfer block and does not change it.

## Native world icon planes

The world-selection icons also call $f3a0 after startup $10a28 has executed
$1069c. Raw HUNK3 word groups first become five contiguous planes with a
complemented opaque-mask bit. They share this preparation and draw decoder
with deity face parts. A private original-CPU harness executes preparation and all
36 relocated HUNK3 descriptors over both blank and patterned backgrounds at
three positions, including top/left and bottom/right clipping. All 216 complete
RGBA framebuffer hashes match Go compositing. The 512 original deity-screen
comparisons guard the shared decoder. This establishes icon pixels; the World
requester application routing remains a separate binding step.

## World requester application binding

The Conquest startup action now opens the original world requester before
play begins. Its complete font/icon framebuffer matches 144 original-CPU
references. The opponent sheet matches 512 complete native text/face frames,
covering all 32 deity biographies, both selected sides, signed reaction labels,
unsigned aggression/clamping and both background patterns. These captures run
the real $1069c resource preparation before the native masked blits.

The world-code continuation performs $11044 template/script/landscape/RNG writes
on the retained session. All 1,000 world loads compare complete retained BSS
and all eight data registers against the original CPU, including modes 2/4/6.
Unlike scene initialization, this call does not compile policy lists or replace
actors, the clock, the saved seed or queued commands. Native profile switching
is an explicit $111ae call before displaying the selected side. Proceed starts
the scene; Cancel returns to startup.

World-code entry, invalid-code acknowledgment, the opponent sheet and admitted
power help use the native requester actions and geometry in Game. All 144
spell-help base/admission cases reproduce the original $517a..$5278 framebuffer
from font, icon and description data. Per-power preview animation starts at the
next source boundary and remains open. Game text editing remains provisional
until the verified keyboard/VBlank modal is bound; interactive GPU validation
and full in-game/native frame routing are also still required.

## In-game requester World continuations

The World adapter now applies all 2,808 CPU-referenced in-game menu action
decisions to the selected live control word, painting flag and command pointer.
Direct solo profile changes use $111ae; multiplayer profile changes queue124.
Save/load/restart/quit only replace command byte1, preserving XY, the clock and
simulation turn until $1744c. This checks World state/continuation binding; it
is not a complete menu register/drawing/input timing proof.

Resume uses the actual $181c0 controller and raw command transport byte. Solo
records complete without network callbacks; multiplayer records require an
actual port. Tests verify that resume does not execute or clear queued commands
and refuses a fabricated multiplayer exchange. The underlying packet/resume/
disconnect controller is covered by 432 original-CPU cases. Game integration
awaits the full ordered frame and asynchronous command-stage continuations.

## Complete follower prepass register continuation

CommonPrepass now accepts the actual full frame context and explicit
register-bearing cleanup/leader callbacks. Its 630 original-CPU cases cover
210 terrain/plague/hero/corpse combinations under three data-register seeds.
Both the isolated composition and raw World adapter reproduce all eight data
registers and the complete retained BSS image. Cleanup and leader composition
reuse their already verified native bodies; farm clearing and sound calls
preserve their original saved data registers. The World adapter reads real
scenario words and retains the original signed, byte-wrapped tile displacement.

The existing follower D4/D5 controllers can now share that full frame. Their
native comparison corpus runs with both the retained nil-frame ABI and the
shared-frame bridge. Farm clearing, settlement and hero terrain-raise wrappers
restore the complete saved context; the town evaluator retains its original
MOVEM.W sign extension when restoring D4.

## Ordered follower pass boundaries

The new $11252 outer pass preserves physical slot order, active redispatch,
peak statistic/working-counter initialization, minimap descriptor admission,
raw population addition and the original first-zero-side result boundary.
Ninety-six independently executed source cases compare complete retained BSS,
all eight data registers and callback order/counts, including an empty pool,
last physical slot, suppressed/visible markers, painting-state reset and both
conquest/custom result gates. The marker lookup uses the signed -16 brief-index
displacement at $12414, rather than an unsigned +240 offset.

These outer-pass comparisons explicitly replace prepass, actor, map-point and
result bodies with controlled register-bearing callback boundaries. They prove
the outer scheduler, not whole follower gameplay or pixels. The actual prepass
has its separate complete proof above; movement and remaining actor register
bodies still require composition before the live strategy/frame loop can use
this pass. Missing active bodies, map drawing and result continuations return
explicit errors; no silent successful placeholder is installed in Game.

## Retained aftermath bodies in the complete follower pass

NativeFollowerAftermathFrameRules translates all fourteen retained aftermath
states at $1199e/$11ce8/$11e00/$11ece/$12074 directly from raw memory. It
preserves animation-word aliases, signed timer flags, cleanup/leader register
outputs, four-cell neighbor scans, town destruction, ruin retention and the
exact $123b4/$12462 return boundary. The 603 independently executed source
cases contain 6,729 supported body frames; all eight data registers, map,
overlays, complete actor/deity banks and RNG match both standalone Go and World.
The other 78 captured exits hand control to ordinary search and are outside
this body family; they are not treated as successful aftermath calls.

The World follower dispatcher now binds these bodies and the independently
verified Whirlwind transport body. For aftermath, 112 complete original $11252
passes over all 400 slots compare the composed real Go prepass, actor body,
cleanup/leader/graph/neighbor operations, counters and full data-register/BSS
output. These passes use native mode8 to suppress result dialogs and view0
to omit minimap pixel drawing. No actor/prepass callback output is replayed
from a fixture in this composed proof. Other follower families and live Game
frame/presentation routing remain required work; a missing body still errors.

## Native two-player transport and startup

The portable serial adapter reproduces handshake, eight-byte player packets,
menu synchronization, disconnect aliases, receive-ring behavior and multiplayer
startup. CPU comparisons cover 72 handshakes, 160 requester pages/960 actions,
256 byte translations, 48 receive interrupts and 128 complete mode6 startups.
Real paired net.Conn tests run 100 protocol stages and 240 World command stages
with 480 original $17500 player commands; both peers apply both players' commands.
Full extended tests, race checks and vet pass.

The localhost TCP test, initially unavailable inside the sandbox, was run by
the user on 2026-10-05: TestNativeSerialTCPConnectionCarriesOriginalBytes passed
without skipping (test0.00s, package0.254s). This verifies actual socket creation
and byte transfer on 127.0.0.1. The desktop endpoint UI, full physics/frame
scheduler and save/reload policy for pending live connections remain separate
required integration; these protocol tests do not establish complete two-player
Game behavior.

## Full winner register and address continuation

WinWithFrame composes the complete existing $1298c winner-resolution body
with its actual data-register outputs and returned A3 reference. The native
corpus executes 87 leader/hero/town/Adonis scenarios across all four decoded
LAND banks and three incoming register seeds: 1,044 comparisons match all
eight data registers, actual returned A3 and the complete retained BSS image
for both isolated Go and World. LAND resources are installed at the original
resource-table destination $3365a before capturing reward data.

The proof includes unsigned reward wrapping, victory counters, mana clamping,
ordinary celebrations, hero deaths, town reform/destruction, original A0
context, Adonis population halving, exhausted slots and the $dc2 birth gate.
Farm/reform/town/Adonis saved-register boundaries remain distinct from the
$12a98 cleanup, whose real D0-D2 outputs survive into the combat caller. The
World adapter borrows raw bytes and does not flush typed records mid-call.
This establishes the winner body; the full combat caller/physics/frame/UI
composition still requires its separate integration and comparison.

## Raw entry, contact and movement composition

NativeFollowerEntryFrameRules reproduces $1275a's signed raw owner/population
admission, linked-order priorities, wall observation, homing movement, merge,
battle/capture and complete town-reform continuation. All 150 original-CPU
cases compare the complete 64KiB BSS image and all eight data registers for
both isolated Go and World, including source $13126 preserving the existing
state/animation, and Helen's actual farm/reference register outputs. These
comparisons execute real Go contact, merge, evaluator and farm bodies rather
than replaying external callback deltas from reference fixtures.

The raw World dispatcher now composes the proven movement controller, actual
$12518 graph writes, entry and broken-wall aftermath. Thirty-two independently
executed complete $11252 passes compare all data registers and retained BSS
for two owners, four travel directions, same/crossed cells and two nonzero
register contexts. The native magnet mode prevents founding in this bounded
composition; town/search/hero tails remain explicit caller continuations.

The minimap point producer $e196 now returns its exact byte/bit/color plan and
register continuation, with a four-plane pixel writer. Its 768 native CPU
references cover sixteen colors, eight bit positions, three rows and two
background patterns; both bitmap hashes and all incoming/surviving register
values match. Full presentation/native frame binding remains separate work.

## Composed combat, winner and follower pass

The raw World dispatcher now binds aggressor state $0e and defender state $10
to the complete register-bearing combat and winner bodies. Victory resolution
retains the original A0 context and actual returned A3 before choosing the
count/next continuation. Failed defender contacts continue directly to $1131c
through the explicit search adapter, without an extra common prepass.

The independent native corpus executes 3,092 combat cases across all four LAND
banks, including 1,120 complete winner calls. All eight data registers, ordered
cleanup/winner inputs, minimap variant and the full 64KiB retained BSS match.
Go runs real cleanup and reward bodies; reference callback mutations are not
replayed. Sixty-four further original $11252 executions compare the complete
400-slot pass, counters, linked records, celebration aftermath, minimap register
production and 70,272 retained bytes for both owners and two caller contexts.

This establishes the composed combat caller in the raw follower dispatcher.
Ordinary search tails and other unbound follower families, live frame/UI
presentation and complete game integration remain required work.

## Town controller in the raw World pass

Town state $06 now uses the full register-bearing $11738 controller in the
World dispatcher. The evaluator's $13550 property cache and 50-word parcel
scratch remain shared across actors; the caller resets only the distinct
$13350 flag and minimap variant. Births execute actual raw record allocation,
linked insertion and the rare $131cc neutral creator without typed-record
flushes. A lost town's direct search continuation remains explicit.

The World adapter matches all 820 independent town-controller references,
including 144 original DIVU-zero prefixes. Sixty-four additional complete
$11252 passes match eight data registers, 70,272 retained bytes and mutable
CODE cache/flags for two towns, both owners, all four LAND banks, work-counter
wrapping and infected/uninfected growth. These complete-pass cases retain
small populations; birth and exhausted-pool behavior are covered by the
controller corpus rather than claimed as complete newborn-search passes.

## Composed search, hero and magnet movement

Ordinary state $02, hero states $24/$26 and magnet states $12/$3a are now
attached to the raw World follower pass. Direct $1131c/$1156c/$12044/$11bb4
tails enter the actual child body without manufacturing a new state or
prepass. Magnet homing $140f0 uses the real $1452e planner, marker/leader
selection, fractional-coordinate adoption, merge and leader registration;
it preserves the caller's D1-D7 at the original merge wrapper.

All 496 independently executed magnet/homing references match eight data
registers and the complete 70,272-byte retained image, including attrition
death, timer wrapping, same-cell merges, owner aliases and direct homing.
An additional 112 complete $11252 passes match the original registers and
retained image across both owners, all four behavior modes, search-to-magnet,
hero selection, hero pursuit, same-cell contact/combat and three random seeds.
These comparisons execute actual World graph/contact/cleanup/terrain bodies.
Waiting/contact, captive/neutral controllers and live presentation remain
separate required integrations; the raw dispatcher is not yet the live Game
scheduler.

## Waiting, contact, terrain and siege dispatcher bindings

The World dispatcher now binds waiting $0a, completed contact $0c, water $16,
conversion $36, burning $3c and siege states $1c/$1e/$22. A contact's direct
$1204e continuation executes Chase without changing the source state. Actual
zero references remain valid selected reserved records; contact and merge flags
distinguish them from no selection. The production World callbacks match all
947 waiting/contact references (including 36 odd-word prefixes, 148 merges
and 229 contacts) and all 1,324 terrain references (including 12 original
address-fault prefixes). Seventy-two complete terrain passes also match through
the dispatcher itself, rather than an external replacement body.

The new siege body retains animation loops, signed-target admission, population
shift/wrap, hero-specific release animation and exact word writes before real
cleanup or town reform. All 1,796 independent native controller executions and
48 complete $11252 passes match eight data registers and 70,272 retained bytes.
The complete passes also run genuine downstream search/movement and aftermath.
These source-family checks advance the raw simulation; captive/neutral/ruin
dispatch and live frame/presentation remain required integration.
