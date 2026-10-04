# Provenance

Les références fournies restent dans `previous/` et n'ont pas été modifiées.

| Source | Usage |
|---|---|
| `previous/go-populous/internal/populous/*.go` | Copie des 20 fichiers de production dans `internal/legacy` |
| `previous/go-populous/internal/fixedstep/fixedstep.go` | Cadence conservée dans `internal/fixedstep` |
| `previous/go-populous/LICENSE` | Licence GPL-3.0 du code réutilisé |
| Disquette B, volume `POPULOUS II` | 26 fichiers graphiques, sonores et de campagne dans `assets/amiga` |
| `previous/Populous2_PatchFR/populous.ii` | Exécutable français valide, utilisé pour ses tables et l'analyse 68000 |
| `previous/Populous2_PatchFR/CHALLENGE` et `.TAM` | Inventoriés ; extension non intégrée au jeu |

Le moteur copié est adapté dans `internal/legacy`, notamment pour les références
16 bits, le générateur aléatoire, les populations, les habitations et le terrain.
Les sources de référence locales restent inchangées. Aucune image ou musique de
Populous 1 n'est utilisée pour le rendu de Populous II.

## Empreintes des sources

| Source | SHA-256 |
|---|---|
| Disquette A | `96e40c15b3eac984e6e54c91b11a2d5cd7fa4ba69d5ecc6ef7e4b17bc1d4a17a` |
| Disquette B | `88f30eb7dc754bd59f42fc5e4b6371ecbcc0b37a0cbc8f1548f9fa0d650a8893` |
| Exécutable FR | `c148b9bdcc543d89bf788d86886eb5ac9320ad66b09ef92792ec24314bfee803` |
| ADF isolé de Populous 1 | `55f74cacf20baca2ff23d7543617d90739c3fb277d670ef9725424b7843a07da` |

[assets-manifest.json](assets-manifest.json) donne la taille et l'empreinte des
27 fichiers embarqués. [ressources.json](ressources.json) conserve les 26
descripteurs lus dans le chargeur du programme.

Le chargeur Hunk strict signale une séparation de segments invalide dans
`POPULOUS.II` de la disquette A. L'exécutable FR fourni se décode normalement :
six segments, dont un CODE et quatre BSS. Le portage utilise ce fichier sans
modifier les deux exécutables d'origine.

Les graphismes et données d'origine appartiennent à leurs ayants droit ; la
licence du code Go ne leur attribue pas une nouvelle licence.
