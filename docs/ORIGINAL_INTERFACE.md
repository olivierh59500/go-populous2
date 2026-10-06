# Original interface restoration

The independent game uses Go screen state and imported artwork. It does not
execute the original program. Its simulation and interface behavior are separate
from the source-comparison implementation in `internal/populous2`.

The startup menu now retains the original 320 × 200 background, palette,
eight-pixel glyphs, five labels and click regions. The local importer decodes the
layout into `startup-menu.json`, containing ordinary glyph text and named action
rectangles. The running game maps those actions to deity creation, conquest,
custom setup, loading and quitting. Multiplayer and help remain accessible with
M and H. The supplied edition's French labels are retained.

The startup composition is compared pixel-for-pixel with the original renderer.
All 64,000 screen positions are compared with its hit testing. Generated artwork
and the original program remain outside version control.

## Remaining presentation work

| Screen or element | Current independent presentation | Original data and comparison implementation |
| --- | --- | --- |
| Deity creation | Text experience values and a compact portrait | Original bright/grey experience strips, face backing and part coordinates in `presentation_native_widgets.go`; requester in `presentation_native.go` |
| World selection | World code, opponent text and a start button | World requester, landscape/opponent labels and available power icons in `ingame_requester.go` and `world_screens.go` |
| Gameplay panel | Text buttons and numeric population/mana | Original icon panel, affordability bars and population indicators in `startup_panel_frame.go` and `hud_native.go` |
| Pointer and selected group | System pointer; no original selected-group panel | Game cursor, map pointer, selected actor and terrain indicators in `render_frame.go` and `render_frame_main.go` |
| In-game menu | Individual options/help/save controls | Original game-mode/profile requester in `ingame_requester.go` |
| Options | Go tabs, checkbox text and setup rows | Original side labels, checkbox glyphs and reaction selector in `presentation_native_widgets.go` and `options_frame.go` |
| Map editor | Detached terrain/people/scenery tools | Original paint requester and numeric fields in `presentation_input.go` |
| Spell help | Actual detached simulation preview with Go labels | Original help descriptions, preview placement and window composition in `spell_help_base.go` and `presentation_native.go` |
| Save/load and overwrite | Filesystem browser and confirmation | Original file requester, field padding and overwrite window in `presentation_native.go` and file-frame comparison modules |
| Results | Calculated score and statistics in a Go window | Original result requester and numeric field placement in `presentation_native.go` |
| Zeus/opponent/about panels | Their information is partly exposed in other screens | Original dedicated requester definitions in `presentation_native.go` and `ingame_requester.go` |

The original font, startup background, QAZ gameplay background, sprite banks,
portrait parts and ending animation are already available as portable assets.
Additional UI exports should describe glyph cells, named placeholders, action
regions and graphic layers. The runtime format must not require executable
addresses, source dispatch tables or mutable program memory.

Each restored screen needs a source pixel comparison for its normal states,
hit-region checks, and a test that its actions operate on independent Go state.
Dynamic values, checkbox/selection marks, disabled actions and input fields must
retain their original layout and palette. Gameplay controls must use the same
validated simulation commands as keyboard and network input.
