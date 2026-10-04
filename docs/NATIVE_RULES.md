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
