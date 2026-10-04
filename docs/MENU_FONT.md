# Original menu bitmap font

The native text routine at CODE `$509a` writes eight rows of four plane bytes
per glyph. Its descriptor points to `$33c68`. The conversion decodes the 96
verified byte codes 32–127 into eight-by-eight color-index glyphs.

`DecodeNativeMenuFont` retains all four color planes. `DrawIndices` follows the
original 320×200 text buffer: X is an eight-pixel column, Y is a pixel row,
newlines restore the starting column and advance eight rows, and drawing stops
when the next ordinary character reaches column 40. Color index zero replaces
the destination too. Lowercase byte codes include interface symbols, such as
the game-option markers, and must retain their original values.

Eight independent executions of the relocated original routine establish the
complete 64,000-pixel color-index output. Cases cover all 96 glyphs, titles,
numbers, option symbols, newlines, clipping and the bottom row. Production
fixtures retain only input strings and output hashes. The isolated instruction
harness remains excluded locally; runtime rendering is Go code.

To inspect the glyph bank with the other decoded images:

```sh
go run ./cmd/assetcheck -images /tmp/populous2-native-images
```

The output directory must be new. `menu-font.png` uses the first landscape
palette for inspection; the original menu palette and complete screen
composition remain separate work. Unsupported byte codes and out-of-buffer
positions return errors rather than reading unrelated executable memory. The
current playable interface still uses its temporary text/layout adapter.
