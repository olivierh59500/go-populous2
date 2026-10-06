# Original interface restoration

The independent game uses Go screen state and imported artwork. It does not
execute the original program. Its simulation and interface behavior are separate
from the source-comparison implementation in `internal/populous2`.

The startup menu now retains the original 320 × 200 background, palette,
eight-pixel glyphs, five labels and click regions. The local importer decodes the
layout into `startup-menu.json`, containing ordinary glyph text and named action
rectangles. The running game maps those actions to deity creation, conquest,
custom setup, loading and quitting. Multiplayer and help remain accessible with
M and H. Its action labels are displayed in English, including when loading
older metadata extracted from a French edition.

The startup composition is compared pixel-for-pixel with the original renderer.
All 64,000 screen positions are compared with its hit testing. Generated artwork
and the original program remain outside version control.

## Restored screens

| Screen or element | Current independent presentation | Original data and comparison implementation |
| --- | --- | --- |
| Deity creation | Original requester, bright/grey experience strips, face backdrop and arrow controls | Pixel comparison for multiple experience states and original action binding |
| World selection | Original English requester, landscape/opponent labels, option marks and power icons | Pixel and hit-region comparisons against English original |
| Gameplay panel | Original icons, affordability bars, population stencils and selection highlights | Eight framebuffer states and the original slanted hit grids |
| Pointer and selected group | Original animated pointer, terrain marker and selected actor/weapon/population panel are restored | Game cursor, map pointer and selected actor in `render_frame.go` and `render_frame_main.go` |
| In-game menu | Original English requester connected to game actions, side, assist and mode marks | Source pixel comparison for ordinary and computer-control states |
| Options | Original single-page rules, reaction indicator and special-code field | Pixel and moving-hit-region comparisons; conquest restrictions retained |
| Map editor | Original paint requester, numeric fields, four brush previews and fifty event slots | Original composition, all hit regions, brush anchors and last-record editing |
| Spell help | Original English descriptions and timed miniature animations | All 29 powers in four landscapes at five animation ages |
| Save/load and overwrite | Original file requester and confirmation windows over Go filesystem operations | Pixel/hit comparisons and unchanged atomic save/load protection |
| Results | Original numeric requester over the game frame | Valid-range numeric and full hit-region comparisons |
| Opponent and about | Original dedicated requester definitions and portrait parts | Opponent layout and normal-state source comparisons |

Zeus's automatic statue question is the original manual-based copy protection.
It remains disabled so that it cannot prevent a legitimate imported game from
being played. The opponent biography and final campaign presentation remain
available. The network window retains the original geometry and uses named
TCP host/join controls in place of Amiga serial speed and modem settings.

The original font, startup background, QAZ gameplay background, sprite banks,
portrait parts and ending animation are already available as portable assets.
UI exports describe glyph cells, named placeholders, action regions and graphic
layers. The runtime format requires no executable addresses, source dispatch
tables or mutable program memory. The ordinary launcher requires the complete
interface export and reports a missing file instead of silently falling back
to the earlier substitute controls.

## Gameplay inspection

The Roman amphitheatre shows the selected group or town, its weapon artwork
and original population indicators. A new game retains the original empty
selection. Enable Inspect with I or the original inspect icon, then click a group or town without
casting a power. Leader and hero indicators retain the original drawing order.
Selected towns transfer to their emigrants and merged groups to the survivor.
Clicking the amphitheatre recenters the map on that selection.
Right-clicking a hero icon scans existing owned heroes, and right-clicking
Rally inspects the leader or recenters the magnet when no leader exists.
F11 over an enabled power icon opens the original animated spell help.
Other secondary spell gestures retain the original effect-scanning rules and
recenter on eligible owned controllers without casting another effect.
Selection and inspect mode are stored in JSON sessions and stay local during
network games.

Main images preserve the original order: advance the display clock, draw the
current actor poses, then perform the following physics pass. Intermediate
50 Hz host updates keep that image. Hit testing and cursor rights use its
visible actors and terrain; accepted commands modify the live world. The
persistent image-state buffer performs no steady-state allocation.

The paint window advances a detached world at the same main-frame cadence,
including its scripted effects. Numeric entry suspends that draft. Its sound
admission and frame buffer remain independent from the live game; canceling
paint restores the untouched live world. Applying it commits the edited state.

Result scoring retains the original observation order. Visible walkers and
towns contribute the scenario rights before the physics pass, and that same
word reaches the result formula. This presentation observation does not alter
AI, random draws, terrain or synchronized network state. An empty view can
still expose the original formula's zero-divisor case; the independent game
reports it instead of inventing a score or reproducing a processor exception.

The imported attached mouse sprite uses the original palette and animated
power frames. The software terrain marker follows the original slope outlines.
The system cursor is hidden while the original software mouse sprite is
available. Requesters retain the normal original pointer; direct spell help
can retain the active power pointer. Presentation assets contain only images,
named roles and coordinates. English labels and ending text are independent
of the original resource edition.

Restored screens have source pixel comparisons for bounded normal states,
hit-region checks, and tests that their actions operate on independent Go state.
Dynamic values, checkbox/selection marks, disabled actions and input fields must
retain their original layout and palette. Gameplay controls must use the same
validated simulation commands as keyboard and network input.

## Physical fire icons and command prices

The original fire panel places Achilles in physical slot 26 and Volcano in
slot 27. Its icon affordability table uses the opposite command-price slots:
the panel gate and bars use the physical slot, while the actual spell uses
its command price. `PanelPowerCost` and `PowerCost` preserve this distinction.
AI permission checks and weighted spell-use statistics use command slots.

World snapshots now use version 3 and interface sessions version 2. Loading
older Go saves migrates stored semantic Achilles/Volcano IDs while preserving
the original physical permission arrays. Existing accumulated statistics stay
as recorded; historical individual spell casts cannot be reconstructed from
their aggregate. Current saves retain the chosen panel and do not migrate twice.

## Result numeric fields

The result requester retains the original English labels, numeric positions and
OK action region. Pixel comparisons cover ordinary and high values up to seven
decimal digits in peak fields, both winners, and the maximum 16-bit score.
The original eight-byte text buffers lose their terminator at eight digits;
later fields can then merge and overwrite the window border. The independent
requester keeps fields bounded and does not reproduce that buffer corruption.
