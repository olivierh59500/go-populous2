# Go Populous II

Portage en cours de **Populous II: Trials of the Olympian Gods** en Go et
Ebitengine, à partir des disquettes Amiga fournies et de la version française
pour disque dur.

Une première base jouable est disponible : terrain isométrique, adorateurs,
habitations, combats, adversaire, pouvoirs, campagne, démonstration et sauvegarde.
Les graphismes et données viennent de Populous II. La simulation utilise encore
le moteur de `previous/go-populous`, complété par des effets expérimentaux.
**Ce n'est pas encore une conversion fidèle de toutes les règles de Populous II.**

![Base jouable](screenshots/game.png)

## Lancer

Go 1.25 ou supérieur, avec les prérequis habituels d'Ebitengine pour la plateforme.
Les données sont embarquées : `previous/` n'est pas nécessaire pour lancer ou
compiler le jeu.

```sh
go run ./cmd/populous2
go run ./cmd/populous2 -play
go run ./cmd/populous2 -custom
go run ./cmd/populous2 -demo -world 0
go run ./cmd/populous2 -play -code DOEGAC
```

La fenêtre s'ouvre en 960×720. Le rendu logique mesure 640×480, avec les éléments
Amiga agrandis deux fois. La simulation avance à huit tours par seconde ; les
commandes sont lues à 60 Hz.

Pour utiliser une installation extraite différente contenant `populous.ii`
et tous les fichiers du chargeur :

```sh
POPULOUS2_DATA_DIR=/chemin/installation go run ./cmd/populous2
```

Le décodeur de tables embarquées est actuellement adapté à l'exécutable français
fourni. Une autre révision peut nécessiter d'autres offsets. Les incompatibilités
de format sont signalées au chargement.

## Commandes

| Action | Commande |
|---|---|
| Lever / baisser | Clic gauche / droit sur le terrain |
| Faire sortir un adorateur | Clic droit sur une habitation |
| Déplacer la vue | WASD, flèches, clic sur la carte |
| Choisir un pouvoir | Élément à gauche, puis pouvoir, puis terrain |
| Route ou mur | Deux clics pour les extrémités |
| Direction d'un effet | Q / E |
| Aimant / retrouver le chef | M / C |
| Coloniser, rassembler, combattre, suivre | 1, 2, 3, 4 |
| Pause / aide | Espace / H |
| Sauvegarder / charger | F5 / F9 |
| Menu / plein écran | Échap / F |
| Après une victoire ou défaite | Entrée |

La conquête reprend les disponibilités de pouvoirs des 1 000 mondes d'origine.
La partie libre les active tous. Les prix affichés sont les coûts de base lus
dans l'exécutable, sans les réductions d'expérience du dieu.

La sauvegarde du portage se trouve dans `go-populous2.sav`, indépendamment des
sauvegardes de Populous 1. `POPULOUS2_SAVE_PATH` permet de choisir son emplacement.
Les sauvegardes Amiga `.GAM` ne sont pas encore prises en charge.

## Extraction et analyse

Les deux bonnes disquettes se trouvent dans
`previous/Populous 2 (Bullfrog + Electronic Arts)/`. Le fichier isolé
`previous/populous 2.adf` contient **Populous 1** ; ses 15 fichiers de données
correspondent exactement à ceux de `previous/go-populous`.

```sh
go run ./cmd/amiga-inspect -input 'previous/populous 2.adf'
go run ./cmd/amiga-inspect -input previous/Populous2_PatchFR -json
go run ./cmd/amiga-inspect \
  -input 'previous/Populous 2 (Bullfrog + Electronic Arts)/Populous 2 (Bullfrog + Electronic Arts) B.adf' \
  -extract /tmp/populous2-data
go run ./cmd/amiga-inspect \
  -input previous/Populous2_PatchFR/populous.ii -dump-hunks /tmp/populous2-hunks
go run ./cmd/assetcheck
go run ./cmd/assetcheck -images /tmp/populous2-images
```

Les répertoires de sortie doivent être nouveaux. Le lecteur ADF valide les
sommes de contrôle, les chaînes de répertoires, les extensions et les données
OFS ; il lit également FFS. L'analyse Hunk conserve les relocations et les
offsets de segments sans exécuter le code Amiga.

Pour un désassemblage 68000 dans un environnement Python local :

```sh
python3 -m venv /tmp/populous2-tools
/tmp/populous2-tools/bin/pip install -r tools/requirements.txt
/tmp/populous2-tools/bin/python tools/disassemble.py \
  /tmp/populous2-hunks/populous.ii/hunk-00-code.bin \
  --start 0x105ca --end 0x1069c --output /tmp/decompression.asm
```

Un désassemblage linéaire peut interpréter les tables et textes contenus dans
le segment CODE comme des instructions. Les adresses et limites confirmées
sont décrites dans [les notes de portage](docs/PORTAGE.md).

## Vérification

```sh
go test ./...
go vet ./...
go test -race ./internal/amiga ./internal/populous2
go run ./cmd/simcheck -world 0 -ticks 4800
go run ./cmd/populous2 -play -frames 660 \
  -capture-update 600 -screenshot /tmp/populous2-game.png
go build ./cmd/populous2
```

`simcheck` n'importe pas Ebitengine et peut tourner sans affichage. Les tests
vérifient les formats, les quatre variantes graphiques, le catalogue de campagne,
les rejets de commandes et la continuation déterministe des sauvegardes. Ils
ne constituent pas une preuve de parité avec la simulation Amiga.

## Organisation

| Dossier | Rôle |
|---|---|
| `internal/amiga` | ADF, Hunk, inventaire et identification |
| `internal/populous2` | Formats spécifiques, ressources, campagne, pouvoirs et simulation de travail |
| `internal/legacy` | Moteur réutilisé de la conversion Populous 1 |
| `internal/fixedstep` | Cadence de simulation réutilisée |
| `internal/game` | Interface Ebitengine, affichage, commandes et sauvegarde |
| `assets/amiga` | Ressources originales extraites et exécutable français |
| `tools` | Aide au désassemblage |
| `docs` | Offsets, provenance, empreintes et travaux restant pour la fidélité |

Le code réutilisé conserve sa licence GPL-3.0. Les fichiers du jeu original ont
une provenance et des droits distincts, décrits dans [PROVENANCE.md](docs/PROVENANCE.md).
