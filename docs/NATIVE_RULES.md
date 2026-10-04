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
speed and search intelligence. Movement speed is not initial mana. The deity
record is cleared before setup, so the new game begins with zero mana.
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
