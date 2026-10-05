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

## Complete follower rule-set assembly

NativeFollowerFrameRules collects every active even state $02..$46 into one
raw $11252 dispatcher, including captive $34, neutral $44 and ruin $46. Its
LAND selection is checked against World, and NewState retains a distinct
original property cache/parcel scratch for each session. The parent still
owns clock, minimap/result presentation and audio; no missing child is replaced
with an empty successful body. Nested Tick calls borrow the authoritative raw
frame; standalone calls reconcile once and hydrate after the whole actor pass.

All 452 independently executed complete passes from the motion, search/hero,
combat, town, terrain, captive/ruin, neutral and siege corpora now match through
this collected configuration: eight data registers and 70,272 retained bytes
agree across original outcomes and all four LAND banks. Separate dispatcher
tests match the 32 captive/ruin and 28 neutral passes through production World
callbacks. This establishes assembly of the active source-state families;
filled-pool physics composition, original rendering/input/audio integration
and replacement of the live inherited Game scheduler remain required work.

## Native sprite pixels and active LAND resources

NativeSpriteBitmapBank prepares the actual HUNK3/S16/S32 mask and color planes
once per resource bank and paints $f0ee/$f3a0 requests directly into the four
native screen planes. An independent reference executes original CODE and
$1069c with a block-DMA model following the Commodore hardware manual's mask,
shift, modulo and minterm rules. All 1,041 comparisons match complete register
and BSS outputs, prepared source hashes and the final 32,000-byte bitmap across
HUD, cursor, camera, slope outlines and direct16/32-pixel clipping requests.
The 38 direct y=-32768 descriptor cases are deliberately outside this prepared
image API: native NEG.W overflow reads adjacent source RAM, which is reported
explicitly rather than fabricated as ordinary clipping. The hardware model
is an analysis reference, not a production CPU emulator. The source rules are
documented in the [Amiga Hardware Reference Manual, chapter6](https://www.theflatnet.de/pub/cbm/amiga/AmigaDevDocs/hard_6.html).

NewWorld now installs its selected original 556-byte LAND resource at CODE
$3365a in a distinct World copy. Previously NativeAI.Code kept LAND0's minimap
colors even when Landscape selected LAND2/3, producing incorrect terrain
redraw words. All four campaign resource selections and isolation from other
Worlds/the immutable Bundle are verified. Filled-pool native physics comparisons
also exercise this resource seam; whole world rendering and live Game binding
remain separate required work.

## Retained native frame session

NativeFrameSession composes the source VBlank gate, real clock/palette wait,
caller-owned main rendering, all follower/AI/FX/wall/scenery/script/audio
stages, actual Copper swap and deferred commands. Raw World ownership remains
held across pending renderer, serial or modal operations; completed work is
not replayed. The image/audio bank changes authority once before physics and
once before swap, so later command drawing is not overwritten by stale audio.

The session's production physics callbacks match all 168 original main-physics
references, including repeated retained passes: full registers, BSS, RNG,
shared audio and both overview $1e/persistent terrain $22 bitmaps agree through
the same before-$072e reference cut. Separate lifecycle tests cover real swap,
deferred command continuation, scoped callback restoration, failure prefixes,
seventeen real palette waits, and terrain drawing into its distinct buffer.
Initialized $190e4 device operations require a real supplied backend; the
original signed $3b4 disabled-device gate is preserved for direct and scheduled
calls. No absent initialized sound operation becomes an empty success.

Main rendering remains a genuine required callback while its remaining source
children are implemented. The live Game still uses its inherited scheduler;
this session and its bounded native comparisons establish integration APIs,
not a completed original interface or audible replay.

## Original BLOCK tile bitmap sink

NativeTileBitmapBank caches the actual BLOCK descriptors and prepared16x8
chunks for each landscape. PaintChunk consumes the original source word and
destination byte displacement, leaving height adjustment, six-part ordering
and zero-offset pointer advances to the world-draw producer. It updates the
four real bitmap planes without allocating or repacking chunks during drawing.

All 3,060 original $bfac pair executions match the final native bitmap across
255 tiles, three pairs and four BLOCK banks. The independent reference runs
actual $1a3f0 preparation and hardware DMA mask/minterm/modulo rules, including
the hardware B pointer advancing between color planes. Both half-tiles draw
at the same row at initial offset/+2; the pair returns at initial offset+320.
This proves the pixel sink and pair layout, while complete $bbe0 traversal,
actor ordering and its live Game integration remain required work.

The prepared sprite bank also reconstructs the selected landscape's original
S16 DIF and S32 DIF/PIF resources before preparation. Only base PAK files are
stored by the original resource set; separate LAND1..3 sprite PAKs do not exist.
All cached mask/color planes now agree pixel-for-pixel with the independently
loaded sprite atlas for each of the four landscapes, and the original
1,041-case DMA renderer regression remains green.

## Session-owned player command continuations

NativeFrameSession now retains one NativeCommandFrameState for each of the
two original $1744c player records. Ordinary commands execute the real World
body once; modal/resource children retain their handler, register context and
child phase. CODE $a2a is the actual mutable pointer-image selector, while
$3f90/$4468 use the retained per-World CODE words. The actual reverse palette
child is available by default; absent file/reset/resource bodies remain errors.

The session adapter matches 768 original player-command envelopes, each run
both immediately and with suspended children. Full D, BSS, CODE words, palette
arguments and captured terrain targets agree. The other 384 references use
the independent $dc4 scenario record and remain covered by its separate command
body proof. A composed session test also suspends the real $3f92 file-requester
command before the second side's actual mana command, then verifies one swap,
one renderer invocation, exact command clearing and outer MOVEM restoration.
This proves orchestration; real $3f92 DOS/modal, reset and resource child bodies
and their Game binding are still required work.

## Full minimap rebuild child

DrawNativeMinimapFrame reproduces the complete $d8cc 64x64 scan and its mutable
CODE coordinate/pixel-procedure selector, using the loaded LAND color table.
All 192 original CPU comparisons match eight data registers and final bitmap
across four landscapes, varied cell codes, shifted selectors, checked-edge
clipping and nonzero backgrounds. No neighbor pixels are cleared by a guessed
whole-image rebuild.

The session exposes AdvanceBuiltInCommandChild for the real palette/minimap
bodies alongside caller-owned DOS/reset/resource operations. $d8cc uses the
original captured A0 target rather than rereading a changed BSS $22 pointer;
the scoped target regression verifies that only the captured buffer is drawn.
Complete terrain/world rendering and real file/reset child operations remain
required before enabling this session as the live Game scheduler.

## World traversal and real adjacent bitmap RAM

NativeBitmapWindow supplies actual RAM surrounding a bitmap with an explicit
origin. PaintChunkWindow preserves native plane/row DMA order and the +8000
plane stride; it neither clamps writes to one plane nor invents zero padding
past the framebuffer. All 480 original adjacent-DMA pair cases match the
complete initialized RAM window, including negative displacements, plane
crossings and writes past the nominal end. Unavailable backing retains the
completed write prefix before reporting the missing address.

The genuine $bbe0 world traversal matches all 340 original CPU/DMA cases:
eight registers, 70,272-byte BSS, framebuffer and adjacent initialized RAM,
shared image counters, actor order and projection/terrain permission flags.
Coverage includes all four BLOCK banks, sixteen slopes and fractional positions,
overlays, viewport corners and mixed signed actor chains drawn tail-to-head.
Four animated tile220 cases genuinely write past the nominal buffer and now
match the same adjacent RAM; no exception or pixel clamp hides them.

This establishes main world drawing and the bitmap sink. Alternate-view $c204,
town challenge/editor inputs, complete main-render orchestration and live Game
activation remain required work.

## Main rendering composition

NativeMainRenderState composes the actual $ea0..$10b6 rendering sequence:
background copy, highlights, HUD, selected actor/editor, countdown, normal or
alternate world, map cursor, camera marker and pointer. The mutable CODE $e8ce
town hit height is shared across selected, normal and alternate actor drawing.
Beginning another frame resets only the program position, preserving those
shared fields. MainFrame is the synchronous wrapper; AdvanceMain retains
editor waits as described below. Failed children keep their mutation prefix
and cannot be silently retried.

All 727 original CPU/DMA references match eight registers, the complete
70,272-byte BSS, the shared image/audio bank, town hit height, framebuffer and
initialized adjacent bitmap RAM. The references include 340 normal and 387
alternate views, all four BLOCK banks and 124 animated tile220 boundary cases.
These captures use the actual base sprite bank; separate bank decoding tests
cover the other three sprite resources. Actual editor/debug children and
wait-safe modal composition remain distinct requirements before live Game
activation. This proves source rendering order, not the completed game loop.

The retained session can now bind this concrete renderer using cached sprite
and BLOCK banks. Its background resolver follows BSS $22; the drawing target
follows $1e and its tile sink borrows the actual surrounding HUNK4 allocation.
An external RAM window must own the same bitmap slice, preventing detached
padding from hiding writes. Integration checks execute normal/alternate/normal
frames through real swaps, preserve the background and shared rendering state,
and retain the drawn prefix while releasing raw World ownership on a missing
editor child. The adapter retains editor waits as described below; actor and
protection continuations remain required for the full interactive loop.
The editor-mode diagnostic target at $2e3a uses separate mutable CODE backing,
not the equal numeric BSS address inside terrain. A targeted integration check
verifies the original draw-buffer pointer and clock register at the diagnostic
child while the terrain word remains unchanged.

## Main rendering across actual editor waits

AdvanceMain retains the genuine $346a call across its numeric editor and
keyboard/VBlank waits. Completed background, HUD and selected-entry work is
not replayed. The editor admission remains frozen until the child returns,
even if the mode word changes during the wait. A source failure is terminal
and retains its mutation prefix.

All twenty complete original $ea0..$10b6 executions match 96 snapshots through
the actual $4bba/$4c14 modal, $620 keyboard interrupts, $3ec VBlank interrupts,
$072e swaps, $378e numeric writes and $2ae2 debug formatter. Every comparison
checks all eight registers, complete BSS, both chip screens/Copper memory,
pointer-image memory, shared image/audio descriptors and mutable CODE fields.
The sound boundary remains explicitly register-preserving in this rendering
proof. No prerecorded modal output is substituted for the editor body.

These comparisons exposed a stale drawing target after an editor swap. Main
rendering now resolves the actual $1e/$22 targets immediately after the child
returns, and the retained session refreshes its physics target too. A separate
session test runs the real editor and keyboard waits while holding raw World
ownership until the entire frame completes. Actor/protection waits and live
Game activation still require their own continuations and integration.

## Selected actor and main-frame protection continuation

AdvanceSelected retains the complete normal $1e18 parent around its actual
$e45c/$314a child. Camera admission, the selection timer and fallback pointer
run once; the original command word stays hidden while the child is pending.
The parent restores it after the actor returns, then draws the actual weapon
and population indicators into the current $1e target. Source $e28/$e4c host
blitter handoffs surround both the actor and each direct population primitive,
with all caller data registers preserved at each handoff.

All 24 complete original selected-panel executions match 1,416 snapshots
through the genuine protection requester, click/IRQ/palette waits and cached
resource return protocol. A further 24 complete $ea0..$10b6 main-render
executions match another 1,416 snapshots, including the normal world and cursor
suffix after the protection returns. Comparisons cover eight data registers,
complete BSS/CODE, both chip screens and Copper state, pointer-image memory,
shared image counters and callback order. The full-world reference includes
the actual DMA channel-pointer writeback between BLOCK color planes.

AdvanceMain and the retained session now preserve this selected continuation.
A controlled child-wait integration test verifies raw World ownership, one
timer update, saved registers, frozen actor selection and delayed restoration
of the command word. It tests orchestration separately from the actual modal
body proof. Real protection waits within normal/alternate world traversal,
the remaining host callbacks and live Game activation are still required.

## Main frame across retained world traversal

AdvanceMain now retains the normal $bbe0 and alternate $c204 traversal when
a projected actor enters protection. Row/column, linked-list order, computed
actor projection and complete caller registers survive each wait. Alternate
clearing and its original view-word load run once, while mutable projection
and scratch words remain available in the supplied CODE backing.

All 68 complete original $ea0..$10b6 executions match 3,944 snapshots across
the real protection requester and IRQ/click/palette waits. The cases cover all
four BLOCK banks, normal/alternate views, mixed same-cell tail-to-head actors
and a following-cell actor. They compare full D/BSS/CODE, both chip screens
and Copper state, pointer RAM and shared image state; base sprite resources
are used, with landscape sprite differences covered separately.

The session tile sink resolves the source A6 address retained at traversal
entry. Actor and overlay images resolve current global $1e. A targeted buffer
test permutes image pointers, then verifies that tile DMA still updates the
original allocation and rejects a mismatched slice. Cursor/debug drawing is
rebound after the traversal returns. These source comparisons establish the
complete rendering continuation; remaining startup/resource/menu/transport
host bindings and live Game activation still require integration.

## Composed in-game menu children

NativeInGameHostState connects the genuine $446a controller to original options
($471c), profile switching ($111ae), panel restoration ($1da0/$1f5e), palette
and audio pause/resume bodies. Nested options/serial/palette state survives
host waits; completion clears only the child invocation. Transport stays an
explicit callback with independent completion/condition flags, and failures
retain the mutated prefix without replaying it.

Twenty-four original composed menu executions match 78 complete snapshots
across both profiles and game modes2/4/6/8: resume, profile-switch/resume and
options edits/exit. The reference executes the actual child bodies, including
initialized icon/sprite preparation, full-register audio wrappers and shared
HUNK3/bitmap drawing. Full D/BSS, both screens/Copper, pointer RAM, requester
CODE fields, shared image state and call/sound order match. The signed
transport-resume result is an explicit register-preserving boundary in this
proof, not a claimed connection. Initialized audio-device behavior and serial
protocol have separate proofs. Live menu/frame entry and host transport binding
remain integration work.

## Source menu and exit gates in the frame session

NativeFrameSession now executes the actual $e76..$e94 entry prefix: test and
clear BSS $dce once, retain the genuine $446a call, then test BSS $3aa before
the $786 VBlank wait. A requested exit completes this session without rendering
or physics and exposes ExitRequested to the host. Typed World hydration still
occurs only after raw ownership is released.

Thirty original CPU entry envelopes match full D/BSS and exact menu-entry
registers, each with immediate and suspended declared menu returns. Separate
tests prove that the clear is not repeated, exit waits for the menu return and
the entry flags are not repolled from within an already admitted video wait.
The concrete MenuFrame adapter also runs all 24 composed menu references and
78 snapshots through the same retained session callback. It requires initialized
shared CODE and real host audio/transport bindings; it supplies the borrowed
World, current presentation, image bank and physical bitmap resolver.

This connects the source entry to the verified menu and rendering APIs. Initial
$10a10/$10a8c startup, physical resource loading, complete transport return
semantics and live Ebitengine loop activation are still required.

## Physical HUNK and encoded-file host backing

NativeHostMemory loads each segment into its actual declared allocation at
explicit caller-supplied addresses and applies validated32-bit relocations.
CODE/data payload, real BSS and allocation tails remain distinct from missing
gaps. Mapped host regions alias their owners; byte operations can cross truly
adjacent regions, but a bitmap window cannot join detached slices or invent
padding. Missing writes retain their completed prefix before reporting the
unavailable address.

NativeResourceFilesystem opens original encoded assets with Amiga-style case
lookup and supplies actual host read counts/payloads. It retains open handles
and short reads, exposes native0/-1 failures for genuine requester handling,
rejects ambiguous names and closes retained handles. Decoded Bundle.Raw never
stands in for packed disk bytes.

All26 embedded encoded resources match their host read counts/bytes. Twenty-four
fresh source19CD0 executions additionally match full registers and every actual
HUNK allocation using the sparse host memory and real file adapter. The source
reference fingerprints declared allocations, without the earlier analysis
model's zero-filled gaps. FX/QAZ require original runtime allocations and are
deliberately unavailable in that HUNK-only integration test; their allocations
and full startup/presentation/World backing remain required integration work.

## Runtime audio and background allocations

NativeStartupAllocationState translates complete $1a43e/$1a4bc. It requests
exactly $1e0dc audio bytes and $7d00 background bytes with flags2, follows the
signed allocation return gates, updates real BSS/descriptor pointers and retains
each genuine resource19CD0 call. Release uses the original positive-pointer
tests and exact sizes. It preserves the actual visible caller register results,
including declared host allocator clobbers, rather than normalizing success.

All36 original CPU allocation/release envelopes match full D/A/BSS/CODE,
with each resource/allocation callback run immediately and suspended. The
external allocator/resource return contracts are explicit in that proof.
NativeHostAllocator additionally maps real Go-owned regions in an explicit
configured address range. Its addresses are host assignments, not claims about
AmigaOS allocation placement. Exhaustion returns native0; releases validate
the allocation and remove its mapped backing.

An integrated startup test executes1A43e through the real host allocator and
encoded filesystem into the proven19CD0 body, verifies decoded FX/QAZ bytes,
actual descriptor pointers and closed handles, then executes1A4bc and verifies
both allocations are unavailable. Remaining initial constructor, coherent
World/presentation backing and live Game-loop activation remain required work.

## Shared physical session memory

NativeSessionMemory maps physical HUNK1 accesses onto the frame session's
actual raw World and input owners. Chip screens and pointer images share their
real HUNK allocations with resource loading and rendering. CODE cursor words
alias the input state, so source IRQ updates remain visible through physical
reads. Other addresses use the host's mapped regions; missing RAM stays an
error instead of acquiring invented padding.

ImportBSS transfers initialized startup bytes while ownership is idle.
BeginRaw then borrows those authoritative records without flushing inherited
typed actors over them. Completion retains the existing final hydration and
ownership release. SnapshotBSS explicitly exports the live callback-backed
bytes; the original physical HUNK1 slice is not a second live World owner.

Targeted tests verify both directions of BSS access, cross-field word/long
operations, RNG and record writes, input/CODE cursor aliases, identical screen
backing, complete startup-byte import and rejection during a borrowed frame.
They also verify that BeginRaw preserves loaded records and releases its
borrow. Relocated CODE sharing and complete startup/Game activation remain
separate integration work.

## Runtime resource ownership

NativeRuntimeHost composes real HUNK allocations, relocation-aware CODE,
callback-backed World BSS, input, chip screens, encoded filesystem handles and
the configured allocator. The constructor imports actual zero-initialized
HUNK1 bytes. It deliberately leaves the Copper builders, resource allocation
and world startup as separate source operations rather than inheriting an
already-generated prototype world.

Integration tests verify identical CODE/screen owners, live logical/physical
pointer updates, source Copper initialization and genuine1A43E/1A4BC loading
and release of FX/QAZ. A missing FX file enters the actual retained requester;
advancing again neither completes startup nor repeats the allocated prefix.
Every callback requires the configured BSS base. Complete startup children,
presentation CODE aliases and live Game-loop activation remain integration
work; construction alone does not make the game ready to run.

## LAND reloads and relocated minimap commands

Native host Worlds now retain their configured shared CODE owner when a LAND
bank changes; standalone Worlds continue to own isolated copies. Minimap
commands select the relocation-aware procedure view for a shared host, while
all scalar coordinates and colors remain in the same physical allocation.

Regression tests load all four LAND banks repeatedly, verify physical bytes
and unchanged owner addresses, and protect the immutable Bundle. A complete
session minimap command with a relocated procedure pointer matches the
previously CPU-verified drawing body in all D registers and every screen byte.
The physical procedure operand remains relocated after drawing.

## Runtime presentation CODE aliases

The runtime now composes NativePresentationCodeAlias above its physical BSS
and mouse owners. Source Copper selector/pointer, clock deadline and complete
interrupt-chain WORD stay visible through the same RAM/CODE callbacks used
by loading and menus. Hardware COP1LC remains separate presentation state.

An integration test exercises actual initialization and swap, then mutates
deadline/interrupt/mouse/World fields through both directions of the composed
memory views. It protects the underlying owners and noncanonical nonzero
interrupt WORDs. Image/audio CODE sharing and remaining startup UI bodies are
still separate prerequisites for live frame activation.

## Runtime startup callback composition

StartupCallbacks binds the retained startup controller to the runtime's real
physical RAM, encoded filesystem, resource/error requester and bitmap owners.
It preserves caller-supplied hardware, audio and genuine UI operations.

An integration test executes source custom-world construction, power-resource
loads, LAND graphics, terrain/scenery/follower creation and minimap drawing
using this shared runtime. It reaches the explicit panel restoration boundary
and resumes there without replaying or mutating the completed constructor.
No source completion or terminal condition flags are claimed while that child
is outstanding. This test proves the assembled prefix, not the complete live
startup or interactive game.

## Runtime image/audio authority and physical device

The host now exposes image-layer Y, shared queue records and four scheduler
channels through its physical CODE callbacks. The retained session selects
Audio immediately after its existing render-to-physics transfer, then Image
after the audio-to-swap transfer and before deferred UI. Authority is never
inferred from a broad frame phase, and neither transfer is repeated on resume.

InitializeAudio uses actual allocated/decoded FX and canonical relocated CODE
for the original driver initialization. A bounded single-owner integration
test starts native music and reads raster-DMA PCM. Shared device streaming and
frame/menu/resource access require host serialization; the PCM's own mutex
does not protect other CODE owners.

Another test retains a render wait followed by a deferred command wait and
verifies physical queue reads, exact authority changes and survival of the
late UI sound write. Session and runtime race checks pass. These tests prove
the shared ownership contract; host streaming serialization and live Game
activation remain required.

## Complete custom-startup runtime composition

RestoreStartupPanel binds the full address-register panel body to actual
prepared sprite RAM, live scalar/logical CODE views and shared image state.
An integration test executes the complete custom10AD8 startup after real
allocation, loading, Copper and audio-device initialization. Original world
creators, graphics, minimap, panel and18474 resume all run their translated
bodies before the controller returns its real terminal zero flag. The resulting
raw records enter BeginRaw without typed regeneration.

A separate suspended-audio case verifies that completed panel/world work is
not replayed. Ownership callbacks deliberately clobber caller registers in that
test, exercising the source panel's full save/restore contract. These are
runtime-composition checks; full in-game visual/interaction validation and the
initial menu/campaign startup paths remain outstanding.
