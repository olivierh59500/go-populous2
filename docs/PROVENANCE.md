# Provenance

The supplied references remain unchanged in the locally excluded `previous/`
directory.

| Source | Use |
|---|---|
| `previous/go-populous/internal/populous/*.go` | Twenty production files copied into `internal/legacy` |
| `previous/go-populous/internal/fixedstep/fixedstep.go` | Scheduler retained in `internal/fixedstep` |
| `previous/go-populous/LICENSE` | GPL-3.0 license for the reused code |
| Disk B, volume `POPULOUS II` | Twenty-six graphics, sound and campaign files in `assets/amiga` |
| `previous/Populous2_PatchFR/populous.ii` | Valid French executable used for its tables and native routine comparisons |
| `previous/Populous2_PatchFR/CHALLENGE` and `.TAM` | Cataloged; this extension is not integrated into the game |

The copied engine is adapted in `internal/legacy`, including its 16-bit
references, random generator, populations, settlements and terrain. The local
reference sources remain unchanged. No Populous I image or music is used to
render or play Populous II.

## Source fingerprints

| Source | SHA-256 |
|---|---|
| Disk A | `96e40c15b3eac984e6e54c91b11a2d5cd7fa4ba69d5ecc6ef7e4b17bc1d4a17a` |
| Disk B | `88f30eb7dc754bd59f42fc5e4b6371ecbcc0b37a0cbc8f1548f9fa0d650a8893` |
| French executable | `c148b9bdcc543d89bf788d86886eb5ac9320ad66b09ef92792ec24314bfee803` |
| Separate Populous I ADF | `55f74cacf20baca2ff23d7543617d90739c3fb277d670ef9725424b7843a07da` |

[assets-manifest.json](assets-manifest.json) records the size and fingerprint
of all 27 embedded files. [ressources.json](ressources.json) preserves the 26
descriptors read from the original resource loader.

The strict Hunk parser reports a missing HUNK_END boundary in the identified
disk A `POPULOUS.II`. The local importer restores that structural separator
for this fingerprint only; the original ADF is unchanged. The resulting English
revision has bounded menu/game/audio validation. The French executable parses
normally and remains the reference revision for full CPU comparison fixtures.
See [ASSET_SETUP.md](ASSET_SETUP.md) for importing either revision.

## Native comparison fixtures

The isolated analysis harness relocates and executes bounded original routines.
It stays local and is not part of the game. Tests in `internal/populous2/testdata`
retain derived numeric states and hashes. CODE workspace snapshots are stored
only as SHA-256 digests, with no original executable or audiovisual payloads.
These fixtures establish the specific routines
and conditions described in [VALIDATION.md](VALIDATION.md), rather than complete
gameplay parity.

Original graphics and data remain the property of their respective rights
holders; the Go code license does not assign them a new license. Original game
files are generated locally and excluded from the repository and its history.
