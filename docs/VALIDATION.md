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
| Scenery oracle | Original object pools/RNG, including register-continuing seed 777 |
| Batholith oracle | Four height/object/RNG references covering raising and boulders |
| Wall oracle | Nine actor/head states, gates, joins and construction ticks |
| Deity oracle | 1,666 native encodes/decodes; allocation, face-cycle and scalar threshold references |
| Actor projection oracle | 576 original slope/fraction positions |
| Fire column oracle | Twelve full traces: actor, RNG and every terrain tile per update |
| Whirlwind oracle | Twelve parent traces, 3,550 matching updates and 713 child requests; actor, RNG and terrain compared |
| Followers | 399 usable records; references above 255 survive save/load |
| Mana | Native divisor thresholds, quarter-mana units and propagated sculpt debits |
| Heroes | Six conversion types and eight-direction composite artwork |
| Ground rules | Persistent fonts/swamps/greenery, two-way faith reversal and plague contact |
| Audio | 31 samples, 133 patterns, signed PCM and read-size-independent playback |
| Save continuation | Mixed native flame/whirlwind records, town work, experience, random state and earlier controller/ID migrations |
| Scenery continuation | Original variants, pool recycling, burial counters and save/load |
| Flame deaths | Retained death frames/slot reservations, current-cell damage and four-neighbor tree spread |
| Scenario runtime | Independent side rules, height admission, atomic prohibited edits, water, sprog, map visibility and save bindings |
| Scenario oracle | 648 native decision/init/attrition cases, including 400 mixed observer/victim-side checks |
| Scenario native oracle | 248 cases: 112 height admissions, eight initializations and 128 per-owner attrition updates |
| Deity interface | Native face parts, name, experience, password import and version 6 profile saves |
| Desktop application | Bounded launch, native score playback and application-buffer PNG capture; three whirlwind composites, shutdown after 180 updates |

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
removal. Death traces stop before native animation helpers, and land movement
still uses the adapted dispatch. These fixtures do not establish full native
follower timing, death transitions or propagated-edit rollback parity.

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
