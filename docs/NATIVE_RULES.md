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

Creation now uses the complete `$142d4/$142fe` routine in World. The raw deity
leader reference is its only admission test: the routine does not independently
reject zero population or a missing leader flag. `$140ae` relocates the magnet
and clears the leader flag when present; a town additionally runs `$135ca` with
replacement tile 15. Conversion sets hero flag bit 1, kind 2, state `$24`, hero
word `$28`, and the original walking-table offset at `$32`. It clears only the
animation word and writes no new leg, target, velocity or timer. Heracles's
population doubles with native 32-bit wrapping. Native sound arguments and
speed saturation are preserved.

1,104 complete original CPU cases cover the six types, both sides, all 19 town
stages, leader flags, speed/experience boundaries, population overflow, missing
leaders and direct conversion. Both the standalone helper and real World
callbacks match every BSS byte from 0 through `$eb18`. Normal game tests cover
conversion debit, marker relocation and 180 updates of saved continuation for
each hero. The fractional-motion adapter now preserves entry-owned flags, so
it cannot discard leader or disease flags while copying an ordinary leg.

Heroes use the shared native routing, contact, motion and combat controllers
described below. These checks do not establish every combined environmental,
wall-climb or animation interaction.

## Starting populations and settlements

`$10b38` and `$10cbe` establish the starting groups from each deity's 58-byte
template. The first four words hold group count, initial population, movement
speed and weapon strength. Allocation at `$10d36` initializes search byte
`$18` to two; `$10d42` copies the fourth template word's low byte to weapon
byte `$19`. That word does not initialize search intelligence. Movement speed
is not initial mana. Parameter 4
supplies initial mana at deity `$02` via `$10b70/$10c0e`; clearing the record
leaves its upper word zero. Parameter 5 supplies the per-side attrition word
at deity `$16` via `$10b6a/$10c08`. Both land `$130e8` and water `$11d64`
subtract it from the corresponding longword at `$14`. Land subtraction occurs
at decision dispatch, rather than at every ordinary fractional movement tick.
Native water/waiting timing and remaining death animations are separate work.
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

Emigration at `$118c8` assigns the town stage to the child's weapon byte and
twice that stage to its search byte. It retains movement speed; it does not
copy the parent town's current weapon/search values. Ten native allocation
cases and all nineteen emigration stages establish these separate fields.

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

The supplied executable also assigns command 24 to whirlpool creation
`$1775a/$15cc8` and slot 31, while command 74 calls the directed basalt routine
`$17b24/$171ea` and slot 30. These names were inverted in earlier Go versions.
Version 12 migrates old numeric IDs 30/31 in effect records, ground marks and
the last selected spell, without mutating the caller's effect slice.

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
Ordinary walking now uses native fractions. Other follower state movement and
full linked actor drawing order remain pending.

## Ordinary follower motion

`$13126` computes signed 8.8 velocity from the unsigned speed byte at record
`$12`. Each nonzero component is exactly that speed; diagonals are not normalized.
A target in another cell uses `floor(256 / speed)` updates. Same-cell targets
use the larger fractional distance. Speed zero raises original processor
exception 5; the Go controller returns an error for that unsupported case.

State 4 at `$1156c` advances a four-frame animation clock independently of
speed, decrements the leg timer, and moves while it is nonnegative. Expiry
redispatches record `$17` during the same update. Ordinary search state 2 can
start and move a new leg immediately, including a second clock advance after
expiry. The magnet handler `$11bb4/$140f0` instead sets return state 18 and
ends the update before its first move; `$14646` recenters its fractions.
Target selection itself remains an inherited adapter.

Crossing checks at `$115c6` run when the high coordinate bytes change, not at
leg completion. Rejected terrain applies the original bounce lookup at `$1171e`
without committing position. Lookup bytes supply signs, not magnitudes; two
bottom-row indices read adjacent instruction bytes, retaining their signs.
`$12518` relocates native linked membership immediately at a committed crossing.
The current world bridge preserves that crossing point, with single-head
occupancy and contact still awaiting the full mixed actor graph.

Hazard prepass `$12c3c` runs before initial dispatch and again before timer-expiry
redispatch. It does not run immediately on the new cell after a committed move.
The integrated controller owns those prepasses for ordinary walking; the
inherited fallback does not advance the same walker a second time.

Walking artwork at `$e658` uses current velocity, the original angle lookup at
`$f71c`, direction offsets `$20d34`, owner/variant banks `$209e0/$209f0`, and the
current four-frame clock. Its asymmetric shift registers can give different
facing at high speeds; a sign-only direction approximation would change the
original images. All ordinary variants retain their composite layers. Their
original walking-frame sound cue words are zero.

Seventy-two independent traces cover eight directions and speeds 20, 40 and
255 on flat ground, slopes and water boundaries. They compare all 528 updates,
rendered animation offsets and every cell head. Two full-dispatch references
also prove clock fallthrough and the one-versus-two prepass count. World tests
replay each fixed leg through the actual Core hook up to its next decision
boundary, verify contact timing, reused-slot generations and saved continuation.

Version 12 saves preserve ordinary fractional motion, clock and timers.
Allocation hooks reinitialize startup, birth and clone slots, including a reused
slot on the same cell with the same owner. Earlier saves start centered with
per-side template speed. Waiting, swimming, hero movement, search, settlement
and combat are still inherited state/decision adapters; these motion traces do
not establish their native parity.

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

## Fungus and Renew Land

Fungus creation at `$15fda` writes tile 145 when its terrain properties match
`$27`, before trying the shared 250-record effect pool. The command handler
does not make its mana debit conditional on successful planting/allocation.
One collecting controller per deity waits 100 effect updates. Recasts extend
its inclusive rectangle without resetting the wait or period. After that
pending reference clears, additional controllers can coexist and evolve in
shared-pool order.

The period is `10 - (Plants experience >> 5)`, and a generation takes period
plus one updates. The transition out of collection falls through into aging
in the same update. Tiles progress through 145–149 and 150–151 back to 15.
Generation at `$14eea` counts neighbors whose properties equal `$10` exactly:
living cells survive with two or three; eligible dead cells are born with
three. Transitional birth/death properties preserve synchronous counts
during the original in-place scan. The controller consumes no random numbers
and applies no area-damage substitute.

The common follower terrain prepass calls `$12ec2` before state dispatch.
Fungus property bit 4 is fatal only when bit 0 is clear: fresh tile 145 is
harmless, while mature tiles 146–150 are dangerous. Heroes use the death
table at `$20a48`; its zero Adonis entry grants immunity. Ordinary death uses
animation `$7dc`, kind `$10`, state `$38`, centered fractions and sound offset
`$104` (descriptor 26). The adapter removes the group from live population
while retaining its decoded death frames and slot/occupancy reservation.
Version 11 saves preserve those states and sound serials, reject invalid
animation spans/duplicate reservations, and resume frame completion.

The inherited follower dispatcher still supplies the surrounding scheduling,
population bookkeeping and occupancy adapter. Native redispatch timing,
full linked death records and remaining terrain-prepass branches remain
verification targets; visible retained artwork alone does not establish them.

Renew Land at `$16a62` scatters tile 245 and never creates this controller.
Its properties are `$01`, so it is eligible ground rather than a living
fungus neighbor. Version 10 saves retain pending references, packed working
coordinates, timers and inclusive bounds.

The original bottom-edge clamp at `$150b0` subtracts the X minimum where a
Y minimum would be expected. The controller preserves those bytes and linear
row aliases. Access beyond the tile map is bounded: reads return zero and
writes are ignored. This differs from accessing adjacent original BSS and
remains an explicit fidelity gap rather than an unreported correction.

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

## Mixed actor map and water controllers

The four-byte cells at BSS `$f44` retain a header, terrain code and signed
reference to the mixed actor chain. References are byte offsets from `$76c0`:
200 sixteen-byte walls, 200 fourteen-byte scenery records, 399 usable
fifty-two-byte followers and 250 thirty-two-byte effects. Next/previous links
are shared fields; the wall's deity-chain word remains separate metadata.

Insertion `$125a0` prepends without changing movement pressure. Removal
`$125da` clears links without decrementing pressure. Movement `$12518` writes
full coordinates and, on a changed tile, adds eight to the destination header.
This wraps; pressure cannot be reconstructed from occupant counts. Real terrain
edits clear the header's upper bits, whereas overlay writes preserve them.
Version 13 retains these cells and links. Remaining inherited handlers use
notifications and a contact adapter; full native contact/death/drawing order
still requires separate translations.

Whirlpool creator `$15cc8` requires four exact zero-code water cells and stamps
terrain codes `$98`–`$9b`. Its unmapped shared-pool controller at `$14cae`
cycles four quad frames, moves every sixteen updates and lowers native vertices
at the coast. Life is 300 plus Water experience. Raw-grid barrier reads and
corner-table retries retain their native addressing quirks. A view-dependent
sound request uses descriptor 125 and does not enter the shared save/hash.
Whirlwind water-child requests use this same creator and ordered pool; later
child slots run during their birth pass.

Basalt creator `$171ea` uses slot 30, writes `$e0`, and links a propagation
actor. Its lifetime starts at 100 plus Water experience. At a random delay
expiry it attempts one child with its remaining lifetime before removing the
parent, even if creation fails. Directions 0/2/4/6 are north/east/south/west.
The terrain persists without raising vertices into an invented straight path.
Subsequent sculpture preserves the `$e0` prefix and updates its shape nibble.
The entire `$e0`–`$ef` family has zero hazard/water properties.

The unpriced direct lowering hook matches 66 native height-grid cases,
including corners and the native operation's lack of an Armageddon input gate.
Basalt world comparisons retain the complete pool, raw grid and RNG through
40 ordered traces. These empty/controlled actor cases do not establish all
wave, volcano, fire and inherited-handler interactions.

## Lightning marker, activation and victims

Commands 28/30/32 place, activate and dismiss the owner marker. Placement at
`$15de2` consumes no mana and preserves an existing marker's lifetime.
Activation at `$15e8a` allocates `2 + (Air experience >> 5)` jittered bolts
in the shared pool; partial allocation is retained. Its command handler charges
slot 18 even when the helper cannot create another bolt. Marker word `$1a`
holds the bolt chain; each bolt's `$1c` identifies its marker. Both actor kinds
retain native map links. Dismissal removes the bolts and plays the marker outro.

The bolts walk the ordered mixed occupancy chain without an owner filter.
Walls stop the scan/scorch path. A struck walker/town enters its native effect
state and loses `(int32(population << 3) >> 7) + 4` per managed update, preserving
32-bit signed arithmetic. These records do not contribute to the current
population total while managed and cannot be reused merely because population
is zero or negative. Referenced-effect liveness tests the positive owner byte,
so an occupied reused slot can keep an older victim reference alive.

Walker cleanup retains its death animation and map reservation, then removes
it on completion; surviving walkers play recovery before ordinary redispatch.
The common terrain prepass can supersede lightning, including the first water
entry. A town survivor uses the native `$13352` support evaluator and its
49-cell farm compositor, preserving the town's work timer and founding tick.
The remaining inherited water/hero transitions and complete founding/contact
state dispatch are still separate requirements.

Beam artwork is four procedural lines in palette index 5, with three native
alternating kinks. The marker sprite is raised 80 native pixels. The beam's
marker endpoint is raised 90 pixels and uses the CURRENT BOLT cell's header
height; it does not use marker fractions or marker-cell height. The endpoint
is clipped at the top. Version 14 saves retain chains, random words and victim
states; old generic radius effects migrate to markers rather than replaying
invented damage.

## Ordinary search and road decisions

State 2 selects targets at `$1134e`–`$1156c` after caller-owned attrition.
The preferred scan uses the native search-index byte and ordered mixed records;
nonpreferred search consumes exactly one random draw and chooses among valid
neighbors using the raw header pressure bits. Boulders remain visible to the
metadata resolver even when their owner byte is zero. Join/fight modes request
the corresponding native owner; magnet and hero routines are delegated.

Road priority is evaluated before the later boulder scan. A road leg adds the
native twenty-unit speed bonus, initializes velocity/timer, then subtracts that
full bonus from the speed byte. Saturation can therefore leave speed 235 with
velocity 255; normalizing velocity back to the stored byte would change the
original leg. The save validator accepts those proven native velocities.

The World adapter replays 49 nondelegated original cases. Target selection does
not settle on the current tile: native settlement and delayed friendly/enemy
contacts begin in `$1275a` and its subsequent state handlers. Their full World
integration, native magnet/hero decisions and the remaining inherited contact
path remain required work.

## Settlement support and farm artwork

The native town evaluator uses all 19 stages and a 49-cell footprint. Its
support scan examines the center and successive rings; trees permit support
while boulders block it. The property word's high owner bits govern support,
independently of the low farm flags. A cached nonzero stage can survive between
its assigned four-tick evaluations when the center retains support properties.

Painting and removal use the original structure tile table and eight neighbor
overlay codes. Healthy centers use the stage table at `$20b14`, not consecutive
sprite numbers. The kind-4 renderer adds eight native pixels before drawing
the composite; its owner flag alternates with the tick and rises with population
according to the stage divisor. Neighbor overlays remain separate. Competing settlements
lose their farms and return to ordinary follower state through the evaluator.
World town updates and lightning recovery share this compositor. The standalone
component matches all 530 native cases; the World adapter matches the 523 cases
that do not require the remaining competitor/contact integration.
The work timer remains an unsigned 16-bit word: incrementing 65,535 wraps to
zero before the production threshold is tested. Record projection and saves
retain that timer independently of the support-stage calculation.

## Retained native record bytes

The retained image covers all 34,800 bytes of the four native actor pools,
including reserved and inactive slots. Typed follower changes patch only fields
that changed, preserving bytes whose meaning depends on the current state.
Map-link changes update the corresponding image fields. Reads and writes use
byte offsets, so an unaligned reference can affect a neighboring physical slot.

Eleven original alias cases verify complete image hashes, and every one of the
1,050 physical slots has coverage. This is evidence for byte storage and its
typed adapters, not proof that all original actor handlers have been integrated.
World entry/combat dispatch and full affected-pool hydration are now integrated;
remaining hero creation, environmental controllers and complete campaign state
still require their own native comparisons.
Save version 15 retains the record image, follower entry state and town overlays.
Regression checks also cover an explicit tile-zero write on raised terrain,
retired effect fields, reused follower generations, and the record projection
of lightning victims and demoted towns.

## Deity records, markers and actor cleanup

The retained runtime window extends the actor image with three 14-byte marker
records at `$e740` and three 314-byte deity records at `$e76a`, ending at `$eb18`.
Owner-zero records and unknown bytes survive. Primitive graph operations accept
the markers' references `$7080`, `$708e` and `$709c` as well as ordinary actors.

Creation at `$14048` assigns centered coordinates, kind `$14`, owner and the
owner's animation pointer. Relocation at `$13fe4` clamps signed coordinate bytes,
removes the old links and prepends the marker without adding pressure. Fifteen
original image/grid comparisons cover these operations. The game initializes
these records from its current player state; complete native game initialization
remains a separate fidelity requirement. Version 16 saves retain the raw globals,
validate marker links/coordinates, and preserve unidentified deity fields.

Cleanup at `$124a2` subtracts two from metric `$44`; loss of a leader also
increments `$46`, subtracts ten more, clears its flag/reference and relocates
the marker. All arithmetic retains original word/byte behavior. The cleared
source byte is `$13` (decimal 19), while hero flags reside at `$0d` (decimal 13).
Hero cleanup can use the owner's deity record as its A0 context; reconstructing
A0 as the source would alter the native writes. Mode zero clears owner and links;
all modes clear population. Positive-owner death/ruin records remain reserved.

Forty-nine full original BSS images, ordered writes and output registers verify
the routine. Raw helpers run without intermediate typed projections; hydration
then updates their affected records, markers and player data. This storage and
cleanup integration does not establish complete native hero or campaign behavior.

## Combat and its retained outcomes

States 14/16 at `$11a86`/`$11b6a` preserve delayed reciprocal contacts. Only the
aggressor applies damage and consumes one random draw. The routine subsequently
overwrites its randomized calculation, so both damage amounts use the aggressor
population's divisor-100 quotient. DIVU overflow retains the dividend's low word.

Winner resolution at `$1298c` uses current LAND reward tables, conditional
leader/hero bonuses, signed loser-mana clamping and the original town/cleanup
order. Adonis halves its population before searching for a free owner-zero slot;
allocation failure retains that halving. Positive-owner zero-population records
cannot be reused. The separate inhibition flag is checked before halving.

Post-combat handlers retain death sprites and map membership until their native
terminals. A recovering winner returns to search on the next update. A destroyed
town becomes a ruin with a 400-word countdown, tested against the native raster
table. These components have complete isolated routine comparisons; their
World contact/combat dispatcher now composes these routines, with the common
prepass before managed states and a raw call boundary around nested operations.
Native movement crossings trigger entry rather than the first game's immediate
contact routine. Friendly contacts retain their homing leg before merging;
attacker/defender states retain their reciprocal links, and owned zero-population
deaths remain mapped through their terminal animations.

Terminal handlers preserve raw aliases in the native dispatch table: states
`$08/$2c/$2e` share `$1199e`, `$18/$20/$32/$38/$3e/$40` share `$11e00`, and
`$1a/$2a/$42` share `$11ce8`. They use their shared handler without rewriting
the stored state. Ordinary starvation keeps animation `$7f4`, positive owner
and map membership until its second, terminal cleanup. A World test verifies
that complete lifecycle.

Component movement fixtures explicitly omit the entry call that their original
CPU harness stubs. Separate World tests cover actual entry, delayed merging,
reciprocal battle, passive-defense RNG, same-update waiting redispatch and saved
death continuation. Bounded 900- and 1,800-update desktop runs completed and their application
capture was inspected. These checks do not establish every hazard/hero path:
hero creation/art boundaries and complete environmental/campaign behavior remain
required. Later integration replaces the ordinary fungus-only prepass with the
complete native prepass and dispatches water/conversion/burning immediately
without a second prepass. A frame-local adapter flag prevents a second
population-total addition.

The hero planner reads the original `$f12` latch, set by Armageddon and cleared
at new-world initialization. It is not inferred from an inherited game option.
Version 17 saves retain this latch and the native allocation-inhibition flag.

## Water, conversion, burning and magnet routes

The `$11d1a/$11e3a/$11caa` handlers have 554 complete original comparisons.
Water uses SUB.L's branch flags, unlike `$130e4`'s later MOVE.L stored-result
test. It writes deity word `$36` for a surviving swimmer and returns to search
immediately on land. Fatal water starts `$196c` and advances its first retained
death frame in that update. Conversion retains fractional coordinates and
original map pressure; burning preserves the low-word carry behavior of ADD.W.

World managed and ordinary dispatch compose these rules with the common prepass.
Tests cover the first water update, exactly one population-total addition, the
deity reference, retained death and terminal release. The former fungus sidecar
test now follows the actual native record through the Core loop and save/load.

Magnet states `$12/$3a` use the genuine hero terrain planner, marker/leader
references, six-tick waiting timer and source flag transfers. Search switching
to magnet mode pays attrition in `$1131c` and again in `$11bb4`; the two calls
retain distinct death boundaries. The default World path uses this controller
instead of inherited target planning. The 663 original comparisons and World
arrival/wait/save cases do not prove all command initialization or campaign AI.

## Neutral actors from rare births

The creator `$131cc` scans owner-zero follower slots and assigns only its
documented fields. Neutral owner 3, kind `$3c`, state `$44` records retain stale
unassigned bytes. Selector 4 can allocate a second copied record; failure to
find its second slot preserves the first actor. The 105 full image/grid cases
include pool exhaustion and the allocated selector-zero failure path.

State `$44` moves with its native velocity, loops its animation and dispatches
the selector's original tile/lowering/whirlwind/tree/fire-column/victim operations.
Its 252 comparisons retain complete raw actor/grid/RNG state with external
primitive bodies recorded at their original boundaries. World now composes the
native primitive creators and neutral effects; full original-machine composed
comparisons, including populated victim neighborhoods and direct-lower register
continuation, remain required before this environmental path is fully proven.

## Captive and crossing integration

Captured state `$34` preserves the original A1 target even after backlink repair
changes word `$2a`. Its 446 original comparisons include the `$35c` alias emitted
by Helen's contact routine. World dispatch composes its native attrition, planner
and return-state `$34` rather than teleporting captives to the captor.

Crossing at `$115c6` reads the destination header and native raster/property
tables, records deity terrain requests, scans mixed boulders/walls and applies
the original signed break versus unsigned passage thresholds. Its XP calculation
clears the upper register bytes, unlike the hero planner's separate wall probe.
The 2,528 complete comparisons retain raw hint/wall writes and exact branch
outcomes. Invalid odd stage words report the native address-error condition.
Both ordinary and managed motion use this admission routine; wall destruction
executes its recovery branch before any position change. The existing native
motion comparisons still own bounce timing and velocity proof.

## Native economy in the World loop

State 6 now executes `$11738/$117de` through raw callbacks, with the active LAND
tables. Per-pass initialization resets deity counters and preserves original
mana/population maxima, matching `$11252`'s boundaries. Native population totals
are added at each handler's `$123b4` return, rather than reconstructed afterward.

Work is a wrapping word. Successful emigration subtracts the quota from the
old population, discarding that cycle's apparent growth. The scan uses only
owner-zero bytes and sets `$dc2` after a full-pool failure; preexisting inhibition
skips the scan and keeps growth. Higher newborn slots run during that same
increasing-address pass, while lower reused slots wait for the next pass.

The rare slot-250 birth keeps both random draws, their overwritten first
coordinate and raw `$131cc` request. Its neutral owner-3 record remains allocated
with population zero and runs without entering either player's total. Raw tree,
whirlwind and fire-column creators preserve native owner-word/XP/RNG behavior.
Neutral whirlwind children create owner-3 whirlpools with zero Water experience.

Version 18 saves include those records/effects and the wrapping rare-creation
deadline. World tests compare production, same-pass births, rare creation, all
four landscapes and byte-identical save continuation. Headless 12,000-update
runs provide regression evidence; native scenario AI, award/score and complete
campaign progression remain separate requirements. A desktop launch for this
slice failed before game startup because Ebitengine could not obtain a macOS
monitor; it supplies no visual fidelity evidence for this particular slice.

## Environmental controller ownership

The shared effect loop dispatches quake, volcano and lava through retained
controller ownership. Their creator can leave byte 0 unchanged: selecting the
controller from that stale kind would incorrectly run a recycled fire, fungus
or basalt record. Quake/volcano controllers are unlinked; lava retains native
mixed-map membership and its original sprite bank. Version 19 saves preserve
controller ownership and validate each native phase, owner and position.

Quake uses the direction table `[0,5,1,4]`, native strength and crack/fade tiles.
It charges its command price even on full-pool failure. Failed edge child
creation can return the parent; full-pool return `$e740` can alias marker bytes
during the subsequent compensation write. The World adapter replays 31 original
scenarios and 3,061 updates, comparing the complete pool, map, height grid and RNG
with actual terrain reconstruction. The standalone catalog also covers the
three synthetic full/parent-alias scenarios.

Volcano growth follows the exact linear-index lowering arguments and cumulative
center operations. Final eruption calls raw fire creation with owner words
`$ff01/$ff02`, retaining the native XP-memory alias. Lava and basalt creators
run at their original boundaries and read links after pushes. Twenty-four
complete unoccupied-pool World eruptions compare raw pool, map, all heights,
source bytes and RNG. Standalone eruption/Lava catalogs cover additional pool
limits and actor/hero interactions.

These comparisons exposed and fixed tile-prefix preservation: `$e0..$ef`
terrain survives height edits according to the native tile family, regardless
of which spell first wrote it. The renderer uses native lava composites and
terrain cracks, while unlinked controllers do not inherit stale sprite kinds.
The missing macOS monitor prevents a new desktop visual check of this slice;
headless simulation and saved continuation remain valid regression evidence.

## Native cloud and meteor weather

Storm command 64 creates raw kind `$36` clouds in positive 0..7 offsets from
its origin. Admission follows the creator's original Z flag: partial creation
followed by pool exhaustion leaves clouds but rejects the debit; clipped attempts
can admit without any new cloud. Each spawn clears word `$1a` at the caller's
A0 context. Local command records are `$eb56/$eb60`, so their writes alias later
bytes of the command/control region. Version 20 retains the bounded region
`$eb18..$eb90` and its actual byte/word/long assignments.

Storm ticks retain cooldown, main cloud animation and word `$1a` flash animation.
The linked `$16542` damage scan handles trees, immunity, retained follower deaths
and complete town destruction. A successful no-hit strike can scan again; it
is not replaced by radius damage. The 710 original cases prove creation and
runtime; World callbacks replay all 546 runtime cases/1,541 updates against
complete retained memory and RNG.

FireRain creates initially unlinked kind `$2c` meteors with delay and lifetime.
Activation links then advances the first falling frame in that update. Falling
Y offsets are already encoded in the `$81c` image bank; adding a life-based
height offset would apply the fall twice. Unlinked delay phase `$1c` is hidden.
Impact uses the original terrain height, scorch and shared linked damage scan;
empty hits start `$49c/$5ec` effects, while hits can remove the meteor immediately.
The 809 original cases include 1,450 runtime updates and exact primitive order.

World tests cover command admission/cost, preserved partial clouds, bounded
command aliases, delayed meteor visibility, normal-loop terminal release and
byte-identical save continuation. Earlier generic weather effects migrate to
native records without replaying the old area-damage logic. Native wind, tsunami,
scenario scripts and all command scheduling still need their complete mappings.
