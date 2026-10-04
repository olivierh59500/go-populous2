# Native rule translations

Addresses below are relative to the CODE hunk of the supplied French Amiga
executable. They identify routines and data, rather than runtime load addresses.

## Mana and elemental experience

`$14768` computes a power price from the unsigned word table at `$21238`.
The category lookup at `$147bc` chooses one of six unsigned experience bytes
in the deity record (`$52` through `$57`). The upper three bits select the
divisor table at `$2105e`: `0, 0, 10, 9, 8, 7, 6, 5`.

For a nonzero divisor the price is `base - floor(base / divisor)`.
Zero divisors leave the base price intact. Reserved `$ffff` slots remain
reserved. The affordability check at `$147e0` and cast debit at `$17e7e`
multiply this price by four to obtain the actual mana balance units.

For example, Fire Column has a base word of 5,625. It costs 22,500 mana
at experience 0 through 63, and 18,000 at experience 224 through 255.
Experience in another element does not affect this price.

The Go translation loads both lookup tables from the executable. Saved worlds
retain both deities' six experience values. Version 1 prototype saves remain
readable with zero experience. Experience awards and campaign progression
still require translation; having the price formula does not establish those
mechanics.

Terrain propagation and combat remain separate verification targets. Passing
the cost tests establishes this routine's arithmetic, not full-game parity.

## Followers and hero creation

The native pool has 400 records of 52 bytes. Record zero is reserved: allocation
at `$11846` starts at the next record and visits 399 usable slots. The Go pool
now preserves that limit and uses 16-bit map references, including in snapshots
and deterministic state hashes. IDs above 255 therefore remain usable.

Hero creation at `$142d4` detaches the leader. Heracles doubles the leader's
population (`$14396`); it does not multiply an inherited weapon statistic.
All six heroes add their elemental experience divided by eight to movement
speed. Odysseus also adds the original speed again. The result saturates at 255.
Ordinary movement speed comes from the campaign template's third word: its low
byte is copied from deity `$5f` to follower `$12` at `$10d3c`.

The animation pointers at `$20a00` address walking sequences in `$23d1a`.
Their frames point into the composite-image table at `$26956`. Signed image
offsets and chained layers are retained; this is essential for the additional
parts of Adonis and Achilles. All six heroes' eight directions are decoded.

These changes establish creation arithmetic and original artwork. The inherited
movement/combat engine does not yet use the native speed byte, and hero-specific
targeting, spawning and abduction still require translation.

## Starting populations and settlements

`$10b38` and `$10cbe` establish the starting groups from each deity's 58-byte
template. The first four words hold group count, initial population, movement
speed and search intelligence. Movement speed is not initial mana. Parameter 4
supplies initial mana at deity `$02` via `$10b70/$10c0e`; clearing the record
leaves its upper word zero. Parameter 5 supplies the per-side attrition word
at deity `$16` via `$10b6a/$10c08`. Both land `$130e8` and water `$11d64`
subtract it from the corresponding longword at `$14`; fractional movement
timing and original death animations still require separate translation.
The original scan starts at tile 1 for the first side and tile 4095 for the
second, first finding flat land and then falling back to nonwater land.

`$133c2` counts a center and eight surrounding support cells. If all nine are
available it checks another sixteen, then tests the outer ring for the largest
settlement. The stage lookup at `$13668` maps support counts to 19 stages.
Its 49 offsets at `$13684` are decoded without assuming that signed packed
coordinates are ordinary byte pairs.

Town work at `$117de` uses each stage's work counter, mana award, population
growth, population limit and emigration divisor from LANDn.DAT. Emigration at
`$11838` transfers `floor((old population + growth) / divisor)` out of the old
population. A full follower pool leaves the town population unchanged. These
tables and counters are now used directly. Terrain bookkeeping and land AI
still use adapters around the supplied first-game engine.

## Sound

FX.DAT contains a size prefix followed by 31 sixteen-byte sample descriptors.
Addresses are relative to the bank after the size prefix. Initial and loop DMA
lengths are unsigned words. Treating the loop length as a period would corrupt
both playback and sample boundaries.

The score at `$3e3f8`, initialized by `$1926c`, has four channel sequences and
133 patterns. The Go reader decodes note lengths, pitch lookups, signed
transposition, volume envelopes, period envelopes and sample changes.
The sound cue table at `$185a8` retains secondary cues, such as the additional
whirlwind sound. Direct cast cues are recovered from the command handlers.

PCM generation is deterministic across read sizes. The exact CIA low timer byte
and hardware mixing remain comparison targets; the current replay uses a zero
low byte for the driver's `$19xx` reload. Effects have bounded separate voices,
so they do not restart the background score.

## Power command identities

`$17524` maps command codes to handlers; `$210b0` maps commands to mana-table
slots. The water hero command 70 calls `$142d4` with hero type 10 and uses power
slot 33. Tsunami command 56 uses slot 34. The earlier prototype inverted these
two slots; save versions 1 and 2 are migrated when loaded by version 3.

## Terrain graphics and pointer targeting

`$be5a-$bef4` reserves graphics bank +16 for raised land. Animated sea-level
tiles use banks 0, +32, +48, +64, +80, +96, +112 and +128. The first game's
sea-level +16 terrain encoding must not be used as a II graphics index.
The renderer now chooses the native bank from the four actual corner heights.
Blue and red farms use tiles 47 and 63, respectively.

Picking tests the projected four-corner surface, including the raised corners,
instead of assuming every tile has the same sea-level diamond. Regression
tests cover the slope masks, water phases and shared corners after repeated
terrain raises and lowers.

## Terrain generation and random streams

The global random generator at `$f622` is a 32-bit multiplicative generator:
zero state is replaced by `$00bc614e`, then the state is multiplied by
`$bb40e62d` modulo 2^32. It returns bits 8 through 22 of that state.
This differs from the 15-bit generator used for world passwords.

The initial campaign state at `$11070` is the packed terrain/seed longword
from record offset 182, plus 727 for each subworld. Truncating it to a 16-bit
seed loses both the terrain contribution and arithmetic carries.

The four hills at `$cd22` use the parameter table at `$2072a`. Each takes two
seeds from the global generator, then uses independent local 15-bit walks
with multiplier `$24a1`, increment `$24df` and steps from -3 through +3.
A walk ends at height eight or when it exits the vertex grid.

Eight reference terrains were generated by executing the original relocated
68000 routine in an isolated memory image. The Go version matches all 4,225
vertex heights for each seed, including the east and south borders. Scenery
allocation is verified separately below. The private execution harness is
not part of the game or repository.

## Persistent ground effects and plague

`$16938`, `$169cc` and `$16a62` place fonts, swamps and greenery using 45
native offsets and a sampled attempt count. The original offset selection is
`floor((random % 90) / 2)`, not `random % 45`. These ground effects use original
tile codes 143, 168 and 245. Their maps and final RNG states match nine reference
casts executed by the original routines on empty flat land.

Fonts change a passing group's faith to the opposite side. Successive adjacent
fonts can reverse it again. Swamps kill walkers of either side. Greenery restores
flat damaged land. Entry animation, special hero immunity and later spread
rules are still verification targets.

Plague selection at `$1730e` infects opposing actors on the target tile.
Infection follows the actor, passes through contact, suppresses town mana and
removes its victims during Armageddon. It is not an expiring ground-radius
effect. The vulture sequence at animation offset `$ddc` is decoded, including
its original caw cue. Full native disease timing and immunity remain pending.

## Roads

Painting at `$1677a` selects straight, joined and slope road tiles using the
neighbor tables at `$168f0-$16938`. Connection bits and reciprocal updates are
preserved. A five-placement reference matches every tile code produced by the
original routine. Removal at `$16744` restores the underlying ground without
charging mana. The interface paints by dragging instead of selecting two ends.
Movement speed and fungus-barrier interactions remain separate work.

## Adonis after combat

The winner path at `$129fa` invokes `$146d8` for Adonis. With population above
twenty and an available follower record, the parent and clone each receive
`floor(population / 2)`. The clone clears combat targets and begins ordinary
hero movement. Recruitment alone does not cause splitting. The Go battle hook
retains the hero type and recreates its binding after save/load. The native
pool-full flag and stale-flag allocation failure remain edge-case comparisons.

## Helen and hazard immunity

Contact at `$12ade` replaces ordinary combat with captive links for Helen.
The victim's owner remains unchanged. State `$34` follows the preceding living
captive or hero, repairs links to Helen, and releases the victim if she dies.
The Go adapter preserves those ownership and lifecycle rules, including saves.
Its tile-based following still awaits the original fractional movement timing.

Helen uses ordinary enemy targeting at `$14414`, excluding already charmed
victims. No separate nearest-sea destination was found. The water-animation
table at `$20a60` gives Helen a zero entry: she can cross water while captives
can drown. The swamp-animation table at `$20a54` similarly gives Heracles a
zero entry. Both immunities are applied. Death animations and Helen's native
southeastern collateral footprint remain independent state-machine work.

## Natural scenery and forest planting

Trees and boulders share the original 200-record, fourteen-byte actor pool
at `$6bd0-$76c0`. Forest initialization `$d9d8` precedes boulders `$dbd4` and
initial followers `$10b38`. Allocation reuses the first inactive record and
preserves the exact random draws, including each successful rare-variant test.
The four original image variants and signed aging/burial counters are retained.

Three initializations executed in the original 68000 code match the Go actor
records and final random states exactly. Native forest casting at `$da0a`
reuses this pool and adds the vegetation experience contribution to its sampled
attempt count. Saved games retain these actors and reconstruct their tile index.

Initial placement can share scenery/occupied cells: the original prepends a
follower to its linked tile list. Ordinary movement and settlement support
reject boulders, while trees can remain under a settlement. Full linked actor
occupancy and the remaining destruction/interaction states still need work.

Register continuation is also retained during startup: `$cd22` leaves `D2=-99`,
and `$d9d8` does not clear it before `$da0a`. Its signed deity check can alias
the Y-fraction byte of scenery slot 45. Once that slot exists, the centered
fraction `$80` adds eight attempts. Seed 777 exercises this original quirk;
its full register-continuing reference produces 65 matching actors and RNG
`$2fe72d25`. Resetting registers between routines would produce 66 actors.

## Batholith

`$df68` samples a nearby point using two bytes of one random draw. A second
draw chooses either terrain raising or original boulder allocation. Successful
boulder creation uses the shared scenery pool and preserves the rare-variant
draw. Four native references match the full height grid, boulder variant and
final random state, covering both branches. Holding the button issues repeated
casts; the interface cadence still awaits original-input timing comparison.

## City walls

Placement `$1626c` uses a separate 200-record, sixteen-byte pool, leaving the
ground tile unchanged. After the first wall, a placement must connect to a
neighboring wall. Connections use `$332c8`; roads choose native gate artwork.
Construction advances through original frames and then holds the final frame.
Nine original actor/head reference states cover joins, gates, ownership, edges,
failed placements and construction ticks. Save validation retains native
inactive-head quirks while rejecting invalid references.

Sculpting rejects every propagated edit that touches a wall, preserving terrain
and mana. Crossing at `$141a2` uses the walker's Earth experience and population;
same-owner walls pass immediately. The native byte move preserves an owner-index
prefix in the register, so the original thresholds differ between sides. Strict
climb/break limits match 120 original CPU cases. Candidate routing leaves walls
intact; a committed strong crossing starts variant-specific break artwork.
Original broken walls retain their actor and terminal destruction frame;
32 native tick comparisons confirm that they are not automatically unlinked.
Fractional climb/hero attack remain separate states.
Walls are no longer substituted with RockBlock.

## Deity creation and profile passwords

`$10a84` gives a new profile five bolts. `$b8f8` consumes one bolt and adds
exactly one to the selected unsigned experience byte, rejecting 255 or an
empty balance. Face parts at deity `$4e-$50` wrap through eight variants.
The creation-screen artwork is decoded from the three FACES.PAK descriptor
banks at `$212ba` and composed with the original strip positions.

The sixteen-letter profile password uses `$1047c/$10564`: eight packed bytes,
bit transposition, fixed XOR words, multiplication by three, base-26 digits
and a character transpose. It stores faces, the low bolt nibble and six
experience bytes; it does not store the name or campaign world. The default
profile encodes as `KIADKCWAZICGZOWD`. Native comparisons cover 1,666 encodes,
1,666 decodes, 144 allocations and 48 face-cycle cases.

The editor imports these codes and applies experience to gameplay. Version 6
saves preserve the profile. Native scalar award and world-step thresholds are
verified independently, but full campaign scoring/statistic collection and
win/loss progression remain separate work.

## Actor projection on slopes

`$e392-$e422` selects one of sixteen piecewise height formulas from the tile's
corner mask. Fractional coordinates use unsigned bytes; X projection uses their
difference, while the Y formulas split the original triangular tile surfaces.
The Go formulas match 576 original instruction executions. Scenery, walls and
walking sprites now use this point of support instead of a fixed center height.
Native fractional movement and full linked actor drawing order remain pending.

## Fire columns

Creation `$15b7c` uses the first free 32-byte record of the 250-slot effects pool.
It samples a 3x3 position, consumes the native unused second random draw, and
starts centered in 8.8 coordinates with life `200 + Fire experience` and speed 16.
Introduction, active movement and extinction use sequences `$1a0`, `$4b8`
and `$660`. Intro completion and extinction transitions fall through during
the same update; map exit removes the actor immediately.

The active route scans eight neighbors from a randomized start, rejects lower
base elevation and uses successive random bits for equal heights. Original
offset-zero fallback behavior is preserved. Its reroute timer is 30 updates.
Original composite flame frames contain up to 47 layers, exceeding the earlier
decoder's artificial 32-layer limit; actual cycles remain rejected.

Burning `$1735a/$16542` affects the current tile, without an owner filter or a
radius damage amount. Suitable flat terrain becomes code 95 and ceases to
support settlements. Achilles is immune to the direct-hit death table;
tree-spread fire instead spares heroic walkers and visits four orthogonal cells.
Original death images retain their follower slots until the sequence completes.
Native fixed-point effect and death state are preserved in version 7 saves.
The twelve isolated traces verify column motion/state and ground mutation on
empty terrain. Actor death rendering/slot reservation is currently an adapter;
complete linked-list death metadata and mixed-actor traces remain verification
targets, rather than being implied by the empty-terrain oracle.

## Whirlwinds

Creation `$15c3e` shares the 250-record effect pool with fire columns. It starts
at the selected cell center without jitter or random draws, retains reused
velocity words, and uses life `200 + Air experience` with speed 24. Phases 8,
10 and 12 use intro/active sequence `$4c8` and extinction sequence `$6cc`.
Completion and expiry fall through in the same update. Active frames retain
the original negative loop offset, rather than restarting every sequence.

Routing `$14a8e` scans eight neighbors from a random starting offset and prefers
lower elevations. Neighbor geometry bit zero adds one to its base height;
equal-height choices consume successive random bits. The offset-zero fallback
is preserved. Although `$14b34` computes `255 / speed`, the next random draw
overwrites it: the actual reroute timer is `random & $78`. Successful movement
consumes another draw and requests a whirlpool when its remainder modulo five
is zero. Map exit retains the last valid position before removing the actor.

Twelve independent 68000 parent traces cover flat ground, a slope, water and
the map edge with Air experience 0, 32 and 255. All 3,550 updates compare actor
fields, the full random state and every terrain code; 713 child requests match.
Other effect records are occupied, so original child creation still executes
but cannot allocate. Empty tile lists exclude follower/town interactions.
This boundary validates the parent controller without implying child parity.

The integrated controller uses original composite frames and audio cues, has
no generic radius damage or scorching, and survives version 9 saves. Earlier
generic whirlwinds migrate into native records while retaining their remaining
life and direction. Original pickup/immunity, town collapse, captured-follower
release and water-only child creation remain separate controller work.

## Main-loop cadence

The main loop runs the clock, people, strategy, effects, walls and scenery in
that order. `$72e` clears a VBlank flag and `$786` waits for the next blank.
The game therefore uses a nominal 50 Hz PAL simulation cadence. Original CPU
load may reduce throughput; special mouse-idle pacing is separate. The long
counter at `$f40` advances once per unpaused loop, with `$f42` as its low word.
The literal eight at `$f0c` is the viewport size, not an eight-Hz tick setting.

## Per-side scenario rules

Each 58-byte player template contains its own option word at parameter 6
(deity `$66`). Bits 0/1 admit editing everywhere or only at sea level; the
native `$d91a/$19de/$1b38` paths inspect vertex height, not nearby population.
Sea-level-only raising requires height zero, while lowering permits zero/one.
Bits 3/4 independently forbid raising/lowering. Bit 2 protects enemy farmland
on every cell affected by propagation; rejected edits preserve heights, farm
codes, counters, overlays and mana. Computer terrain orders use the same path.

Fatal water (bit 5) is evaluated per follower side. Minimap enemy/disaster
visibility (6/8) uses the observer's options. Right-button release (7) is checked
before the ordinary lowering restriction. A shallow swamp (9) restores the
victim's tile when that side's rule is set. Version 8 saves preserve both raw
option words, including unidentified bits, and restore their runtime bindings.
Special editor/battle admission modes and scripted world commands remain
separate verification targets.

The rules requester displays both sides independently. Conquest parameters are
read-only; custom-game flags can be toggled without discarding unidentified raw
bits. Custom choices survive restarting and loading. Menu previews use the
selected campaign world rather than the last played world's rules.
