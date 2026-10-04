# Original requester windows

The shared compiler at CODE `$4eb6` converts encoded interface definitions into
printable bitmap-font bytes. The Go translation preserves the original origin,
line counter, height, substitutions, fixed-width field tails and padding glyphs.
Its width is the native final-line counter, rather than a recomputed maximum.

High-bit definition bytes lose `$25`. A `{` marker copies an argument while
consuming positions in the definition; a nonempty `v...w` field displays the
argument's tail and fills unused places with glyph `k`. Empty field arguments
retain the original definition. Parameters remain byte strings, because the
lowercase range contains native interface symbols rather than a normal font.

Click processing at `$4dac` classifies those prepared bytes. Positive markers
number actions by two, negative markers end an active region, and radio glyphs
`c`/`d` toggle. Opening the game-options window at `$471c` enables `y`/`z`
markers by changing the classification word to `$0101`; `$481a` clears it.
The conversion retains this separate classification setting.

## Startup and palettes

The startup definition at `$8854` has seven rows. The main two-disk game limits
its height to 40 pixels at `$3b98` and writes a terminator after 99 prepared
bytes at `$3bac`. Five choices are visible: create deity, conquest, custom game,
load and quit. The two additional definition rows are not ordinary startup
choices. `NativeRequesterRules.Startup` applies the verified caller override.

Startup uses the separate palette at `$3c528`. Other modal windows use the
selected landscape's first palette from LANDn.DAT. `NativeRequester.Image`
renders the original color indices using a caller-supplied palette; composing
the surrounding background remains the interface's responsibility.

## Validation and inspection

Sixteen independent executions compare prepared bytes, native width/height,
click commands and radio transitions. Cases cover substitutions, truncated
fields, borders, the options marker setting, save/load, and startup's actual
visible-row override. The instruction harness remains local; production
fixtures contain derived interface observations, not executable instructions.

```sh
go run ./cmd/assetcheck -images /tmp/populous2-requesters
```

The destination must be new. In addition to the graphics atlases, this exports
`menu-font.png`, `requester-startup.png` and `requester-options.png`.
The options export inspects its encoded definition, rather than claiming to
represent a particular saved game's flags. Native requester compilation and
click semantics are available; the playable application's temporary menu
layout has not yet been replaced throughout all game paths.

Malformed definitions and out-of-buffer click probes return errors/no action
instead of reading adjacent original scratch memory. That boundary handling
is separate from the verified valid-template comparisons.
