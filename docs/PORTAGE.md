# Conversion and fidelity

This is a playable conversion in progress. Original resources and several
native routines have been translated, while the supplied Populous I engine
still provides parts of movement, combat, terrain bookkeeping and land AI.
[FEATURES.md](FEATURES.md) tracks the complete original feature inventory and
all 29 powers. [NATIVE_RULES.md](NATIVE_RULES.md) records the newer translations.

## Native formats

Addresses refer to offsets within the supplied French executable's CODE hunk.
Relocated references can target other hunks; a printed raw address is not
necessarily an address inside CODE. The segment also contains data and text,
so linear disassembly alone is insufficient to identify routines.

| Offset | Translation |
|---|---|
| `$103c6` / `$103f8` | World passwords and the 64-syllable table |
| `$105ca`–`$1069c` | Backward decompression and XOR validation |
| `$10df2` / `$11044` | Player templates and 200 campaign records, five worlds each |
| `$117de` | Nineteen-stage settlement work and economy |
| `$bef4` / `$bfac` | Tiles made from six 16×8 fragments |
| `$19cd0` / `$19e4a` | The 26-resource loader catalog |
| `$1a3f0` | Tile fragment arrangement for the blitter |
| `$1a51e` | Sprite differences, including landscape variations |
| `$21238` | Unsigned base cost words and reserved power slots |
| `$21626` | 830 relocated sprite descriptors |

LANDn.DAT contains six 19-word tables, three parameters, 256 map-color values,
two sixteen-color Amiga palettes and a final word. Thirty-two-pixel sprites
contain two mask/plane groups per row. Treating them as five 32-bit planes
corrupts the images.

## Original-machine comparisons

The Go terrain generator matches eight independently executed native reference
terrains, including all border vertices. Fonts, swamps and greenery match nine
original flat-ground cast maps and final random states. Runtime gameplay uses
Go translations; the isolated instruction-execution harness is local analysis
material and is not shipped with the game.

Native simulation parity is not yet established for the other systems.
In particular, deterministic saves and reproducible demonstration runs are
regression checks, not evidence of original movement or combat behavior.

## Remaining conversion work

- Native movement, linked tile occupancy, combat, inventions and land AI.
- Exact natural vegetation, city/farm composition and environmental automata.
- Original state machines for the remaining disasters, routes, walls and heroes.
- Deity creation, experience allocation/awards, all scenario options, opponent
  personalities, original campaign scoring and world progression.
- Original menus, statistics, transitions and final sequence.
- Native animation-triggered sound events and hardware timing/mixing comparison.
- Original GAM interoperability and two-player transport.

The separate Challenge executable is an additional scenario extension, outside
the main two-disk game.

## Format references

- [ADFlib ADF documentation](https://adflib.github.io/FAQ/adf_info.html)
- [AmigaDOS Technical Reference Manual](https://www.pagetable.com/docs/amigados_tripos/amigados_manual.pdf)

Game-specific layouts are established from the supplied local executable and
resources. Compressed-resource XOR checks are validated during decoding.
