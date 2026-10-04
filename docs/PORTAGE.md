# Portage et fidélité

Cette version est une base jouable de travail. L'extraction et les décodeurs sont
issus des données de Populous II et de ses instructions 68000. La simulation
n'a pas encore été validée par comparaison avec des parties exécutées sur Amiga.

## Éléments établis sur les sources

Toutes les adresses ci-dessous sont des **offsets relatifs au segment 0** de
l'exécutable FR `c148b9bd...bfee803`. Le segment CODE commence à l'offset fichier
`0x34` et contient également des textes, tables et données. Un offset n'est donc
pas automatiquement une adresse d'instruction.

| Offset | Observation et traduction |
|---|---|
| `0x103c6` | Codes de mondes : LCG 15 bits, puis syllabes de deux lettres lues par groupes de six bits. Le monde 0 est `DOEGAC`. |
| `0x103f8` | Table des 64 syllabes propre à Populous II. |
| `0x105ca`–`0x1069c` | Décompresseur arrière : lectures de bits avec sentinelle, littéraux, références et contrôle XOR. Traduit dans `packed.go`. |
| `0x1069c` | Réorganisation en place des sprites : masque inversé puis séparation des plans. Le portage décode les données sources directement. |
| `0x10df2` | Copie des modèles par camp : deux blocs de 58 octets et paramètres du monde. |
| `0x11044` | Campagne : 200 enregistrements de 250 octets, cinq mondes par enregistrement, décalage de graine `0x2d7`. |
| `0x117de` | Économie des habitations : six tableaux de 19 valeurs, contre onze dans le moteur de Populous 1. |
| `0xbef4` / `0xbfac` | Tuiles 32×24 composées de trois rangées de deux fragments 16×8. Descripteur de six offsets 16 bits. |
| `0x19cd0` | Lecture d'un descripteur de ressource, ouverture, chargement, décompression éventuelle et réorganisation des sprites. |
| `0x19e4a` | Table des 26 ressources, enregistrements de 48 octets ; le drapeau de compression est à +16, le nom à +18. |
| `0x1a3f0` | Réorganisation des fragments de tuiles pour le blitter. Confirme le masque et quatre plans intercalés dans la source. |
| `0x1a51e` | Application des différences de sprites : source, destination et longueur moins un, enregistrements de dix octets. |
| `0x21238` | Prix de base des pouvoirs, six catégories de six emplacements, avec des emplacements réservés. |
| `0x21626` | Table des 830 descripteurs de sprites, douze octets chacun ; relocations nécessaires pour identifier le tampon. |

`LANDn.DAT` contient exactement 556 octets : six tableaux de 19 mots, trois
paramètres supplémentaires, 256 couleurs de minimap, deux palettes de 16 couleurs
Amiga 12 bits et un dernier mot. Les paramètres dont le sens reste incertain
conservent leur valeur brute.

Les sprites 32 pixels contiennent **deux groupes de 16 pixels par rangée**, chacun
avec son masque et ses quatre plans. Les lire comme cinq plans de 32 bits
déforme les bâtiments. Les `.PIF` rétablissent les variations propres aux quatre
décors sur une copie indépendante du tampon de base.

## Réutilisation du moteur de Populous 1

La propagation des altitudes, la génération de terrain, les mouvements,
l'établissement des habitations, les combats et l'IA de terrain viennent de
`internal/legacy`. La cadence et les snapshots ont été réutilisés également.
Le programme de travail utilise ses propres contrôles et les seules images de
Populous II pour afficher cet état.

Les données de campagne sont décodées complètement, mais tous les paramètres
bruts n'ont pas encore été identifiés ni appliqués. Les tableaux d'économie à
19 niveaux sont actuellement projetés sur les onze niveaux du moteur hérité.
Les scènes produites ne sont pas des reproductions certifiées des paysages
générés par l'Amiga.

## Travaux encore nécessaires

- Traduire la simulation de Populous II : pool de 400 adorateurs et références
  16 bits, données de groupe de 52 octets, dix-neuf niveaux d'habitation,
  croissance, migrations, météo et combat. Le moteur hérité conserve 208 groupes
  et ses règles de Populous 1.
- Identifier et appliquer chaque paramètre de campagne par camp, y compris les
  restrictions de construction et d'affichage. La base actuelle applique les
  disponibilités de pouvoirs, la graine et le décor.
- Remplacer les effets expérimentaux par les routines correspondantes. Séisme,
  marais, volcan et bataille finale utilisent les effets du premier jeu avec
  les nouveaux coûts de base. Peste, feu, champignon, eau et conversions ont des
  règles provisoires. Route et mur sont encore des représentations de terrain,
  avec une influence incomplète sur les déplacements. Vent, tsunami et les six
  héros n'ont pas encore tous leurs comportements spécifiques.
- Porter l'expérience du dieu, ses réductions de coût (`0x14768`), sa création,
  les adversaires, le calcul de progression après victoire et l'IA de pouvoirs.
  La base avance actuellement d'un monde après chaque victoire.
- Porter le séquenceur sonore Amiga et la banque `fx.dat`. Le fichier est extrait
  et conservé, mais **le jeu ne joue pas encore de son**. La musique du premier
  jeu n'est pas substituée à celle de Populous II.
- Porter les menus et animations d'origine, les sauvegardes `.GAM`, puis
  l'extension Challenge séparément. Les `.TAM` fournis sont ses consignes.
- Comparer les traces de commandes, RNG, altitudes, populations, mana et effets
  sur des états identiques dans le portage et l'original exécuté en émulation.
  Les tests de déterminisme actuels ne vérifient pas cette parité.

Ces différences empêchent de qualifier la conversion de terminée. L'isolation
des formats et du moteur hérité permet de remplacer progressivement les règles
sans perdre l'extraction, le rendu et les contrôles fonctionnels.

## Références de formats

- [ADFlib : format ADF et fichiers AmigaDOS](https://adflib.github.io/FAQ/adf_info.html),
  pour les structures OFS/FFS, chaînes et contrôles d'intégrité.
- [AmigaDOS Technical Reference Manual](https://www.pagetable.com/docs/amigados_tripos/amigados_manual.pdf),
  pour les segments Hunk et relocations.

Les formats propres au jeu ont été analysés à partir des fichiers locaux fournis,
et les sommes XOR de toutes les ressources compressées ont été vérifiées.
