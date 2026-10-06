# Independent engine verification

Review date: 6 October 2026. The default `cmd/populous2` launcher and its
`cmd/populous2-go` alias use the independent Go engine. The earlier executable
translation remains available under `cmd/populous2-native` for reference.

## Runtime independence

The default dependency graph includes `internal/desktop`, `internal/app`,
`internal/engine`, `internal/music`, `internal/visualassets`, `internal/gamcodec`
and `internal/network`. Dependency tests reject the original translation,
Amiga/HUNK loader, legacy engine and native application packages.

The embedded package contains exported PNG artwork, named animation/font/palette
metadata, signed PCM, semantic music events and campaign/landscape data. It has
no instruction stream, register context, executable dump or relocated memory.
The game has been built and run outside the repository. Clean source archives
compile with the asset placeholder; playing requires local asset preparation.
Only import/reference tools consult the original executable.

## Implemented boundaries

| Area | Independent implementation and checks |
|---|---|
| Terrain | Four-hill generation, corner propagation, admission/debit, 19-stage town support/economy, persistent ground and slope rendering |
| Followers | Fixed-point motion, ordered searches/contacts, waiting, merging/leader transfer, combat, emigration, six heroes, claims, conversion, disease and retained deaths |
| Powers | All 29 admitted commands, bounded shared pools, mixed actor creation/movement/removal and inter-effect ordering |
| AI/scenarios | Campaign policies, prepared commands, explicit terrain/water requests, deferred execution, ten-event scripts and six neutral inventions |
| Campaign | 1,000 worlds/passwords, deity profiles/experience, scores, once-only awards, progression and original animated ending |
| Interface | Main menu, world/profile entry, custom rules, detached options/editor, real paced power previews, pause and visible errors |
| Persistence | Lossless JSON session, strict detached validation, overwrite confirmation and original GAM file codec |
| Multiplayer | Host/join menu, deterministic commands/snapshots, asynchronous TCP work, local-side controls/results and disconnect pause |
| Presentation | Original art, dynamic towns, all active actor/effect states, 8 appearance banks, source depth/anchors/backdrops/overview and Go music/sample replay |

## Evidence

The optional integrated reference comparison matches 160 complete no-input
main frames in world 12, landscape 0, seed `0x15b0`. It includes the original
rendering, clock advancement, simulation and deferred-command execution, then
compares all 4,225 height corners, 400 follower slots' identity/population/state/
fixed-point position/town stage, both mana balances, foundation metrics and RNG.
A separate original mixed lava comparison covers follower, tree, wall and magnet.

Every power is exercised for more than 400 passes with three semantic save
checkpoints and equal continued state. Representative worlds 0, 12, 27, 100,
250, 500 and 999 retain valid saved continuation. Campaign result/progression
and final-world ending routing are covered; ending pixels and PAL timing match
150 reference frames.

Rendering comparisons cover 576 fractional surface cases, 340 row/mixed-chain
traversal cases, original actor anchors, masked-plane high-object overlap,
storm shortened-sprite pixels and 162 edge-backdrop configurations. The semantic
audio exporter produces the same PCM as the earlier replay over 20 seconds
with soundtrack and effect cues. App-level cue gating prevents display redraws
from restarting a sound several times per simulation pass.

Original GAM tests cover unchanged original bytes, custom settings, metadata,
AI requests, contacts, six hero claims, combat/deaths, carrying/lightning and
consumed scenario events. Malformed JSON actor chains, inactive links and stale
positions reject before replacing the live session. TCP tests cover complete
and fragmented rounds, matching state, cancelled/disconnected sessions and
rule/digest mismatches.

## Limits

The comparison corpus is bounded. It does not establish identity for every
possible combination of all powers in every campaign world. Invalid out-of-map
memory behavior is bounded by checked Go coordinates rather than reproduced
through a CPU memory model.

Original GAM cannot encode every Go-only continuation field. Pending commands,
active deferred settlement deadlines and unknown original records are explicit
export/import errors; JSON remains lossless. TCP reconnection/resynchronization
is not automatic, and live rule editing/save/load requires leaving the session.

The desktop measurements and tests do not constitute Android validation. No
Android launcher/build target for this independent game is included in this
refactor. A later mobile adapter must preserve input, asset, audio and lifecycle
semantics and be benchmarked on the target device.

The new interface uses maintainable Go widgets over the original art. Its
capabilities are present, but its menu layouts are not claimed pixel-identical
to the original requesters. Full audible-event coverage and broader simultaneous
effect combinations can extend the existing checks without changing the
independent runtime boundary.
