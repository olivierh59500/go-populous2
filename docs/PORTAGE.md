# Conversion and fidelity

This is a conversion in progress. The native integration target executes Go
translations of the original register-bearing startup, rendering, input,
simulation, menus, audio and result/progression controllers. The original-game
runtime is the default command; the inherited diagnostic engine remains
separately available as cmd/populous2-legacy.
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
| `$13352` | Settlement support, 49-cell farms and neighbor overlays |
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

Independent CPU corpora now compare source followers, AI, effects, terrain,
towns, scripts, menus, rendering, input, audio and result/award/ending paths.
Configured mixed simulation and separate complete paired-frame tests cover
documented compositions. These proofs establish their explicit inputs and
boundaries; deterministic saves or repeatable demonstration runs alone do
not establish full original-game fidelity.

## Remaining conversion work

- Complete stock and campaign end-to-end comparisons across every landscape,
  including rare linked occupancy, combat, inventions and environmental states.
- Broader mixed disaster, wall and hero interactions beyond the documented
  configured simulation corpora.
- Complete winning result/award/deity/reset continuation and long campaign
  scoring, pacing, scenario and opponent behavior.
- Interactive native terrain/picking, menus, statistics and final-sequence
  transitions in the default native runtime.
- Animation-triggered audio timing and remaining hardware/analog fidelity.
- Interactive original GAM save/load and longer paired serial/TCP gameplay,
  including connection failure and save interoperability.

The separate Challenge executable is an additional scenario extension, outside
the main two-disk game.

## Terrain regression coverage

Raising or lowering a vertex affects every tile sharing that corner. The
conversion derives each tile's shape from all four vertex heights and keeps
raised land separate from animated sea-level graphics. Regression checks
exercise repeated edits, shared edges, representable slopes and picking on
the projected surface. Prohibited edits restore the full propagated terrain
and mana state. These checks address visible neighboring-tile discontinuities;
native disaster/editor interactions remain in the feature inventory.

## Format references

- [ADFlib ADF documentation](https://adflib.github.io/FAQ/adf_info.html)
- [AmigaDOS Technical Reference Manual](https://www.pagetable.com/docs/amigados_tripos/amigados_manual.pdf)

Game-specific layouts are established from the supplied local executable and
resources. Compressed-resource XOR checks are validated during decoding.
