# Preparing original game data

The repository distributes Go sources, extraction tools, documentation and
validation fixtures. It does not distribute the original Amiga executable,
graphics, samples or campaign files. Supply your own Populous II disk images
before building the game.

The default game uses an independent Go engine and a portable exported asset
package. The original executable is read only by local asset exporters and
reference tools; it is not loaded by the playing application. The architecture
and compatibility boundaries are described in [GO_ENGINE.md](GO_ENGINE.md).

## Import the two original disks

### Identifying the disk images

The [Planet Emu Amiga ADF catalogue, letter P](https://www.planetemu.net/roms/commodore-amiga-games-adf?page=P)
is a useful reference for identifying the game. Search for **Populous II -
Trials of the Olympian Gods** (Bullfrog, 1991), with **Disk 1 of 2** for the
boot disk and **Disk 2 of 2** for the data disk. The catalogue also lists
modified editions and the separate Challenge extension; select the main
two-disk game.

Extract the `.adf` files from their archives before passing them to the
importer. Catalogue names alone do not guarantee compatibility: the importer
checks the required file contents, and the disk SHA-256 fingerprints below
identify the images used for validation. A different whole-disk fingerprint
can still contain identical required files, but an unsupported executable or
modified resource is rejected.

The importer runs using only Go and the included AmigaDOS reader. It extracts
only the 27 required game files, checks their SHA-256 fingerprints and leaves
unrelated Amiga operating-system files on the disks.

```sh
sh tools/exclude-local-assets.sh
go run ./cmd/import-assets \
  -adf "/path/to/Populous II disk A.adf" \
  -adf "/path/to/Populous II disk B.adf"
go run ./cmd/assetcheck
go run ./cmd/export-visual-assets -output assets/runtime/data
go run ./cmd/export-audio-assets -output assets/runtime/data/audio
go run ./cmd/populous2
```

The default output is `assets/amiga/`. This directory's original game files
are local import/reference inputs. Exported art, music and campaign data in
`assets/runtime/data/` are embedded in the game when compiled. Existing matching
imported files are reused and differing files are never overwritten.

The recognized boot-disk executable is missing a `HUNK_END` structural word
between its CODE relocations and the next BSS hunk. The importer inserts this
four-byte separator locally. It does not add instructions, artwork or a
translation patch. Unrecognized executables are rejected.

The supplied two-disk revision has been checked through resource decoding,
terrain generation, desktop startup, menu/profile/conquest/help traversal,
ordinary gameplay and native PCM output. Its English interface differs from
the French reference used by the complete CPU comparison fixtures; those
strict fixture hashes are revision-specific.

## French executable alternative

To reproduce the French reference build, supply a compatible original
`populous.ii` separately with the data disk:

```sh
go run ./cmd/import-assets \
  -adf "/path/to/Populous II disk B.adf" \
  -executable "/path/to/French/populous.ii"
go test ./internal/populous2
```

No executable or patch is downloaded by the importer. The separate Challenge
extension is not a substitute for the main game.

## External installation

An installation can be generated outside `assets/amiga` and loaded at runtime:

```sh
go run ./cmd/import-assets \
  -adf "/path/to/disk A.adf" -adf "/path/to/disk B.adf" \
  -output "/path/to/private/populous2-data"
POPULOUS2_DATA_DIR="/path/to/private/populous2-data" go run ./cmd/export-visual-assets -output /path/to/portable-assets
POPULOUS2_DATA_DIR="/path/to/private/populous2-data" go run ./cmd/export-audio-assets -output /path/to/portable-assets/audio
go run ./cmd/populous2 -data /path/to/portable-assets
go run ./cmd/assetcheck -data "/path/to/private/populous2-data"
```

`POPULOUS2_DATA_DIR` selects original import/reference resources. The independent
game selects portable resources with `-data`, or embeds them locally. A clean
clone can compile the game and import tools without original data. Playing
requires the prepared portable package.
Original-reference tests also require the corresponding reference revision.

## Identified revisions

| Input | SHA-256 |
|---|---|
| Disk A | `96e40c15b3eac984e6e54c91b11a2d5cd7fa4ba69d5ecc6ef7e4b17bc1d4a17a` |
| Disk B | `88f30eb7dc754bd59f42fc5e4b6371ecbcc0b37a0cbc8f1548f9fa0d650a8893` |
| Original disk executable | `f5fa1a02d6fe7bd4f298498d0bcbcc3ebc03b83b102dbe170781114bfcdfa271` |
| Locally repaired disk executable | `4fa6fe0ff7960100a3a007eef2d8ad6ab3f86e67adf480cf6ddfc0fdc5b73a45` |
| French executable | `c148b9bdcc543d89bf788d86886eb5ac9320ad66b09ef92792ec24314bfee803` |

The data-file fingerprints are in [assets-manifest.json](assets-manifest.json).
They are metadata, not copies of the original data. Original-reference fixture
captures are sanitized with `tools/sanitize_fixture_payloads.py`: full CODE
workspace snapshots are replaced by SHA-256 fingerprints before publication.

## English interface export

The playing application uses an English interface. The ordinary two-disk import
above provides the supported English executable automatically. Exporting artwork
checks original startup and result labels before producing interface files.
An incompatible language revision fails explicitly instead of producing a mix
of French and English windows.

If your imported graphics and campaign data came from the French revision,
provide a compatible original English executable for the offline interface
export:

```sh
go run ./cmd/export-visual-assets \
  -input "/path/to/private/populous2-data" \
  -interface-executable "/path/to/English/populous.ii" \
  -output assets/runtime/data
```

Use the executable extracted and structurally validated by the disk importer.
`-interface-executable` takes precedence over `POPULOUS2_INTERFACE_EXECUTABLE`;
without either override, the exporter checks the executable in its input data.
The English program is read only during local export. The playable build embeds
ordinary PNG artwork and named JSON layouts, and does not require an executable.
