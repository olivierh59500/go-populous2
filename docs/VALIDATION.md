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
| Scenery oracle | Three matching original object pools and final random states |
| Followers | 399 usable records; references above 255 survive save/load |
| Mana | Native divisor thresholds, quarter-mana units and propagated sculpt debits |
| Heroes | Six conversion types and eight-direction composite artwork |
| Ground rules | Persistent fonts/swamps/greenery, two-way faith reversal and plague contact |
| Audio | 31 samples, 133 patterns, signed PCM and read-size-independent playback |
| Save continuation | Effects, town work, experience, random state and earlier water-ID migration |
| Scenery continuation | Original variants, pool recycling, burial counters and save/load |
| Desktop application | Bounded launch, native score playback and application-buffer PNG capture |

The native oracle executes the supplied executable's relocated routines inside
an isolated memory image. Its harness and raw results remain local; production
tests store only the derived reference hashes. The game executes Go code.

Graphics-bank tests cover the shore/raised-land distinction and every water
animation phase. Repeated raises/lowers verify shared-corner continuity and
slope representability. Picking follows the actual four-corner surface.

These tests validate the documented portions of the conversion. They do not
establish complete original-game parity; [FEATURES.md](FEATURES.md) lists the
remaining work. Previous prototype simulation hashes are superseded by the
native economy, random generator and terrain changes.
