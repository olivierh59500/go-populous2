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
