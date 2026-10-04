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
| Ground-effect oracle | Nine matching tile maps and final 32-bit random states |
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
| Starting/emigrant attributes | Ten original allocation cases and all nineteen emigration stages distinguish search byte from weapon strength |
| Follower motion oracle | 72 traces / 528 updates, direction-image banks, cell heads and two full-dispatch/prepass references |
| Follower motion integration | Fixed-leg Core replays, precise crossing contacts, same-slot generations and save 12 continuation |
| Mana | Native divisor thresholds, quarter-mana units and propagated sculpt debits |
| Heroes | Six conversion types and eight-direction composite artwork |
| Ground rules | Persistent fonts/swamps/greenery, two-way faith reversal and plague contact |
| Audio | 31 samples, 133 patterns, signed PCM and read-size-independent playback |
| Save continuation | Mixed native flame/whirlwind records, town work, experience, random state and earlier controller/ID migrations |
| Fungus continuation | Version 10 preserves pending controllers, working bounds and tile stages; prototype marks migrate to seeds |
| Scenery continuation | Original variants, pool recycling, burial counters and save/load |
| Flame deaths | Retained death frames/slot reservations, current-cell damage and four-neighbor tree spread |
| Scenario runtime | Independent side rules, height admission, atomic prohibited edits, water, sprog, map visibility and save bindings |
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
A bounded world-zero simulation can finish unusually early under the current
movement/opponent adapters. These checks do not establish native campaign pacing.

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
